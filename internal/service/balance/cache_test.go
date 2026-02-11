package balance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
)

// TestCacheAdapter_Get_Exists tests retrieving an existing cache entry.
func TestCacheAdapter_Get_Exists(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	adapter := NewCacheAdapter(balanceCache)

	// Set entry directly in cache
	now := time.Now()
	cacheEntry := cache.BalanceCacheEntry{
		Chain:       chain.BSV,
		Address:     "1ABC123",
		Balance:     "0.001",
		Unconfirmed: "0.0",
		Symbol:      "BSV",
		Token:       "",
		Decimals:    8,
		UpdatedAt:   now,
	}
	balanceCache.Set(cacheEntry)

	// Get via adapter
	entry, exists, age := adapter.Get(chain.BSV, "1ABC123", "")

	require.True(t, exists)
	require.NotNil(t, entry)
	assert.Equal(t, chain.BSV, entry.Chain)
	assert.Equal(t, "1ABC123", entry.Address)
	assert.Equal(t, "0.001", entry.Balance)
	assert.Equal(t, "0.0", entry.Unconfirmed)
	assert.Equal(t, "BSV", entry.Symbol)
	assert.Empty(t, entry.Token)
	assert.Equal(t, 8, entry.Decimals)
	assert.Less(t, age, 1*time.Second) // Should be very recent
	assert.NotZero(t, entry.UpdatedAt)
}

// TestCacheAdapter_Get_NotExists tests retrieving a non-existent cache entry.
func TestCacheAdapter_Get_NotExists(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	adapter := NewCacheAdapter(balanceCache)

	// Try to get non-existent entry
	entry, exists, age := adapter.Get(chain.BSV, "1NOTFOUND", "")

	assert.False(t, exists)
	assert.Nil(t, entry)
	assert.Zero(t, age)
}

// TestCacheAdapter_Get_WithToken tests retrieving a token balance.
func TestCacheAdapter_Get_WithToken(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	adapter := NewCacheAdapter(balanceCache)

	// Set USDC token entry
	usdcAddr := "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
	cacheEntry := cache.BalanceCacheEntry{
		Chain:       chain.ETH,
		Address:     "0x123",
		Balance:     "100.5",
		Unconfirmed: "0.0",
		Symbol:      "USDC",
		Token:       usdcAddr,
		Decimals:    6,
		UpdatedAt:   time.Now(),
	}
	balanceCache.Set(cacheEntry)

	// Get USDC balance
	entry, exists, age := adapter.Get(chain.ETH, "0x123", usdcAddr)

	require.True(t, exists)
	require.NotNil(t, entry)
	assert.Equal(t, chain.ETH, entry.Chain)
	assert.Equal(t, "0x123", entry.Address)
	assert.Equal(t, "100.5", entry.Balance)
	assert.Equal(t, "USDC", entry.Symbol)
	assert.Equal(t, usdcAddr, entry.Token)
	assert.Equal(t, 6, entry.Decimals)
	assert.Less(t, age, 1*time.Second) // Just set
}

// TestCacheAdapter_Set tests storing a balance in the cache.
func TestCacheAdapter_Set(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	adapter := NewCacheAdapter(balanceCache)

	now := time.Now()
	entry := CacheEntry{
		Chain:       chain.ETH,
		Address:     "0xABC",
		Balance:     "2.5",
		Unconfirmed: "0.1",
		Symbol:      "ETH",
		Token:       "",
		Decimals:    18,
		UpdatedAt:   now,
	}

	// Set via adapter
	adapter.Set(entry)

	// Verify stored in cache
	retrieved, exists, age := adapter.Get(chain.ETH, "0xABC", "")
	require.True(t, exists)
	require.NotNil(t, retrieved)
	assert.Equal(t, chain.ETH, retrieved.Chain)
	assert.Equal(t, "0xABC", retrieved.Address)
	assert.Equal(t, "2.5", retrieved.Balance)
	assert.Equal(t, "0.1", retrieved.Unconfirmed)
	assert.Equal(t, "ETH", retrieved.Symbol)
	assert.Empty(t, retrieved.Token)
	assert.Equal(t, 18, retrieved.Decimals)
	assert.Less(t, age, 1*time.Second)
}

// TestCacheAdapter_Set_OverwritesExisting tests that Set overwrites existing entries.
func TestCacheAdapter_Set_OverwritesExisting(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	adapter := NewCacheAdapter(balanceCache)

	// Set initial entry
	entry1 := CacheEntry{
		Chain:     chain.BSV,
		Address:   "1ABC",
		Balance:   "1.0",
		Symbol:    "BSV",
		Decimals:  8,
		UpdatedAt: time.Now().Add(-10 * time.Minute),
	}
	adapter.Set(entry1)

	// Overwrite with new balance
	entry2 := CacheEntry{
		Chain:     chain.BSV,
		Address:   "1ABC",
		Balance:   "2.0",
		Symbol:    "BSV",
		Decimals:  8,
		UpdatedAt: time.Now(),
	}
	adapter.Set(entry2)

	// Verify latest entry
	retrieved, exists, age := adapter.Get(chain.BSV, "1ABC", "")
	require.True(t, exists)
	assert.Equal(t, "2.0", retrieved.Balance)
	assert.Less(t, age, 1*time.Second) // Should be recent
}

// TestGetCachedBalancesForAddress_NativeOnly tests getting native balance only.
func TestGetCachedBalancesForAddress_NativeOnly(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()

	// Set native BSV balance
	nativeEntry := &CacheEntry{
		Chain:    chain.BSV,
		Address:  "1ABC",
		Balance:  "0.5",
		Symbol:   "BSV",
		Token:    "",
		Decimals: 8,
	}
	provider.entries[string(chain.BSV)+":1ABC"] = nativeEntry

	results := getCachedBalancesForAddress(chain.BSV, "1ABC", provider)

	require.Len(t, results, 1)
	assert.Equal(t, chain.BSV, results[0].Chain)
	assert.Equal(t, "1ABC", results[0].Address)
	assert.Equal(t, "0.5", results[0].Balance)
	assert.Equal(t, "BSV", results[0].Symbol)
}

// TestGetCachedBalancesForAddress_ETH_WithUSDC tests getting ETH native + USDC.
func TestGetCachedBalancesForAddress_ETH_WithUSDC(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()
	usdcAddr := "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"

	// Set ETH native balance
	ethEntry := &CacheEntry{
		Chain:    chain.ETH,
		Address:  "0x123",
		Balance:  "1.5",
		Symbol:   "ETH",
		Token:    "",
		Decimals: 18,
	}
	provider.entries[string(chain.ETH)+":0x123"] = ethEntry

	// Set USDC balance
	usdcEntry := &CacheEntry{
		Chain:    chain.ETH,
		Address:  "0x123",
		Balance:  "100.0",
		Symbol:   "USDC",
		Token:    usdcAddr,
		Decimals: 6,
	}
	provider.entries[string(chain.ETH)+":0x123:"+usdcAddr] = usdcEntry

	results := getCachedBalancesForAddress(chain.ETH, "0x123", provider)

	require.Len(t, results, 2)

	// First should be ETH native
	assert.Equal(t, chain.ETH, results[0].Chain)
	assert.Equal(t, "1.5", results[0].Balance)
	assert.Equal(t, "ETH", results[0].Symbol)
	assert.Empty(t, results[0].Token)

	// Second should be USDC
	assert.Equal(t, chain.ETH, results[1].Chain)
	assert.Equal(t, "100.0", results[1].Balance)
	assert.Equal(t, "USDC", results[1].Symbol)
	assert.Equal(t, usdcAddr, results[1].Token)
}

// TestGetCachedBalancesForAddress_ETH_OnlyNative tests ETH with no USDC cached.
func TestGetCachedBalancesForAddress_ETH_OnlyNative(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()

	// Set only ETH native balance (no USDC)
	ethEntry := &CacheEntry{
		Chain:    chain.ETH,
		Address:  "0x456",
		Balance:  "3.0",
		Symbol:   "ETH",
		Token:    "",
		Decimals: 18,
	}
	provider.entries[string(chain.ETH)+":0x456"] = ethEntry

	results := getCachedBalancesForAddress(chain.ETH, "0x456", provider)

	require.Len(t, results, 1)
	assert.Equal(t, chain.ETH, results[0].Chain)
	assert.Equal(t, "3.0", results[0].Balance)
	assert.Equal(t, "ETH", results[0].Symbol)
}

// TestGetCachedBalancesForAddress_NoCacheEntries tests handling of no cached entries.
func TestGetCachedBalancesForAddress_NoCacheEntries(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()

	results := getCachedBalancesForAddress(chain.BSV, "1NOTFOUND", provider)

	assert.Empty(t, results, "should return empty slice when no cache entries")
}

// TestGetCachedBalancesForAddress_NonETH_IgnoresUSDC tests non-ETH chains don't check USDC.
func TestGetCachedBalancesForAddress_NonETH_IgnoresUSDC(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()
	usdcAddr := "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"

	// Set BSV native
	bsvEntry := &CacheEntry{
		Chain:    chain.BSV,
		Address:  "1BSV",
		Balance:  "1.0",
		Symbol:   "BSV",
		Token:    "",
		Decimals: 8,
	}
	provider.entries[string(chain.BSV)+":1BSV"] = bsvEntry

	// Also set a USDC entry (shouldn't be retrieved for BSV)
	usdcEntry := &CacheEntry{
		Chain:    chain.BSV,
		Address:  "1BSV",
		Balance:  "100.0",
		Symbol:   "USDC",
		Token:    usdcAddr,
		Decimals: 6,
	}
	provider.entries[string(chain.BSV)+":1BSV:"+usdcAddr] = usdcEntry

	results := getCachedBalancesForAddress(chain.BSV, "1BSV", provider)

	require.Len(t, results, 1, "should only return native balance for non-ETH chains")
	assert.Equal(t, "BSV", results[0].Symbol)
}
