package balance

import (
	"testing"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
)

// Mock metadata provider for testing
type mockMetadataProvider struct {
	metadata map[string]*AddressMetadata
}

func newMockMetadataProvider() *mockMetadataProvider {
	return &mockMetadataProvider{
		metadata: make(map[string]*AddressMetadata),
	}
}

func (m *mockMetadataProvider) GetAddress(chainID chain.ID, address string) *AddressMetadata {
	key := string(chainID) + ":" + address
	return m.metadata[key]
}

func (m *mockMetadataProvider) setMetadata(address string, meta *AddressMetadata) {
	key := string(chain.BSV) + ":" + address
	m.metadata[key] = meta
}

// TestIsMetadataFresherThanCache tests the timestamp comparison logic
func TestIsMetadataFresherThanCache(t *testing.T) {
	tests := []struct {
		name           string
		metadata       *AddressMetadata
		cacheUpdatedAt time.Time
		wantFresher    bool
	}{
		{
			name: "Metadata scanned 1 minute after cache update - should be fresher",
			metadata: &AddressMetadata{
				LastScanned: time.Now(),
				HasActivity: true,
			},
			cacheUpdatedAt: time.Now().Add(-1 * time.Minute),
			wantFresher:    true,
		},
		{
			name: "Metadata scanned 5 seconds after cache - within tolerance",
			metadata: &AddressMetadata{
				LastScanned: time.Now(),
				HasActivity: true,
			},
			cacheUpdatedAt: time.Now().Add(-5 * time.Second),
			wantFresher:    false,
		},
		{
			name: "Metadata scanned 15 seconds after cache - beyond tolerance",
			metadata: &AddressMetadata{
				LastScanned: time.Now(),
				HasActivity: true,
			},
			cacheUpdatedAt: time.Now().Add(-15 * time.Second),
			wantFresher:    true,
		},
		{
			name: "Cache newer than metadata - should not be fresher",
			metadata: &AddressMetadata{
				LastScanned: time.Now().Add(-1 * time.Minute),
				HasActivity: true,
			},
			cacheUpdatedAt: time.Now(),
			wantFresher:    false,
		},
		{
			name:           "Nil metadata - should not be fresher",
			metadata:       nil,
			cacheUpdatedAt: time.Now(),
			wantFresher:    false,
		},
		{
			name: "Zero LastScanned - should not be fresher",
			metadata: &AddressMetadata{
				LastScanned: time.Time{},
				HasActivity: false,
			},
			cacheUpdatedAt: time.Now(),
			wantFresher:    false,
		},
		{
			name: "Metadata scanned 1 hour after cache - significantly fresher",
			metadata: &AddressMetadata{
				LastScanned: time.Now(),
				HasActivity: true,
			},
			cacheUpdatedAt: time.Now().Add(-1 * time.Hour),
			wantFresher:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &RefreshPolicy{}
			cacheEntry := &CacheEntry{
				UpdatedAt: tt.cacheUpdatedAt,
			}

			result := policy.isMetadataFresherThanCache(tt.metadata, cacheEntry)

			if result != tt.wantFresher {
				t.Errorf("isMetadataFresherThanCache() = %v, want %v", result, tt.wantFresher)
			}
		})
	}
}

// TestShouldRefresh_TimestampMismatch tests the full refresh decision with timestamp mismatch
func TestShouldRefresh_TimestampMismatch(t *testing.T) {
	tests := []struct {
		name          string
		setupCache    func(*mockCacheProvider)
		setupMetadata func(*mockMetadataProvider)
		wantDecision  RefreshDecision
		description   string
	}{
		{
			name: "UTXO store refreshed recently, cache is stale - should refresh",
			setupCache: func(cache *mockCacheProvider) {
				cache.Set(CacheEntry{
					Chain:     chain.BSV,
					Address:   "1ABC",
					Balance:   "0.0",
					Symbol:    "BSV",
					Decimals:  8,
					UpdatedAt: time.Now().Add(-1 * time.Hour), // Old cache
				})
			},
			setupMetadata: func(meta *mockMetadataProvider) {
				meta.setMetadata("1ABC", &AddressMetadata{
					ChainID:     chain.BSV,
					Address:     "1ABC",
					HasActivity: true,
					LastScanned: time.Now(), // Just scanned
				})
			},
			wantDecision: RefreshRequired,
			description:  "Simulates receive --check updating UTXO store without updating cache",
		},
		{
			name: "Both cache and UTXO store updated together - newly scanned address refreshes",
			setupCache: func(cache *mockCacheProvider) {
				cache.Set(CacheEntry{
					Chain:     chain.BSV,
					Address:   "1ABC",
					Balance:   "0.0",
					Symbol:    "BSV",
					Decimals:  8,
					UpdatedAt: time.Now(), // Fresh cache
				})
			},
			setupMetadata: func(meta *mockMetadataProvider) {
				meta.setMetadata("1ABC", &AddressMetadata{
					ChainID:     chain.BSV,
					Address:     "1ABC",
					HasActivity: true,
					LastScanned: time.Now().Add(-5 * time.Second), // Within tolerance
				})
			},
			wantDecision: RefreshRequired,
			description:  "Both stores updated together, but newly created address always refreshes",
		},
		{
			name: "Cache fresher than UTXO store - should use cache",
			setupCache: func(cache *mockCacheProvider) {
				cache.Set(CacheEntry{
					Chain:     chain.BSV,
					Address:   "1ABC",
					Balance:   "1.5",
					Symbol:    "BSV",
					Decimals:  8,
					UpdatedAt: time.Now(), // Fresh cache
				})
			},
			setupMetadata: func(meta *mockMetadataProvider) {
				meta.setMetadata("1ABC", &AddressMetadata{
					ChainID:     chain.BSV,
					Address:     "1ABC",
					HasActivity: true,
					LastScanned: time.Now().Add(-1 * time.Minute), // Older scan
				})
			},
			wantDecision: RefreshRequired,
			description:  "Cache fresher but has balance - high priority always refreshes",
		},
		{
			name: "Zero LastScanned - falls through to normal logic",
			setupCache: func(cache *mockCacheProvider) {
				cache.Set(CacheEntry{
					Chain:     chain.BSV,
					Address:   "1ABC",
					Balance:   "0.0",
					Symbol:    "BSV",
					Decimals:  8,
					UpdatedAt: time.Now().Add(-10 * time.Minute),
				})
			},
			setupMetadata: func(meta *mockMetadataProvider) {
				meta.setMetadata("1ABC", &AddressMetadata{
					ChainID:     chain.BSV,
					Address:     "1ABC",
					HasActivity: true,
					LastScanned: time.Time{}, // Never scanned
				})
			},
			wantDecision: CacheOK,
			description:  "Zero LastScanned bypasses timestamp check, uses normal policy",
		},
		{
			name: "No metadata - should refresh",
			setupCache: func(cache *mockCacheProvider) {
				cache.Set(CacheEntry{
					Chain:     chain.BSV,
					Address:   "1XYZ",
					Balance:   "0.0",
					Symbol:    "BSV",
					Decimals:  8,
					UpdatedAt: time.Now(),
				})
			},
			setupMetadata: func(_ *mockMetadataProvider) {
				// Don't add metadata
			},
			wantDecision: RefreshRequired,
			description:  "No metadata exists - safe fallback to refresh",
		},
		{
			name: "No cache - should refresh",
			setupCache: func(_ *mockCacheProvider) {
				// Don't add cache entry
			},
			setupMetadata: func(meta *mockMetadataProvider) {
				meta.setMetadata("1ABC", &AddressMetadata{
					ChainID:     chain.BSV,
					Address:     "1ABC",
					HasActivity: true,
					LastScanned: time.Now(),
				})
			},
			wantDecision: RefreshRequired,
			description:  "No cache exists - must fetch fresh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newMockCacheProvider()
			metadata := newMockMetadataProvider()

			tt.setupCache(cache)
			tt.setupMetadata(metadata)

			policy := NewRefreshPolicy(metadata, cache)
			decision := policy.ShouldRefresh(chain.BSV, "1ABC")

			if decision != tt.wantDecision {
				t.Errorf("%s: ShouldRefresh() = %v, want %v", tt.description, decision, tt.wantDecision)
			}
		})
	}
}

// TestShouldRefresh_PriorityTiers tests the priority-based refresh logic
// using old LastScanned times to avoid "newly created address" behavior
func TestShouldRefresh_PriorityTiers(t *testing.T) {
	// Use a fixed past time for LastScanned to avoid "newly created" check
	oldScanTime := time.Now().Add(-48 * time.Hour)

	tests := []struct {
		name         string
		hasActivity  bool
		hasBalance   bool
		cacheAge     time.Duration
		wantDecision RefreshDecision
	}{
		{
			name:         "High priority: has activity and balance - always refresh",
			hasActivity:  true,
			hasBalance:   true,
			cacheAge:     1 * time.Minute,
			wantDecision: RefreshRequired,
		},
		{
			name:         "Medium priority: has activity, no balance, 10-min cache - use cache",
			hasActivity:  true,
			hasBalance:   false,
			cacheAge:     10 * time.Minute,
			wantDecision: CacheOK,
		},
		{
			name:         "Medium priority: has activity, no balance, 31-min cache - refresh",
			hasActivity:  true,
			hasBalance:   false,
			cacheAge:     31 * time.Minute,
			wantDecision: RefreshRequired,
		},
		{
			name:         "Low priority: no activity, no balance, 1-hour cache - use cache",
			hasActivity:  false,
			hasBalance:   false,
			cacheAge:     1 * time.Hour,
			wantDecision: CacheOK,
		},
		{
			name:         "Low priority: no activity, no balance, 3-hour cache - refresh",
			hasActivity:  false,
			hasBalance:   false,
			cacheAge:     3 * time.Hour,
			wantDecision: RefreshRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newMockCacheProvider()
			metadata := newMockMetadataProvider()

			balanceStr := "0.0"
			if tt.hasBalance {
				balanceStr = "1.5"
			}

			now := time.Now()
			cache.Set(CacheEntry{
				Chain:     chain.BSV,
				Address:   "1ABC",
				Balance:   balanceStr,
				Symbol:    "BSV",
				Decimals:  8,
				UpdatedAt: now.Add(-tt.cacheAge),
			})

			// Use old scan time (48 hours ago) to avoid "newly created" check
			// Set it slightly before cache UpdatedAt to avoid timestamp mismatch trigger
			metadata.setMetadata("1ABC", &AddressMetadata{
				ChainID:     chain.BSV,
				Address:     "1ABC",
				HasActivity: tt.hasActivity,
				LastScanned: oldScanTime,
			})

			policy := NewRefreshPolicy(metadata, cache)
			decision := policy.ShouldRefresh(chain.BSV, "1ABC")

			if decision != tt.wantDecision {
				t.Errorf("ShouldRefresh() = %v, want %v (hasActivity=%v, hasBalance=%v, cacheAge=%v)",
					decision, tt.wantDecision, tt.hasActivity, tt.hasBalance, tt.cacheAge)
			}
		})
	}
}

// TestShouldRefresh_NewlyCreatedAddress tests the "newly created address" behavior
func TestShouldRefresh_NewlyCreatedAddress(t *testing.T) {
	tests := []struct {
		name         string
		lastScanned  time.Time
		cacheAge     time.Duration
		wantDecision RefreshDecision
	}{
		{
			name:         "Scanned 1 hour ago - newly created, always refresh",
			lastScanned:  time.Now().Add(-1 * time.Hour),
			cacheAge:     1 * time.Hour,
			wantDecision: RefreshRequired,
		},
		{
			name:         "Scanned 12 hours ago - newly created, always refresh",
			lastScanned:  time.Now().Add(-12 * time.Hour),
			cacheAge:     12 * time.Hour,
			wantDecision: RefreshRequired,
		},
		{
			name:         "Scanned 23 hours ago - newly created, always refresh",
			lastScanned:  time.Now().Add(-23 * time.Hour),
			cacheAge:     23 * time.Hour,
			wantDecision: RefreshRequired,
		},
		{
			name:         "Scanned 25 hours ago - not newly created, cache fresh, use cache",
			lastScanned:  time.Now().Add(-25 * time.Hour),
			cacheAge:     10 * time.Minute, // Fresh cache
			wantDecision: CacheOK,          // hasActivity=true, hasBalance=false, < 30 min
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := newMockCacheProvider()
			metadata := newMockMetadataProvider()

			// Set cache age separately from lastScanned
			cache.Set(CacheEntry{
				Chain:     chain.BSV,
				Address:   "1ABC",
				Balance:   "0.0",
				Symbol:    "BSV",
				Decimals:  8,
				UpdatedAt: time.Now().Add(-tt.cacheAge),
			})

			metadata.setMetadata("1ABC", &AddressMetadata{
				ChainID:     chain.BSV,
				Address:     "1ABC",
				HasActivity: true,
				LastScanned: tt.lastScanned,
			})

			policy := NewRefreshPolicy(metadata, cache)
			decision := policy.ShouldRefresh(chain.BSV, "1ABC")

			if decision != tt.wantDecision {
				t.Errorf("ShouldRefresh() = %v, want %v (lastScanned=%v ago, cacheAge=%v)",
					decision, tt.wantDecision, time.Since(tt.lastScanned), tt.cacheAge)
			}
		})
	}
}
