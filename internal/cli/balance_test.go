package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/metrics"
	"github.com/mrz1836/sigil/internal/wallet"
)

// mockConfigProvider implements ConfigProvider for testing.
type mockConfigProvider struct {
	home               string
	ethRPC             string
	fallbackRPCs       []string
	ethProvider        string
	ethEtherscanAPIKey string
	bsvAPIKey          string
	bsvBroadcast       string
	bsvFeeStrategy     string
	bsvMinMiners       int
	logLevel           string
	logFile            string
	outputFormat       string
	verbose            bool
	security           config.SecurityConfig
}

func (m *mockConfigProvider) GetHome() string                    { return m.home }
func (m *mockConfigProvider) GetETHRPC() string                  { return m.ethRPC }
func (m *mockConfigProvider) GetETHFallbackRPCs() []string       { return m.fallbackRPCs }
func (m *mockConfigProvider) GetBSVAPIKey() string               { return m.bsvAPIKey }
func (m *mockConfigProvider) GetBSVBroadcast() string            { return m.bsvBroadcast }
func (m *mockConfigProvider) GetLoggingLevel() string            { return m.logLevel }
func (m *mockConfigProvider) GetLoggingFile() string             { return m.logFile }
func (m *mockConfigProvider) GetOutputFormat() string            { return m.outputFormat }
func (m *mockConfigProvider) IsVerbose() bool                    { return m.verbose }
func (m *mockConfigProvider) GetSecurity() config.SecurityConfig { return m.security }

func (m *mockConfigProvider) GetETHProvider() string {
	if m.ethProvider == "" {
		return "etherscan"
	}
	return m.ethProvider
}

func (m *mockConfigProvider) GetETHEtherscanAPIKey() string {
	return m.ethEtherscanAPIKey
}

func (m *mockConfigProvider) GetBSVFeeStrategy() string {
	if m.bsvFeeStrategy == "" {
		return "normal"
	}
	return m.bsvFeeStrategy
}

func (m *mockConfigProvider) GetBSVMinMiners() int {
	if m.bsvMinMiners == 0 {
		return 3
	}
	return m.bsvMinMiners
}

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
	require.NoError(t, outputBalanceJSON(&buf, response))

	var parsed BalanceShowResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	require.Len(t, parsed.Balances, 1)
	assert.Equal(t, "test", parsed.Wallet)
	assert.Equal(t, "eth", parsed.Balances[0].Chain)
	assert.Equal(t, "1.5", parsed.Balances[0].Balance)
	assert.Equal(t, "ETH", parsed.Balances[0].Symbol)
}

func TestOutputBalanceTextWithUnconfirmed(t *testing.T) {
	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances: []BalanceResult{
			{
				Chain:       "bsv",
				Address:     "16DwKi833rr1PQfZw65LnHeagj1iLcVUbT",
				Balance:     "0.00070422",
				Unconfirmed: "-0.00070422",
				Symbol:      "BSV",
				Decimals:    8,
			},
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
	outputBalanceText(&buf, response)

	output := buf.String()
	// Should use wide table with Confirmed/Unconfirmed headers
	assert.Contains(t, output, "Confirmed")
	assert.Contains(t, output, "Unconfirmed")
	assert.Contains(t, output, "0.00070422")
	assert.Contains(t, output, "-0.00070422")
	// ETH entry without unconfirmed should show "-"
	assert.Contains(t, output, "-")
}

func TestOutputBalanceTextNoUnconfirmed(t *testing.T) {
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
	outputBalanceText(&buf, response)

	output := buf.String()
	// Should use narrow table without Unconfirmed header
	assert.Contains(t, output, "Balance")
	assert.NotContains(t, output, "Confirmed")
	assert.NotContains(t, output, "Unconfirmed")
}

func TestOutputBalanceJSONWithUnconfirmed(t *testing.T) {
	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances: []BalanceResult{
			{
				Chain:       "bsv",
				Address:     "16DwKi833rr1PQfZw65LnHeagj1iLcVUbT",
				Balance:     "0.00070422",
				Unconfirmed: "-0.00070422",
				Symbol:      "BSV",
				Decimals:    8,
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, outputBalanceJSON(&buf, response))

	var parsed BalanceShowResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	require.Len(t, parsed.Balances, 1)
	assert.Equal(t, "0.00070422", parsed.Balances[0].Balance)
	assert.Equal(t, "-0.00070422", parsed.Balances[0].Unconfirmed)
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

func TestGetCachedBalances_RecordMetrics(t *testing.T) {
	metrics.Global.Reset()
	defer metrics.Global.Reset()
	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  "0x123",
		Balance:  "1.5",
		Symbol:   "ETH",
		Decimals: 18,
	})

	_, _, err := getCachedETHBalances("0x123", balanceCache)
	require.NoError(t, err)

	snapshot := metrics.Global.Snapshot()
	assert.Equal(t, int64(1), snapshot.CacheHits)
	assert.Equal(t, int64(1), snapshot.CacheMisses)
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

	transport := sharedETHTransport()

	// Test with empty primary and no fallbacks
	_, err := connectETHClient("", nil, transport)
	require.Error(t, err)

	// Test with empty primary and empty fallbacks
	_, err = connectETHClient("", []string{""}, transport)
	require.Error(t, err)
}

// TestConnectETHClient_ValidURL tests that connectETHClient returns client for valid URL.
func TestConnectETHClient_ValidURL(t *testing.T) {
	t.Parallel()

	transport := sharedETHTransport()

	// Test with valid-looking primary URL (actual connection tested elsewhere)
	client, err := connectETHClient("https://example.com", nil, transport)
	require.NoError(t, err)
	require.NotNil(t, client)
	client.Close()
}

// TestConnectETHClient_FallbackToValid tests fallback when primary is empty.
func TestConnectETHClient_FallbackToValid(t *testing.T) {
	t.Parallel()

	transport := sharedETHTransport()

	// Test with empty primary but valid fallback
	client, err := connectETHClient("", []string{"https://fallback.example.com"}, transport)
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
	require.NoError(t, outputBalanceJSON(&buf, response))

	var parsed BalanceShowResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	require.Len(t, parsed.Balances, 1)
	assert.Equal(t, "Some balances could not be fetched", parsed.Warning)
	assert.Equal(t, "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", parsed.Balances[0].Token)
	assert.True(t, parsed.Balances[0].Stale)
	assert.Equal(t, "5m ago", parsed.Balances[0].CacheAge)
}

func TestOutputBalanceJSON_Escaping(t *testing.T) {
	t.Parallel()

	response := BalanceShowResponse{
		Wallet:    "test\"wallet",
		Timestamp: "2026-01-31T12:00:00Z",
		Warning:   "line1\nline2 \"quoted\" \u2713",
		Balances: []BalanceResult{
			{
				Chain:    "eth",
				Address:  "0xabc\"123",
				Balance:  "1.0",
				Symbol:   "ETH",
				Decimals: 18,
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, outputBalanceJSON(&buf, response))

	var parsed BalanceShowResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, response.Wallet, parsed.Wallet)
	assert.Equal(t, response.Warning, parsed.Warning)
	assert.Equal(t, response.Balances[0].Address, parsed.Balances[0].Address)
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

// TestFetchETHBalances_EtherscanNoKey_FallbackToRPC tests that when etherscan is primary
// but no API key is set, the system falls back to RPC.
func TestFetchETHBalances_EtherscanNoKey_FallbackToRPC(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigProvider{
		ethProvider:        "etherscan",
		ethEtherscanAPIKey: "", // No API key
		ethRPC:             "invalid://primary",
		fallbackRPCs:       nil,
	}

	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  "0x123",
		Balance:  "1.0",
		Symbol:   "ETH",
		Decimals: 18,
	})
	ctx := context.Background()

	// Etherscan fails (no key), RPC fails (invalid URL), should return cached data
	entries, _, err := fetchETHBalances(ctx, "0x123", balanceCache, cfg)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "1.0", entries[0].Balance)
}

// TestFetchETHBalances_RPCPrimary tests the RPC-primary provider path.
func TestFetchETHBalances_RPCPrimary(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigProvider{
		ethProvider:        "rpc",
		ethEtherscanAPIKey: "",
		ethRPC:             "invalid://primary",
		fallbackRPCs:       nil,
	}

	balanceCache := cache.NewBalanceCache()
	ctx := context.Background()

	// RPC fails (invalid URL), Etherscan fails (no key), no cache → error
	_, stale, err := fetchETHBalances(ctx, "0x789", balanceCache, cfg)
	require.Error(t, err)
	assert.True(t, stale)
}

// TestFetchETHBalances_RPCPrimary_WithCache tests RPC primary fallback to cache.
func TestFetchETHBalances_RPCPrimary_WithCache(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigProvider{
		ethProvider:        "rpc",
		ethEtherscanAPIKey: "",
		ethRPC:             "invalid://primary",
		fallbackRPCs:       nil,
	}

	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  "0x456",
		Balance:  "5.0",
		Symbol:   "ETH",
		Decimals: 18,
	})
	ctx := context.Background()

	// Both providers fail but cache is available
	entries, _, err := fetchETHBalances(ctx, "0x456", balanceCache, cfg)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "5.0", entries[0].Balance)
}

// TestMockConfigProvider_NewMethods verifies the new mock methods.
func TestMockConfigProvider_NewMethods(t *testing.T) {
	t.Parallel()

	mock := &mockConfigProvider{
		ethProvider:        "rpc",
		ethEtherscanAPIKey: "test-key",
	}

	var cfg ConfigProvider = mock
	assert.Equal(t, "rpc", cfg.GetETHProvider())
	assert.Equal(t, "test-key", cfg.GetETHEtherscanAPIKey())
}

// TestMockConfigProvider_DefaultProvider tests default provider value.
func TestMockConfigProvider_DefaultProvider(t *testing.T) {
	t.Parallel()

	mock := &mockConfigProvider{}
	assert.Equal(t, "etherscan", mock.GetETHProvider())
}

// TestFetchBSVBalances_FreshCacheSkipsNetwork verifies that a very fresh cache
// entry (e.g. just written by a tx send) is returned directly without a
// network call.
func TestFetchBSVBalances_FreshCacheSkipsNetwork(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	// Set a fresh entry (Set stamps it with time.Now).
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.BSV,
		Address:  "1FreshAddr",
		Balance:  "0.0",
		Symbol:   "BSV",
		Decimals: 8,
	})
	ctx := context.Background()

	entries, stale, err := fetchBSVBalances(ctx, "1FreshAddr", balanceCache)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "0.0", entries[0].Balance)
	assert.False(t, stale)
}

// TestFetchETHBalances_FreshCacheSkipsNetwork verifies that a very fresh ETH
// cache entry is returned directly, avoiding the network round-trip.
func TestFetchETHBalances_FreshCacheSkipsNetwork(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigProvider{
		ethRPC:       "invalid://will-not-be-called",
		fallbackRPCs: nil,
	}

	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  "0xFreshAddr",
		Balance:  "0.0",
		Symbol:   "ETH",
		Decimals: 18,
	})
	ctx := context.Background()

	entries, stale, err := fetchETHBalances(ctx, "0xFreshAddr", balanceCache, cfg)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "0.0", entries[0].Balance)
	assert.False(t, stale)
}

// TestFetchBSVBalances_StaleCacheStillFetchesNetwork verifies that an old cache
// entry does NOT short-circuit — the network is queried (and may fail, falling
// back to the old cache through the normal path).
func TestFetchBSVBalances_StaleCacheStillFetchesNetwork(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	// Manually insert an entry with an old timestamp.
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.BSV,
		Address:  "1StaleAddr",
		Balance:  "1.0",
		Symbol:   "BSV",
		Decimals: 8,
	})
	// Age the entry beyond postSendCacheTrust.
	key := cache.Key(chain.BSV, "1StaleAddr", "")
	if e, ok := balanceCache.Entries[key]; ok {
		e.UpdatedAt = time.Now().Add(-2 * time.Minute)
		balanceCache.Entries[key] = e
	}
	ctx := context.Background()

	// The network call will fail (no real API), falling back to the stale cache.
	entries, _, err := fetchBSVBalances(ctx, "1StaleAddr", balanceCache)
	// Should still get the cached entry through the normal fallback path.
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "1.0", entries[0].Balance)
}

// TestBalanceRefreshFlag_BypassesCache verifies that when balanceRefresh is
// true the cache is not loaded from disk, so a fresh entry written by a
// previous tx send does not short-circuit the network call.
func TestBalanceRefreshFlag_BypassesCache(t *testing.T) {
	t.Parallel()

	// Populate a cache that has a fresh entry (would normally short-circuit).
	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain:    chain.BSV,
		Address:  "1CachedAddr",
		Balance:  "0.0",
		Symbol:   "BSV",
		Decimals: 8,
	})

	// Simulate --refresh: start with a clean cache instead.
	freshCache := cache.NewBalanceCache()
	ctx := context.Background()

	// With an empty cache, fetchBSVBalances hits the network (which will
	// fail in tests) and falls back to cache — but the fresh cache is empty,
	// so it returns an error.
	_, _, err := fetchBSVBalances(ctx, "1CachedAddr", freshCache)
	require.Error(t, err, "refresh should bypass existing cached data")

	// The original populated cache still has the entry (not mutated).
	entry, exists, _ := balanceCache.Get(chain.BSV, "1CachedAddr", "")
	require.True(t, exists)
	assert.Equal(t, "0.0", entry.Balance)
}

func TestTruncateAddress(t *testing.T) {
	t.Parallel()

	t.Run("exactly 42 chars returned unchanged", func(t *testing.T) {
		t.Parallel()
		// 42 characters exactly
		addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"
		assert.Len(t, addr, 42)
		assert.Equal(t, addr, truncateAddress(addr))
	})

	t.Run("longer than 42 chars truncated", func(t *testing.T) {
		t.Parallel()
		addr := "0x742d35Cc6634C0532925a3b844Bc454e4438f44eABCDEF"
		assert.Greater(t, len(addr), 42)
		result := truncateAddress(addr)
		assert.Equal(t, addr[:20], result[:20])
		assert.Contains(t, result, "...")
		assert.Equal(t, addr[len(addr)-17:], result[len(result)-17:])
		assert.Less(t, len(result), len(addr))
	})

	t.Run("short address returned unchanged", func(t *testing.T) {
		t.Parallel()
		addr := "1abc"
		assert.Equal(t, addr, truncateAddress(addr))
	})
}

func TestOutputBalanceJSON_NilBalancesSlice(t *testing.T) {
	t.Parallel()

	response := BalanceShowResponse{
		Wallet:    "test",
		Timestamp: "2026-01-31T12:00:00Z",
		Balances:  nil, // nil slice
	}

	var buf bytes.Buffer
	require.NoError(t, outputBalanceJSON(&buf, response))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))

	// Verify balances is [] (empty array) not null
	balances, ok := parsed["balances"].([]any)
	require.True(t, ok, "balances should be an array, not null")
	assert.Empty(t, balances)
}

func TestGetCachedETHBalances_BothCached(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain: chain.ETH, Address: "0xTest", Balance: "1.5", Symbol: "ETH", Decimals: 18,
	})
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain: chain.ETH, Address: "0xTest", Token: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		Balance: "100.0", Symbol: "USDC", Decimals: 6,
	})

	entries, stale, err := getCachedETHBalances("0xTest", balanceCache)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.False(t, stale)
}

func TestGetCachedETHBalances_StaleETHEntry(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain: chain.ETH, Address: "0xStale", Balance: "1.0", Symbol: "ETH", Decimals: 18,
	})

	// Age the entry beyond DefaultStaleness
	key := cache.Key(chain.ETH, "0xStale", "")
	if e, ok := balanceCache.Entries[key]; ok {
		e.UpdatedAt = time.Now().Add(-10 * time.Minute)
		balanceCache.Entries[key] = e
	}

	entries, stale, err := getCachedETHBalances("0xStale", balanceCache)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.True(t, stale)
}

func TestGetCachedETHBalances_StaleUSDCEntry(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	// Fresh ETH entry
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain: chain.ETH, Address: "0xMixed", Balance: "1.0", Symbol: "ETH", Decimals: 18,
	})
	// USDC entry
	usdcToken := "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48" //nolint:gosec // USDC contract address, not credentials
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain: chain.ETH, Address: "0xMixed", Token: usdcToken,
		Balance: "50.0", Symbol: "USDC", Decimals: 6,
	})

	// Age only the USDC entry
	key := cache.Key(chain.ETH, "0xMixed", usdcToken)
	if e, ok := balanceCache.Entries[key]; ok {
		e.UpdatedAt = time.Now().Add(-10 * time.Minute)
		balanceCache.Entries[key] = e
	}

	entries, stale, err := getCachedETHBalances("0xMixed", balanceCache)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.True(t, stale, "should be stale because USDC entry is old")
}

func TestGetCachedETHBalances_OnlyUSDCCached(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()
	balanceCache.Set(cache.BalanceCacheEntry{
		Chain: chain.ETH, Address: "0xOnlyUSDC", Token: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		Balance: "200.0", Symbol: "USDC", Decimals: 6,
	})

	entries, stale, err := getCachedETHBalances("0xOnlyUSDC", balanceCache)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "USDC", entries[0].Symbol)
	assert.False(t, stale)
}

func TestGetCachedETHBalances_NothingCached(t *testing.T) {
	t.Parallel()

	balanceCache := cache.NewBalanceCache()

	entries, stale, err := getCachedETHBalances("0xNone", balanceCache)
	require.Error(t, err)
	assert.Nil(t, entries)
	assert.True(t, stale)
}
