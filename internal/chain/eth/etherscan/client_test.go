package etherscan

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain/eth"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("creates client with valid API key", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("test-key", nil)
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, DefaultBaseURL, client.baseURL)
		assert.Equal(t, DefaultChainID, client.chainID)
	})

	t.Run("returns error for empty API key", func(t *testing.T) {
		t.Parallel()
		_, err := NewClient("", nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAPIKeyRequired)
	})

	t.Run("applies custom base URL", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("test-key", &ClientOptions{BaseURL: "https://custom.api"})
		require.NoError(t, err)
		assert.Equal(t, "https://custom.api", client.baseURL)
	})

	t.Run("applies custom HTTP client", func(t *testing.T) {
		t.Parallel()
		httpClient := &http.Client{Timeout: 5 * time.Second}
		client, err := NewClient("test-key", &ClientOptions{HTTPClient: httpClient})
		require.NoError(t, err)
		assert.Equal(t, httpClient, client.httpClient)
	})

	t.Run("applies custom chain ID", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("test-key", &ClientOptions{ChainID: "137"})
		require.NoError(t, err)
		assert.Equal(t, "137", client.chainID)
	})

	t.Run("defaults chain ID when not specified", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("test-key", &ClientOptions{BaseURL: "https://custom.api"})
		require.NoError(t, err)
		assert.Equal(t, DefaultChainID, client.chainID)
	})
}

func TestGetNativeBalance(t *testing.T) {
	t.Parallel()

	t.Run("returns ETH balance for valid address", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "account", r.URL.Query().Get("module"))
			assert.Equal(t, "balance", r.URL.Query().Get("action"))
			assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", r.URL.Query().Get("address"))
			assert.Equal(t, "latest", r.URL.Query().Get("tag"))
			assert.Equal(t, "test-key", r.URL.Query().Get("apikey"))
			assert.Equal(t, "1", r.URL.Query().Get("chainid"))

			resp := apiResponse{
				Status:  "1",
				Message: "OK",
				Result:  "1000000000000000000", // 1 ETH in wei
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		balance, err := client.GetNativeBalance(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.NoError(t, err)
		assert.Equal(t, "ETH", balance.Symbol)
		assert.Equal(t, 18, balance.Decimals)
		assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", balance.Address)
		expected := new(big.Int)
		expected.SetString("1000000000000000000", 10)
		assert.Equal(t, expected, balance.Amount)
	})

	t.Run("returns zero balance", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := apiResponse{Status: "1", Message: "OK", Result: "0"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		balance, err := client.GetNativeBalance(context.Background(), "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(0), balance.Amount)
	})

	t.Run("handles API error response", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := apiResponse{
				Status:  "0",
				Message: "NOTOK",
				Result:  "Error! Invalid address format",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.GetNativeBalance(context.Background(), "0xinvalid")
		require.Error(t, err)
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

		_, err = client.GetNativeBalance(context.Background(), "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.Error(t, err)
	})

	t.Run("handles JSON rate limit message", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := apiResponse{
				Status:  "0",
				Message: "NOTOK",
				Result:  "Max rate limit reached",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.GetNativeBalance(context.Background(), "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRateLimited)
	})

	t.Run("handles non-200 status code", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal error"))
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.GetNativeBalance(context.Background(), "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.Error(t, err)
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

		_, err = client.GetNativeBalance(context.Background(), "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.Error(t, err)
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(2 * time.Second)
			resp := apiResponse{Status: "1", Message: "OK", Result: "0"}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err = client.GetNativeBalance(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.Error(t, err)
	})

	t.Run("handles invalid number in result", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := apiResponse{Status: "1", Message: "OK", Result: "not-a-number"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.GetNativeBalance(context.Background(), "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidBalance)
	})
}

func TestGetTokenBalance(t *testing.T) {
	t.Parallel()

	t.Run("returns ERC20 token balance", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "account", r.URL.Query().Get("module"))
			assert.Equal(t, "tokenbalance", r.URL.Query().Get("action"))
			assert.Equal(t, eth.USDCMainnet, r.URL.Query().Get("contractaddress"))
			assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", r.URL.Query().Get("address"))
			assert.Equal(t, "latest", r.URL.Query().Get("tag"))
			assert.Equal(t, "1", r.URL.Query().Get("chainid"))

			resp := apiResponse{
				Status:  "1",
				Message: "OK",
				Result:  "500000000", // 500 USDC (6 decimals)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		balance, err := client.GetTokenBalance(context.Background(), "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", eth.USDCMainnet)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(500_000_000), balance.Amount)
		assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", balance.Address)
	})

	t.Run("handles API error", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := apiResponse{Status: "0", Message: "NOTOK", Result: "Invalid contract address"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.GetTokenBalance(context.Background(), "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", "0xinvalid")
		require.Error(t, err)
	})
}

func TestGetUSDCBalance(t *testing.T) {
	t.Parallel()

	t.Run("returns USDC balance with correct metadata", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, eth.USDCMainnet, r.URL.Query().Get("contractaddress"))

			resp := apiResponse{Status: "1", Message: "OK", Result: "500000000"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		balance, err := client.GetUSDCBalance(context.Background(), "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.NoError(t, err)
		assert.Equal(t, "USDC", balance.Symbol)
		assert.Equal(t, 6, balance.Decimals)
		assert.Equal(t, eth.USDCMainnet, balance.Token)
		assert.Equal(t, big.NewInt(500_000_000), balance.Amount)
	})
}

func TestCustomChainIDInRequest(t *testing.T) {
	t.Parallel()

	t.Run("sends custom chain ID in request", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "137", r.URL.Query().Get("chainid"))

			resp := apiResponse{Status: "1", Message: "OK", Result: "0"}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL, ChainID: "137"})
		require.NoError(t, err)

		_, err = client.GetNativeBalance(context.Background(), "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.NoError(t, err)
	})
}

func TestV2APIDeprecationError(t *testing.T) {
	t.Parallel()

	t.Run("handles v1 deprecation error response", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := apiResponse{
				Status:  "0",
				Message: "NOTOK",
				Result:  "You are using a deprecated V1 endpoint, switch to Etherscan API V2.",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.GetNativeBalance(context.Background(), "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAPIError)
	})
}

func TestDefaultBaseURL(t *testing.T) {
	t.Parallel()

	t.Run("uses v2 API endpoint", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "https://api.etherscan.io/v2", DefaultBaseURL)
	})
}

func TestTruncateBody(t *testing.T) {
	t.Parallel()

	t.Run("short string unchanged", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "hello", truncateBody("hello", 10))
	})

	t.Run("long string truncated", func(t *testing.T) {
		t.Parallel()
		result := truncateBody("hello world", 5)
		assert.Equal(t, "hello...", result)
	})

	t.Run("exact length unchanged", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "hello", truncateBody("hello", 5))
	})
}
