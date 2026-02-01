package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

func TestFileStorage(t *testing.T) {
	t.Parallel()
	// Create a temporary directory for tests
	tmpDir, err := os.MkdirTemp("", "cache-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cachePath := filepath.Join(tmpDir, "balances.json")
	storage := NewFileStorage(cachePath)

	t.Run("Save and Load round-trip", func(t *testing.T) {
		cache := NewBalanceCache()
		cache.Set(BalanceCacheEntry{
			Chain:    chain.ETH,
			Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			Balance:  "1.5",
			Symbol:   "ETH",
			Decimals: 18,
		})
		cache.Set(BalanceCacheEntry{
			Chain:    chain.BSV,
			Address:  "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			Balance:  "0.5",
			Symbol:   "BSV",
			Decimals: 8,
		})

		// Save
		err := storage.Save(cache)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(cachePath)
		require.NoError(t, err)

		// Load
		loaded, err := storage.Load()
		require.NoError(t, err)

		// Verify entries
		assert.Equal(t, 2, loaded.Size())

		ethEntry, exists, _ := loaded.Get(chain.ETH, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", "")
		require.True(t, exists)
		assert.Equal(t, "1.5", ethEntry.Balance)
		assert.Equal(t, "ETH", ethEntry.Symbol)

		bsvEntry, exists, _ := loaded.Get(chain.BSV, "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "")
		require.True(t, exists)
		assert.Equal(t, "0.5", bsvEntry.Balance)
		assert.Equal(t, "BSV", bsvEntry.Symbol)
	})

	t.Run("Load returns empty cache when file doesn't exist", func(t *testing.T) {
		nonExistent := filepath.Join(tmpDir, "nonexistent.json")
		storage := NewFileStorage(nonExistent)

		cache, err := storage.Load()
		require.NoError(t, err)
		assert.NotNil(t, cache)
		assert.Equal(t, 0, cache.Size())
	})

	t.Run("Creates parent directories on save", func(t *testing.T) {
		nestedPath := filepath.Join(tmpDir, "nested", "dir", "cache.json")
		storage := NewFileStorage(nestedPath)

		cache := NewBalanceCache()
		cache.Set(BalanceCacheEntry{
			Chain:   chain.ETH,
			Address: "0x123",
			Balance: "1",
		})

		err := storage.Save(cache)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(nestedPath)
		require.NoError(t, err)
	})
}

func TestBalanceCache(t *testing.T) {
	t.Parallel()
	t.Run("Get returns entry and age", func(t *testing.T) {
		t.Parallel()
		cache := NewBalanceCache()
		cache.Set(BalanceCacheEntry{
			Chain:    chain.ETH,
			Address:  "0x123",
			Balance:  "1.0",
			Symbol:   "ETH",
			Decimals: 18,
		})

		entry, exists, age := cache.Get(chain.ETH, "0x123", "")
		assert.True(t, exists)
		assert.NotNil(t, entry)
		assert.Equal(t, "1.0", entry.Balance)
		assert.Less(t, age, time.Second) // Just set, should be very recent
	})

	t.Run("Get returns not found for missing entry", func(t *testing.T) {
		t.Parallel()
		cache := NewBalanceCache()

		entry, exists, _ := cache.Get(chain.ETH, "nonexistent", "")
		assert.False(t, exists)
		assert.Nil(t, entry)
	})

	t.Run("Handles token entries separately", func(t *testing.T) {
		t.Parallel()
		cache := NewBalanceCache()

		// Set native balance
		cache.Set(BalanceCacheEntry{
			Chain:    chain.ETH,
			Address:  "0x123",
			Balance:  "1.0",
			Symbol:   "ETH",
			Decimals: 18,
		})

		// Set USDC balance
		cache.Set(BalanceCacheEntry{
			Chain:    chain.ETH,
			Address:  "0x123",
			Balance:  "500.0",
			Symbol:   "USDC",
			Decimals: 6,
			Token:    "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		})

		// Verify both exist separately
		ethEntry, exists, _ := cache.Get(chain.ETH, "0x123", "")
		assert.True(t, exists)
		assert.Equal(t, "ETH", ethEntry.Symbol)

		usdcEntry, exists, _ := cache.Get(chain.ETH, "0x123", "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
		assert.True(t, exists)
		assert.Equal(t, "USDC", usdcEntry.Symbol)
	})

	t.Run("IsStale returns true for old entries", func(t *testing.T) {
		cache := NewBalanceCache()
		cache.Set(BalanceCacheEntry{
			Chain:   chain.ETH,
			Address: "0x123",
			Balance: "1.0",
		})

		// Manually set old timestamp
		key := Key(chain.ETH, "0x123", "")
		entry := cache.Entries[key]
		entry.UpdatedAt = time.Now().Add(-10 * time.Minute)
		cache.Entries[key] = entry

		assert.True(t, cache.IsStale(chain.ETH, "0x123", ""))
	})

	t.Run("IsStale returns false for recent entries", func(t *testing.T) {
		cache := NewBalanceCache()
		cache.Set(BalanceCacheEntry{
			Chain:   chain.ETH,
			Address: "0x123",
			Balance: "1.0",
		})

		assert.False(t, cache.IsStale(chain.ETH, "0x123", ""))
	})

	t.Run("Delete removes entry", func(t *testing.T) {
		t.Parallel()
		cache := NewBalanceCache()
		cache.Set(BalanceCacheEntry{
			Chain:   chain.ETH,
			Address: "0x123",
			Balance: "1.0",
		})

		cache.Delete(chain.ETH, "0x123", "")

		_, exists, _ := cache.Get(chain.ETH, "0x123", "")
		assert.False(t, exists)
	})

	t.Run("Clear removes all entries", func(t *testing.T) {
		t.Parallel()
		cache := NewBalanceCache()
		cache.Set(BalanceCacheEntry{Chain: chain.ETH, Address: "0x123", Balance: "1"})
		cache.Set(BalanceCacheEntry{Chain: chain.BSV, Address: "1abc", Balance: "2"})

		assert.Equal(t, 2, cache.Size())

		cache.Clear()
		assert.Equal(t, 0, cache.Size())
	})

	t.Run("Prune removes old entries", func(t *testing.T) {
		cache := NewBalanceCache()

		// Add recent entry
		cache.Set(BalanceCacheEntry{Chain: chain.ETH, Address: "0x123", Balance: "1"})

		// Add old entry
		cache.Set(BalanceCacheEntry{Chain: chain.BSV, Address: "1abc", Balance: "2"})
		key := Key(chain.BSV, "1abc", "")
		entry := cache.Entries[key]
		entry.UpdatedAt = time.Now().Add(-1 * time.Hour)
		cache.Entries[key] = entry

		// Prune entries older than 30 minutes
		removed := cache.Prune(30 * time.Minute)

		assert.Equal(t, 1, removed)
		assert.Equal(t, 1, cache.Size())

		_, exists, _ := cache.Get(chain.ETH, "0x123", "")
		assert.True(t, exists)

		_, exists, _ = cache.Get(chain.BSV, "1abc", "")
		assert.False(t, exists)
	})
}
