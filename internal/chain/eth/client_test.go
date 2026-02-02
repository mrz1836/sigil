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

	"github.com/mrz1836/sigil/internal/chain"
)

// TestNewClient tests client creation.
func TestNewClient(t *testing.T) {
	t.Parallel()
	t.Run("creates client with valid URL", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("http://localhost:8545", nil)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		t.Parallel()
		_, err := NewClient("", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "RPC URL is required")
	})
}

// TestGetBalance tests ETH balance queries.
func TestGetBalance(t *testing.T) {
	t.Parallel()
	t.Run("returns balance for valid address", func(t *testing.T) {
		t.Parallel()
		// Mock RPC server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case "eth_chainId":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1", // Mainnet
				}
			case "eth_getBalance":
				// Return 1 ETH in wei (1e18)
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0xde0b6b3a7640000", // 1 ETH = 10^18 wei
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

		balance, err := client.GetBalance(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.NoError(t, err)

		expected := new(big.Int)
		expected.SetString("1000000000000000000", 10) // 1 ETH
		assert.Equal(t, expected, balance)
	})

	t.Run("returns error for invalid address format", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("should not reach server")
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = client.GetBalance(ctx, "invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid Ethereum address")
	})
}

// TestGetTokenBalance tests ERC-20 balance queries.
func TestGetTokenBalance(t *testing.T) {
	t.Parallel()
	t.Run("returns USDC balance", func(t *testing.T) {
		t.Parallel()
		// Mock RPC server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case "eth_chainId":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1", // Mainnet
				}
			case "eth_call":
				// Return 500 USDC (6 decimals = 500 * 10^6)
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x000000000000000000000000000000000000000000000000000000001dcd6500", // 500000000
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

		balance, err := client.GetTokenBalance(
			ctx,
			"0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // USDC mainnet
		)
		require.NoError(t, err)

		expected := big.NewInt(500000000) // 500 USDC in base units
		assert.Equal(t, expected, balance)
	})

	t.Run("returns error for invalid token address", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("should not reach server")
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = client.GetTokenBalance(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", "invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid token address")
	})
}

// TestValidateAddress tests address validation.
func TestValidateAddress(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "valid checksummed address",
			address: "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			wantErr: false,
		},
		{
			name:    "valid lowercase address",
			address: "0x742d35cc6634c0532925a3b844bc454e4438f44e",
			wantErr: false,
		},
		{
			name:    "invalid - too short",
			address: "0x742d35Cc",
			wantErr: true,
		},
		{
			name:    "invalid - no 0x prefix",
			address: "742d35Cc6634C0532925a3b844Bc454e4438f44e",
			wantErr: true,
		},
		{
			name:    "invalid - contains non-hex characters",
			address: "0xGHIJKL35Cc6634C0532925a3b844Bc454e4438f44e",
			wantErr: true,
		},
		{
			name:    "empty address",
			address: "",
			wantErr: true,
		},
	}

	client := &Client{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := client.ValidateAddress(tt.address)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestFormatAmount tests amount formatting.
func TestFormatAmount(t *testing.T) {
	t.Parallel()
	client := &Client{}

	tests := []struct {
		name     string
		amount   *big.Int
		expected string
	}{
		{
			name:     "1 ETH",
			amount:   big.NewInt(1000000000000000000),
			expected: "1.000000000000000000",
		},
		{
			name:     "0.5 ETH",
			amount:   big.NewInt(500000000000000000),
			expected: "0.500000000000000000",
		},
		{
			name:     "tiny amount (1 wei)",
			amount:   big.NewInt(1),
			expected: "0.000000000000000001",
		},
		{
			name:     "zero",
			amount:   big.NewInt(0),
			expected: "0.000000000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := client.FormatAmount(tt.amount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParseAmount tests amount parsing.
func TestParseAmount(t *testing.T) {
	t.Parallel()
	client := &Client{}

	tests := []struct {
		name     string
		input    string
		expected *big.Int
		wantErr  bool
	}{
		{
			name:     "1 ETH",
			input:    "1",
			expected: big.NewInt(1000000000000000000),
			wantErr:  false,
		},
		{
			name:     "0.5 ETH",
			input:    "0.5",
			expected: big.NewInt(500000000000000000),
			wantErr:  false,
		},
		{
			name:     "1.234 ETH",
			input:    "1.234",
			expected: big.NewInt(1234000000000000000),
			wantErr:  false,
		},
		{
			name:    "invalid - not a number",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid - negative",
			input:   "-1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := client.ParseAmount(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestFormatAmount_Nil tests nil handling in FormatAmount.
func TestFormatAmount_Nil(t *testing.T) {
	t.Parallel()
	client := &Client{}

	result := client.FormatAmount(nil)
	assert.Equal(t, "0.000000000000000000", result)
}

// TestClientID tests the ID method.
func TestClientID(t *testing.T) {
	t.Parallel()

	client := &Client{}
	id := client.ID()

	assert.Equal(t, chain.ETH, id)
	assert.Equal(t, "eth", string(id))
}

// TestClientClose tests the Close method.
func TestClientClose(t *testing.T) {
	t.Parallel()

	t.Run("close without connection", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("http://localhost:8545", nil)
		require.NoError(t, err)

		// Should not panic when closing without ever connecting
		assert.NotPanics(t, func() {
			client.Close()
		})
	})

	t.Run("close sets rpcClient to nil", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("http://localhost:8545", nil)
		require.NoError(t, err)

		client.Close()
		assert.Nil(t, client.rpcClient)
	})

	t.Run("double close is safe", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("http://localhost:8545", nil)
		require.NoError(t, err)

		assert.NotPanics(t, func() {
			client.Close()
			client.Close()
		})
	})
}

// TestClientWithOptions tests client creation with options.
func TestClientWithOptions(t *testing.T) {
	t.Parallel()

	t.Run("with custom chain ID", func(t *testing.T) {
		t.Parallel()
		opts := &ClientOptions{
			ChainID: big.NewInt(5), // Goerli
		}
		client, err := NewClient("http://localhost:8545", opts)
		require.NoError(t, err)
		assert.Equal(t, big.NewInt(5), client.chainID)
	})

	t.Run("with nil options", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient("http://localhost:8545", nil)
		require.NoError(t, err)
		assert.Nil(t, client.chainID)
	})

	t.Run("with empty options", func(t *testing.T) {
		t.Parallel()
		opts := &ClientOptions{}
		client, err := NewClient("http://localhost:8545", opts)
		require.NoError(t, err)
		assert.Nil(t, client.chainID)
	})
}

// TestEstimateFee tests fee estimation.
func TestEstimateFee(t *testing.T) {
	t.Parallel()

	t.Run("returns fee for valid addresses", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case "eth_chainId":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1",
				}
			case "eth_gasPrice":
				// 20 Gwei
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x4a817c800", // 20 Gwei
				}
			case "eth_estimateGas":
				// 21000 gas for simple transfer
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x5208", // 21000
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

		fee, err := client.EstimateFee(
			ctx,
			"0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			"0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
			big.NewInt(1000000000000000000),
		)
		require.NoError(t, err)
		assert.NotNil(t, fee)
		assert.Positive(t, fee.Sign(), "fee should be positive")
	})

	t.Run("returns error for invalid from address", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("should not reach server")
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = client.EstimateFee(ctx, "invalid", "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", big.NewInt(100))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from address")
	})

	t.Run("returns error for invalid to address", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			t.Fatal("should not reach server")
		}))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)

		ctx := context.Background()
		_, err = client.EstimateFee(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", "invalid", big.NewInt(100))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "to address")
	})
}

// TestGetNativeBalance tests native ETH balance queries.
func TestGetNativeBalance(t *testing.T) {
	t.Parallel()

	t.Run("returns ETH balance with metadata", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case "eth_chainId":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1",
				}
			case "eth_getBalance":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0xde0b6b3a7640000", // 1 ETH
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

		balance, err := client.GetNativeBalance(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.NoError(t, err)

		assert.Equal(t, "ETH", balance.Symbol)
		assert.Equal(t, 18, balance.Decimals)
		assert.Empty(t, balance.Token)
		assert.Equal(t, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", balance.Address)
	})
}

// TestGetUSDCBalance tests USDC balance queries.
func TestGetUSDCBalance(t *testing.T) {
	t.Parallel()

	t.Run("returns USDC balance with metadata", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case "eth_chainId":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1",
				}
			case "eth_call":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x000000000000000000000000000000000000000000000000000000001dcd6500", // 500 USDC
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

		balance, err := client.GetUSDCBalance(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.NoError(t, err)

		assert.Equal(t, "USDC", balance.Symbol)
		assert.Equal(t, 6, balance.Decimals)
		assert.Equal(t, USDCMainnet, balance.Token)
	})
}

// TestGetAllBalances tests combined balance queries.
func TestGetAllBalances(t *testing.T) {
	t.Parallel()

	t.Run("returns both ETH and USDC balances", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case "eth_chainId":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1",
				}
			case "eth_getBalance":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0xde0b6b3a7640000", // 1 ETH
				}
			case "eth_call":
				resp = map[string]any{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x000000000000000000000000000000000000000000000000000000001dcd6500", // 500 USDC
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

		balances, err := client.GetAllBalances(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
		require.NoError(t, err)

		assert.Len(t, balances, 2)
		assert.Equal(t, "ETH", balances[0].Symbol)
		assert.Equal(t, "USDC", balances[1].Symbol)
	})
}
