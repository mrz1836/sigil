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
)

func TestGetGasOracle(t *testing.T) {
	t.Parallel()

	t.Run("successful response returns gas prices", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "gastracker", r.URL.Query().Get("module"))
			assert.Equal(t, "gasoracle", r.URL.Query().Get("action"))
			assert.Equal(t, "1", r.URL.Query().Get("chainid"))
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			resp := gasOracleAPIResponse{
				Status:  "1",
				Message: "OK",
				Result: GasOracleResult{
					SafeGasPrice:    "8",
					ProposeGasPrice: "10",
					FastGasPrice:    "12",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.GetGasOracle(ctx)
		require.NoError(t, err)
		assert.Equal(t, "8", result.SafeGasPrice)
		assert.Equal(t, "10", result.ProposeGasPrice)
		assert.Equal(t, "12", result.FastGasPrice)
	})

	t.Run("handles decimal gas prices", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := gasOracleAPIResponse{
				Status:  "1",
				Message: "OK",
				Result: GasOracleResult{
					SafeGasPrice:    "7.5",
					ProposeGasPrice: "10.25",
					FastGasPrice:    "15.75",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		result, err := client.GetGasOracle(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "7.5", result.SafeGasPrice)
		assert.Equal(t, "10.25", result.ProposeGasPrice)
		assert.Equal(t, "15.75", result.FastGasPrice)
	})

	t.Run("handles API error status", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := gasOracleAPIResponse{
				Status:  "0",
				Message: "NOTOK",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.GetGasOracle(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrRateLimited)
	})

	t.Run("handles non-NOTOK API error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := gasOracleAPIResponse{
				Status:  "0",
				Message: "Invalid API Key",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		_, err = client.GetGasOracle(context.Background())
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrGasOracleFailed)
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

		_, err = client.GetGasOracle(context.Background())
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

		_, err = client.GetGasOracle(context.Background())
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

		_, err = client.GetGasOracle(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing gas oracle response")
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			resp := gasOracleAPIResponse{
				Status:  "1",
				Message: "OK",
				Result:  GasOracleResult{SafeGasPrice: "10", ProposeGasPrice: "15", FastGasPrice: "20"},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err = client.GetGasOracle(ctx)
		require.Error(t, err)
	})

	t.Run("uses custom chain ID", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "137", r.URL.Query().Get("chainid"))

			resp := gasOracleAPIResponse{
				Status:  "1",
				Message: "OK",
				Result:  GasOracleResult{SafeGasPrice: "30", ProposeGasPrice: "35", FastGasPrice: "40"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL, ChainID: "137"})
		require.NoError(t, err)

		result, err := client.GetGasOracle(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "30", result.SafeGasPrice)
	})
}

func TestGweiToWei(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected *big.Int
		wantErr  bool
	}{
		{
			name:     "integer gwei",
			input:    "10",
			expected: big.NewInt(10_000_000_000),
		},
		{
			name:     "single digit gwei",
			input:    "1",
			expected: big.NewInt(1_000_000_000),
		},
		{
			name:     "large gwei",
			input:    "100",
			expected: big.NewInt(100_000_000_000),
		},
		{
			name:     "decimal gwei half",
			input:    "7.5",
			expected: big.NewInt(7_500_000_000),
		},
		{
			name:     "decimal gwei quarter",
			input:    "10.25",
			expected: big.NewInt(10_250_000_000),
		},
		{
			name:     "decimal gwei three quarters",
			input:    "15.75",
			expected: big.NewInt(15_750_000_000),
		},
		{
			name:     "decimal gwei many places",
			input:    "1.123456789",
			expected: big.NewInt(1_123_456_789),
		},
		{
			name:     "zero",
			input:    "0",
			expected: big.NewInt(0),
		},
		{
			name:     "zero point zero",
			input:    "0.0",
			expected: big.NewInt(0),
		},
		{
			name:     "very small decimal",
			input:    "0.000000001",
			expected: big.NewInt(1),
		},
		{
			name:     "decimal truncated beyond 9 places",
			input:    "1.1234567899999",
			expected: big.NewInt(1_123_456_789),
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid string",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid fractional part",
			input:   "10.abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := gweiToWei(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, 0, tt.expected.Cmp(result),
					"expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGasPriceAdapter_GetGasPrices(t *testing.T) {
	t.Parallel()

	t.Run("converts gwei strings to wei correctly", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := gasOracleAPIResponse{
				Status:  "1",
				Message: "OK",
				Result: GasOracleResult{
					SafeGasPrice:    "8",
					ProposeGasPrice: "10",
					FastGasPrice:    "12",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		adapter := NewGasPriceAdapter(client)
		slow, medium, fast, err := adapter.GetGasPrices(context.Background())
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(8_000_000_000), slow)    // 8 Gwei
		assert.Equal(t, big.NewInt(10_000_000_000), medium) // 10 Gwei
		assert.Equal(t, big.NewInt(12_000_000_000), fast)   // 12 Gwei
	})

	t.Run("converts decimal gwei to wei", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := gasOracleAPIResponse{
				Status:  "1",
				Message: "OK",
				Result: GasOracleResult{
					SafeGasPrice:    "7.5",
					ProposeGasPrice: "10.25",
					FastGasPrice:    "15.75",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		adapter := NewGasPriceAdapter(client)
		slow, medium, fast, err := adapter.GetGasPrices(context.Background())
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(7_500_000_000), slow)
		assert.Equal(t, big.NewInt(10_250_000_000), medium)
		assert.Equal(t, big.NewInt(15_750_000_000), fast)
	})

	t.Run("returns error when oracle fails", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		adapter := NewGasPriceAdapter(client)
		_, _, _, err = adapter.GetGasPrices(context.Background())
		require.Error(t, err)
	})

	t.Run("returns error for invalid SafeGasPrice", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := gasOracleAPIResponse{
				Status:  "1",
				Message: "OK",
				Result: GasOracleResult{
					SafeGasPrice:    "invalid",
					ProposeGasPrice: "10",
					FastGasPrice:    "12",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		adapter := NewGasPriceAdapter(client)
		_, _, _, err = adapter.GetGasPrices(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SafeGasPrice")
	})

	t.Run("returns error for invalid ProposeGasPrice", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := gasOracleAPIResponse{
				Status:  "1",
				Message: "OK",
				Result: GasOracleResult{
					SafeGasPrice:    "8",
					ProposeGasPrice: "invalid",
					FastGasPrice:    "12",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		adapter := NewGasPriceAdapter(client)
		_, _, _, err = adapter.GetGasPrices(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ProposeGasPrice")
	})

	t.Run("returns error for invalid FastGasPrice", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := gasOracleAPIResponse{
				Status:  "1",
				Message: "OK",
				Result: GasOracleResult{
					SafeGasPrice:    "8",
					ProposeGasPrice: "10",
					FastGasPrice:    "invalid",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		adapter := NewGasPriceAdapter(client)
		_, _, _, err = adapter.GetGasPrices(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "FastGasPrice")
	})

	t.Run("slow <= medium <= fast ordering", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			resp := gasOracleAPIResponse{
				Status:  "1",
				Message: "OK",
				Result: GasOracleResult{
					SafeGasPrice:    "5",
					ProposeGasPrice: "20",
					FastGasPrice:    "50",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewClient("test-key", &ClientOptions{BaseURL: server.URL})
		require.NoError(t, err)

		adapter := NewGasPriceAdapter(client)
		slow, medium, fast, err := adapter.GetGasPrices(context.Background())
		require.NoError(t, err)

		assert.LessOrEqual(t, slow.Cmp(medium), 0, "slow should be <= medium")
		assert.LessOrEqual(t, medium.Cmp(fast), 0, "medium should be <= fast")
	})
}

func TestNewGasPriceAdapter(t *testing.T) {
	t.Parallel()

	t.Run("creates adapter with client", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("test-key", nil)
		require.NoError(t, err)

		adapter := NewGasPriceAdapter(client)
		assert.NotNil(t, adapter)
		assert.Equal(t, client, adapter.client)
	})
}
