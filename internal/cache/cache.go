// Package cache provides balance caching functionality.
package cache

import (
	"sync"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
)

// DefaultStaleness is the default duration after which cache entries are considered stale.
const DefaultStaleness = 5 * time.Minute

// Cache defines the interface for balance caching operations.
type Cache interface {
	// Get retrieves a cached balance entry.
	Get(chainID chain.ID, address, token string) (*BalanceCacheEntry, bool, time.Duration)

	// Set stores a balance entry in the cache.
	Set(entry BalanceCacheEntry)

	// IsStale checks if a cache entry is stale.
	IsStale(chainID chain.ID, address, token string) bool

	// IsStaleWithDuration checks staleness with custom duration.
	IsStaleWithDuration(chainID chain.ID, address, token string, staleness time.Duration) bool

	// Delete removes a cache entry.
	Delete(chainID chain.ID, address, token string)

	// Clear removes all cache entries.
	Clear()

	// Size returns the number of cache entries.
	Size() int

	// GetAllForAddress returns all cached balances for an address.
	GetAllForAddress(address string) []BalanceCacheEntry

	// Prune removes entries older than maxAge.
	Prune(maxAge time.Duration) int
}

// Compile-time interface check
var _ Cache = (*BalanceCache)(nil)

// BalanceCache stores cached balance information.
type BalanceCache struct {
	mu      sync.RWMutex                 `json:"-"`
	Entries map[string]BalanceCacheEntry `json:"entries"`
}

// BalanceCacheEntry represents a single cached balance.
type BalanceCacheEntry struct {
	Chain     chain.ID  `json:"chain"`
	Address   string    `json:"address"`
	Balance   string    `json:"balance"`
	Symbol    string    `json:"symbol"`
	Decimals  int       `json:"decimals"`
	Token     string    `json:"token,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewBalanceCache creates a new empty balance cache.
func NewBalanceCache() *BalanceCache {
	return &BalanceCache{
		Entries: make(map[string]BalanceCacheEntry),
	}
}

// Key generates a cache key for an address and optional token.
func Key(chainID chain.ID, address, token string) string {
	if token != "" {
		return string(chainID) + ":" + address + ":" + token
	}
	return string(chainID) + ":" + address
}

// Get retrieves a cached balance entry.
// Returns the entry, whether it exists, and its age.
func (c *BalanceCache) Get(chainID chain.ID, address, token string) (*BalanceCacheEntry, bool, time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := Key(chainID, address, token)
	entry, exists := c.Entries[key]
	if !exists {
		return nil, false, 0
	}

	age := time.Since(entry.UpdatedAt)
	return &entry, true, age
}

// Set stores a balance entry in the cache.
func (c *BalanceCache) Set(entry BalanceCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := Key(entry.Chain, entry.Address, entry.Token)
	entry.UpdatedAt = time.Now()
	c.Entries[key] = entry
}

// IsStale checks if a cache entry is stale based on the default staleness duration.
func (c *BalanceCache) IsStale(chainID chain.ID, address, token string) bool {
	return c.IsStaleWithDuration(chainID, address, token, DefaultStaleness)
}

// IsStaleWithDuration checks if a cache entry is stale based on a custom duration.
func (c *BalanceCache) IsStaleWithDuration(chainID chain.ID, address, token string, staleness time.Duration) bool {
	_, exists, age := c.Get(chainID, address, token)
	if !exists {
		return true
	}
	return age > staleness
}

// Delete removes a cache entry.
func (c *BalanceCache) Delete(chainID chain.ID, address, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := Key(chainID, address, token)
	delete(c.Entries, key)
}

// Clear removes all cache entries.
func (c *BalanceCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Entries = make(map[string]BalanceCacheEntry)
}

// Size returns the number of cache entries.
func (c *BalanceCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.Entries)
}

// GetAllForAddress returns all cached balances for an address across all chains.
func (c *BalanceCache) GetAllForAddress(address string) []BalanceCacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var entries []BalanceCacheEntry
	for _, entry := range c.Entries {
		if entry.Address == address {
			entries = append(entries, entry)
		}
	}
	return entries
}

// Prune removes entries older than the specified duration.
func (c *BalanceCache) Prune(maxAge time.Duration) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0
	cutoff := time.Now().Add(-maxAge)

	for key, entry := range c.Entries {
		if entry.UpdatedAt.Before(cutoff) {
			delete(c.Entries, key)
			removed++
		}
	}

	return removed
}
