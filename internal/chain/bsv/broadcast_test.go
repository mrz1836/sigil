package bsv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- WhatsOnChain SDK Broadcaster Tests ---

func TestWoCSDKBroadcaster_Success(t *testing.T) {
	t.Parallel()

	expectedTxID := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	mock := &mockWOCClient{
		broadcastFunc: func(_ context.Context, _ string) (string, error) {
			return expectedTxID, nil
		},
	}

	b := &WOCSDKBroadcaster{woc: mock}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	txid, err := b.Broadcast(ctx, "deadbeef")
	require.NoError(t, err)
	assert.Equal(t, expectedTxID, txid)
}

func TestWoCSDKBroadcaster_AlreadyInMempool_WithTxID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"already in mempool with txid", fmt.Errorf("%w: %s", errTestMempoolBase, testValidTxID)},
		{"txn-already-known with txid", fmt.Errorf("txn-already-known %w %s", errTestMempoolBase, testValidTxID)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testErr := tt.err
			mock := &mockWOCClient{
				broadcastFunc: func(_ context.Context, _ string) (string, error) {
					return "", testErr
				},
			}

			b := &WOCSDKBroadcaster{woc: mock}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			txid, err := b.Broadcast(ctx, "deadbeef")
			require.NoError(t, err)
			assert.Equal(t, testValidTxID, txid)
		})
	}
}

func TestWoCSDKBroadcaster_AlreadyInMempool_NoTxID(t *testing.T) {
	t.Parallel()

	// Error messages without a valid txid should return an error.
	tests := []struct {
		name string
		err  error
	}{
		{"already in mempool", errTestAlreadyInMempool},
		{"already in the mempool", errTestAlreadyInTheMempool},
		{"txn-already-known", errTestTxnAlreadyKnown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testErr := tt.err
			mock := &mockWOCClient{
				broadcastFunc: func(_ context.Context, _ string) (string, error) {
					return "", testErr
				},
			}

			b := &WOCSDKBroadcaster{woc: mock}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := b.Broadcast(ctx, "deadbeef")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "already in mempool")
		})
	}
}

func TestWoCSDKBroadcaster_Error(t *testing.T) {
	t.Parallel()

	mock := &mockWOCClient{
		broadcastFunc: func(_ context.Context, _ string) (string, error) {
			return "", errTestMissingInputs
		},
	}

	b := &WOCSDKBroadcaster{woc: mock}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.Broadcast(ctx, "deadbeef")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Missing inputs")
}

func TestWoCSDKBroadcaster_EmptyResponse(t *testing.T) {
	t.Parallel()

	mock := &mockWOCClient{
		broadcastFunc: func(_ context.Context, _ string) (string, error) {
			return "", nil
		},
	}

	b := &WOCSDKBroadcaster{woc: mock}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.Broadcast(ctx, "deadbeef")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty txid")
}

func TestWoCSDKBroadcaster_Name(t *testing.T) {
	t.Parallel()

	b := &WOCSDKBroadcaster{}
	assert.Equal(t, "whatsonchain", b.Name())
}

// --- GorillaPool ARC Broadcaster Tests ---

func TestARCBroadcaster_Success(t *testing.T) {
	t.Parallel()

	expectedTxID := "b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"blockHash": "", "blockHeight": 0, "extraInfo": "",
			"status": 200, "timestamp": "2026-01-01T00:00:00Z",
			"title": "OK", "txStatus": "SEEN_ON_NETWORK",
			"txid": expectedTxID, "merklePath": "",
		})
	}))
	defer server.Close()

	b := &GorillaPoolARCBroadcaster{BaseURL: server.URL, httpClient: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	txid, err := b.Broadcast(ctx, "deadbeef")
	require.NoError(t, err)
	assert.Equal(t, expectedTxID, txid)
}

func TestARCBroadcaster_RequestFormat(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.URL.Path, "/v1/tx")

		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		assert.NoError(t, json.Unmarshal(body, &payload))
		assert.Equal(t, "deadbeef", payload["rawTx"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(arcTXInfo{TxID: "c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2", TXStatus: "SEEN_ON_NETWORK"})
	}))
	defer server.Close()

	b := &GorillaPoolARCBroadcaster{BaseURL: server.URL, httpClient: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.Broadcast(ctx, "deadbeef")
	require.NoError(t, err)
}

func TestARCBroadcaster_ErrorWithDetail(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(463)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":   "https://bitcoin-sv.github.io/arc/#/errors?id=_463",
			"title":  "Invalid outputs",
			"status": 463,
			"detail": "Transaction output 0 satoshis is invalid",
		})
	}))
	defer server.Close()

	b := &GorillaPoolARCBroadcaster{BaseURL: server.URL, httpClient: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.Broadcast(ctx, "deadbeef")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Transaction output 0 satoshis is invalid")
}

func TestARCBroadcaster_FeeTooLow(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(arcStatusFeeTooLow)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":   "https://bitcoin-sv.github.io/arc/#/errors?id=_465",
			"title":  "Fee too low",
			"status": arcStatusFeeTooLow,
			"detail": "The fee in the transaction is too low",
		})
	}))
	defer server.Close()

	b := &GorillaPoolARCBroadcaster{BaseURL: server.URL, httpClient: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.Broadcast(ctx, "deadbeef")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fee too low")
}

func TestARCBroadcaster_Unauthorized(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":   "https://bitcoin-sv.github.io/arc/#/errors",
			"title":  "Unauthorized",
			"status": 401,
			"detail": "Missing or invalid API key",
		})
	}))
	defer server.Close()

	b := &GorillaPoolARCBroadcaster{BaseURL: server.URL, httpClient: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.Broadcast(ctx, "deadbeef")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestARCBroadcaster_NetworkError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	server.Close()

	b := &GorillaPoolARCBroadcaster{BaseURL: server.URL, httpClient: &http.Client{Timeout: time.Second}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.Broadcast(ctx, "deadbeef")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network")
}

func TestARCBroadcaster_EmptyTxID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"txid":     "",
			"txStatus": "UNKNOWN",
			"status":   200,
		})
	}))
	defer server.Close()

	b := &GorillaPoolARCBroadcaster{BaseURL: server.URL, httpClient: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.Broadcast(ctx, "deadbeef")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty txid")
}

// --- Fallback Behavior Tests ---

// mockBroadcaster is a test broadcaster that records calls.
type mockBroadcaster struct {
	name   string
	txid   string
	err    error
	called atomic.Int64
}

func (m *mockBroadcaster) Name() string { return m.name }

func (m *mockBroadcaster) Broadcast(_ context.Context, _ string) (string, error) {
	m.called.Add(1)
	return m.txid, m.err
}

func TestBroadcastFallback_PrimarySucceeds(t *testing.T) {
	t.Parallel()

	primary := &mockBroadcaster{name: "primary", txid: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	secondary := &mockBroadcaster{name: "secondary", txid: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}

	client := &Client{
		broadcasters: []Broadcaster{primary, secondary},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	txid, err := client.BroadcastTransaction(ctx, []byte{0xde, 0xad})
	require.NoError(t, err)
	assert.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", txid)
	assert.Equal(t, int64(1), primary.called.Load())
	assert.Equal(t, int64(0), secondary.called.Load()) // Not called.
}

func TestBroadcastFallback_PrimaryFailsSecondarySucceeds(t *testing.T) {
	t.Parallel()

	primary := &mockBroadcaster{name: "primary", err: ErrBroadcastFailed}
	secondary := &mockBroadcaster{name: "secondary", txid: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}

	client := &Client{
		broadcasters: []Broadcaster{primary, secondary},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	txid, err := client.BroadcastTransaction(ctx, []byte{0xde, 0xad})
	require.NoError(t, err)
	assert.Equal(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", txid)
	assert.Equal(t, int64(1), primary.called.Load())
	assert.Equal(t, int64(1), secondary.called.Load())
}

func TestBroadcastFallback_AllFail(t *testing.T) {
	t.Parallel()

	primary := &mockBroadcaster{name: "primary", err: ErrBroadcastFailed}
	secondary := &mockBroadcaster{name: "secondary", err: ErrBroadcastFailed}

	client := &Client{
		broadcasters: []Broadcaster{primary, secondary},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.BroadcastTransaction(ctx, []byte{0xde, 0xad})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all providers failed")
}

func TestBroadcastFallback_NoBroadcasters(t *testing.T) {
	t.Parallel()

	client := &Client{
		broadcasters: []Broadcaster{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.BroadcastTransaction(ctx, []byte{0xde, 0xad})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no broadcast providers")
}

func TestBroadcastFallback_AlreadyKnownWithTxID(t *testing.T) {
	t.Parallel()

	// WoC SDK "already in mempool" with a txid in the message returns success.
	mock := &mockWOCClient{
		broadcastFunc: func(_ context.Context, _ string) (string, error) {
			return "", fmt.Errorf("%w: %s", errTestMempoolBase, testValidTxID)
		},
	}

	secondary := &mockBroadcaster{name: "secondary", txid: "should_not_be_called"}

	client := &Client{
		broadcasters: []Broadcaster{
			&WOCSDKBroadcaster{woc: mock},
			secondary,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	txid, err := client.BroadcastTransaction(ctx, []byte{0xde, 0xad})
	require.NoError(t, err)
	assert.Equal(t, testValidTxID, txid)
	assert.Equal(t, int64(0), secondary.called.Load()) // Not called.
}

// --- Logger Tests ---

// testLogger captures log messages for assertions.
type testLogger struct {
	debugMsgs []string
	errorMsgs []string
}

func (l *testLogger) Debug(format string, args ...any) {
	l.debugMsgs = append(l.debugMsgs, fmt.Sprintf(format, args...))
}

func (l *testLogger) Error(format string, args ...any) {
	l.errorMsgs = append(l.errorMsgs, fmt.Sprintf(format, args...))
}

func TestBroadcastTransaction_LogsErrorOnFailure(t *testing.T) {
	t.Parallel()

	logger := &testLogger{}
	primary := &mockBroadcaster{name: "primary", err: ErrBroadcastFailed}
	secondary := &mockBroadcaster{name: "secondary", err: ErrBroadcastFailed}

	client := &Client{
		broadcasters: []Broadcaster{primary, secondary},
		logger:       logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.BroadcastTransaction(ctx, []byte{0xde, 0xad})
	require.Error(t, err)

	// Should have error-level logs for each failure and the final "all failed" message
	require.Len(t, logger.errorMsgs, 3)
	assert.Contains(t, logger.errorMsgs[0], "broadcast failed via primary")
	assert.Contains(t, logger.errorMsgs[1], "broadcast failed via secondary")
	assert.Contains(t, logger.errorMsgs[2], "all broadcast providers failed")
}

func TestBroadcastTransaction_LogsDebugOnSuccess(t *testing.T) {
	t.Parallel()

	logger := &testLogger{}
	primary := &mockBroadcaster{name: "primary", txid: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}

	client := &Client{
		broadcasters: []Broadcaster{primary},
		logger:       logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	txid, err := client.BroadcastTransaction(ctx, []byte{0xde, 0xad})
	require.NoError(t, err)
	assert.Equal(t, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", txid)

	// Should have debug logs for attempt and success
	require.Len(t, logger.debugMsgs, 2)
	assert.Contains(t, logger.debugMsgs[0], "broadcasting via primary")
	assert.Contains(t, logger.debugMsgs[1], "broadcast successful via primary")
	assert.Empty(t, logger.errorMsgs)
}
