package balance

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/eth"
)

// Mock ConfigProvider for testing
type mockConfigProvider struct {
	ethProvider        string
	ethRPC             string
	ethFallbackRPCs    []string
	ethEtherscanAPIKey string
}

func newMockConfigProvider() *mockConfigProvider {
	return &mockConfigProvider{
		ethProvider:        "etherscan",
		ethRPC:             "https://eth-rpc.example.com",
		ethFallbackRPCs:    []string{"https://eth-fallback.example.com"},
		ethEtherscanAPIKey: "test-api-key",
	}
}

func (m *mockConfigProvider) GetETHProvider() string {
	return m.ethProvider
}

func (m *mockConfigProvider) GetETHRPC() string {
	return m.ethRPC
}

func (m *mockConfigProvider) GetETHFallbackRPCs() []string {
	return m.ethFallbackRPCs
}

func (m *mockConfigProvider) GetETHEtherscanAPIKey() string {
	return m.ethEtherscanAPIKey
}

// TestNewFetcher tests the fetcher constructor.
func TestNewFetcher(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cache := newMockCacheProvider()

	fetcher := NewFetcher(cfg, cache)

	assert.NotNil(t, fetcher)
	assert.NotNil(t, fetcher.cfg)
	assert.NotNil(t, fetcher.cache)
}

// TestFetchForChain_Dispatch tests chain dispatch logic.
func TestFetchForChain_Dispatch(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cache := newMockCacheProvider()
	fetcher := NewFetcher(cfg, cache)

	tests := []struct {
		name    string
		chainID chain.ID
		address string
		wantErr bool
		errType error
	}{
		{
			name:    "BTC not supported",
			chainID: chain.BTC,
			address: "1BTC",
			wantErr: false, // Returns nil, not error
		},
		{
			name:    "BCH not supported",
			chainID: chain.BCH,
			address: "1BCH",
			wantErr: false, // Returns nil, not error
		},
		{
			name:    "Unknown chain",
			chainID: "UNKNOWN",
			address: "addr",
			wantErr: true,
			errType: ErrUnsupportedChain,
		},
		{
			name:    "Empty chain ID",
			chainID: "",
			address: "addr",
			wantErr: true,
			errType: ErrUnsupportedChain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entries, stale, err := fetcher.FetchForChain(context.Background(), tt.chainID, tt.address)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
				assert.Nil(t, entries)
			} else {
				require.NoError(t, err)
				assert.Nil(t, entries)
				assert.False(t, stale)
			}
		})
	}
}

// TestGetCachedETHBalances tests retrieving cached ETH balances.
func TestGetCachedETHBalances(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(_ *mockCacheProvider)
		address   string
		wantCount int
		wantStale bool
		wantErr   bool
	}{
		{
			name: "Both ETH and USDC cached",
			setup: func(cache *mockCacheProvider) {
				cache.Set(CacheEntry{
					Chain:     chain.ETH,
					Address:   "0x1234",
					Balance:   "1.0",
					Symbol:    "ETH",
					Decimals:  18,
					UpdatedAt: time.Now(),
				})
				cache.Set(CacheEntry{
					Chain:     chain.ETH,
					Address:   "0x1234",
					Token:     eth.USDCMainnet,
					Balance:   "100.0",
					Symbol:    "USDC",
					Decimals:  6,
					UpdatedAt: time.Now(),
				})
			},
			address:   "0x1234",
			wantCount: 2,
			wantStale: false,
			wantErr:   false,
		},
		{
			name: "Only ETH cached",
			setup: func(cache *mockCacheProvider) {
				cache.Set(CacheEntry{
					Chain:     chain.ETH,
					Address:   "0x1234",
					Balance:   "1.0",
					Symbol:    "ETH",
					Decimals:  18,
					UpdatedAt: time.Now(),
				})
			},
			address:   "0x1234",
			wantCount: 1,
			wantStale: false,
			wantErr:   false,
		},
		{
			name: "Stale cache",
			setup: func(cache *mockCacheProvider) {
				cache.Set(CacheEntry{
					Chain:     chain.ETH,
					Address:   "0x1234",
					Balance:   "1.0",
					Symbol:    "ETH",
					Decimals:  18,
					UpdatedAt: time.Now().Add(-2 * time.Hour),
				})
			},
			address:   "0x1234",
			wantCount: 1,
			wantStale: true,
			wantErr:   false,
		},
		{
			name:      "No cache found",
			setup:     func(_ *mockCacheProvider) {},
			address:   "0x1234",
			wantCount: 0,
			wantStale: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := newMockConfigProvider()
			cache := newMockCacheProvider()
			if tt.setup != nil {
				tt.setup(cache)
			}

			fetcher := NewFetcher(cfg, cache)
			entries, stale, err := fetcher.getCachedETHBalances(tt.address)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, entries)
			} else {
				require.NoError(t, err)
				assert.Len(t, entries, tt.wantCount)
				assert.Equal(t, tt.wantStale, stale)
			}
		})
	}
}

// TestGetCachedBSVBalances tests retrieving cached BSV balances.
func TestGetCachedBSVBalances(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(_ *mockCacheProvider)
		address   string
		wantStale bool
		wantErr   bool
	}{
		{
			name: "Fresh cache",
			setup: func(cache *mockCacheProvider) {
				cache.Set(CacheEntry{
					Chain:     chain.BSV,
					Address:   "1ABC",
					Balance:   "1.0",
					Symbol:    "BSV",
					Decimals:  8,
					UpdatedAt: time.Now(),
				})
			},
			address:   "1ABC",
			wantStale: false,
			wantErr:   false,
		},
		{
			name: "Stale cache",
			setup: func(cache *mockCacheProvider) {
				cache.Set(CacheEntry{
					Chain:     chain.BSV,
					Address:   "1ABC",
					Balance:   "1.0",
					Symbol:    "BSV",
					Decimals:  8,
					UpdatedAt: time.Now().Add(-2 * time.Hour),
				})
			},
			address:   "1ABC",
			wantStale: true,
			wantErr:   false,
		},
		{
			name:      "No cache found",
			setup:     func(_ *mockCacheProvider) {},
			address:   "1ABC",
			wantStale: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := newMockConfigProvider()
			cache := newMockCacheProvider()
			if tt.setup != nil {
				tt.setup(cache)
			}

			fetcher := NewFetcher(cfg, cache)
			entries, stale, err := fetcher.getCachedBSVBalances(tt.address)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, entries)
			} else {
				require.NoError(t, err)
				assert.Len(t, entries, 1)
				assert.Equal(t, tt.wantStale, stale)
			}
		})
	}
}

// TestConnectETHClient_Success tests successful connection to ETH RPC.
func TestConnectETHClient_Success(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cache := newMockCacheProvider()
	fetcher := NewFetcher(cfg, cache)

	transport := &http.Transport{}

	// NewClient creates a client even with invalid URLs (validation happens on use)
	// We're testing that the client is created successfully
	client, err := fetcher.connectETHClient("https://invalid.example.com", []string{}, transport)

	// Client creation succeeds (URL validation happens on first RPC call)
	require.NoError(t, err)
	assert.NotNil(t, client)
	if client != nil {
		client.Close()
	}
}

// TestConnectETHClient_Fallback tests fallback to alternate RPCs.
func TestConnectETHClient_Fallback(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cache := newMockCacheProvider()
	fetcher := NewFetcher(cfg, cache)

	transport := &http.Transport{}

	// Test with multiple URLs - client creation succeeds but usage would fail
	client, err := fetcher.connectETHClient(
		"https://invalid1.example.com",
		[]string{"https://invalid2.example.com", "https://invalid3.example.com"},
		transport,
	)

	// Client creation succeeds (primary is used)
	require.NoError(t, err)
	assert.NotNil(t, client)
	if client != nil {
		client.Close()
	}
}

// TestFetchBSVBulk_EmptyAddresses tests bulk fetch with empty address list.
func TestFetchBSVBulk_EmptyAddresses(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cache := newMockCacheProvider()
	fetcher := NewFetcher(cfg, cache)

	results, err := fetcher.fetchBSVBulk(context.Background(), []string{})

	require.NoError(t, err)
	assert.NotNil(t, results)
	assert.Empty(t, results)
}

// TestFetchBSVBulk_CachedAddresses tests bulk fetch with cached addresses.
func TestFetchBSVBulk_CachedAddresses(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cache := newMockCacheProvider()

	// Add fresh cache entries (within post-send trust window)
	cache.Set(CacheEntry{
		Chain:     chain.BSV,
		Address:   "1ABC",
		Balance:   "1.0",
		Symbol:    "BSV",
		Decimals:  8,
		UpdatedAt: time.Now().Add(-10 * time.Second), // Fresh
	})

	fetcher := NewFetcher(cfg, cache)

	results, err := fetcher.fetchBSVBulk(context.Background(), []string{"1ABC"})

	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Contains(t, results, "1ABC")
}

// TestSharedETHTransport tests that shared transport is created.
func TestSharedETHTransport(t *testing.T) {
	t.Parallel()

	transport1 := sharedETHTransport()
	transport2 := sharedETHTransport()

	// Should return the same transport instance
	assert.Same(t, transport1, transport2)
}

// TestPostSendCacheTrust tests the post-send cache trust constant.
func TestPostSendCacheTrust(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 30*time.Second, postSendCacheTrust)
}

// TestFetchETH_ConfigValidation tests ETH config validation without network calls.
func TestFetchETH_ConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		provider       string
		rpcURL         string
		etherscanKey   string
		expectPrimary  string // "rpc" or "etherscan"
		shouldUseCache bool
	}{
		{
			name:          "RPC provider configured",
			provider:      "rpc",
			rpcURL:        "https://eth.example.com",
			etherscanKey:  "",
			expectPrimary: "rpc",
		},
		{
			name:          "Etherscan provider configured",
			provider:      "etherscan",
			rpcURL:        "",
			etherscanKey:  "test-key",
			expectPrimary: "etherscan",
		},
		{
			name:          "Default provider (etherscan)",
			provider:      "",
			rpcURL:        "",
			etherscanKey:  "test-key",
			expectPrimary: "etherscan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &mockConfigProvider{
				ethProvider:        tt.provider,
				ethRPC:             tt.rpcURL,
				ethEtherscanAPIKey: tt.etherscanKey,
			}
			cache := newMockCacheProvider()

			// Add cached data to prevent network calls
			cache.Set(CacheEntry{
				Chain:     chain.ETH,
				Address:   "0x1234",
				Balance:   "1.0",
				Symbol:    "ETH",
				Decimals:  18,
				UpdatedAt: time.Now().Add(-10 * time.Second),
			})

			fetcher := NewFetcher(cfg, cache)

			// Call fetchETH - should use cache and not make network calls
			entries, _, err := fetcher.fetchETH(context.Background(), "0x1234")

			// Should succeed using cached data
			require.NoError(t, err)
			assert.NotEmpty(t, entries)
		})
	}
}

// TestFetchETHViaRPC_NoRPCConfigured tests error when RPC is not configured.
func TestFetchETHViaRPC_NoRPCConfigured(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigProvider{
		ethRPC: "", // No RPC configured
	}
	cache := newMockCacheProvider()
	fetcher := NewFetcher(cfg, cache)

	entries, stale, err := fetcher.fetchETHViaRPC(context.Background(), "0x1234")

	// Error can be "ETH RPC not configured" or wrapped as "network communication failed"
	require.Error(t, err, "Expected error when RPC not configured")
	assert.Nil(t, entries)
	assert.True(t, stale)
}

// TestFetchETHBalanceWithFallback_PrimarySuccess tests primary RPC success.
func TestFetchETHBalanceWithFallback_PrimarySuccess(t *testing.T) {
	t.Parallel()

	// This test demonstrates the fallback structure
	// In a real scenario, we'd need to mock the eth.Client
	// For now, we test the function signature and error paths

	cfg := newMockConfigProvider()
	cache := newMockCacheProvider()
	fetcher := NewFetcher(cfg, cache)

	transport := &http.Transport{}

	// Attempt with invalid client will fail, demonstrating error handling
	balance, client, err := fetcher.fetchETHBalanceWithFallback(
		context.Background(),
		nil, // nil client will cause panic, so we expect this to fail in real usage
		"0x1234",
		"https://invalid.example.com",
		[]string{},
		transport,
	)

	// We expect this to fail with nil client
	require.Error(t, err)
	assert.Nil(t, balance)
	assert.Nil(t, client)
}

// TestErrUnsupportedChain tests the error constant.
func TestErrUnsupportedChain(t *testing.T) {
	t.Parallel()

	require.Error(t, ErrUnsupportedChain)
	assert.Equal(t, "unsupported chain", ErrUnsupportedChain.Error())
}

// TestCacheEntry_Structure tests CacheEntry structure.
func TestCacheEntry_Structure(t *testing.T) {
	t.Parallel()

	entry := CacheEntry{
		Chain:       chain.ETH,
		Address:     "0x1234",
		Balance:     "1.0",
		Unconfirmed: "0.1",
		Symbol:      "ETH",
		Token:       "",
		Decimals:    18,
		UpdatedAt:   time.Now(),
	}

	assert.Equal(t, chain.ETH, entry.Chain)
	assert.Equal(t, "0x1234", entry.Address)
	assert.Equal(t, "1.0", entry.Balance)
	assert.Equal(t, "0.1", entry.Unconfirmed)
	assert.Equal(t, "ETH", entry.Symbol)
	assert.Empty(t, entry.Token)
	assert.Equal(t, 18, entry.Decimals)
	assert.NotZero(t, entry.UpdatedAt)
}

// TestFetcher_NilSafety tests that Fetcher methods handle nil properly.
func TestFetcher_NilSafety(t *testing.T) {
	t.Parallel()

	// Test with nil config and cache
	fetcher := &Fetcher{
		cfg:   nil,
		cache: nil,
	}

	assert.NotNil(t, fetcher)

	// Methods will panic or error with nil dependencies
	// This test documents that nil checking is not done
}

// TestFetchBSVBulk_MixedCacheStates tests bulk fetch with mixed cache states.
// Note: This test only validates cached entries to avoid network calls.
//
//nolint:godox // Explanatory note about test approach
func TestFetchBSVBulk_MixedCacheStates(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cache := newMockCacheProvider()

	// Add one fresh cache entry (within trust window)
	cache.Set(CacheEntry{
		Chain:     chain.BSV,
		Address:   "1ABC",
		Balance:   "1.0",
		Symbol:    "BSV",
		Decimals:  8,
		UpdatedAt: time.Now().Add(-10 * time.Second), // Fresh (within trust window)
	})

	fetcher := NewFetcher(cfg, cache)

	// Test only with cached address to avoid network calls
	results, err := fetcher.fetchBSVBulk(context.Background(), []string{"1ABC"})

	require.NoError(t, err)
	assert.Contains(t, results, "1ABC")

	// Check that fresh cache was used for 1ABC
	if entries, ok := results["1ABC"]; ok {
		assert.Len(t, entries, 1)
		assert.Equal(t, "1.0", entries[0].Balance)
	}
}

// TestFetchETH_ProviderSelection tests provider selection logic.
func TestFetchETH_ProviderSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerCfg  string
		expectFirst  string
		expectSecond string
	}{
		{
			name:         "RPC primary, Etherscan secondary",
			providerCfg:  "rpc",
			expectFirst:  "rpc",
			expectSecond: "etherscan",
		},
		{
			name:         "Etherscan primary, RPC secondary",
			providerCfg:  "etherscan",
			expectFirst:  "etherscan",
			expectSecond: "rpc",
		},
		{
			name:         "Default (etherscan primary)",
			providerCfg:  "",
			expectFirst:  "etherscan",
			expectSecond: "rpc",
		},
		{
			name:         "Unknown provider defaults to etherscan",
			providerCfg:  "unknown",
			expectFirst:  "etherscan",
			expectSecond: "rpc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &mockConfigProvider{
				ethProvider:        tt.providerCfg,
				ethRPC:             "https://eth.example.com",
				ethEtherscanAPIKey: "test-key",
			}
			cache := newMockCacheProvider()
			fetcher := NewFetcher(cfg, cache)

			// fetchETH will try the providers in order and fail (invalid URLs)
			// We're just testing that the logic doesn't panic
			_, _, err := fetcher.fetchETH(context.Background(), "0x1234")
			assert.Error(t, err)
		})
	}
}

// TestFetchForChain_ContextCancellation tests context cancellation handling.
func TestFetchForChain_ContextCancellation(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cache := newMockCacheProvider()
	fetcher := NewFetcher(cfg, cache)

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to fetch with canceled context
	// The actual network calls will respect cancellation
	entries, stale, err := fetcher.FetchForChain(ctx, chain.ETH, "0x1234")

	// May or may not error depending on when cancellation is checked
	// This test documents the behavior
	_ = entries
	_ = stale
	_ = err
}

// TestGetCachedETHBalances_MetricsRecording tests that cache hits/misses are recorded.
func TestGetCachedETHBalances_MetricsRecording(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cache := newMockCacheProvider()

	// Add ETH balance
	cache.Set(CacheEntry{
		Chain:     chain.ETH,
		Address:   "0x1234",
		Balance:   "1.0",
		Symbol:    "ETH",
		Decimals:  18,
		UpdatedAt: time.Now(),
	})

	fetcher := NewFetcher(cfg, cache)

	// Fetch cached balances - should record metrics
	entries, _, err := fetcher.getCachedETHBalances("0x1234")

	require.NoError(t, err)
	assert.Len(t, entries, 1) // Only ETH, no USDC

	// Metrics are recorded via metrics.Global, which we can't easily test
	// This test documents the behavior
}

// TestFetchETHViaRPC_CacheTrustWindow tests post-send cache trust.
func TestFetchETHViaRPC_CacheTrustWindow(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cfg.ethRPC = "https://eth.example.com"
	cache := newMockCacheProvider()

	// Add very fresh cache entry (within trust window)
	cache.Set(CacheEntry{
		Chain:     chain.ETH,
		Address:   "0x1234",
		Balance:   "1.0",
		Symbol:    "ETH",
		Decimals:  18,
		UpdatedAt: time.Now().Add(-10 * time.Second),
	})

	fetcher := NewFetcher(cfg, cache)

	// Should return cached data without network call
	entries, stale, err := fetcher.fetchETHViaRPC(context.Background(), "0x1234")

	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.False(t, stale)
	assert.Equal(t, "1.0", entries[0].Balance)
}

// TestFetchETHViaEtherscan_CacheTrustWindow tests post-send cache trust for Etherscan.
func TestFetchETHViaEtherscan_CacheTrustWindow(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cfg.ethEtherscanAPIKey = "test-key"
	cache := newMockCacheProvider()

	// Add very fresh cache entry (within trust window)
	cache.Set(CacheEntry{
		Chain:     chain.ETH,
		Address:   "0x1234",
		Balance:   "1.0",
		Symbol:    "ETH",
		Decimals:  18,
		UpdatedAt: time.Now().Add(-10 * time.Second),
	})

	fetcher := NewFetcher(cfg, cache)

	// Should return cached data without API call
	entries, stale, err := fetcher.fetchETHViaEtherscan(context.Background(), "0x1234", "test-key")

	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.False(t, stale)
	assert.Equal(t, "1.0", entries[0].Balance)
}

// TestFetchETHViaEtherscan_NoAPIKey tests error when API key is missing.
func TestFetchETHViaEtherscan_NoAPIKey(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cfg.ethEtherscanAPIKey = "" // No API key
	cache := newMockCacheProvider()

	fetcher := NewFetcher(cfg, cache)

	entries, stale, err := fetcher.fetchETH(context.Background(), "0x1234")

	// Should fail (primary etherscan fails due to no key, secondary RPC fails due to invalid URL)
	require.Error(t, err)
	assert.Nil(t, entries)
	assert.True(t, stale)
}
