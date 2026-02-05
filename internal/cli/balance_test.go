package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/wallet"
)

// mockConfigProvider implements ConfigProvider for testing.
type mockConfigProvider struct {
	home         string
	ethRPC       string
	fallbackRPCs []string
	bsvAPIKey    string
	logLevel     string
	logFile      string
	outputFormat string
	verbose      bool
	security     config.SecurityConfig
}

func (m *mockConfigProvider) GetHome() string                    { return m.home }
func (m *mockConfigProvider) GetETHRPC() string                  { return m.ethRPC }
func (m *mockConfigProvider) GetETHFallbackRPCs() []string       { return m.fallbackRPCs }
func (m *mockConfigProvider) GetBSVAPIKey() string               { return m.bsvAPIKey }
func (m *mockConfigProvider) GetLoggingLevel() string            { return m.logLevel }
func (m *mockConfigProvider) GetLoggingFile() string             { return m.logFile }
func (m *mockConfigProvider) GetOutputFormat() string            { return m.outputFormat }
func (m *mockConfigProvider) IsVerbose() bool                    { return m.verbose }
func (m *mockConfigProvider) GetSecurity() config.SecurityConfig { return m.security }

func TestFormatCacheAge(t *testing.T) {
	tests := []struct {
		name     string
		age      time.Duration
		expected string
	}{
		{
			name:     "seconds ago",
			age:      30 * time.Second,
			expected: "30s ago",
		},
		{
			name:     "minutes ago",
			age:      5 * time.Minute,
			expected: "5m ago",
		},
		{
			name:     "hours ago",
			age:      3 * time.Hour,
			expected: "3h ago",
		},
		{
			name:     "days ago",
			age:      48 * time.Hour,
			expected: "2d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timestamp := time.Now().Add(-tt.age)
			result := formatCacheAge(timestamp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOutputBalanceText(t *testing.T) {
	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances: []BalanceResult{
			{
				Chain:    "eth",
				Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
				Balance:  "1.5",
				Symbol:   "ETH",
				Decimals: 18,
			},
			{
				Chain:    "bsv",
				Address:  "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
				Balance:  "0.1",
				Symbol:   "BSV",
				Decimals: 8,
			},
		},
	}

	var buf bytes.Buffer
	outputBalanceText(&buf, response)

	output := buf.String()
	assert.Contains(t, output, "test")
	assert.Contains(t, output, "ETH")
	assert.Contains(t, output, "BSV")
	assert.Contains(t, output, "1.5")
	assert.Contains(t, output, "0.1")
}

func TestOutputBalanceJSON(t *testing.T) {
	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances: []BalanceResult{
			{
				Chain:    "eth",
				Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
				Balance:  "1.5",
				Symbol:   "ETH",
				Decimals: 18,
			},
		},
	}

	var buf bytes.Buffer
	outputBalanceJSON(&buf, response)

	output := buf.String()
	assert.Contains(t, output, `"wallet": "test"`)
	assert.Contains(t, output, `"chain": "eth"`)
	assert.Contains(t, output, `"balance": "1.5"`)
	assert.Contains(t, output, `"symbol": "ETH"`)
}

func TestOutputBalanceTextWithStaleData(t *testing.T) {
	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Warning:   "Some balances could not be fetched",
		Balances: []BalanceResult{
			{
				Chain:    "eth",
				Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
				Balance:  "1.5",
				Symbol:   "ETH",
				Decimals: 18,
				Stale:    true,
				CacheAge: "5m ago",
			},
		},
	}

	var buf bytes.Buffer
	outputBalanceText(&buf, response)

	output := buf.String()
	assert.Contains(t, output, "Warning")
	assert.Contains(t, output, "1.5 *")
	assert.Contains(t, output, "Cached data")
}

func TestGetCachedETHBalances(t *testing.T) {
	balanceCache := cache.NewBalanceCache()

	// Set up cached ETH balance
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  "0x123",
		Balance:  "1.5",
		Symbol:   "ETH",
		Decimals: 18,
	})

	entries, stale, err := getCachedETHBalances("0x123", balanceCache)

	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.False(t, stale)
	assert.Equal(t, "1.5", entries[0].Balance)
}

func TestGetCachedBSVBalances(t *testing.T) {
	balanceCache := cache.NewBalanceCache()

	// Set up cached BSV balance
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.BSV,
		Address:  "1abc",
		Balance:  "0.5",
		Symbol:   "BSV",
		Decimals: 8,
	})

	entries, stale, err := getCachedBSVBalances("1abc", balanceCache)

	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.False(t, stale)
	assert.Equal(t, "0.5", entries[0].Balance)
}

func TestGetCachedBalancesNotFound(t *testing.T) {
	balanceCache := cache.NewBalanceCache()

	_, _, err := getCachedETHBalances("nonexistent", balanceCache)
	require.Error(t, err)

	_, _, err = getCachedBSVBalances("nonexistent", balanceCache)
	require.Error(t, err)
}

func TestBalanceCacheIntegration(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "balance-cli-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cachePath := filepath.Join(tmpDir, "cache", "balances.json")
	cacheStorage := cache.NewFileStorage(cachePath)

	// Create and save cache
	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  "0x123",
		Balance:  "1.0",
		Symbol:   "ETH",
		Decimals: 18,
	})

	err = cacheStorage.Save(balanceCache)
	require.NoError(t, err)

	// Load and verify
	loaded, err := cacheStorage.Load()
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.Size())

	entry, exists, _ := loaded.Get(chain.ETH, "0x123", "")
	require.True(t, exists)
	assert.Equal(t, "1.0", entry.Balance)
}

// TestConnectETHClient_EmptyURL tests that connectETHClient returns error for empty URL.
func TestConnectETHClient_EmptyURL(t *testing.T) {
	t.Parallel()

	// Test with empty primary and no fallbacks
	_, err := connectETHClient("", nil)
	require.Error(t, err)

	// Test with empty primary and empty fallbacks
	_, err = connectETHClient("", []string{""})
	require.Error(t, err)
}

// TestConnectETHClient_ValidURL tests that connectETHClient returns client for valid URL.
func TestConnectETHClient_ValidURL(t *testing.T) {
	t.Parallel()

	// Test with valid-looking primary URL (actual connection tested elsewhere)
	client, err := connectETHClient("https://example.com", nil)
	require.NoError(t, err)
	require.NotNil(t, client)
	client.Close()
}

// TestConnectETHClient_FallbackToValid tests fallback when primary is empty.
func TestConnectETHClient_FallbackToValid(t *testing.T) {
	t.Parallel()

	// Test with empty primary but valid fallback
	client, err := connectETHClient("", []string{"https://fallback.example.com"})
	require.NoError(t, err)
	require.NotNil(t, client)
	client.Close()
}

// TestFetchETHBalances_NoRPC tests fetchETHBalances with no RPC configured.
func TestFetchETHBalances_NoRPC(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigProvider{
		ethRPC:       "",
		fallbackRPCs: nil,
	}

	balanceCache := cache.NewBalanceCache()
	ctx := context.Background()

	// Should return error when RPC is not configured and no cache
	_, _, err := fetchETHBalances(ctx, "0x123", balanceCache, cfg)
	require.Error(t, err)
}

// TestFetchETHBalances_NoRPC_WithCache tests fetchETHBalances returns cached data when RPC empty.
func TestFetchETHBalances_NoRPC_WithCache(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigProvider{
		ethRPC:       "",
		fallbackRPCs: nil,
	}

	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  "0x123",
		Balance:  "2.5",
		Symbol:   "ETH",
		Decimals: 18,
	})
	ctx := context.Background()

	// Should return cached data when RPC is not configured
	entries, stale, err := fetchETHBalances(ctx, "0x123", balanceCache, cfg)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "2.5", entries[0].Balance)
	// Recent cache shouldn't be stale
	assert.False(t, stale)
}

// TestFetchETHBalances_InvalidRPCWithFallback tests fallback on connection failure.
func TestFetchETHBalances_InvalidRPCWithFallback(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigProvider{
		ethRPC:       "invalid://primary",
		fallbackRPCs: []string{"also-invalid://fallback1", "still-invalid://fallback2"},
	}

	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  "0x456",
		Balance:  "3.0",
		Symbol:   "ETH",
		Decimals: 18,
	})
	ctx := context.Background()

	// All RPCs are invalid, so should fall back to cache
	entries, stale, err := fetchETHBalances(ctx, "0x456", balanceCache, cfg)
	// Cache should be returned even if err is non-nil
	require.Len(t, entries, 1)
	assert.Equal(t, "3.0", entries[0].Balance)
	// With connection error, stale depends on cache age
	_ = stale
	_ = err
}

// TestFetchETHBalances_InvalidRPCNoFallbackNoCache tests error when everything fails.
func TestFetchETHBalances_InvalidRPCNoFallbackNoCache(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigProvider{
		ethRPC:       "invalid://primary",
		fallbackRPCs: nil,
	}

	balanceCache := cache.NewBalanceCache()
	ctx := context.Background()

	// No valid RPC, no fallbacks, no cache - should return error
	entries, stale, err := fetchETHBalances(ctx, "0x789", balanceCache, cfg)
	// Should get an error since nothing worked and no cache
	require.Error(t, err)
	assert.Empty(t, entries)
	assert.True(t, stale)
}

// TestMockConfigProvider_Interface verifies mockConfigProvider implements ConfigProvider.
func TestMockConfigProvider_Interface(t *testing.T) {
	t.Parallel()

	mock := &mockConfigProvider{
		home:         "/test/home",
		ethRPC:       "https://test-rpc.example.com",
		fallbackRPCs: []string{"https://fallback1.example.com", "https://fallback2.example.com"},
		bsvAPIKey:    "test-key",
		logLevel:     "debug",
		logFile:      "/test/log.txt",
		outputFormat: "json",
		verbose:      true,
		security: config.SecurityConfig{
			SessionEnabled:    true,
			SessionTTLMinutes: 30,
		},
	}

	// Verify all interface methods work
	var cfg ConfigProvider = mock
	assert.Equal(t, "/test/home", cfg.GetHome())
	assert.Equal(t, "https://test-rpc.example.com", cfg.GetETHRPC())
	assert.Len(t, cfg.GetETHFallbackRPCs(), 2)
	assert.Equal(t, "https://fallback1.example.com", cfg.GetETHFallbackRPCs()[0])
	assert.Equal(t, "test-key", cfg.GetBSVAPIKey())
	assert.Equal(t, "debug", cfg.GetLoggingLevel())
	assert.Equal(t, "/test/log.txt", cfg.GetLoggingFile())
	assert.Equal(t, "json", cfg.GetOutputFormat())
	assert.True(t, cfg.IsVerbose())
	assert.True(t, cfg.GetSecurity().SessionEnabled)
	assert.Equal(t, 30, cfg.GetSecurity().SessionTTLMinutes)
}

// TestFetchETHBalances_FallbackRPCOrder tests that fallbacks are tried in order.
func TestFetchETHBalances_FallbackRPCOrder(t *testing.T) {
	t.Parallel()

	// This test verifies that when primary fails, fallbacks are attempted
	// We can't easily test successful fallback without a mock server,
	// but we can verify the function handles multiple fallback failures gracefully

	cfg := &mockConfigProvider{
		ethRPC: "invalid://primary",
		fallbackRPCs: []string{
			"invalid://fallback1",
			"invalid://fallback2",
			"invalid://fallback3",
		},
	}

	balanceCache := cache.NewBalanceCache()
	ctx := context.Background()

	// All fail, should get an error
	_, stale, err := fetchETHBalances(ctx, "0xabc", balanceCache, cfg)
	require.Error(t, err)
	assert.True(t, stale) // Should be stale since we couldn't fetch
}

func TestGetChainSymbol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		chainID  wallet.ChainID
		expected string
	}{
		{
			name:     "ETH chain",
			chainID:  wallet.ChainETH,
			expected: "ETH",
		},
		{
			name:     "BSV chain",
			chainID:  wallet.ChainBSV,
			expected: "BSV",
		},
		{
			name:     "BTC chain",
			chainID:  wallet.ChainBTC,
			expected: "BTC",
		},
		{
			name:     "BCH chain",
			chainID:  wallet.ChainBCH,
			expected: "BCH",
		},
		{
			name:     "unknown chain",
			chainID:  wallet.ChainID("unknown"),
			expected: "???",
		},
		{
			name:     "empty chain",
			chainID:  wallet.ChainID(""),
			expected: "???",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := getChainSymbol(tc.chainID)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestOutputBalanceJSON_TokenAndStale(t *testing.T) {
	t.Parallel()

	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Warning:   "Some balances could not be fetched",
		Balances: []BalanceResult{
			{
				Chain:    "eth",
				Address:  "0x123",
				Balance:  "100.0",
				Symbol:   "USDC",
				Token:    "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
				Decimals: 6,
				Stale:    true,
				CacheAge: "5m ago",
			},
		},
	}

	var buf bytes.Buffer
	outputBalanceJSON(&buf, response)
	out := buf.String()

	assert.Contains(t, out, `"token": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"`)
	assert.Contains(t, out, `"stale": true`)
	assert.Contains(t, out, `"cache_age": "5m ago"`)
	assert.Contains(t, out, `"warning":`)
}

func TestOutputBalanceText_Empty(t *testing.T) {
	t.Parallel()

	response := BalanceShowResponse{
		Wallet:    "empty",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances:  nil,
	}

	var buf bytes.Buffer
	outputBalanceText(&buf, response)
	out := buf.String()

	assert.Contains(t, out, "No balances found")
}

func TestFetchBalancesForAddress_BTC_ReturnsNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	balanceCache := cache.NewBalanceCache()
	cfg := &mockConfigProvider{}

	entries, stale, err := fetchBalancesForAddress(ctx, wallet.ChainBTC, "1btcaddr", balanceCache, cfg)
	assert.Nil(t, entries)
	assert.False(t, stale)
	assert.NoError(t, err)
}

func TestFetchBalancesForAddress_BCH_ReturnsNil(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	balanceCache := cache.NewBalanceCache()
	cfg := &mockConfigProvider{}

	entries, stale, err := fetchBalancesForAddress(ctx, wallet.ChainBCH, "1bchaddr", balanceCache, cfg)
	assert.Nil(t, entries)
	assert.False(t, stale)
	assert.NoError(t, err)
}

// TestFetchBSVBalances_NoAPIKey_WithCache tests that fetchBSVBalances returns cached data
// when the API call fails (e.g. no API key configured) but cache is populated.
func TestFetchBSVBalances_NoAPIKey_WithCache(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.BSV,
		Address:  "1BSVTestAddr",
		Balance:  "0.5",
		Symbol:   "BSV",
		Decimals: 8,
	})
	ctx := context.Background()

	// fetchBSVBalances will fail to connect (no real API), so it should fall back to cache
	entries, stale, err := fetchBSVBalances(ctx, "1BSVTestAddr", balanceCache)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "0.5", entries[0].Balance)
	assert.False(t, stale)
}

// TestFetchBSVBalances_NoAPIKey_NoCache tests that fetchBSVBalances returns error
// when API call fails and no cache exists.
func TestFetchBSVBalances_NoAPIKey_NoCache(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	ctx := context.Background()

	// No cache, API will fail → should return error
	_, _, err := fetchBSVBalances(ctx, "1BSVNonexistent", balanceCache)
	require.Error(t, err)
}

// TestFetchBalancesForAddress_ETH_Direct tests the ETH path through fetchBalancesForAddress.
func TestFetchBalancesForAddress_ETH_Direct(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	balanceCache := cache.NewBalanceCache()
	cfg := &mockConfigProvider{ethRPC: ""}

	// ETH with no RPC and no cache should return error
	entries, stale, err := fetchBalancesForAddress(ctx, wallet.ChainETH, "0xTestAddr", balanceCache, cfg)
	require.Error(t, err)
	assert.Empty(t, entries)
	assert.True(t, stale)
}

// TestFetchBalancesForAddress_BSV_Direct tests the BSV path through fetchBalancesForAddress.
func TestFetchBalancesForAddress_BSV_Direct(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	balanceCache := cache.NewBalanceCache()
	cfg := &mockConfigProvider{}

	// BSV with no API and no cache should return error
	_, _, err := fetchBalancesForAddress(ctx, wallet.ChainBSV, "1BSVTestAddr", balanceCache, cfg)
	require.Error(t, err)
}
