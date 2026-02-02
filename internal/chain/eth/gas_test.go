package eth

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

const (
	rpcMethodChainID  = "eth_chainId"
	rpcMethodGasPrice = "eth_gasPrice"
)

func TestParseGasSpeed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected GasSpeed
		wantErr  bool
	}{
		{
			name:     "slow",
			input:    "slow",
			expected: GasSpeedSlow,
			wantErr:  false,
		},
		{
			name:     "medium",
			input:    "medium",
			expected: GasSpeedMedium,
			wantErr:  false,
		},
		{
			name:     "fast",
			input:    "fast",
			expected: GasSpeedFast,
			wantErr:  false,
		},
		{
			name:     "empty string defaults to slow",
			input:    "",
			expected: GasSpeedSlow,
			wantErr:  false,
		},
		{
			name:    "invalid speed",
			input:   "turbo",
			wantErr: true,
		},
		{
			name:    "invalid speed - uppercase",
			input:   "FAST",
			wantErr: true,
		},
		{
			name:    "invalid speed - mixed case",
			input:   "Medium",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			speed, err := ParseGasSpeed(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "slow, medium, or fast")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, speed)
			}
		})
	}
}

func TestFormatGasPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		weiPrice *big.Int
		expected string
	}{
		{
			name:     "nil returns 0 Gwei",
			weiPrice: nil,
			expected: "0 Gwei",
		},
		{
			name:     "zero wei",
			weiPrice: big.NewInt(0),
			expected: "0.00 Gwei",
		},
		{
			name:     "1 Gwei (10^9 wei)",
			weiPrice: big.NewInt(1_000_000_000),
			expected: "1.00 Gwei",
		},
		{
			name:     "20 Gwei",
			weiPrice: big.NewInt(20_000_000_000),
			expected: "20.00 Gwei",
		},
		{
			name:     "0.5 Gwei",
			weiPrice: big.NewInt(500_000_000),
			expected: "0.50 Gwei",
		},
		{
			name:     "100 Gwei",
			weiPrice: big.NewInt(100_000_000_000),
			expected: "100.00 Gwei",
		},
		{
			name:     "1.23 Gwei",
			weiPrice: big.NewInt(1_230_000_000),
			expected: "1.23 Gwei",
		},
		{
			name:     "tiny amount (1 wei)",
			weiPrice: big.NewInt(1),
			expected: "0.00 Gwei",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := FormatGasPrice(tt.weiPrice)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMultiplyBigInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      *big.Int
		multiplier float64
		// For floating point math, we check within a tolerance
		minExpected *big.Int
		maxExpected *big.Int
	}{
		{
			name:        "multiply by 1.0 (identity)",
			value:       big.NewInt(1000),
			multiplier:  1.0,
			minExpected: big.NewInt(1000),
			maxExpected: big.NewInt(1000),
		},
		{
			name:        "multiply by 0.8 (slow gas multiplier)",
			value:       big.NewInt(1000),
			multiplier:  0.8,
			minExpected: big.NewInt(799),
			maxExpected: big.NewInt(800),
		},
		{
			name:        "multiply by 1.2 (fast gas multiplier)",
			value:       big.NewInt(1000),
			multiplier:  1.2,
			minExpected: big.NewInt(1199),
			maxExpected: big.NewInt(1200),
		},
		{
			name:        "multiply by 0.5",
			value:       big.NewInt(100),
			multiplier:  0.5,
			minExpected: big.NewInt(50),
			maxExpected: big.NewInt(50),
		},
		{
			name:        "multiply by 2.0",
			value:       big.NewInt(100),
			multiplier:  2.0,
			minExpected: big.NewInt(200),
			maxExpected: big.NewInt(200),
		},
		{
			name:        "multiply zero",
			value:       big.NewInt(0),
			multiplier:  1.5,
			minExpected: big.NewInt(0),
			maxExpected: big.NewInt(0),
		},
		{
			name:        "multiply by zero",
			value:       big.NewInt(1000),
			multiplier:  0.0,
			minExpected: big.NewInt(0),
			maxExpected: big.NewInt(0),
		},
		{
			name:        "large value with slow multiplier",
			value:       big.NewInt(20_000_000_000), // 20 Gwei
			multiplier:  0.8,
			minExpected: big.NewInt(15_999_999_999),
			maxExpected: big.NewInt(16_000_000_000),
		},
		{
			name:        "large value with fast multiplier",
			value:       big.NewInt(20_000_000_000), // 20 Gwei
			multiplier:  1.2,
			minExpected: big.NewInt(23_999_999_999),
			maxExpected: big.NewInt(24_000_000_000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := multiplyBigInt(tt.value, tt.multiplier)
			// Check result is within expected range (due to floating point precision)
			assert.GreaterOrEqual(t, result.Cmp(tt.minExpected), 0,
				"result %s should be >= %s", result, tt.minExpected)
			assert.LessOrEqual(t, result.Cmp(tt.maxExpected), 0,
				"result %s should be <= %s", result, tt.maxExpected)
		})
	}
}

func TestGasSpeedConstants(t *testing.T) {
	t.Parallel()

	t.Run("gas speed values", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, GasSpeedSlow, GasSpeed("slow"))
		assert.Equal(t, GasSpeedMedium, GasSpeed("medium"))
		assert.Equal(t, GasSpeedFast, GasSpeed("fast"))
	})

	t.Run("gas limit constants", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, GasLimitETHTransfer, uint64(21000))
		assert.Equal(t, GasLimitERC20Transfer, uint64(65000))
		assert.Equal(t, GasLimitERC20Approve, uint64(50000))
	})
}

func TestGasEstimateStruct(t *testing.T) {
	t.Parallel()

	t.Run("gas estimate initialization", func(t *testing.T) {
		t.Parallel()
		estimate := &GasEstimate{
			GasPrice: big.NewInt(20_000_000_000), // 20 Gwei
			GasLimit: 21000,
			Total:    big.NewInt(420_000_000_000_000), // 0.00042 ETH
		}

		assert.Equal(t, big.NewInt(20_000_000_000), estimate.GasPrice)
		assert.Equal(t, uint64(21000), estimate.GasLimit)
		assert.Equal(t, big.NewInt(420_000_000_000_000), estimate.Total)
	})
}

func TestGasPricesStruct(t *testing.T) {
	t.Parallel()

	t.Run("gas prices initialization", func(t *testing.T) {
		t.Parallel()
		prices := &GasPrices{
			Slow:   big.NewInt(16_000_000_000), // 16 Gwei
			Medium: big.NewInt(20_000_000_000), // 20 Gwei
			Fast:   big.NewInt(24_000_000_000), // 24 Gwei
		}

		assert.Negative(t, prices.Slow.Cmp(prices.Medium))
		assert.Negative(t, prices.Medium.Cmp(prices.Fast))
	})
}

func TestGetGasPrices(t *testing.T) {
	t.Parallel()

	t.Run("returns gas prices for all speeds", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case rpcMethodChainID:
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1",
				}
			case rpcMethodGasPrice:
				// 20 Gwei
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x4a817c800", // 20 Gwei
				}
			default:
				t.Errorf("unexpected method: %s", method)
				return
			}

			err = json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		prices, err := client.GetGasPrices(ctx)
		require.NoError(t, err)

		// Slow should be less than medium
		assert.Negative(t, prices.Slow.Cmp(prices.Medium))
		// Fast should be greater than medium
		assert.Positive(t, prices.Fast.Cmp(prices.Medium))
	})
}

func TestGetGasPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		speed GasSpeed
	}{
		{name: "slow speed", speed: GasSpeedSlow},
		{name: "medium speed", speed: GasSpeedMedium},
		{name: "fast speed", speed: GasSpeedFast},
		{name: "unknown speed defaults to medium", speed: GasSpeed("unknown")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req map[string]any
				err := json.NewDecoder(r.Body).Decode(&req)
				assert.NoError(t, err)

				method := req["method"].(string)
				var resp map[string]any

				switch method {
				case rpcMethodChainID:
					resp = map[string]any{
						"jsonrpc": "2.0",
						"id":      req["id"],
						"result":  "0x1",
					}
				case rpcMethodGasPrice:
					resp = map[string]any{
						"jsonrpc": "2.0",
						"id":      req["id"],
						"result":  "0x4a817c800", // 20 Gwei
					}
				default:
					t.Errorf("unexpected method: %s", method)
					return
				}

				err = json.NewEncoder(w).Encode(resp)
				assert.NoError(t, err)
			}))
			defer server.Close()

			client, err := NewClient(server.URL, nil)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			price, err := client.GetGasPrice(ctx, tt.speed)
			require.NoError(t, err)
			assert.NotNil(t, price)
			assert.Positive(t, price.Sign())
		})
	}
}

func TestEstimateGasForETHTransfer(t *testing.T) {
	t.Parallel()

	t.Run("returns estimate for ETH transfer", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case rpcMethodChainID:
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1",
				}
			case rpcMethodGasPrice:
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x4a817c800", // 20 Gwei
				}
			default:
				t.Errorf("unexpected method: %s", method)
				return
			}

			err = json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		estimate, err := client.EstimateGasForETHTransfer(ctx, GasSpeedMedium)
		require.NoError(t, err)

		assert.NotNil(t, estimate.GasPrice)
		assert.Equal(t, GasLimitETHTransfer, estimate.GasLimit)
		assert.NotNil(t, estimate.Total)
	})
}

func TestEstimateGasForERC20Transfer(t *testing.T) {
	t.Parallel()

	t.Run("returns estimate for ERC20 transfer", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case rpcMethodChainID:
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1",
				}
			case rpcMethodGasPrice:
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x4a817c800", // 20 Gwei
				}
			default:
				t.Errorf("unexpected method: %s", method)
				return
			}

			err = json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		estimate, err := client.EstimateGasForERC20Transfer(ctx, GasSpeedMedium)
		require.NoError(t, err)

		assert.NotNil(t, estimate.GasPrice)
		assert.Equal(t, GasLimitERC20Transfer, estimate.GasLimit)
		assert.NotNil(t, estimate.Total)
	})
}

func TestGetNonce(t *testing.T) {
	t.Parallel()

	t.Run("returns nonce for valid address", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case rpcMethodChainID:
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1",
				}
			case "eth_getTransactionCount":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x5", // nonce = 5
				}
			default:
				t.Errorf("unexpected method: %s", method)
				return
			}

			err = json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		nonce, err := client.GetNonce(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.NoError(t, err)
		assert.Equal(t, uint64(5), nonce)
	})

	t.Run("returns error for invalid address", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case rpcMethodChainID:
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1",
				}
			default:
				t.Errorf("unexpected method: %s", method)
				return
			}

			err = json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = client.GetNonce(ctx, "invalid")
		require.Error(t, err)
	})
}

func TestGetChainID(t *testing.T) {
	t.Parallel()

	t.Run("returns chain ID", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case rpcMethodChainID:
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1", // Mainnet
				}
			default:
				t.Errorf("unexpected method: %s", method)
				return
			}

			err = json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		chainID, err := client.GetChainID(ctx)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(1), chainID)
	})
}

func TestEstimateGasWithData(t *testing.T) {
	t.Parallel()

	t.Run("returns estimate for transaction with data", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case rpcMethodChainID:
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1",
				}
			case rpcMethodGasPrice:
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x4a817c800", // 20 Gwei
				}
			default:
				t.Errorf("unexpected method: %s", method)
				return
			}

			err = json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		data := []byte{0xa9, 0x05, 0x9c, 0xbb} // ERC20 transfer selector
		estimate, err := client.EstimateGasWithData(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", data, GasSpeedMedium)
		require.NoError(t, err)

		assert.NotNil(t, estimate.GasPrice)
		assert.Positive(t, estimate.GasLimit)
		assert.NotNil(t, estimate.Total)
	})

	t.Run("returns error for invalid address", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result":  "0x1",
			}
			err = json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = client.EstimateGasWithData(ctx, "invalid", []byte{}, GasSpeedMedium)
		require.Error(t, err)
	})
}
