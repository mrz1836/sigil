package eth

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errOracleUnavailable = errors.New("oracle unavailable")

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
			name:     "empty string defaults to medium",
			input:    "",
			expected: GasSpeedMedium,
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

	testFrom := "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	testTo := "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359"
	testValue := big.NewInt(1000000000000000000) // 1 ETH

	t.Run("uses eth_estimateGas with buffer", func(t *testing.T) {
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
			case "eth_estimateGas":
				// Return 30000 gas (contract destination)
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x7530", // 30000
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

		estimate, err := client.EstimateGasForETHTransfer(ctx, testFrom, testTo, testValue, GasSpeedMedium)
		require.NoError(t, err)

		assert.NotNil(t, estimate.GasPrice)
		// 30000 * 1.2 ≈ 36000 (floating-point may round down by 1)
		assert.GreaterOrEqual(t, estimate.GasLimit, uint64(35999))
		assert.LessOrEqual(t, estimate.GasLimit, uint64(36000))
		assert.NotNil(t, estimate.Total)
	})

	t.Run("falls back to 21000 on estimateGas error", func(t *testing.T) {
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
			case "eth_estimateGas":
				// Return error to trigger fallback
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"error":   map[string]any{"code": -32000, "message": "execution reverted"},
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

		estimate, err := client.EstimateGasForETHTransfer(ctx, testFrom, testTo, testValue, GasSpeedMedium)
		require.NoError(t, err)

		assert.NotNil(t, estimate.GasPrice)
		assert.Equal(t, GasLimitETHTransfer, estimate.GasLimit)
		assert.NotNil(t, estimate.Total)
	})
}

func TestEstimateGasForERC20Transfer(t *testing.T) {
	t.Parallel()

	testFrom := "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
	testData, _ := BuildERC20TransferData("0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359", big.NewInt(1000000))

	t.Run("uses eth_estimateGas with buffer", func(t *testing.T) {
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
			case "eth_estimateGas":
				// Return 55000 gas for ERC-20 transfer
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0xd6d8", // 55000
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

		estimate, err := client.EstimateGasForERC20Transfer(ctx, testFrom, USDCMainnet, testData, GasSpeedMedium)
		require.NoError(t, err)

		assert.NotNil(t, estimate.GasPrice)
		// 55000 * 1.2 ≈ 66000 (floating-point may round down by 1)
		assert.GreaterOrEqual(t, estimate.GasLimit, uint64(65999))
		assert.LessOrEqual(t, estimate.GasLimit, uint64(66000))
		assert.NotNil(t, estimate.Total)
	})

	t.Run("falls back to 65000 on estimateGas error", func(t *testing.T) {
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
			case "eth_estimateGas":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"error":   map[string]any{"code": -32000, "message": "execution reverted"},
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

		estimate, err := client.EstimateGasForERC20Transfer(ctx, testFrom, USDCMainnet, testData, GasSpeedMedium)
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
			case "eth_estimateGas":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0xc350", // 50000
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
		// 50000 * 1.2 ≈ 60000 (floating-point may round down by 1)
		assert.GreaterOrEqual(t, estimate.GasLimit, uint64(59999))
		assert.LessOrEqual(t, estimate.GasLimit, uint64(60000))
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

// mockGasPriceOracle implements GasPriceOracle for testing.
type mockGasPriceOracle struct {
	slow, medium, fast *big.Int
	err                error
}

func (m *mockGasPriceOracle) GetGasPrices(_ context.Context) (slow, medium, fast *big.Int, err error) {
	if m.err != nil {
		return nil, nil, nil, m.err
	}
	return m.slow, m.medium, m.fast, nil
}

// newTestRPCServer creates a mock RPC server that responds to eth_chainId and eth_gasPrice.
func newTestRPCServer(t *testing.T, chainID, gasPriceHex string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				"result":  chainID,
			}
		case rpcMethodGasPrice:
			resp = map[string]any{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result":  gasPriceHex,
			}
		default:
			t.Errorf("unexpected method: %s", method)
			return
		}

		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
}

// newTestRPCServerGasPriceOnly creates a mock RPC server that only responds to eth_gasPrice.
// Used for fallback RPC servers that don't need to handle eth_chainId.
func newTestRPCServerGasPriceOnly(t *testing.T, gasPriceHex string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  gasPriceHex,
		}

		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
}

// newTestRPCServerError creates a mock RPC server that always returns errors.
func newTestRPCServerError(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			resp = map[string]any{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"error":   map[string]any{"code": -32000, "message": "internal error"},
			}
		}

		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
}

func TestGetGasPrices_OracleUsedWhenAvailable(t *testing.T) {
	t.Parallel()

	// Primary RPC returns 0.06 Gwei (way too low)
	primary := newTestRPCServer(t, "0x1", "0x3938700") // ~0.06 Gwei
	defer primary.Close()

	// Oracle returns sane prices
	oracle := &mockGasPriceOracle{
		slow:   big.NewInt(8_000_000_000),  // 8 Gwei
		medium: big.NewInt(10_000_000_000), // 10 Gwei
		fast:   big.NewInt(12_000_000_000), // 12 Gwei
	}

	client, err := NewClient(primary.URL, &ClientOptions{
		GasPriceOracle: oracle,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prices, err := client.GetGasPrices(ctx)
	require.NoError(t, err)

	// Should use oracle prices, not the low RPC price
	assert.Equal(t, big.NewInt(8_000_000_000), prices.Slow)
	assert.Equal(t, big.NewInt(10_000_000_000), prices.Medium)
	assert.Equal(t, big.NewInt(12_000_000_000), prices.Fast)
}

func TestGetGasPrices_OracleFailsFallsBackToMultiRPC(t *testing.T) {
	t.Parallel()

	// Primary RPC returns 20 Gwei
	primary := newTestRPCServer(t, "0x1", "0x4a817c800") // 20 Gwei
	defer primary.Close()

	// Oracle fails
	oracle := &mockGasPriceOracle{
		err: errOracleUnavailable,
	}

	client, err := NewClient(primary.URL, &ClientOptions{
		GasPriceOracle: oracle,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prices, err := client.GetGasPrices(ctx)
	require.NoError(t, err)

	// Should fall back to RPC price (20 Gwei) with multipliers
	assert.Equal(t, big.NewInt(20_000_000_000), prices.Medium)
	assert.Negative(t, prices.Slow.Cmp(prices.Medium), "slow should be less than medium")
	assert.Positive(t, prices.Fast.Cmp(prices.Medium), "fast should be greater than medium")
}

func TestGetGasPrices_MultiRPCMedian(t *testing.T) {
	t.Parallel()

	// Three RPCs returning different prices:
	// Primary: 0.06 Gwei (bad)
	// Fallback 1: 20 Gwei (good)
	// Fallback 2: 30 Gwei (good)
	// Median should be 20 Gwei
	primary := newTestRPCServer(t, "0x1", "0x3938700") // ~0.06 Gwei
	defer primary.Close()

	fallback1 := newTestRPCServerGasPriceOnly(t, "0x4a817c800") // 20 Gwei
	defer fallback1.Close()

	fallback2 := newTestRPCServerGasPriceOnly(t, "0x6fc23ac00") // 30 Gwei
	defer fallback2.Close()

	client, err := NewClient(primary.URL, &ClientOptions{
		FallbackRPCs: []string{fallback1.URL, fallback2.URL},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prices, err := client.GetGasPrices(ctx)
	require.NoError(t, err)

	// Median of [0.06 Gwei, 20 Gwei, 30 Gwei] = 20 Gwei
	assert.Equal(t, big.NewInt(20_000_000_000), prices.Medium)
}

func TestGetGasPrices_MultiRPCMedianTwoServers(t *testing.T) {
	t.Parallel()

	// Two RPCs: average of two values
	// Primary: 20 Gwei, Fallback: 30 Gwei → median = 25 Gwei
	primary := newTestRPCServer(t, "0x1", "0x4a817c800") // 20 Gwei
	defer primary.Close()

	fallback := newTestRPCServerGasPriceOnly(t, "0x6fc23ac00") // 30 Gwei
	defer fallback.Close()

	client, err := NewClient(primary.URL, &ClientOptions{
		FallbackRPCs: []string{fallback.URL},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prices, err := client.GetGasPrices(ctx)
	require.NoError(t, err)

	// Median of [20 Gwei, 30 Gwei] = (20 + 30) / 2 = 25 Gwei
	assert.Equal(t, big.NewInt(25_000_000_000), prices.Medium)
}

func TestGetGasPrices_FloorAppliedOnMainnet(t *testing.T) {
	t.Parallel()

	// RPC returns 0.06 Gwei on mainnet — floor should kick in
	primary := newTestRPCServer(t, "0x1", "0x3938700") // ~0.06 Gwei (60_000_000 wei)
	defer primary.Close()

	client, err := NewClient(primary.URL, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prices, err := client.GetGasPrices(ctx)
	require.NoError(t, err)

	// All prices should be at least 1 Gwei on mainnet
	floor := big.NewInt(1_000_000_000)
	assert.GreaterOrEqual(t, prices.Slow.Cmp(floor), 0,
		"slow %s should be >= floor %s", prices.Slow, floor)
	assert.GreaterOrEqual(t, prices.Medium.Cmp(floor), 0,
		"medium %s should be >= floor %s", prices.Medium, floor)
	assert.GreaterOrEqual(t, prices.Fast.Cmp(floor), 0,
		"fast %s should be >= floor %s", prices.Fast, floor)
}

func TestGetGasPrices_FloorNotAppliedOnL2(t *testing.T) {
	t.Parallel()

	// Very low gas price on a non-mainnet chain (e.g. L2 with chain ID 42161)
	primary := newTestRPCServer(t, "0xa4b1", "0x5f5e100") // 0.1 Gwei on Arbitrum
	defer primary.Close()

	client, err := NewClient(primary.URL, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prices, err := client.GetGasPrices(ctx)
	require.NoError(t, err)

	// On L2, 0.1 Gwei should pass through without floor
	assert.Negative(t, prices.Medium.Cmp(big.NewInt(1_000_000_000)),
		"on L2, sub-gwei prices should not be floored")
}

func TestGetGasPrices_FloorAppliedToOracleResults(t *testing.T) {
	t.Parallel()

	// Oracle returns very low prices on mainnet — floor should still apply
	primary := newTestRPCServer(t, "0x1", "0x4a817c800") // 20 Gwei
	defer primary.Close()

	oracle := &mockGasPriceOracle{
		slow:   big.NewInt(100_000_000), // 0.1 Gwei
		medium: big.NewInt(200_000_000), // 0.2 Gwei
		fast:   big.NewInt(300_000_000), // 0.3 Gwei
	}

	client, err := NewClient(primary.URL, &ClientOptions{
		GasPriceOracle: oracle,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prices, err := client.GetGasPrices(ctx)
	require.NoError(t, err)

	floor := big.NewInt(1_000_000_000) // 1 Gwei
	assert.Equal(t, 0, prices.Slow.Cmp(floor), "slow should be floored to 1 Gwei")
	assert.Equal(t, 0, prices.Medium.Cmp(floor), "medium should be floored to 1 Gwei")
	assert.Equal(t, 0, prices.Fast.Cmp(floor), "fast should be floored to 1 Gwei")
}

func TestGetGasPrices_AllRPCsFail(t *testing.T) {
	t.Parallel()

	// Primary and fallback RPCs all fail
	primary := newTestRPCServerError(t)
	defer primary.Close()

	fallback := newTestRPCServerError(t)
	defer fallback.Close()

	client, err := NewClient(primary.URL, &ClientOptions{
		FallbackRPCs: []string{fallback.URL},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.GetGasPrices(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all RPC endpoints failed")
}

func TestGetGasPrices_PrimaryFailsFallbackSucceeds(t *testing.T) {
	t.Parallel()

	// Primary RPC fails, but fallback returns 20 Gwei
	primary := newTestRPCServerError(t)
	defer primary.Close()

	fallback := newTestRPCServerGasPriceOnly(t, "0x4a817c800") // 20 Gwei
	defer fallback.Close()

	client, err := NewClient(primary.URL, &ClientOptions{
		FallbackRPCs: []string{fallback.URL},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prices, err := client.GetGasPrices(ctx)
	require.NoError(t, err)

	// Should use the fallback's price
	assert.Equal(t, big.NewInt(20_000_000_000), prices.Medium)
}

func TestGetGasPrices_SkipsDuplicateFallbackURL(t *testing.T) {
	t.Parallel()

	// Primary RPC URL included as a fallback — should be skipped
	primary := newTestRPCServer(t, "0x1", "0x4a817c800") // 20 Gwei
	defer primary.Close()

	client, err := NewClient(primary.URL, &ClientOptions{
		FallbackRPCs: []string{primary.URL}, // Same as primary
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prices, err := client.GetGasPrices(ctx)
	require.NoError(t, err)

	// Only primary should be queried (fallback is skipped), so medium = primary's price
	assert.Equal(t, big.NewInt(20_000_000_000), prices.Medium)
}

func TestGetGasPrices_SingleRPCNormalPrice(t *testing.T) {
	t.Parallel()

	// Single RPC returns a normal price — no fallbacks, no oracle
	primary := newTestRPCServer(t, "0x1", "0x4a817c800") // 20 Gwei
	defer primary.Close()

	client, err := NewClient(primary.URL, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	prices, err := client.GetGasPrices(ctx)
	require.NoError(t, err)

	// Medium should be exactly 20 Gwei (above floor, so no change)
	assert.Equal(t, big.NewInt(20_000_000_000), prices.Medium)
	// Slow = 20 * 0.8 = 16 Gwei (floating-point may round down by 1)
	assert.GreaterOrEqual(t, prices.Slow.Int64(), int64(15_999_999_999))
	assert.LessOrEqual(t, prices.Slow.Int64(), int64(16_000_000_000))
	// Fast = 20 * 1.2 = 24 Gwei (floating-point may round down by 1)
	assert.GreaterOrEqual(t, prices.Fast.Int64(), int64(23_999_999_999))
	assert.LessOrEqual(t, prices.Fast.Int64(), int64(24_000_000_000))
}

func TestMedianBigInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		values   []*big.Int
		expected *big.Int
	}{
		{
			name:     "single element",
			values:   []*big.Int{big.NewInt(10)},
			expected: big.NewInt(10),
		},
		{
			name:     "two elements",
			values:   []*big.Int{big.NewInt(10), big.NewInt(20)},
			expected: big.NewInt(15), // average
		},
		{
			name:     "three elements odd",
			values:   []*big.Int{big.NewInt(10), big.NewInt(20), big.NewInt(30)},
			expected: big.NewInt(20),
		},
		{
			name:     "three elements unsorted",
			values:   []*big.Int{big.NewInt(30), big.NewInt(10), big.NewInt(20)},
			expected: big.NewInt(20),
		},
		{
			name:     "four elements even",
			values:   []*big.Int{big.NewInt(10), big.NewInt(20), big.NewInt(30), big.NewInt(40)},
			expected: big.NewInt(25), // average of 20 and 30
		},
		{
			name:     "five elements with outlier",
			values:   []*big.Int{big.NewInt(1), big.NewInt(20), big.NewInt(25), big.NewInt(30), big.NewInt(1000)},
			expected: big.NewInt(25), // median is middle value
		},
		{
			name:     "all same values",
			values:   []*big.Int{big.NewInt(10), big.NewInt(10), big.NewInt(10)},
			expected: big.NewInt(10),
		},
		{
			name:     "empty slice",
			values:   []*big.Int{},
			expected: big.NewInt(0),
		},
		{
			name:     "large values (gwei range)",
			values:   []*big.Int{big.NewInt(60_000_000), big.NewInt(20_000_000_000), big.NewInt(30_000_000_000)},
			expected: big.NewInt(20_000_000_000), // 0.06 Gwei, 20 Gwei, 30 Gwei → median is 20 Gwei
		},
		{
			name:     "reverse sorted",
			values:   []*big.Int{big.NewInt(30), big.NewInt(20), big.NewInt(10)},
			expected: big.NewInt(20),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Make copies to verify input isn't mutated
			origValues := make([]*big.Int, len(tt.values))
			for i, v := range tt.values {
				origValues[i] = new(big.Int).Set(v)
			}

			result := medianBigInt(tt.values)
			assert.Equal(t, 0, tt.expected.Cmp(result),
				"expected %s, got %s", tt.expected, result)

			// Verify input wasn't mutated
			for i, v := range tt.values {
				assert.Equal(t, 0, origValues[i].Cmp(v),
					"input slice was mutated at index %d", i)
			}
		})
	}
}

func TestApplyGasPriceFloor(t *testing.T) {
	t.Parallel()

	t.Run("applies floor on mainnet when below minimum", func(t *testing.T) {
		t.Parallel()
		primary := newTestRPCServer(t, "0x1", "0x4a817c800")
		defer primary.Close()

		client, err := NewClient(primary.URL, nil)
		require.NoError(t, err)

		// Force connect to set chainID
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, client.connect(ctx))

		prices := &GasPrices{
			Slow:   big.NewInt(100_000_000), // 0.1 Gwei
			Medium: big.NewInt(200_000_000), // 0.2 Gwei
			Fast:   big.NewInt(300_000_000), // 0.3 Gwei
		}

		result := client.applyGasPriceFloor(prices)
		floor := big.NewInt(1_000_000_000)
		assert.Equal(t, 0, result.Slow.Cmp(floor))
		assert.Equal(t, 0, result.Medium.Cmp(floor))
		assert.Equal(t, 0, result.Fast.Cmp(floor))
	})

	t.Run("does not change prices above floor on mainnet", func(t *testing.T) {
		t.Parallel()
		primary := newTestRPCServer(t, "0x1", "0x4a817c800")
		defer primary.Close()

		client, err := NewClient(primary.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, client.connect(ctx))

		prices := &GasPrices{
			Slow:   big.NewInt(16_000_000_000), // 16 Gwei
			Medium: big.NewInt(20_000_000_000), // 20 Gwei
			Fast:   big.NewInt(24_000_000_000), // 24 Gwei
		}

		result := client.applyGasPriceFloor(prices)
		assert.Equal(t, big.NewInt(16_000_000_000), result.Slow)
		assert.Equal(t, big.NewInt(20_000_000_000), result.Medium)
		assert.Equal(t, big.NewInt(24_000_000_000), result.Fast)
	})

	t.Run("does not apply floor on non-mainnet", func(t *testing.T) {
		t.Parallel()
		// Arbitrum chain ID
		primary := newTestRPCServer(t, "0xa4b1", "0x5f5e100")
		defer primary.Close()

		client, err := NewClient(primary.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, client.connect(ctx))

		prices := &GasPrices{
			Slow:   big.NewInt(50_000_000),  // 0.05 Gwei
			Medium: big.NewInt(100_000_000), // 0.1 Gwei
			Fast:   big.NewInt(150_000_000), // 0.15 Gwei
		}

		result := client.applyGasPriceFloor(prices)
		assert.Equal(t, big.NewInt(50_000_000), result.Slow)
		assert.Equal(t, big.NewInt(100_000_000), result.Medium)
		assert.Equal(t, big.NewInt(150_000_000), result.Fast)
	})

	t.Run("handles nil chainID gracefully", func(t *testing.T) {
		t.Parallel()
		client := &Client{} // No chainID set

		prices := &GasPrices{
			Slow:   big.NewInt(50_000_000),
			Medium: big.NewInt(100_000_000),
			Fast:   big.NewInt(150_000_000),
		}

		// Should not panic, should return prices unchanged
		result := client.applyGasPriceFloor(prices)
		assert.Equal(t, big.NewInt(50_000_000), result.Slow)
		assert.Equal(t, big.NewInt(100_000_000), result.Medium)
		assert.Equal(t, big.NewInt(150_000_000), result.Fast)
	})

	t.Run("partially applies floor when only some prices are below", func(t *testing.T) {
		t.Parallel()
		primary := newTestRPCServer(t, "0x1", "0x4a817c800")
		defer primary.Close()

		client, err := NewClient(primary.URL, nil)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, client.connect(ctx))

		prices := &GasPrices{
			Slow:   big.NewInt(500_000_000),   // 0.5 Gwei — below floor
			Medium: big.NewInt(1_000_000_000), // 1 Gwei — at floor
			Fast:   big.NewInt(2_000_000_000), // 2 Gwei — above floor
		}

		result := client.applyGasPriceFloor(prices)
		assert.Equal(t, big.NewInt(1_000_000_000), result.Slow)   // floored
		assert.Equal(t, big.NewInt(1_000_000_000), result.Medium) // unchanged (at floor)
		assert.Equal(t, big.NewInt(2_000_000_000), result.Fast)   // unchanged (above floor)
	})
}
