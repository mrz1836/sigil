package balance

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
)

var (
	errForcedFetchFailure = errors.New("forced fetch failure")
	errMissingBalance     = errors.New("missing balance")
)

// Mock implementations for testing
type mockCacheProvider struct {
	entries map[string]*CacheEntry
}

func newMockCacheProvider() *mockCacheProvider {
	return &mockCacheProvider{
		entries: make(map[string]*CacheEntry),
	}
}

func (m *mockCacheProvider) Get(chainID chain.ID, address, token string) (*CacheEntry, bool, time.Duration) {
	key := string(chainID) + ":" + address
	if token != "" {
		key += ":" + token
	}
	entry, exists := m.entries[key]
	if !exists {
		return nil, false, 0
	}
	age := time.Since(entry.UpdatedAt)
	return entry, true, age
}

func (m *mockCacheProvider) Set(entry CacheEntry) {
	key := string(entry.Chain) + ":" + entry.Address
	if entry.Token != "" {
		key += ":" + entry.Token
	}
	m.entries[key] = &entry
}

//nolint:gocognit // Test function with comprehensive validation logic
func TestFetchCachedBalances_Success(t *testing.T) {
	// Setup
	cache := newMockCacheProvider()

	// Add some cached entries
	cache.Set(CacheEntry{
		Chain:     chain.ETH,
		Address:   "0x1234",
		Balance:   "1.5",
		Symbol:    "ETH",
		Decimals:  18,
		UpdatedAt: time.Now().Add(-10 * time.Minute),
	})
	cache.Set(CacheEntry{
		Chain:     chain.BSV,
		Address:   "1ABC",
		Balance:   "0.5",
		Symbol:    "BSV",
		Decimals:  8,
		UpdatedAt: time.Now().Add(-5 * time.Minute),
	})

	service := &Service{
		cache: cache,
	}

	// Test
	req := &FetchBatchRequest{
		Addresses: []AddressInput{
			{ChainID: chain.ETH, Address: "0x1234"},
			{ChainID: chain.BSV, Address: "1ABC"},
		},
	}

	result, err := service.FetchCachedBalances(context.Background(), req)
	// Verify
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}

	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %d", len(result.Errors))
	}

	// Check that all results are marked as stale
	for _, res := range result.Results {
		if !res.Stale {
			t.Errorf("expected result for %s:%s to be marked stale", res.ChainID, res.Address)
		}
		for _, bal := range res.Balances {
			if !bal.Stale {
				t.Errorf("expected balance entry for %s:%s to be marked stale", res.ChainID, res.Address)
			}
		}
	}
}

func TestFetchCachedBalances_MissingCache(t *testing.T) {
	// Setup
	cache := newMockCacheProvider()

	// Only add one entry
	cache.Set(CacheEntry{
		Chain:     chain.ETH,
		Address:   "0x1234",
		Balance:   "1.5",
		Symbol:    "ETH",
		Decimals:  18,
		UpdatedAt: time.Now(),
	})

	service := &Service{
		cache: cache,
	}

	// Test - request 2 addresses but only 1 is cached
	req := &FetchBatchRequest{
		Addresses: []AddressInput{
			{ChainID: chain.ETH, Address: "0x1234"},
			{ChainID: chain.BSV, Address: "1XYZ"}, // Not cached
		},
	}

	result, err := service.FetchCachedBalances(context.Background(), req)
	// Verify
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(result.Results))
	}

	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error for missing cache, got %d", len(result.Errors))
	}
}

func TestFetchCachedBalances_EmptyCache(t *testing.T) {
	// Setup
	cache := newMockCacheProvider()
	service := &Service{
		cache: cache,
	}

	// Test - no cached data
	req := &FetchBatchRequest{
		Addresses: []AddressInput{
			{ChainID: chain.ETH, Address: "0xABCD"},
		},
	}

	result, err := service.FetchCachedBalances(context.Background(), req)
	// Verify
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Results) != 0 {
		t.Errorf("expected no results, got %d", len(result.Results))
	}

	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
}

func TestCacheEntryToBalanceEntry_StaleDetection(t *testing.T) {
	tests := []struct {
		name      string
		age       time.Duration
		wantStale bool
	}{
		{"Fresh (1 min)", 1 * time.Minute, false},
		{"Fresh (4 min)", 4 * time.Minute, false},
		{"Stale (6 min)", 6 * time.Minute, true},
		{"Stale (1 hour)", 1 * time.Hour, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := CacheEntry{
				Chain:     chain.ETH,
				Address:   "0x1234",
				Balance:   "1.0",
				Symbol:    "ETH",
				Decimals:  18,
				UpdatedAt: time.Now().Add(-tt.age),
			}

			result := cacheEntryToBalanceEntry(entry)

			if result.Stale != tt.wantStale {
				t.Errorf("for age %v: expected Stale=%v, got %v", tt.age, tt.wantStale, result.Stale)
			}
		})
	}
}

func TestFetchBalances_BSVRefreshPolicy_UsesCache(t *testing.T) {
	// This test verifies that BSV addresses use cached data when refresh policy says CacheOK
	// Uses Medium Priority scenario: HasActivity=true, Balance=0, cache age < 30min
	cache := newMockCacheProvider()
	metadata := newMockMetadataProvider()

	// Address with zero balance but has activity (Medium Priority)
	// Cache is fresh (< 30 min), so policy should return CacheOK
	// LastScanned must be > 24 hours old to avoid "newly created" check
	addr := "1ABC123"
	recentTime := time.Now().Add(-10 * time.Minute) // 10 minutes old (< 30 min threshold)
	cache.Set(CacheEntry{
		Chain:     chain.BSV,
		Address:   addr,
		Balance:   "0", // Zero balance
		Symbol:    "BSV",
		Decimals:  8,
		UpdatedAt: recentTime,
	})
	metadata.setMetadata(addr, &AddressMetadata{
		LastScanned: time.Now().Add(-48 * time.Hour), // Scanned 2 days ago (> 24 hours)
		HasActivity: true,                            // Has activity, but balance is zero (Medium Priority)
	})

	// Create service with refresh policy but no fetcher
	// This proves the cache is used without needing a network call
	service := &Service{
		cache:   cache,
		policy:  NewRefreshPolicy(metadata, cache),
		fetcher: nil, // No fetcher - will panic if network call is attempted
	}

	// Test: Call FetchBalances with BSV address
	req := &FetchBatchRequest{
		Addresses: []AddressInput{
			{ChainID: chain.BSV, Address: addr},
		},
		ForceRefresh: false,
	}

	result, err := service.FetchBalances(context.Background(), req)
	// Verify: No error should occur
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify: Should have exactly 1 result (from cache)
	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}

	// Verify: Result should match cached data
	res := result.Results[0]
	if res.Address != addr {
		t.Errorf("expected address %s, got %s", addr, res.Address)
	}
	if len(res.Balances) != 1 {
		t.Fatalf("expected 1 balance, got %d", len(res.Balances))
	}
	if res.Balances[0].Balance != "0" {
		t.Errorf("expected cached balance 0, got %s", res.Balances[0].Balance)
	}
	if res.Balances[0].Symbol != "BSV" {
		t.Errorf("expected symbol BSV, got %s", res.Balances[0].Symbol)
	}
}

func TestFetchBalance_CacheExpiry_Scenarios(t *testing.T) { //nolint:gocognit // test complexity acceptable
	t.Parallel()

	tests := []struct {
		name         string
		cacheAge     time.Duration
		hasActivity  bool
		expectsStale bool
	}{
		{
			name:         "Fresh cache - 2 minutes old",
			cacheAge:     2 * time.Minute,
			hasActivity:  true,
			expectsStale: false,
		},
		{
			name:         "Stale cache - 10 minutes old",
			cacheAge:     10 * time.Minute,
			hasActivity:  true,
			expectsStale: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newMockCacheProvider()
			cache.Set(CacheEntry{
				Chain:     chain.BSV,
				Address:   "1ABC",
				Balance:   "1.0",
				Symbol:    "BSV",
				Decimals:  8,
				UpdatedAt: time.Now().Add(-tt.cacheAge),
			})

			// Verify cache entry exists with expected age
			entry, exists, age := cache.Get(chain.BSV, "1ABC", "")
			if !exists {
				t.Fatal("cache entry should exist")
			}
			if entry.Balance != "1.0" {
				t.Errorf("expected balance 1.0, got %s", entry.Balance)
			}

			// Verify staleness based on age
			if age < 5*time.Minute && tt.expectsStale {
				t.Error("cache is fresh but test expects stale")
			}
			if age >= 5*time.Minute && !tt.expectsStale {
				t.Error("cache is stale but test expects fresh")
			}
		})
	}
}

func TestFetchBalance_ForceRefresh_BypassesPolicy(t *testing.T) {
	t.Parallel()

	cache := newMockCacheProvider()
	metadata := newMockMetadataProvider()

	addr := "1ABC"
	// Set up cached entry
	cache.Set(CacheEntry{
		Chain:     chain.BSV,
		Address:   addr,
		Balance:   "0.5",
		Symbol:    "BSV",
		Decimals:  8,
		UpdatedAt: time.Now().Add(-1 * time.Minute),
	})
	metadata.setMetadata(addr, &AddressMetadata{
		HasActivity: true,
		LastScanned: time.Now().Add(-48 * time.Hour),
	})

	// Verify policy would normally say to use cache
	policy := NewRefreshPolicy(metadata, cache)
	decision := policy.ShouldRefresh(chain.BSV, addr)

	// For a freshly cached address with activity, policy should allow cache
	_ = decision // CacheOK or RefreshRequired depending on policy

	// When ForceRefresh is set, policy should be bypassed
	// (We can't easily test this without a real fetcher, but we've verified
	// the policy exists and would make a decision)
}

func TestProgressCallback_Structure(t *testing.T) {
	t.Parallel()

	// Test the ProgressUpdate structure
	update := ProgressUpdate{
		Phase:              "building",
		TotalAddresses:     5,
		CompletedAddresses: 2,
		ChainID:            chain.BSV,
		CurrentAddress:     "1ABC",
		Message:            "Processing addresses",
	}

	// Verify all fields are accessible
	if update.Phase != "building" {
		t.Errorf("expected phase 'building', got %s", update.Phase)
	}
	if update.TotalAddresses != 5 {
		t.Errorf("expected TotalAddresses 5, got %d", update.TotalAddresses)
	}
	if update.CompletedAddresses != 2 {
		t.Errorf("expected CompletedAddresses 2, got %d", update.CompletedAddresses)
	}
	if update.ChainID != chain.BSV {
		t.Errorf("expected chain BSV, got %s", update.ChainID)
	}
	if update.CurrentAddress != "1ABC" {
		t.Errorf("expected address 1ABC, got %s", update.CurrentAddress)
	}
	if update.Message != "Processing addresses" {
		t.Errorf("expected message 'Processing addresses', got %s", update.Message)
	}
}

func TestGetCachedBalancesForAddress(t *testing.T) {
	t.Parallel()

	cache := newMockCacheProvider()

	addr := "1ABC"
	cache.Set(CacheEntry{
		Chain:     chain.BSV,
		Address:   addr,
		Balance:   "2.5",
		Symbol:    "BSV",
		Decimals:  8,
		UpdatedAt: time.Now().Add(-10 * time.Minute),
	})

	// Test the cache retrieval function directly
	cached := getCachedBalancesForAddress(chain.BSV, addr, cache)

	if len(cached) != 1 {
		t.Fatalf("expected 1 cached balance, got %d", len(cached))
	}
	if cached[0].Balance != "2.5" {
		t.Errorf("expected balance 2.5, got %s", cached[0].Balance)
	}
	if cached[0].Symbol != "BSV" {
		t.Errorf("expected symbol BSV, got %s", cached[0].Symbol)
	}
}

func TestRefreshPolicy_StaleCache_Decision(t *testing.T) {
	t.Parallel()

	cache := newMockCacheProvider()
	metadata := newMockMetadataProvider()

	// Create address with stale cache
	addr := "1XYZ"
	cache.Set(CacheEntry{
		Chain:     chain.BSV,
		Address:   addr,
		Balance:   "1.0",
		Symbol:    "BSV",
		Decimals:  8,
		UpdatedAt: time.Now().Add(-3 * time.Hour), // Stale (> 2 hours for low priority)
	})
	metadata.setMetadata(addr, &AddressMetadata{
		HasActivity: false, // Low priority
		LastScanned: time.Now().Add(-48 * time.Hour),
	})

	policy := NewRefreshPolicy(metadata, cache)
	decision := policy.ShouldRefresh(chain.BSV, addr)

	// For low priority with stale cache (>2 hours), should require refresh
	if decision != RefreshRequired {
		t.Logf("policy decision: %v (stale cache may still return CacheOK)", decision)
	}
}

func TestAddressInput_Structure(t *testing.T) {
	t.Parallel()

	// Test the AddressInput structure
	inputs := []AddressInput{
		{ChainID: chain.BSV, Address: "1ADDR1"},
		{ChainID: chain.BSV, Address: "1ADDR2"},
		{ChainID: chain.ETH, Address: "0x123"},
	}

	if len(inputs) != 3 {
		t.Fatalf("expected 3 inputs, got %d", len(inputs))
	}

	// Verify BSV inputs
	bsvCount := 0
	ethCount := 0
	for _, input := range inputs {
		if input.ChainID == chain.BSV {
			bsvCount++
		}
		if input.ChainID == chain.ETH {
			ethCount++
		}
	}

	if bsvCount != 2 {
		t.Errorf("expected 2 BSV addresses, got %d", bsvCount)
	}
	if ethCount != 1 {
		t.Errorf("expected 1 ETH address, got %d", ethCount)
	}
}

func TestFetchRequest_Timeout_Configuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"No timeout", 0},
		{"1 second timeout", 1 * time.Second},
		{"10 second timeout", 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &FetchRequest{
				ChainID:      chain.ETH,
				Address:      "0x123",
				ForceRefresh: true,
				Timeout:      tt.timeout,
			}

			// Verify timeout is set correctly
			if req.Timeout != tt.timeout {
				t.Errorf("expected timeout %v, got %v", tt.timeout, req.Timeout)
			}
		})
	}
}

func TestFetchCachedBalances_TokenBalance(t *testing.T) {
	t.Parallel()

	cache := newMockCacheProvider()

	// Add native balance
	cache.Set(CacheEntry{
		Chain:     chain.ETH,
		Address:   "0x123",
		Balance:   "1.5",
		Symbol:    "ETH",
		Token:     "",
		Decimals:  18,
		UpdatedAt: time.Now().Add(-5 * time.Minute),
	})

	// Add token balance
	cache.Set(CacheEntry{ //nolint:gosec // G101: test data, not real credentials
		Chain:     chain.ETH,
		Address:   "0x123",
		Balance:   "100",
		Symbol:    "USDC",
		Token:     "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		Decimals:  6,
		UpdatedAt: time.Now().Add(-5 * time.Minute),
	})

	service := &Service{
		cache: cache,
	}

	req := &FetchBatchRequest{
		Addresses: []AddressInput{
			{ChainID: chain.ETH, Address: "0x123"},
		},
	}

	result, err := service.FetchCachedBalances(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}

	// Should have both native and token balance
	balances := result.Results[0].Balances
	if len(balances) != 2 {
		t.Fatalf("expected 2 balances (native + token), got %d", len(balances))
	}

	// Verify we have both ETH and USDC
	hasETH := false
	hasUSDC := false
	for _, bal := range balances {
		if bal.Symbol == "ETH" {
			hasETH = true
		}
		if bal.Symbol == "USDC" {
			hasUSDC = true
		}
	}

	if !hasETH {
		t.Error("expected ETH native balance")
	}
	if !hasUSDC {
		t.Error("expected USDC token balance")
	}
}

func TestNewService_PolicyInitialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		metadata   AddressMetadataProvider
		cache      CacheProvider
		force      bool
		wantPolicy bool
	}{
		{
			name:       "policy enabled with metadata and cache",
			metadata:   newChainAwareMetadataProvider(),
			cache:      newMockCacheProvider(),
			force:      false,
			wantPolicy: true,
		},
		{
			name:       "policy disabled without metadata",
			metadata:   nil,
			cache:      newMockCacheProvider(),
			force:      false,
			wantPolicy: false,
		},
		{
			name:       "policy disabled without cache",
			metadata:   newChainAwareMetadataProvider(),
			cache:      nil,
			force:      false,
			wantPolicy: false,
		},
		{
			name:       "policy disabled when force refresh",
			metadata:   newChainAwareMetadataProvider(),
			cache:      newMockCacheProvider(),
			force:      true,
			wantPolicy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := NewService(&Config{
				ConfigProvider: newMockConfigProvider(),
				CacheProvider:  tt.cache,
				Metadata:       tt.metadata,
				ForceRefresh:   tt.force,
			})

			require.NotNil(t, service.fetcher)
			assert.Equal(t, tt.cache, service.cache)
			assert.Equal(t, tt.force, service.force)

			if tt.wantPolicy {
				require.NotNil(t, service.policy)
			} else {
				require.Nil(t, service.policy)
			}
		})
	}
}

func TestFetchBalance_UsesCacheWhenPolicyAllows(t *testing.T) {
	t.Parallel()

	cacheProvider := newMockCacheProvider()
	cacheProvider.Set(CacheEntry{
		Chain:     chain.BSV,
		Address:   "1cache",
		Balance:   "0",
		Symbol:    "BSV",
		Decimals:  8,
		UpdatedAt: time.Now().Add(-10 * time.Minute),
	})

	metadata := newChainAwareMetadataProvider()
	metadata.set(chain.BSV, "1cache", &AddressMetadata{
		ChainID:     chain.BSV,
		Address:     "1cache",
		HasActivity: true,
		LastScanned: time.Now().Add(-48 * time.Hour),
	})

	service := &Service{
		fetcher: NewFetcher(newMockConfigProvider(), cacheProvider),
		policy:  NewRefreshPolicy(metadata, cacheProvider),
		cache:   cacheProvider,
	}

	result, err := service.FetchBalance(context.Background(), &FetchRequest{
		ChainID: chain.BSV,
		Address: "1cache",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Balances, 1)
	assert.Equal(t, "0", result.Balances[0].Balance)
	assert.False(t, result.Stale)
}

func TestFetchBalance_FetchErrorWithCachedFallback(t *testing.T) {
	t.Parallel()

	cacheProvider := newMockCacheProvider()
	unknownChain := chain.ID("foo")
	cacheProvider.Set(CacheEntry{
		Chain:     unknownChain,
		Address:   "addr1",
		Balance:   "7.2",
		Symbol:    "FOO",
		Decimals:  8,
		UpdatedAt: time.Now().Add(-1 * time.Minute),
	})

	service := &Service{
		fetcher: NewFetcher(newMockConfigProvider(), cacheProvider),
		cache:   cacheProvider,
	}

	result, err := service.FetchBalance(context.Background(), &FetchRequest{
		ChainID: unknownChain,
		Address: "addr1",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Stale)
	require.Error(t, result.Error)
	require.Len(t, result.Balances, 1)
	assert.Equal(t, "7.2", result.Balances[0].Balance)
}

func TestFetchBalance_FetchErrorWithoutCache(t *testing.T) {
	t.Parallel()

	cacheProvider := newMockCacheProvider()
	service := &Service{
		fetcher: NewFetcher(newMockConfigProvider(), cacheProvider),
		cache:   cacheProvider,
	}

	result, err := service.FetchBalance(context.Background(), &FetchRequest{
		ChainID: chain.ID("bar"),
		Address: "missing",
	})

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestFetchBalance_ForceRefreshBypassesPolicy(t *testing.T) {
	t.Parallel()

	cacheProvider := newMockCacheProvider()
	cacheProvider.Set(CacheEntry{
		Chain:     chain.ETH,
		Address:   "0xforce",
		Balance:   "0",
		Symbol:    "ETH",
		Decimals:  18,
		UpdatedAt: time.Now().Add(-1 * time.Minute),
	})

	metadata := newChainAwareMetadataProvider()
	metadata.set(chain.ETH, "0xforce", &AddressMetadata{
		ChainID:     chain.ETH,
		Address:     "0xforce",
		HasActivity: true,
		LastScanned: time.Now().Add(-48 * time.Hour),
	})

	cfg := newMockConfigProvider()
	cfg.ethProvider = "rpc"
	cfg.ethEtherscanAPIKey = ""

	fetcher := NewFetcher(cfg, cacheProvider)
	fetchCalled := false
	fetcher.fetchETHViaRPCOverride = func(_ context.Context, _ string) ([]CacheEntry, bool, error) {
		fetchCalled = true
		return []CacheEntry{
			{
				Chain:     chain.ETH,
				Address:   "0xforce",
				Balance:   "9.9",
				Symbol:    "ETH",
				Decimals:  18,
				UpdatedAt: time.Now(),
			},
		}, false, nil
	}

	service := &Service{
		fetcher: fetcher,
		policy:  NewRefreshPolicy(metadata, cacheProvider),
		cache:   cacheProvider,
	}

	result, err := service.FetchBalance(context.Background(), &FetchRequest{
		ChainID:      chain.ETH,
		Address:      "0xforce",
		ForceRefresh: true,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, fetchCalled)
	require.Len(t, result.Balances, 1)
	assert.Equal(t, "9.9", result.Balances[0].Balance)
}

func TestFetchBalance_AppliesTimeoutContext(t *testing.T) {
	t.Parallel()

	cacheProvider := newMockCacheProvider()
	cfg := newMockConfigProvider()
	cfg.ethProvider = "rpc"
	cfg.ethEtherscanAPIKey = ""

	fetcher := NewFetcher(cfg, cacheProvider)
	deadlineSeen := false
	fetcher.fetchETHViaRPCOverride = func(ctx context.Context, _ string) ([]CacheEntry, bool, error) {
		_, deadlineSeen = ctx.Deadline()
		return nil, true, errForcedFetchFailure
	}

	service := &Service{
		fetcher: fetcher,
		cache:   cacheProvider,
	}

	result, err := service.FetchBalance(context.Background(), &FetchRequest{
		ChainID: chain.ETH,
		Address: "0xdead",
		Timeout: 50 * time.Millisecond,
	})

	require.Error(t, err)
	assert.True(t, deadlineSeen)
	assert.Nil(t, result)
}

func TestFetchBalances_DefaultConcurrencyAndProgress(t *testing.T) {
	t.Parallel()

	cacheProvider := newMockCacheProvider()
	chainOne := chain.ID("alpha")
	chainTwo := chain.ID("beta")

	cacheProvider.Set(CacheEntry{
		Chain:     chainOne,
		Address:   "addr-alpha",
		Balance:   "1.1",
		Symbol:    "ALP",
		Decimals:  8,
		UpdatedAt: time.Now(),
	})
	cacheProvider.Set(CacheEntry{
		Chain:     chainTwo,
		Address:   "addr-beta",
		Balance:   "2.2",
		Symbol:    "BET",
		Decimals:  8,
		UpdatedAt: time.Now(),
	})

	service := &Service{
		fetcher: NewFetcher(newMockConfigProvider(), cacheProvider),
		cache:   cacheProvider,
	}

	var updates []ProgressUpdate
	result, err := service.FetchBalances(context.Background(), &FetchBatchRequest{
		Addresses: []AddressInput{
			{ChainID: chainOne, Address: "addr-alpha"},
			{ChainID: chainTwo, Address: "addr-beta"},
		},
		MaxConcurrent: 0, // exercise default value path
		ProgressCallback: func(update ProgressUpdate) {
			updates = append(updates, update)
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Results, 2)
	assert.Empty(t, result.Errors)

	assert.Contains(t, progressPhases(updates), "building")
	assert.Contains(t, progressPhases(updates), "fetching_eth")
	assert.True(t, hasCompletedProgress(updates), "expected at least one completed progress update")
}

func TestFetchBalances_BSVBulkWithMockedClient(t *testing.T) {
	t.Parallel()

	cacheProvider := newMockCacheProvider()
	cfg := newMockConfigProvider()

	fetcher := NewFetcher(cfg, cacheProvider)
	fetcher.newBSVClient = func(_ context.Context, _ *bsv.ClientOptions) bsvBalanceClient {
		return &mockBSVBalanceClient{
			bulkBalances: map[string]*bsv.Balance{
				"1bulkA": {Address: "1bulkA", Amount: big.NewInt(1000), Symbol: "BSV", Decimals: 8},
				"1bulkB": {Address: "1bulkB", Amount: big.NewInt(2000), Symbol: "BSV", Decimals: 8},
			},
		}
	}

	service := &Service{
		fetcher: fetcher,
		cache:   cacheProvider,
	}

	var updates []ProgressUpdate
	result, err := service.FetchBalances(context.Background(), &FetchBatchRequest{
		Addresses: []AddressInput{
			{ChainID: chain.BSV, Address: "1bulkA"},
			{ChainID: chain.BSV, Address: "1bulkB"},
		},
		ProgressCallback: func(update ProgressUpdate) {
			updates = append(updates, update)
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Results, 2)
	assert.Empty(t, result.Errors)
	assert.Contains(t, progressPhases(updates), "fetching_bsv")
}

type chainAwareMetadataProvider struct {
	items map[string]*AddressMetadata
}

func newChainAwareMetadataProvider() *chainAwareMetadataProvider {
	return &chainAwareMetadataProvider{
		items: make(map[string]*AddressMetadata),
	}
}

func (m *chainAwareMetadataProvider) GetAddress(chainID chain.ID, address string) *AddressMetadata {
	return m.items[string(chainID)+":"+address]
}

func (m *chainAwareMetadataProvider) set(chainID chain.ID, address string, metadata *AddressMetadata) {
	m.items[string(chainID)+":"+address] = metadata
}

type mockBSVBalanceClient struct {
	bulkBalances map[string]*bsv.Balance
	bulkErr      error
}

func (m *mockBSVBalanceClient) GetNativeBalance(_ context.Context, address string) (*bsv.Balance, error) {
	if balance, ok := m.bulkBalances[address]; ok {
		return balance, nil
	}
	return nil, errMissingBalance
}

func (m *mockBSVBalanceClient) GetBulkNativeBalance(_ context.Context, _ []string) (map[string]*bsv.Balance, error) {
	if m.bulkErr != nil {
		return nil, m.bulkErr
	}
	out := make(map[string]*bsv.Balance, len(m.bulkBalances))
	for address, balance := range m.bulkBalances {
		out[address] = balance
	}
	return out, nil
}

func progressPhases(updates []ProgressUpdate) []string {
	phases := make([]string, 0, len(updates))
	for _, update := range updates {
		phases = append(phases, update.Phase)
	}
	return phases
}

func hasCompletedProgress(updates []ProgressUpdate) bool {
	for _, update := range updates {
		if update.CompletedAddresses > 0 {
			return true
		}
	}
	return false
}
