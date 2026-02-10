package balance

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/chain/eth"
	"github.com/mrz1836/sigil/internal/chain/eth/etherscan"
	"github.com/mrz1836/sigil/internal/metrics"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// ErrUnsupportedChain is returned when a chain is not supported for balance fetching.
var ErrUnsupportedChain = errors.New("unsupported chain")

// Fetcher handles fetching balances from chain providers.
type Fetcher struct {
	cfg   ConfigProvider
	cache CacheProvider
}

// NewFetcher creates a new balance fetcher.
func NewFetcher(cfg ConfigProvider, cache CacheProvider) *Fetcher {
	return &Fetcher{
		cfg:   cfg,
		cache: cache,
	}
}

// postSendCacheTrust is the duration after a send during which locally-computed
// cached balances are trusted over network queries. This covers the window
// where the blockchain indexer may not yet reflect the broadcast transaction.
const postSendCacheTrust = 30 * time.Second

// FetchForChain fetches balances for a single address on the specified chain.
// Returns balance entries, whether data is stale, and any error.
func (f *Fetcher) FetchForChain(ctx context.Context, chainID chain.ID, address string) ([]CacheEntry, bool, error) {
	switch chainID {
	case chain.ETH:
		return f.fetchETH(ctx, address)
	case chain.BSV:
		return f.fetchBSV(ctx, address)
	case chain.BTC, chain.BCH:
		// BTC and BCH not supported in MVP
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("%w: %s", ErrUnsupportedChain, chainID)
	}
}

// fetchETH fetches ETH and USDC balances using the configured provider with failover.
func (f *Fetcher) fetchETH(ctx context.Context, address string) ([]CacheEntry, bool, error) {
	provider := f.cfg.GetETHProvider()

	type fetchFn func() ([]CacheEntry, bool, error)

	etherscanFn := func() ([]CacheEntry, bool, error) {
		apiKey := f.cfg.GetETHEtherscanAPIKey()
		if apiKey == "" {
			return nil, true, etherscan.ErrAPIKeyRequired
		}
		return f.fetchETHViaEtherscan(ctx, address, apiKey)
	}

	rpcFn := func() ([]CacheEntry, bool, error) {
		return f.fetchETHViaRPC(ctx, address)
	}

	var primaryFn, secondaryFn fetchFn
	if provider == "rpc" {
		primaryFn = rpcFn
		secondaryFn = etherscanFn
	} else {
		// Default: etherscan primary, rpc secondary
		primaryFn = etherscanFn
		secondaryFn = rpcFn
	}

	// Try primary
	entries, stale, err := primaryFn()
	if err == nil {
		return entries, stale, nil
	}

	// Primary failed: try secondary (failover)
	fallbackEntries, fallbackStale, fallbackErr := secondaryFn()
	if fallbackErr == nil {
		return fallbackEntries, fallbackStale, nil
	}

	// Both providers failed: return cached data
	cached, cachedStale, cacheErr := f.getCachedETHBalances(address)
	if cacheErr == nil {
		return cached, cachedStale, nil
	}

	return nil, true, err
}

// fetchETHViaEtherscan fetches ETH and USDC balances using the Etherscan API.
func (f *Fetcher) fetchETHViaEtherscan(ctx context.Context, address, apiKey string) ([]CacheEntry, bool, error) {
	// Trust very fresh cache entries (set by a recent tx send).
	if _, exists, age := f.cache.Get(chain.ETH, address, ""); exists && age < postSendCacheTrust {
		return f.getCachedETHBalances(address)
	}

	var entries []CacheEntry

	client, err := etherscan.NewClient(apiKey, nil)
	if err != nil {
		return nil, true, err
	}

	// Fetch ETH balance
	ethBalance, err := client.GetNativeBalance(ctx, address)
	if err != nil {
		return nil, true, err
	}

	ethEntry := CacheEntry{
		Chain:     chain.ETH,
		Address:   address,
		Balance:   chain.FormatDecimalAmount(ethBalance.Amount, ethBalance.Decimals),
		Symbol:    ethBalance.Symbol,
		Decimals:  ethBalance.Decimals,
		UpdatedAt: time.Now().UTC(),
	}
	f.cache.Set(ethEntry)
	entries = append(entries, ethEntry)

	// Fetch USDC balance
	usdcBalance, err := client.GetUSDCBalance(ctx, address)
	if err == nil {
		usdcEntry := CacheEntry{
			Chain:     chain.ETH,
			Address:   address,
			Balance:   chain.FormatDecimalAmount(usdcBalance.Amount, usdcBalance.Decimals),
			Symbol:    usdcBalance.Symbol,
			Token:     usdcBalance.Token,
			Decimals:  usdcBalance.Decimals,
			UpdatedAt: time.Now().UTC(),
		}
		f.cache.Set(usdcEntry)
		entries = append(entries, usdcEntry)
	}

	return entries, false, nil
}

// fetchETHViaRPC fetches ETH and USDC balances using JSON-RPC.
func (f *Fetcher) fetchETHViaRPC(ctx context.Context, address string) ([]CacheEntry, bool, error) {
	// Trust very fresh cache entries (set by a recent tx send).
	if _, exists, age := f.cache.Get(chain.ETH, address, ""); exists && age < postSendCacheTrust {
		return f.getCachedETHBalances(address)
	}

	var entries []CacheEntry
	var stale bool

	rpcURL := f.cfg.GetETHRPC()
	if rpcURL == "" {
		return nil, true, sigilerr.WithSuggestion(
			sigilerr.ErrNetworkError,
			"ETH RPC not configured. Set SIGIL_ETH_RPC or configure networks.eth.rpc in config.yaml",
		)
	}

	fallbackRPCs := f.cfg.GetETHFallbackRPCs()
	transport := sharedETHTransport()
	client, err := f.connectETHClient(rpcURL, fallbackRPCs, transport)
	if err != nil {
		return nil, true, err
	}
	defer client.Close()

	// Fetch ETH balance with fallback support
	ethBalance, client, err := f.fetchETHBalanceWithFallback(ctx, client, address, rpcURL, fallbackRPCs, transport)
	if err != nil {
		return nil, true, err
	}

	// Format unconfirmed (only set if non-zero)
	var ethUnconfirmedStr string
	if ethBalance.Unconfirmed != nil && ethBalance.Unconfirmed.Sign() != 0 {
		ethUnconfirmedStr = chain.FormatSignedDecimalAmount(ethBalance.Unconfirmed, ethBalance.Decimals)
	}

	// Store in cache
	ethEntry := CacheEntry{
		Chain:       chain.ETH,
		Address:     address,
		Balance:     chain.FormatDecimalAmount(ethBalance.Amount, ethBalance.Decimals),
		Unconfirmed: ethUnconfirmedStr,
		Symbol:      ethBalance.Symbol,
		Decimals:    ethBalance.Decimals,
		UpdatedAt:   time.Now().UTC(),
	}
	f.cache.Set(ethEntry)
	entries = append(entries, ethEntry)

	// Fetch USDC balance
	usdcBalance, err := client.GetUSDCBalance(ctx, address)
	if err == nil {
		usdcEntry := CacheEntry{
			Chain:     chain.ETH,
			Address:   address,
			Balance:   chain.FormatDecimalAmount(usdcBalance.Amount, usdcBalance.Decimals),
			Symbol:    usdcBalance.Symbol,
			Token:     usdcBalance.Token,
			Decimals:  usdcBalance.Decimals,
			UpdatedAt: time.Now().UTC(),
		}
		f.cache.Set(usdcEntry)
		entries = append(entries, usdcEntry)
	}

	return entries, stale, nil
}

// connectETHClient attempts to connect to the primary RPC, falling back to alternates on failure.
func (f *Fetcher) connectETHClient(rpcURL string, fallbackRPCs []string, transport *http.Transport) (*eth.Client, error) {
	opts := &eth.ClientOptions{Transport: transport}
	client, err := eth.NewClient(rpcURL, opts)
	if err == nil {
		return client, nil
	}
	// Try fallback RPCs
	for _, fallbackURL := range fallbackRPCs {
		client, err = eth.NewClient(fallbackURL, opts)
		if err == nil {
			return client, nil
		}
	}
	return nil, err
}

// fetchETHBalanceWithFallback fetches ETH balance, trying fallback RPCs on failure.
func (f *Fetcher) fetchETHBalanceWithFallback(ctx context.Context, client *eth.Client, address, primaryRPC string, fallbackRPCs []string, transport *http.Transport) (*eth.Balance, *eth.Client, error) {
	// Try primary client first
	balance, err := chain.Retry(ctx, func() (*eth.Balance, error) {
		bal, fetchErr := client.GetNativeBalance(ctx, address)
		if fetchErr != nil {
			return nil, chain.WrapRetryable(fetchErr)
		}
		return bal, nil
	})
	if err == nil {
		return balance, client, nil
	}

	// Try fallback RPCs, sharing the same transport.
	// The old client is intentionally not closed here because Close() calls
	// CloseIdleConnections() on the shared transport, which would disrupt
	// other goroutines using the same transport for concurrent requests.
	opts := &eth.ClientOptions{Transport: transport}
	for _, fallbackURL := range fallbackRPCs {
		if fallbackURL == primaryRPC {
			continue
		}
		fallbackClient, clientErr := eth.NewClient(fallbackURL, opts)
		if clientErr != nil {
			continue
		}
		balance, err = fallbackClient.GetNativeBalance(ctx, address)
		if err == nil {
			return balance, fallbackClient, nil
		}
	}

	return nil, client, err
}

// getCachedETHBalances returns cached ETH balances if available.
func (f *Fetcher) getCachedETHBalances(address string) ([]CacheEntry, bool, error) {
	entries := make([]CacheEntry, 0, 2)
	stale := false

	// Check for ETH
	entry, exists, age := f.cache.Get(chain.ETH, address, "")
	if exists {
		metrics.Global.RecordCacheHit()
		entries = append(entries, *entry)
		if age > cache.DefaultStaleness {
			stale = true
		}
	} else {
		metrics.Global.RecordCacheMiss()
	}

	// Check for USDC
	usdcEntry, exists, age := f.cache.Get(chain.ETH, address, eth.USDCMainnet)
	if exists {
		metrics.Global.RecordCacheHit()
		entries = append(entries, *usdcEntry)
		if age > cache.DefaultStaleness {
			stale = true
		}
	} else {
		metrics.Global.RecordCacheMiss()
	}

	if len(entries) == 0 {
		return nil, true, sigilerr.ErrCacheNotFound
	}

	return entries, stale, nil
}

// fetchBSV fetches BSV balances.
func (f *Fetcher) fetchBSV(ctx context.Context, address string) ([]CacheEntry, bool, error) {
	// Trust very fresh cache entries (set by a recent tx send) over the
	// network, which may not have indexed the transaction yet.
	if entry, exists, age := f.cache.Get(chain.BSV, address, ""); exists && age < postSendCacheTrust {
		return []CacheEntry{*entry}, false, nil
	}

	entries := make([]CacheEntry, 0, 1)
	var stale bool

	client := bsv.NewClient(ctx, nil)

	// Fetch BSV balance
	bsvBalance, err := client.GetNativeBalance(ctx, address)
	if err != nil {
		// Fall back to cache
		return f.getCachedBSVBalances(address)
	}

	// Format unconfirmed (only set if non-zero)
	var unconfirmedStr string
	if bsvBalance.Unconfirmed != nil && bsvBalance.Unconfirmed.Sign() != 0 {
		unconfirmedStr = chain.FormatSignedDecimalAmount(bsvBalance.Unconfirmed, bsvBalance.Decimals)
	}

	// Store in cache
	entry := CacheEntry{
		Chain:       chain.BSV,
		Address:     address,
		Balance:     chain.FormatDecimalAmount(bsvBalance.Amount, bsvBalance.Decimals),
		Unconfirmed: unconfirmedStr,
		Symbol:      bsvBalance.Symbol,
		Decimals:    bsvBalance.Decimals,
		UpdatedAt:   time.Now().UTC(),
	}
	f.cache.Set(entry)
	entries = append(entries, entry)

	return entries, stale, nil
}

// getCachedBSVBalances returns cached BSV balances if available.
func (f *Fetcher) getCachedBSVBalances(address string) ([]CacheEntry, bool, error) {
	entry, exists, age := f.cache.Get(chain.BSV, address, "")
	if !exists {
		metrics.Global.RecordCacheMiss()
		return nil, true, sigilerr.ErrCacheNotFound
	}
	metrics.Global.RecordCacheHit()

	stale := age > cache.DefaultStaleness
	return []CacheEntry{*entry}, stale, nil
}

// fetchBSVBulk fetches balances for multiple BSV addresses using bulk API.
// Returns a map of address -> entries. More efficient than individual calls.
//
//nolint:gocognit,gocyclo // Complex business logic for bulk balance fetching with caching
func (f *Fetcher) fetchBSVBulk(ctx context.Context, addresses []string) (map[string][]CacheEntry, error) {
	if len(addresses) == 0 {
		return make(map[string][]CacheEntry), nil
	}

	// Check post-send cache trust for all addresses
	addressesToFetch := make([]string, 0, len(addresses))
	results := make(map[string][]CacheEntry)

	for _, addr := range addresses {
		if entry, exists, age := f.cache.Get(chain.BSV, addr, ""); exists && age < postSendCacheTrust {
			// Use trusted fresh cache
			results[addr] = []CacheEntry{*entry}
		} else {
			addressesToFetch = append(addressesToFetch, addr)
		}
	}

	if len(addressesToFetch) == 0 {
		return results, nil
	}

	// Bulk fetch remaining addresses
	client := bsv.NewClient(ctx, nil)
	bulkBalances, err := client.GetBulkNativeBalance(ctx, addressesToFetch)
	if err != nil {
		// On error, fall back to cached data for all addresses
		for _, addr := range addressesToFetch {
			if cachedEntries, _, cacheErr := f.getCachedBSVBalances(addr); cacheErr == nil {
				results[addr] = cachedEntries
			}
		}
		return results, sigilerr.Wrap(err, "bulk BSV fetch failed, using cached data")
	}

	// Convert bulk results to cache entries
	for addr, balance := range bulkBalances {
		var unconfirmedStr string
		if balance.Unconfirmed != nil && balance.Unconfirmed.Sign() != 0 {
			unconfirmedStr = chain.FormatSignedDecimalAmount(balance.Unconfirmed, balance.Decimals)
		}

		entry := CacheEntry{
			Chain:       chain.BSV,
			Address:     addr,
			Balance:     chain.FormatDecimalAmount(balance.Amount, balance.Decimals),
			Unconfirmed: unconfirmedStr,
			Symbol:      balance.Symbol,
			Decimals:    balance.Decimals,
			UpdatedAt:   time.Now().UTC(),
		}

		f.cache.Set(entry)
		results[addr] = []CacheEntry{entry}
	}

	// Handle addresses not in bulk response (API returned no data for them).
	// This includes:
	// 1. Addresses completely absent from the bulk API response
	// 2. Addresses with nil Balance (filtered out by GetBulkNativeBalance)
	// Fall back to individual fetch for these addresses.
	for _, addr := range addressesToFetch {
		if _, found := results[addr]; found {
			continue
		}

		// Address was not in bulk response, try individual fetch
		entries, _, err := f.fetchBSV(ctx, addr)
		if err == nil && len(entries) > 0 {
			results[addr] = entries
			continue
		}

		// If individual fetch also fails, try to use cached data
		if cachedEntries, _, cacheErr := f.getCachedBSVBalances(addr); cacheErr == nil {
			results[addr] = cachedEntries
		}
	}

	return results, nil
}
