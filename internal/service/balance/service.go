package balance

import (
	"context"
	"sync"
	"time"

	"github.com/mrz1836/sigil/internal/cache"
)

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
//
//nolint:gocognit // Complex business logic for concurrent batch fetching
func (s *Service) FetchBalances(ctx context.Context, req *FetchBatchRequest) (*FetchBatchResult, error) {
	batchResult := &FetchBatchResult{
		Results: make([]*FetchResult, 0, len(req.Addresses)),
	}

	maxConcurrent := req.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 8
	}

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, addr := range req.Addresses {
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

	wg.Wait()

	return batchResult, nil
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
