package btc

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTxID = "6dfb16dd580698242bcfd8e433d557ed8c642272a368894de27292a8844a4e75"

func TestMempoolBroadcaster_Success(t *testing.T) {
	t.Parallel()

	var gotBody, gotPath, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte(testTxID))
	}))
	t.Cleanup(srv.Close)

	b := &MempoolBroadcaster{BaseURL: srv.URL, httpClient: srv.Client()}
	txid, err := b.Broadcast(context.Background(), "deadbeef")
	require.NoError(t, err)
	assert.Equal(t, testTxID, txid)
	assert.Equal(t, "/tx", gotPath)
	assert.Equal(t, "deadbeef", gotBody)
	assert.Equal(t, "text/plain", gotContentType)
}

func TestMempoolBroadcaster_AlreadyKnown(t *testing.T) {
	t.Parallel()

	t.Run("error body with txid extracts it", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("sendrawtransaction RPC error: txn-already-known " + testTxID))
		}))
		t.Cleanup(srv.Close)

		b := &MempoolBroadcaster{BaseURL: srv.URL, httpClient: srv.Client()}
		txid, err := b.Broadcast(context.Background(), "deadbeef")
		require.NoError(t, err)
		assert.Equal(t, testTxID, txid)
	})

	t.Run("error body without txid returns errAlreadyKnown", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Transaction already in block chain"))
		}))
		t.Cleanup(srv.Close)

		b := &MempoolBroadcaster{BaseURL: srv.URL, httpClient: srv.Client()}
		_, err := b.Broadcast(context.Background(), "deadbeef")
		assert.ErrorIs(t, err, errAlreadyKnown)
	})
}

func TestMempoolBroadcaster_Error(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad-txns-inputs-missingorspent"))
	}))
	t.Cleanup(srv.Close)

	b := &MempoolBroadcaster{BaseURL: srv.URL, httpClient: srv.Client()}
	_, err := b.Broadcast(context.Background(), "deadbeef")
	assert.ErrorIs(t, err, ErrBroadcastFailed)
}

func TestMempoolBroadcaster_InvalidTxIDResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-a-txid"))
	}))
	t.Cleanup(srv.Close)

	b := &MempoolBroadcaster{BaseURL: srv.URL, httpClient: srv.Client()}
	_, err := b.Broadcast(context.Background(), "deadbeef")
	assert.ErrorIs(t, err, ErrBroadcastFailed)
}

// TestBroadcastTransaction_Fallback verifies the client tries the second
// broadcaster when the first fails.
func TestBroadcastTransaction_Fallback(t *testing.T) {
	t.Parallel()

	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad-txns-inputs-missingorspent"))
	}))
	t.Cleanup(failSrv.Close)
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testTxID))
	}))
	t.Cleanup(okSrv.Close)

	c := &Client{
		network: NetworkMainnet,
		broadcasters: []Broadcaster{
			&MempoolBroadcaster{BaseURL: failSrv.URL, httpClient: failSrv.Client()},
			&BlockstreamBroadcaster{BaseURL: okSrv.URL, httpClient: okSrv.Client()},
		},
	}

	txid, err := c.BroadcastTransaction(context.Background(), []byte{0xde, 0xad, 0xbe, 0xef})
	require.NoError(t, err)
	assert.Equal(t, testTxID, txid)
}

// TestBroadcastTransaction_AlreadyKnownUsesComputedTxID verifies that an
// already-known response resolves to the locally-computed txid.
func TestBroadcastTransaction_AlreadyKnownUsesComputedTxID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Transaction already in block chain"))
	}))
	t.Cleanup(srv.Close)

	rawTx := []byte{0x01, 0x02, 0x03, 0x04}
	c := &Client{
		network:      NetworkMainnet,
		broadcasters: []Broadcaster{&MempoolBroadcaster{BaseURL: srv.URL, httpClient: srv.Client()}},
	}

	txid, err := c.BroadcastTransaction(context.Background(), rawTx)
	require.NoError(t, err)
	assert.Equal(t, computeTxID(rawTx), txid)
	assert.Len(t, txid, 64)
}

func TestBroadcastTransaction_AllFail(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad-txns-inputs-missingorspent"))
	}))
	t.Cleanup(srv.Close)

	c := &Client{
		network:      NetworkMainnet,
		broadcasters: []Broadcaster{&MempoolBroadcaster{BaseURL: srv.URL, httpClient: srv.Client()}},
	}
	_, err := c.BroadcastTransaction(context.Background(), []byte{0x01})
	assert.ErrorIs(t, err, ErrBroadcastFailed)
}
