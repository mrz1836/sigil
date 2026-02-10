// Package balance provides balance fetching, caching, and refresh services.
package balance

import (
	"time"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
)

// CacheAdapter adapts cache.BalanceCache to the CacheProvider interface.
// This decouples the service from the concrete cache implementation.
type CacheAdapter struct {
	cache *cache.BalanceCache
}

// NewCacheAdapter creates a new cache adapter.
func NewCacheAdapter(c *cache.BalanceCache) *CacheAdapter {
	return &CacheAdapter{cache: c}
}

// Get retrieves a balance from the cache.
func (a *CacheAdapter) Get(chainID chain.ID, address, token string) (*CacheEntry, bool, time.Duration) {
	entry, exists, age := a.cache.Get(chainID, address, token)
	if !exists {
		return nil, false, 0
	}

	// Convert cache.BalanceCacheEntry to service CacheEntry
	serviceEntry := &CacheEntry{
		Chain:       entry.Chain,
		Address:     entry.Address,
		Balance:     entry.Balance,
		Unconfirmed: entry.Unconfirmed,
		Symbol:      entry.Symbol,
		Token:       entry.Token,
		Decimals:    entry.Decimals,
		UpdatedAt:   entry.UpdatedAt,
	}

	return serviceEntry, true, age
}

// Set stores a balance in the cache.
func (a *CacheAdapter) Set(entry CacheEntry) {
	// Convert service CacheEntry to cache.BalanceCacheEntry
	cacheEntry := cache.BalanceCacheEntry{
		Chain:       entry.Chain,
		Address:     entry.Address,
		Balance:     entry.Balance,
		Unconfirmed: entry.Unconfirmed,
		Symbol:      entry.Symbol,
		Token:       entry.Token,
		Decimals:    entry.Decimals,
		UpdatedAt:   entry.UpdatedAt,
	}

	a.cache.Set(cacheEntry)
}

// getCachedBalancesForAddress retrieves all cached balances for an address.
// Returns empty slice if no cache entries found.
// This is a helper function used by the service.
func getCachedBalancesForAddress(chainID chain.ID, address string, cache CacheProvider) []CacheEntry {
	var results []CacheEntry

	// Check native balance
	if entry, exists, _ := cache.Get(chainID, address, ""); exists {
		results = append(results, *entry)
	}

	// For ETH, also check USDC
	if chainID == chain.ETH {
		if entry, exists, _ := cache.Get(chainID, address, "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"); exists {
			results = append(results, *entry)
		}
	}

	return results
}
