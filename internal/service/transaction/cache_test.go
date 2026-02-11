package transaction

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
)

// mockCacheProvider for testing cache operations
type mockCacheProvider struct {
	balanceCache *cache.BalanceCache
	loadErr      error
	saveErr      error
	loadCalled   int
	saveCalled   int
}

func newMockCacheProvider() *mockCacheProvider {
	return &mockCacheProvider{
		balanceCache: cache.NewBalanceCache(),
	}
}

func (m *mockCacheProvider) Load() (*cache.BalanceCache, error) {
	m.loadCalled++
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.balanceCache, nil
}

func (m *mockCacheProvider) Save(bc *cache.BalanceCache) error {
	m.saveCalled++
	if m.saveErr != nil {
		return m.saveErr
	}
	m.balanceCache = bc
	return nil
}

// TestInvalidateBalanceCache_DeleteEntry tests deletion when expectedBalance is empty.
func TestInvalidateBalanceCache_DeleteEntry(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()
	logger := newMockLogWriter()

	// Pre-populate cache with an entry
	provider.balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.BSV,
		Address:  "1ABC",
		Balance:  "1.5",
		Symbol:   "BSV",
		Decimals: 8,
	})

	// Verify entry exists
	_, exists, _ := provider.balanceCache.Get(chain.BSV, "1ABC", "")
	assert.True(t, exists)

	// Delete entry by passing empty expectedBalance
	invalidateBalanceCache(logger, provider, chain.BSV, "1ABC", "", "")

	// Verify entry was deleted
	_, exists, _ = provider.balanceCache.Get(chain.BSV, "1ABC", "")
	assert.False(t, exists, "entry should be deleted")
	assert.Equal(t, 1, provider.loadCalled)
	assert.Equal(t, 1, provider.saveCalled)
	assert.Empty(t, logger.errorMessages)
}

// TestInvalidateBalanceCache_UpdateEntry tests updating with expected balance.
func TestInvalidateBalanceCache_UpdateEntry(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()
	logger := newMockLogWriter()

	// Pre-populate cache with an entry
	provider.balanceCache.Set(cache.BalanceCacheEntry{
		Chain:       chain.BSV,
		Address:     "1ABC",
		Balance:     "1.5",
		Unconfirmed: "0.1",
		Symbol:      "BSV",
		Decimals:    8,
	})

	// Update entry with sweep-all balance
	invalidateBalanceCache(logger, provider, chain.BSV, "1ABC", "", "0.0")

	// Verify entry was updated
	entry, exists, _ := provider.balanceCache.Get(chain.BSV, "1ABC", "")
	require.True(t, exists, "entry should still exist")
	assert.Equal(t, "0.0", entry.Balance)
	assert.Empty(t, entry.Unconfirmed, "unconfirmed should be cleared")
	assert.Equal(t, "BSV", entry.Symbol, "symbol should be preserved")
	assert.Equal(t, 8, entry.Decimals, "decimals should be preserved")
	assert.Equal(t, 1, provider.loadCalled)
	assert.Equal(t, 1, provider.saveCalled)
	assert.Empty(t, logger.errorMessages)
}

// TestInvalidateBalanceCache_LoadError tests handling of cache load errors.
func TestInvalidateBalanceCache_LoadError(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()
	provider.loadErr = errors.New("disk read error") //nolint:err113 // Test error
	logger := newMockLogWriter()

	// Should handle error gracefully
	invalidateBalanceCache(logger, provider, chain.BSV, "1ABC", "", "0.0")

	// Verify error was logged
	assert.Equal(t, 1, provider.loadCalled)
	assert.Equal(t, 0, provider.saveCalled, "should not attempt save after load error")
	require.Len(t, logger.errorMessages, 1)
	assert.Contains(t, logger.errorMessages[0], "failed to load balance cache")
}

// TestInvalidateBalanceCache_SaveError tests handling of cache save errors.
func TestInvalidateBalanceCache_SaveError(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()
	provider.saveErr = errors.New("disk write error") //nolint:err113 // Test error
	logger := newMockLogWriter()

	// Pre-populate cache
	provider.balanceCache.Set(cache.BalanceCacheEntry{
		Chain:   chain.BSV,
		Address: "1ABC",
		Balance: "1.5",
	})

	// Should handle save error gracefully
	invalidateBalanceCache(logger, provider, chain.BSV, "1ABC", "", "0.0")

	// Verify error was logged
	assert.Equal(t, 1, provider.loadCalled)
	assert.Equal(t, 1, provider.saveCalled)
	require.Len(t, logger.errorMessages, 1)
	assert.Contains(t, logger.errorMessages[0], "failed to save balance cache")
}

// TestInvalidateBalanceCache_NilCache tests handling when cache is nil.
func TestInvalidateBalanceCache_NilCache(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()
	provider.balanceCache = nil
	logger := newMockLogWriter()

	// Should create new cache when nil
	invalidateBalanceCache(logger, provider, chain.BSV, "1ABC", "", "0.0")

	// Verify new cache was created and saved
	assert.Equal(t, 1, provider.loadCalled)
	assert.Equal(t, 1, provider.saveCalled)
	assert.NotNil(t, provider.balanceCache, "should create new cache")
	assert.Empty(t, logger.errorMessages)
}

// TestInvalidateBalanceCache_NilLogger tests that nil logger doesn't cause panic.
func TestInvalidateBalanceCache_NilLogger(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()
	provider.loadErr = errors.New("load error") //nolint:err113 // Test error

	// Should not panic with nil logger
	invalidateBalanceCache(nil, provider, chain.BSV, "1ABC", "", "0.0")

	assert.Equal(t, 1, provider.loadCalled)
}

// TestInvalidateBalanceCache_WithToken tests cache operations for tokens.
func TestInvalidateBalanceCache_WithToken(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()
	logger := newMockLogWriter()

	// Pre-populate with USDC token entry
	provider.balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  "0xABC",
		Token:    "USDC",
		Balance:  "100.0",
		Symbol:   "USDC",
		Decimals: 6,
	})

	// Update token balance
	invalidateBalanceCache(logger, provider, chain.ETH, "0xABC", "USDC", "95.0")

	// Verify token entry was updated
	entry, exists, _ := provider.balanceCache.Get(chain.ETH, "0xABC", "USDC")
	require.True(t, exists)
	assert.Equal(t, "95.0", entry.Balance)
	assert.Equal(t, "USDC", entry.Token)
	assert.Equal(t, "USDC", entry.Symbol)
	assert.Equal(t, 6, entry.Decimals)
	assert.Empty(t, logger.errorMessages)
}

// TestBuildPostSendEntry_ExistingEntry tests building entry when existing entry is present.
func TestBuildPostSendEntry_ExistingEntry(t *testing.T) {
	t.Parallel()

	bc := cache.NewBalanceCache()

	// Pre-populate with existing entry
	bc.Set(cache.BalanceCacheEntry{
		Chain:       chain.BSV,
		Address:     "1ABC",
		Balance:     "1.5",
		Unconfirmed: "0.1",
		Symbol:      "BSV",
		Decimals:    8,
	})

	// Build post-send entry
	entry := buildPostSendEntry(bc, chain.BSV, "1ABC", "", "0.5")

	// Verify existing metadata was preserved
	assert.Equal(t, chain.BSV, entry.Chain)
	assert.Equal(t, "1ABC", entry.Address)
	assert.Equal(t, "0.5", entry.Balance, "balance should be updated")
	assert.Empty(t, entry.Unconfirmed, "unconfirmed should be cleared")
	assert.Equal(t, "BSV", entry.Symbol, "symbol should be preserved")
	assert.Equal(t, 8, entry.Decimals, "decimals should be preserved")
}

// TestBuildPostSendEntry_NoExistingEntry tests building entry when no existing entry.
func TestBuildPostSendEntry_NoExistingEntry(t *testing.T) {
	t.Parallel()

	bc := cache.NewBalanceCache()

	// Build post-send entry with no existing entry
	entry := buildPostSendEntry(bc, chain.BSV, "1ABC", "", "0.0")

	// Verify new entry was created with basic fields
	assert.Equal(t, chain.BSV, entry.Chain)
	assert.Equal(t, "1ABC", entry.Address)
	assert.Equal(t, "0.0", entry.Balance)
	assert.Empty(t, entry.Token)
	assert.Empty(t, entry.Symbol, "symbol should be empty for new entry")
	assert.Equal(t, 0, entry.Decimals, "decimals should be zero for new entry")
	assert.Empty(t, entry.Unconfirmed)
}

// TestBuildPostSendEntry_WithToken tests building entry for token transactions.
func TestBuildPostSendEntry_WithToken(t *testing.T) {
	t.Parallel()

	bc := cache.NewBalanceCache()

	// Pre-populate with USDT token entry
	bc.Set(cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  "0xABC",
		Token:    "USDT",
		Balance:  "500.0",
		Symbol:   "USDT",
		Decimals: 6,
	})

	// Build post-send entry for token
	entry := buildPostSendEntry(bc, chain.ETH, "0xABC", "USDT", "450.0")

	// Verify token metadata was preserved
	assert.Equal(t, chain.ETH, entry.Chain)
	assert.Equal(t, "0xABC", entry.Address)
	assert.Equal(t, "USDT", entry.Token)
	assert.Equal(t, "450.0", entry.Balance)
	assert.Equal(t, "USDT", entry.Symbol)
	assert.Equal(t, 6, entry.Decimals)
	assert.Empty(t, entry.Unconfirmed)
}

// TestBuildPostSendEntry_ClearsUnconfirmed tests that unconfirmed balance is cleared.
func TestBuildPostSendEntry_ClearsUnconfirmed(t *testing.T) {
	t.Parallel()

	bc := cache.NewBalanceCache()

	// Pre-populate with entry that has unconfirmed balance
	bc.Set(cache.BalanceCacheEntry{
		Chain:       chain.BSV,
		Address:     "1ABC",
		Balance:     "1.0",
		Unconfirmed: "0.5",
		Symbol:      "BSV",
		Decimals:    8,
	})

	// Build post-send entry
	entry := buildPostSendEntry(bc, chain.BSV, "1ABC", "", "0.5")

	// Verify unconfirmed was cleared
	assert.Equal(t, "0.5", entry.Balance)
	assert.Empty(t, entry.Unconfirmed, "unconfirmed should be cleared after send")
	assert.Equal(t, "BSV", entry.Symbol)
}

// TestInvalidateBalanceCache_MultipleChains tests handling different chains independently.
func TestInvalidateBalanceCache_MultipleChains(t *testing.T) {
	t.Parallel()

	provider := newMockCacheProvider()
	logger := newMockLogWriter()

	// Pre-populate with entries for different chains
	provider.balanceCache.Set(cache.BalanceCacheEntry{
		Chain:   chain.BSV,
		Address: "1ABC",
		Balance: "1.0",
	})
	provider.balanceCache.Set(cache.BalanceCacheEntry{
		Chain:   chain.ETH,
		Address: "0xABC",
		Balance: "5.0",
	})

	// Update BSV entry
	invalidateBalanceCache(logger, provider, chain.BSV, "1ABC", "", "0.5")

	// Verify BSV entry updated
	bsvEntry, exists, _ := provider.balanceCache.Get(chain.BSV, "1ABC", "")
	require.True(t, exists)
	assert.Equal(t, "0.5", bsvEntry.Balance)

	// Verify ETH entry unchanged
	ethEntry, exists, _ := provider.balanceCache.Get(chain.ETH, "0xABC", "")
	require.True(t, exists)
	assert.Equal(t, "5.0", ethEntry.Balance)
	assert.Empty(t, logger.errorMessages)
}
