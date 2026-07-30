package btc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// newTestEsplora wires an esploraHTTP to a test server's handler.
func newTestEsplora(t *testing.T, handler http.HandlerFunc) *esploraHTTP {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return newEsploraHTTP(srv.URL, "", srv.Client(), nil)
}

func TestEsplora_AddressStats(t *testing.T) {
	t.Parallel()

	const addr = "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"
	var gotPath string
	e := newTestEsplora(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		assert.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte(`{"address":"` + addr + `",
			"chain_stats":{"funded_txo_sum":150000,"spent_txo_sum":50000,"tx_count":3},
			"mempool_stats":{"funded_txo_sum":2000,"spent_txo_sum":500,"tx_count":1}}`))
	})

	stats, err := e.AddressStats(context.Background(), addr)
	require.NoError(t, err)
	assert.Equal(t, "/address/"+addr, gotPath)
	assert.Equal(t, int64(100000), stats.ChainStats.FundedTxoSum-stats.ChainStats.SpentTxoSum)
	assert.Equal(t, int64(1500), stats.MempoolStats.FundedTxoSum-stats.MempoolStats.SpentTxoSum)
}

func TestEsplora_AddressUTXOs(t *testing.T) {
	t.Parallel()

	const addr = "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"
	var gotPath string
	e := newTestEsplora(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`[
			{"txid":"` + fixedTxID + `","vout":0,"value":100000,"status":{"confirmed":true,"block_height":800000}},
			{"txid":"` + fixedTxID2 + `","vout":1,"value":2500,"status":{"confirmed":false}}
		]`))
	})

	utxos, err := e.AddressUTXOs(context.Background(), addr)
	require.NoError(t, err)
	assert.Equal(t, "/address/"+addr+"/utxo", gotPath)
	require.Len(t, utxos, 2)
	assert.Equal(t, uint64(100000), utxos[0].Value)
	assert.True(t, utxos[0].Status.Confirmed)
	assert.False(t, utxos[1].Status.Confirmed)
}

func TestEsplora_AddressUTXOs_TooMany(t *testing.T) {
	t.Parallel()

	e := newTestEsplora(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Too many history entries"))
	})

	_, err := e.AddressUTXOs(context.Background(), "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2")
	assert.ErrorIs(t, err, ErrTooManyUTXOs)
}

func TestEsplora_FeeEstimates(t *testing.T) {
	t.Parallel()

	var gotPath string
	e := newTestEsplora(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"fastestFee":30,"halfHourFee":20,"hourFee":15,"economyFee":5,"minimumFee":1}`))
	})

	fees, err := e.FeeEstimates(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "/v1/fees/recommended", gotPath)
	assert.InDelta(t, 30.0, fees.FastestFee, 0.001)
	assert.InDelta(t, 20.0, fees.HalfHourFee, 0.001)
	assert.InDelta(t, 1.0, fees.MinimumFee, 0.001)
}

func TestEsplora_APIKeyHeader(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"fastestFee":1,"halfHourFee":1,"hourFee":1,"economyFee":1,"minimumFee":1}`))
	}))
	t.Cleanup(srv.Close)

	e := newEsploraHTTP(srv.URL, "secret-key", srv.Client(), nil)
	_, err := e.FeeEstimates(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "Bearer secret-key", gotAuth)
}

// TestEsplora_StatusMapping tests single-attempt status classification via the
// low-level get(), avoiding the multi-second retry backoff of doGet.
func TestEsplora_StatusMapping(t *testing.T) {
	t.Parallel()

	t.Run("429 rate limited", func(t *testing.T) {
		t.Parallel()
		e := newTestEsplora(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		})
		_, err := e.get(context.Background(), "/x")
		require.ErrorIs(t, err, chain.ErrRateLimited)
		assert.True(t, chain.IsRetryable(err))
	})

	t.Run("500 retryable network error", func(t *testing.T) {
		t.Parallel()
		e := newTestEsplora(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := e.get(context.Background(), "/x")
		require.ErrorIs(t, err, sigilerr.ErrNetworkError)
		assert.True(t, chain.IsRetryable(err))
	})

	t.Run("404 non-retryable network error", func(t *testing.T) {
		t.Parallel()
		e := newTestEsplora(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		_, err := e.get(context.Background(), "/x")
		require.ErrorIs(t, err, sigilerr.ErrNetworkError)
		assert.False(t, chain.IsRetryable(err))
	})
}

func TestBaseURLForNetwork(t *testing.T) {
	t.Parallel()
	assert.Equal(t, MempoolMainnetAPI, baseURLForNetwork(NetworkMainnet))
	assert.Equal(t, MempoolTestnet4API, baseURLForNetwork(NetworkTestnet))
	assert.Equal(t, MempoolMainnetAPI, baseURLForNetwork(""))
}
