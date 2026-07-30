package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain/httpx"
)

func TestRPCError_ErrorString(t *testing.T) {
	t.Parallel()

	err := &rpcError{Code: -32000, Message: "execution reverted"}
	assert.Equal(t, "RPC error -32000: execution reverted", err.Error())
}

func TestEstimateGas(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "eth_estimateGas", req["method"])
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  "0x5208", // 21000
		}))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gas, err := client.EstimateGas(ctx, CallMsg{To: "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"})
	require.NoError(t, err)
	assert.Equal(t, uint64(21000), gas)
}

func TestGetBalance_RPCError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"error":   map[string]any{"code": -32000, "message": "boom"},
		}))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The application-level RPC error must surface as a Go error.
	_, err := client.GetBalance(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", "latest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestClient_Close(t *testing.T) {
	t.Parallel()

	// Close releases idle connections and must be safe to call.
	client := NewClient("http://localhost:0")
	assert.NotPanics(t, client.Close)
}

func TestClient_HandleHTTPError(t *testing.T) {
	t.Parallel()

	client := NewClient("http://unused.invalid")

	tests := []struct {
		name    string
		status  int
		wantErr error
	}{
		{name: "429 rate limited", status: http.StatusTooManyRequests, wantErr: ErrRPCRateLimited},
		{name: "408 timeout", status: http.StatusRequestTimeout, wantErr: ErrRPCTimeout},
		{name: "504 gateway timeout", status: http.StatusGatewayTimeout, wantErr: ErrRPCTimeout},
		{name: "500 retryable", status: http.StatusInternalServerError, wantErr: ErrRPCRetryable},
		{name: "502 retryable", status: http.StatusBadGateway, wantErr: ErrRPCRetryable},
		{name: "400 generic request error", status: http.StatusBadRequest, wantErr: ErrRPCRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := &httpx.Response{
				StatusCode: tc.status,
				Header:     http.Header{"Retry-After": []string{"5"}},
				Body:       []byte("upstream error body"),
			}
			err := client.handleHTTPError(resp)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}
