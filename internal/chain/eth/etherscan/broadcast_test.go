package etherscan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBroadcastRawTransaction(t *testing.T) {
	t.Parallel()

	t.Run("successful broadcast returns tx hash", func(t *testing.T) {
		t.Parallel()

		expectedHash := "0xabc123def456789012345678901234567890123456789012345678901234abcd"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "proxy", r.URL.Query().Get("module"))
			assert.Equal(t, "eth_sendRawTransaction", r.URL.Query().Get("action"))
			assert.Equal(t, "1", r.URL.Query().Get("chainid"))
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			// Verify hex param starts with 0x
			hexParam := r.URL.Query().Get("hex")
			assert.Greater(t, len(hexParam), 2, "hex param should be non-empty")
			assert.Equal(t, "0x", hexParam[:2])

			resp := proxyResponse{
				JSONRPC: "2.0",
				ID:      1,
				Result:  expectedHash,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		rawTx := []byte{0xf8, 0x65, 0x80, 0x85, 0x04, 0xa8, 0x17, 0xc8}
		txHash, err := client.BroadcastRawTransaction(ctx, rawTx)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, txHash)
	})

	t.Run("handles JSON-RPC error response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := proxyResponse{
				JSONRPC: "2.0",
				ID:      1,
				Error: &proxyRPCError{
					Code:    -32000,
					Message: "already known",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.BroadcastRawTransaction(context.Background(), []byte{0x01, 0x02})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrBroadcastFailed)
	})

	t.Run("handles empty result", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := proxyResponse{
				JSONRPC: "2.0",
				ID:      1,
				Result:  "",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.BroadcastRawTransaction(context.Background(), []byte{0x01})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrBroadcastFailed)
	})

	t.Run("handles HTTP 429 rate limiting", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("rate limited"))
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.BroadcastRawTransaction(context.Background(), []byte{0x01})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRateLimited)
	})

	t.Run("handles HTTP 500 error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal error"))
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.BroadcastRawTransaction(context.Background(), []byte{0x01})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAPIError)
	})

	t.Run("handles invalid JSON response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("not json"))
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.BroadcastRawTransaction(context.Background(), []byte{0x01})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing proxy response")
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			resp := proxyResponse{JSONRPC: "2.0", ID: 1, Result: "0xabc"}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err = client.BroadcastRawTransaction(ctx, []byte{0x01})
		require.Error(t, err)
	})

	t.Run("uses custom chain ID", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "137", r.URL.Query().Get("chainid"))

			resp := proxyResponse{JSONRPC: "2.0", ID: 1, Result: "0xabc123"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL, ChainID: "137"})
		require.NoError(t, err)

		txHash, err := client.BroadcastRawTransaction(context.Background(), []byte{0x01})
		require.NoError(t, err)
		assert.Equal(t, "0xabc123", txHash)
	})
}
