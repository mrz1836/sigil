package balance

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mrz1836/sigil/internal/cache"
)

// ErrNoCachedBalance is returned when no cached balance exists for an address.
var ErrNoCachedBalance = errors.New("no cached balance for address")

// Config holds the configuration for the balance service.
type Config struct {
	ConfigProvider ConfigProvider
	CacheProvider  CacheProvider
	Metadata       AddressMetadataProvider
	ForceRefresh   bool
}

// Service provides balance fetching functionality with caching and refresh policy.
type Service struct {
	fetcher *Fetcher
	policy  *RefreshPolicy
	cache   CacheProvider
	force   bool
}

// NewService creates a new balance service.
func NewService(cfg *Config) *Service {
	fetcher := NewFetcher(cfg.ConfigProvider, cfg.CacheProvider)

	var policy *RefreshPolicy
	if cfg.Metadata != nil && cfg.CacheProvider != nil && !cfg.ForceRefresh {
		policy = NewRefreshPolicy(cfg.Metadata, cfg.CacheProvider)
	}

	return &Service{
		fetcher: fetcher,
		policy:  policy,
		cache:   cfg.CacheProvider,
		force:   cfg.ForceRefresh,
	}
}

// FetchBalance fetches balance for a single address.
//
//nolint:gocognit,gocyclo // Complex business logic for balance fetching with caching and refresh policy
func (s *Service) FetchBalance(ctx context.Context, req *FetchRequest) (*FetchResult, error) {
	result := &FetchResult{
		ChainID: req.ChainID,
		Address: req.Address,
	}

	// Check refresh policy (unless force refresh)
	if s.policy != nil && !req.ForceRefresh && !s.force {
		decision := s.policy.ShouldRefresh(req.ChainID, req.Address)
		if decision == CacheOK {
			// Use cached data
			cachedBalances := getCachedBalancesForAddress(req.ChainID, req.Address, s.cache)
			for _, cached := range cachedBalances {
				result.Balances = append(result.Balances, cacheEntryToBalanceEntry(cached))
			}
			result.Stale = false
			return result, nil
		}
	}

	// Fetch from network
	fetchCtx := ctx
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		fetchCtx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	entries, stale, err := s.fetcher.FetchForChain(fetchCtx, req.ChainID, req.Address)
	if err != nil {
		// On error, try to return cached data
		cachedBalances := getCachedBalancesForAddress(req.ChainID, req.Address, s.cache)
		if len(cachedBalances) > 0 {
			for _, cached := range cachedBalances {
				result.Balances = append(result.Balances, cacheEntryToBalanceEntry(cached))
			}
			result.Stale = true
			result.Error = err
			return result, nil
		}
		return nil, err
	}

	// Convert to balance entries
	for _, entry := range entries {
		result.Balances = append(result.Balances, cacheEntryToBalanceEntry(entry))
	}
	result.Stale = stale

	return result, nil
}

// FetchBalances fetches balances for multiple addresses concurrently.
// Uses bulk API for BSV addresses and individual calls for other chains.
//
//nolint:gocognit,gocyclo // Complex business logic for concurrent batch fetching
func (s *Service) FetchBalances(ctx context.Context, req *FetchBatchRequest) (*FetchBatchResult, error) {
	// Group addresses by chain for bulk operations
	bsvAddresses := make([]string, 0)
	otherAddresses := make([]AddressInput, 0)

	for _, addr := range req.Addresses {
		if addr.ChainID == "bsv" {
			bsvAddresses = append(bsvAddresses, addr.Address)
		} else {
			otherAddresses = append(otherAddresses, addr)
		}
	}

	batchResult := &FetchBatchResult{
		Results: make([]*FetchResult, 0, len(req.Addresses)),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Apply refresh policy to BSV addresses before bulk fetch
	bsvAddressesToFetch := make([]string, 0, len(bsvAddresses))
	for _, addr := range bsvAddresses {
		needsFetch, cachedResult := s.processBSVAddress(addr, req.ForceRefresh)
		if needsFetch {
			bsvAddressesToFetch = append(bsvAddressesToFetch, addr)
		} else if cachedResult != nil {
			batchResult.Results = append(batchResult.Results, cachedResult)
		}
	}

	// Fetch BSV addresses that need refresh (if any)
	if len(bsvAddressesToFetch) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			bulkResults, err := s.fetcher.fetchBSVBulk(ctx, bsvAddressesToFetch)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				batchResult.Errors = append(batchResult.Errors, err)
			}

			for addr, entries := range bulkResults {
				result := &FetchResult{
					ChainID:  "bsv",
					Address:  addr,
					Balances: make([]BalanceEntry, len(entries)),
				}
				for i, entry := range entries {
					result.Balances[i] = cacheEntryToBalanceEntry(entry)
				}
				batchResult.Results = append(batchResult.Results, result)
			}
		}()
	}

	// Fetch other chain addresses individually with concurrency control
	if len(otherAddresses) > 0 {
		maxConcurrent := req.MaxConcurrent
		if maxConcurrent <= 0 {
			maxConcurrent = 8
		}

		sem := make(chan struct{}, maxConcurrent)

		for _, addr := range otherAddresses {
			wg.Add(1)

			go func(input AddressInput) {
				defer wg.Done()

				// Acquire semaphore
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					return
				}
				defer func() {
					<-sem
				}()

				// Fetch balance
				fetchReq := &FetchRequest{
					ChainID:      input.ChainID,
					Address:      input.Address,
					ForceRefresh: req.ForceRefresh,
					Timeout:      req.Timeout,
				}

				result, err := s.FetchBalance(ctx, fetchReq)

				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					batchResult.Errors = append(batchResult.Errors, err)
				}
				if result != nil {
					batchResult.Results = append(batchResult.Results, result)
				}
			}(addr)
		}
	}

	wg.Wait()

	return batchResult, nil
}

// FetchCachedBalances fetches balances from cache only, without network calls.
// Returns cached data with stale markers. Returns error if no cache exists for any address.
func (s *Service) FetchCachedBalances(_ context.Context, req *FetchBatchRequest) (*FetchBatchResult, error) {
	batchResult := &FetchBatchResult{
		Results: make([]*FetchResult, 0, len(req.Addresses)),
	}

	for _, addr := range req.Addresses {
		result := &FetchResult{
			ChainID: addr.ChainID,
			Address: addr.Address,
		}

		// Get cached balances
		cachedBalances := getCachedBalancesForAddress(addr.ChainID, addr.Address, s.cache)

		if len(cachedBalances) == 0 {
			// No cache for this address
			batchResult.Errors = append(batchResult.Errors,
				fmt.Errorf("%w: %s:%s", ErrNoCachedBalance, addr.ChainID, addr.Address))
			continue
		}

		// Convert to balance entries and mark as stale
		for _, cached := range cachedBalances {
			entry := cacheEntryToBalanceEntry(cached)
			entry.Stale = true // Always mark cached-only data as potentially stale
			result.Balances = append(result.Balances, entry)
		}

		result.Stale = true
		batchResult.Results = append(batchResult.Results, result)
	}

	return batchResult, nil
}

// processBSVAddress determines if a BSV address needs fetching or can use cached data.
// Returns (needsFetch, cachedResult).
func (s *Service) processBSVAddress(addr string, forceRefresh bool) (bool, *FetchResult) {
	// Skip policy check if forcing refresh or no policy configured
	if s.policy == nil || forceRefresh || s.force {
		return true, nil
	}

	// Consult refresh policy
	decision := s.policy.ShouldRefresh("bsv", addr)
	if decision != CacheOK {
		// RefreshRequired
		return true, nil
	}

	// Use cached data
	cachedBalances := getCachedBalancesForAddress("bsv", addr, s.cache)
	if len(cachedBalances) == 0 {
		// No cache exists, need to fetch
		return true, nil
	}

	result := &FetchResult{
		ChainID:  "bsv",
		Address:  addr,
		Balances: make([]BalanceEntry, len(cachedBalances)),
	}
	for i, cached := range cachedBalances {
		result.Balances[i] = cacheEntryToBalanceEntry(cached)
	}
	return false, result
}

// cacheEntryToBalanceEntry converts a CacheEntry to a BalanceEntry.
func cacheEntryToBalanceEntry(entry CacheEntry) BalanceEntry {
	age := time.Since(entry.UpdatedAt)
	return BalanceEntry{
		Chain:       entry.Chain,
		Address:     entry.Address,
		Balance:     entry.Balance,
		Unconfirmed: entry.Unconfirmed,
		Symbol:      entry.Symbol,
		Token:       entry.Token,
		Decimals:    entry.Decimals,
		Stale:       age > cache.DefaultStaleness,
		UpdatedAt:   entry.UpdatedAt,
	}
}
