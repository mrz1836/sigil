package balance

import (
	"context"
	"testing"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
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
