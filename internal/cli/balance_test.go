package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
)

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
