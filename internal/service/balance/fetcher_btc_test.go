package balance

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/btc"
)

// mockBTCBalanceClient is a map-backed fake btcBalanceClient.
type mockBTCBalanceClient struct {
	balances  map[string]*btc.Balance
	nativeErr error
	bulkErr   error
}

func (m *mockBTCBalanceClient) GetNativeBalance(_ context.Context, address string) (*btc.Balance, error) {
	if m.nativeErr != nil {
		return nil, m.nativeErr
	}
	if b, ok := m.balances[address]; ok {
		return b, nil
	}
	return nil, errMissingBalance
}

func (m *mockBTCBalanceClient) GetBulkNativeBalance(_ context.Context, addresses []string) (map[string]*btc.Balance, error) {
	if m.bulkErr != nil {
		return nil, m.bulkErr
	}
	out := make(map[string]*btc.Balance, len(addresses))
	for _, addr := range addresses {
		if b, ok := m.balances[addr]; ok {
			out[addr] = b
		}
	}
	return out, nil
}

func newBTCFetcher(t *testing.T, client btcBalanceClient) (*Fetcher, *mockCacheProvider) {
	t.Helper()
	cache := newMockCacheProvider()
	fetcher := NewFetcher(newMockConfigProvider(), cache)
	fetcher.newBTCClient = func(_ context.Context, _ *btc.ClientOptions) btcBalanceClient {
		return client
	}
	return fetcher, cache
}

func TestFetchBTC_Success(t *testing.T) {
	t.Parallel()

	fetcher, _ := newBTCFetcher(t, &mockBTCBalanceClient{
		balances: map[string]*btc.Balance{
			"addr": {Address: "addr", Amount: big.NewInt(150000), Unconfirmed: big.NewInt(2000), Symbol: "BTC", Decimals: 8},
		},
	})

	entries, stale, err := fetcher.fetchBTC(context.Background(), "addr")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, chain.BTC, entries[0].Chain)
	assert.Equal(t, "BTC", entries[0].Symbol)
	assert.Equal(t, "0.0015", entries[0].Balance)
	assert.NotEmpty(t, entries[0].Unconfirmed, "unconfirmed delta should be formatted")
	assert.False(t, stale)
}

func TestFetchBTC_CacheFallbackOnError(t *testing.T) {
	t.Parallel()

	fetcher, cache := newBTCFetcher(t, &mockBTCBalanceClient{nativeErr: errMissingBalance})
	cache.Set(CacheEntry{
		Chain:     chain.BTC,
		Address:   "addr",
		Balance:   "0.00042000",
		Symbol:    "BTC",
		Decimals:  8,
		UpdatedAt: time.Now().Add(-10 * time.Minute),
	})

	entries, _, err := fetcher.fetchBTC(context.Background(), "addr")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "0.00042000", entries[0].Balance)
}

func TestFetchBTCBulk_PartialWithFallback(t *testing.T) {
	t.Parallel()

	// "known" is served by the bulk client; "missing" is absent from bulk and has
	// no cache, so it falls back to an individual fetch (also served by the mock).
	fetcher, _ := newBTCFetcher(t, &mockBTCBalanceClient{
		balances: map[string]*btc.Balance{
			"known":   {Address: "known", Amount: big.NewInt(1000), Symbol: "BTC", Decimals: 8},
			"missing": {Address: "missing", Amount: big.NewInt(2000), Symbol: "BTC", Decimals: 8},
		},
	})

	results, err := fetcher.fetchBTCBulk(context.Background(), []string{"known", "missing"})
	require.NoError(t, err)
	require.Contains(t, results, "known")
	require.Contains(t, results, "missing")
	assert.Equal(t, "0.00001", results["known"][0].Balance)
	assert.Equal(t, "0.00002", results["missing"][0].Balance)
}

func TestFetchBTCBulk_ErrorFallsBackToCache(t *testing.T) {
	t.Parallel()

	fetcher, cache := newBTCFetcher(t, &mockBTCBalanceClient{bulkErr: errMissingBalance})
	cache.Set(CacheEntry{
		Chain:     chain.BTC,
		Address:   "addr",
		Balance:   "0.00009000",
		Symbol:    "BTC",
		Decimals:  8,
		UpdatedAt: time.Now().Add(-time.Hour),
	})

	results, err := fetcher.fetchBTCBulk(context.Background(), []string{"addr"})
	require.Error(t, err, "bulk error surfaces so caller knows data is cached")
	require.Contains(t, results, "addr")
	assert.Equal(t, "0.00009000", results["addr"][0].Balance)
}
