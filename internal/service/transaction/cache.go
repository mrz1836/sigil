package transaction

import (
	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
)

// invalidateBalanceCache updates the on-disk balance cache after a successful
// transaction broadcast. If expectedBalance is non-empty, the cached entry is
// updated with that value (e.g., "0.0" for sweep-all). Otherwise the entry is
// deleted, forcing the next balance query to fetch from the network.
// Errors are logged but never returned — cache invalidation is best-effort.
// Migrated from cli/tx.go lines 934-962
func invalidateBalanceCache(logger LogWriter, cacheProvider CacheProvider, chainID chain.ID, address, token, expectedBalance string) {
	balanceCache, err := cacheProvider.Load()
	if err != nil {
		if logger != nil {
			logger.Error("failed to load balance cache for post-send update: %v", err)
		}
		return
	}
	if balanceCache == nil {
		balanceCache = cache.NewBalanceCache()
	}

	if expectedBalance == "" {
		// Unknown expected balance — delete to force a fresh network fetch.
		balanceCache.Delete(chainID, address, token)
	} else {
		// Known expected balance (e.g., sweep-all → "0.0").
		// Preserve symbol/decimals from the existing entry if available.
		entry := buildPostSendEntry(balanceCache, chainID, address, token, expectedBalance)
		balanceCache.Set(entry)
	}

	if err := cacheProvider.Save(balanceCache); err != nil {
		if logger != nil {
			logger.Error("failed to save balance cache after send: %v", err)
		}
	}
}

// buildPostSendEntry creates a cache entry with the expected post-send balance,
// preserving symbol and decimals from any existing entry.
// Migrated from cli/tx.go lines 964-978
func buildPostSendEntry(bc *cache.BalanceCache, chainID chain.ID, address, token, balance string) cache.BalanceCacheEntry {
	if existing, exists, _ := bc.Get(chainID, address, token); exists {
		existing.Balance = balance
		existing.Unconfirmed = "" // Clear stale unconfirmed data after send
		return *existing
	}
	return cache.BalanceCacheEntry{
		Chain:   chainID,
		Address: address,
		Token:   token,
		Balance: balance,
	}
}

// InvalidateBalanceCache is the exported version for external use.
func InvalidateBalanceCache(logger LogWriter, cacheProvider CacheProvider, chainID chain.ID, address, token, expectedBalance string) {
	invalidateBalanceCache(logger, cacheProvider, chainID, address, token, expectedBalance)
}
