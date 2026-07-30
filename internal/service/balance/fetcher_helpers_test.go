package balance

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/chain/btc"
	"github.com/mrz1836/sigil/internal/chain/eth"
	"github.com/mrz1836/sigil/internal/chain/eth/etherscan"
)

// TestBSVNetworkString verifies the per-fetcher override takes precedence over
// the ConfigProvider value, and that an empty override falls back to config.
func TestBSVNetworkString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		override string
		want     string
	}{
		// A loaded wallet's stamped network overrides the configured default.
		{name: "override wins over config", override: "test", want: "test"},
		// Empty override falls back to the mock config default ("main").
		{name: "empty override falls back to config", override: "", want: "main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := &Fetcher{cfg: newMockConfigProvider(), bsvNetwork: tt.override}
			assert.Equal(t, tt.want, f.bsvNetworkString())
		})
	}
}

// TestBTCNetworkString verifies the per-fetcher override takes precedence over
// the ConfigProvider value, and that an empty override falls back to config.
func TestBTCNetworkString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		override string
		want     string
	}{
		// A loaded wallet's stamped network overrides the configured default.
		{name: "override wins over config", override: "test", want: "test"},
		// Empty override falls back to the mock config default ("main").
		{name: "empty override falls back to config", override: "", want: "main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := &Fetcher{cfg: newMockConfigProvider(), btcNetwork: tt.override}
			assert.Equal(t, tt.want, f.btcNetworkString())
		})
	}
}

// TestFetchForChain_DispatchToChains covers the supported per-chain arms of the
// FetchForChain switch (ETH, BSV, BTC) plus the not-supported-in-MVP arm (LTC),
// all via injected clients/overrides so no network calls occur.
func TestFetchForChain_DispatchToChains(t *testing.T) {
	t.Parallel()

	cfg := newMockConfigProvider()
	cache := newMockCacheProvider()
	fetcher := NewFetcher(cfg, cache)

	// ETH: default provider is etherscan; override its fetch to avoid the network.
	fetcher.fetchETHViaEtherscanOverride = func(_ context.Context, address, _ string) ([]CacheEntry, bool, error) {
		return []CacheEntry{{
			Chain:    chain.ETH,
			Address:  address,
			Balance:  "1.0",
			Symbol:   "ETH",
			Decimals: 18,
		}}, false, nil
	}
	// BSV: 50000 sats at 8 decimals -> "0.0005".
	fetcher.newBSVClient = func(_ context.Context, _ *bsv.ClientOptions) bsvBalanceClient {
		return &mockBSVBalanceClient{
			bulkBalances: map[string]*bsv.Balance{
				"1ABC": {Address: "1ABC", Amount: big.NewInt(50000), Symbol: "BSV", Decimals: 8},
			},
		}
	}
	// BTC: 150000 sats at 8 decimals -> "0.0015".
	fetcher.newBTCClient = func(_ context.Context, _ *btc.ClientOptions) btcBalanceClient {
		return &mockBTCBalanceClient{
			balances: map[string]*btc.Balance{
				"1BTC": {Address: "1BTC", Amount: big.NewInt(150000), Symbol: "BTC", Decimals: 8},
			},
		}
	}

	ctx := context.Background()

	// ETH arm
	ethEntries, ethStale, err := fetcher.FetchForChain(ctx, chain.ETH, "0x1234")
	require.NoError(t, err)
	require.Len(t, ethEntries, 1)
	assert.Equal(t, chain.ETH, ethEntries[0].Chain)
	assert.Equal(t, "ETH", ethEntries[0].Symbol)
	assert.False(t, ethStale)

	// BSV arm
	bsvEntries, bsvStale, err := fetcher.FetchForChain(ctx, chain.BSV, "1ABC")
	require.NoError(t, err)
	require.Len(t, bsvEntries, 1)
	assert.Equal(t, chain.BSV, bsvEntries[0].Chain)
	assert.Equal(t, "0.0005", bsvEntries[0].Balance)
	assert.False(t, bsvStale)

	// BTC arm
	btcEntries, btcStale, err := fetcher.FetchForChain(ctx, chain.BTC, "1BTC")
	require.NoError(t, err)
	require.Len(t, btcEntries, 1)
	assert.Equal(t, chain.BTC, btcEntries[0].Chain)
	assert.Equal(t, "0.0015", btcEntries[0].Balance)
	assert.False(t, btcStale)

	// LTC arm: not supported in MVP -> nil entries, not stale, no error.
	ltcEntries, ltcStale, err := fetcher.FetchForChain(ctx, chain.LTC, "ltc-addr")
	require.NoError(t, err)
	assert.Nil(t, ltcEntries)
	assert.False(t, ltcStale)
}

// TestNewBSVBalanceClient_DefaultFactory covers the default-factory branch of
// newBSVBalanceClient (no injected factory), which must return a real client.
func TestNewBSVBalanceClient_DefaultFactory(t *testing.T) {
	t.Parallel()

	f := &Fetcher{} // newBSVClient is nil -> default factory path
	client := f.newBSVBalanceClient(context.Background(), &bsv.ClientOptions{Network: bsv.NetworkMainnet})
	assert.NotNil(t, client)
}

// TestNewBTCBalanceClient_DefaultFactory covers the default-factory branch of
// newBTCBalanceClient (no injected factory), which must return a real client.
func TestNewBTCBalanceClient_DefaultFactory(t *testing.T) {
	t.Parallel()

	f := &Fetcher{} // newBTCClient is nil -> default factory path
	client := f.newBTCBalanceClient(context.Background(), &btc.ClientOptions{Network: btc.NetworkMainnet})
	assert.NotNil(t, client)
}

// TestNewETHBalanceClient_DefaultFactory covers the default-factory branch of
// newETHBalanceClient (no injected factory): a valid RPC URL yields a client,
// and an empty URL surfaces the constructor error.
func TestNewETHBalanceClient_DefaultFactory(t *testing.T) {
	t.Parallel()

	f := &Fetcher{} // newETHClient is nil -> default factory path

	client, err := f.newETHBalanceClient("https://eth.example.com", &eth.ClientOptions{})
	require.NoError(t, err)
	require.NotNil(t, client)
	client.Close()

	// Empty RPC URL propagates the eth.NewClient error.
	bad, err := f.newETHBalanceClient("", nil)
	require.Error(t, err)
	assert.Nil(t, bad)
}

// TestNewEtherscanBalanceClient covers both branches of newEtherscanBalanceClient:
// the default factory (with its empty-key error path) and an injected factory.
func TestNewEtherscanBalanceClient(t *testing.T) {
	t.Parallel()

	f := &Fetcher{} // newEtherscanClient is nil -> default factory path

	client, err := f.newEtherscanBalanceClient("test-api-key", nil)
	require.NoError(t, err)
	assert.NotNil(t, client)

	// Empty API key propagates the etherscan.NewClient error (opts is ignored on
	// this path — passed non-nil here to exercise the options-forwarding arg).
	bad, err := f.newEtherscanBalanceClient("", &etherscan.ClientOptions{})
	require.Error(t, err)
	assert.Nil(t, bad)

	// Injected factory branch is used when set.
	f.newEtherscanClient = func(_ string, _ *etherscan.ClientOptions) (ethBalanceReader, error) {
		return &mockETHBalanceClient{}, nil
	}
	injected, err := f.newEtherscanBalanceClient("test-api-key", nil)
	require.NoError(t, err)
	assert.NotNil(t, injected)
}
