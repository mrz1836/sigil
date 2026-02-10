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

	t.Run("Corrupt cache is renamed and returns ErrCorruptCache", func(t *testing.T) {
		corruptPath := filepath.Join(tmpDir, "corrupt.json")
		storage := NewFileStorage(corruptPath)

		require.NoError(t, os.WriteFile(corruptPath, []byte("{invalid json"), 0o640)) //nolint:gosec // G306: Test file needs matching cache perms

		loaded, err := storage.Load()
		require.ErrorIs(t, err, ErrCorruptCache)
		assert.NotNil(t, loaded)
		assert.Equal(t, 0, loaded.Size())

		matches, globErr := filepath.Glob(corruptPath + ".corrupt.*")
		require.NoError(t, globErr)
		require.Len(t, matches, 1)

		_, statErr := os.Stat(corruptPath)
		assert.True(t, os.IsNotExist(statErr))
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

func TestFileStorage_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "concurrent.json")
	storage := NewFileStorage(cachePath)

	const (
		numWriters  = 10
		numReaders  = 10
		numDeleters = 10
	)

	done := make(chan bool, numWriters+numReaders+numDeleters)

	// Initialize with some data
	initialCache := NewBalanceCache()
	initialCache.Set(BalanceCacheEntry{
		Chain:   chain.ETH,
		Address: "0xinit",
		Balance: "1.0",
	})
	require.NoError(t, storage.Save(initialCache))

	// Concurrent writers
	for i := 0; i < numWriters; i++ {
		go func(id int) {
			cache := NewBalanceCache()
			cache.Set(BalanceCacheEntry{
				Chain:   chain.ETH,
				Address: "0xwriter" + string(rune('0'+id)),
				Balance: "1.0",
			})
			_ = storage.Save(cache)
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < numReaders; i++ {
		go func() {
			_, _ = storage.Load()
			done <- true
		}()
	}

	// Concurrent deleters
	for i := 0; i < numDeleters; i++ {
		go func() {
			cache, err := storage.Load()
			if err == nil {
				cache.Delete(chain.ETH, "0xinit", "")
				_ = storage.Save(cache)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numWriters+numReaders+numDeleters; i++ {
		<-done
	}

	// Verify cache is still valid (not corrupted)
	finalCache, err := storage.Load()
	require.NoError(t, err)
	assert.NotNil(t, finalCache)
	t.Logf("Final cache size: %d", finalCache.Size())
}

func TestFileStorage_SaveAtomicity(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "atomic.json")
	storage := NewFileStorage(cachePath)

	const numSaves = 20
	done := make(chan bool, numSaves)

	// Concurrent saves
	for i := 0; i < numSaves; i++ {
		go func(id int) {
			cache := NewBalanceCache()
			cache.Set(BalanceCacheEntry{
				Chain:   chain.ETH,
				Address: "0xsave" + string(rune('0'+id%10)),
				Balance: "1.0",
			})
			err := storage.Save(cache)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all saves
	for i := 0; i < numSaves; i++ {
		<-done
	}

	// Verify cache is valid and not corrupted
	cache, err := storage.Load()
	require.NoError(t, err)
	assert.NotNil(t, cache)
	t.Logf("Cache after %d concurrent saves: size=%d", numSaves, cache.Size())
}

func TestBalanceCache_KeyCollisions(t *testing.T) {
	t.Parallel()

	cache := NewBalanceCache()

	// Test that addresses with colons don't collide
	cache.Set(BalanceCacheEntry{
		Chain:   chain.ETH,
		Address: "0x123:456",
		Balance: "1.0",
		Symbol:  "TEST1",
	})

	cache.Set(BalanceCacheEntry{
		Chain:   chain.ETH,
		Address: "0x123",
		Balance: "2.0",
		Symbol:  "TEST2",
		Token:   "456",
	})

	// These should be different entries
	entry1, exists1, _ := cache.Get(chain.ETH, "0x123:456", "")
	assert.True(t, exists1)
	assert.Equal(t, "TEST1", entry1.Symbol)
	assert.Equal(t, "1.0", entry1.Balance)

	entry2, exists2, _ := cache.Get(chain.ETH, "0x123", "456")
	assert.True(t, exists2)
	assert.Equal(t, "TEST2", entry2.Symbol)
	assert.Equal(t, "2.0", entry2.Balance)

	// Verify size is 2
	assert.Equal(t, 2, cache.Size())
}

func TestFileStorage_Delete(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "delete.json")
	storage := NewFileStorage(cachePath)

	// Create a cache file
	cache := NewBalanceCache()
	cache.Set(BalanceCacheEntry{
		Chain:   chain.ETH,
		Address: "0x123",
		Balance: "1.0",
	})
	require.NoError(t, storage.Save(cache))
	require.True(t, storage.Exists())

	// Delete it
	err := storage.Delete()
	require.NoError(t, err)
	assert.False(t, storage.Exists())

	// Delete again should not error
	err = storage.Delete()
	require.NoError(t, err)
}

func TestFileStorage_Exists(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "exists.json")
	storage := NewFileStorage(cachePath)

	// Initially doesn't exist
	assert.False(t, storage.Exists())

	// After save, it exists
	cache := NewBalanceCache()
	require.NoError(t, storage.Save(cache))
	assert.True(t, storage.Exists())
}

func TestFileStorage_Path(t *testing.T) {
	t.Parallel()

	path := "/tmp/test.json"
	storage := NewFileStorage(path)
	assert.Equal(t, path, storage.Path())
}
