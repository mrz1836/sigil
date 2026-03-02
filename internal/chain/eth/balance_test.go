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

func TestFormatDecimalAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   *big.Int
		decimals int
		expected string
	}{
		{
			name:     "nil amount returns 0",
			amount:   nil,
			decimals: 18,
			expected: "0",
		},
		{
			name:     "zero ETH (18 decimals)",
			amount:   big.NewInt(0),
			decimals: 18,
			expected: "0.0",
		},
		{
			name:     "1 ETH (18 decimals)",
			amount:   big.NewInt(1_000_000_000_000_000_000),
			decimals: 18,
			expected: "1.0",
		},
		{
			name:     "0.5 ETH (18 decimals)",
			amount:   big.NewInt(500_000_000_000_000_000),
			decimals: 18,
			expected: "0.5",
		},
		{
			name:     "1.234 ETH (18 decimals)",
			amount:   big.NewInt(1_234_000_000_000_000_000),
			decimals: 18,
			expected: "1.234",
		},
		{
			name:     "tiny ETH amount (1 wei)",
			amount:   big.NewInt(1),
			decimals: 18,
			expected: "0.000000000000000001",
		},
		{
			name:     "zero USDC (6 decimals)",
			amount:   big.NewInt(0),
			decimals: 6,
			expected: "0.0",
		},
		{
			name:     "1 USDC (6 decimals)",
			amount:   big.NewInt(1_000_000),
			decimals: 6,
			expected: "1.0",
		},
		{
			name:     "500 USDC (6 decimals)",
			amount:   big.NewInt(500_000_000),
			decimals: 6,
			expected: "500.0",
		},
		{
			name:     "0.01 USDC (6 decimals)",
			amount:   big.NewInt(10_000),
			decimals: 6,
			expected: "0.01",
		},
		{
			name:     "tiny USDC amount (1 base unit)",
			amount:   big.NewInt(1),
			decimals: 6,
			expected: "0.000001",
		},
		{
			name:     "large ETH amount",
			amount:   new(big.Int).Mul(big.NewInt(1000), big.NewInt(1_000_000_000_000_000_000)),
			decimals: 18,
			expected: "1000.0",
		},
		{
			name:     "8 decimals token",
			amount:   big.NewInt(100_000_000),
			decimals: 8,
			expected: "1.0",
		},
		{
			name:     "0 decimals token",
			amount:   big.NewInt(42),
			decimals: 0,
			expected: "42.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := chain.FormatDecimalAmount(tt.amount, tt.decimals)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatSignedDecimalAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   *big.Int
		decimals int
		expected string
	}{
		{
			name:     "nil returns zero",
			amount:   nil,
			decimals: 18,
			expected: "0",
		},
		{
			name:     "zero value",
			amount:   big.NewInt(0),
			decimals: 18,
			expected: "0.0",
		},
		{
			name:     "positive delegates to chain.FormatDecimalAmount",
			amount:   big.NewInt(1_000_000_000_000_000_000),
			decimals: 18,
			expected: "1.0",
		},
		{
			name:     "negative 1 wei",
			amount:   big.NewInt(-1),
			decimals: 18,
			expected: "-0.000000000000000001",
		},
		{
			name:     "negative 1 ETH",
			amount:   big.NewInt(-1_000_000_000_000_000_000),
			decimals: 18,
			expected: "-1.0",
		},
		{
			name:     "negative 0.5 ETH",
			amount:   big.NewInt(-500_000_000_000_000_000),
			decimals: 18,
			expected: "-0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := chain.FormatSignedDecimalAmount(tt.amount, tt.decimals)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetNativeBalancePendingGuard(t *testing.T) {
	t.Parallel()

	const testAddr = "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"

	// rpcHandler builds an httptest handler that returns latestBal for "latest"
	// and pendingBal for "pending" eth_getBalance calls.
	rpcHandler := func(t *testing.T, latestHex, pendingHex string) http.Handler {
		t.Helper()
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req map[string]any
			err := json.NewDecoder(r.Body).Decode(&req)
			assert.NoError(t, err)

			method := req["method"].(string)
			var resp map[string]any

			switch method {
			case "eth_chainId":
				resp = map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": "0x1"}
			case "eth_getBalance":
				params := req["params"].([]any)
				block := params[1].(string)
				hex := latestHex
				if block == "pending" {
					hex = pendingHex
				}
				resp = map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": hex}
			default:
				t.Errorf("unexpected method: %s", method)
				return
			}
			err = json.NewEncoder(w).Encode(resp)
			assert.NoError(t, err)
		})
	}

	t.Run("discards stale zero pending balance", func(t *testing.T) {
		t.Parallel()
		// Simulate: confirmed = 1 ETH, pending = 0 (stale RPC data)
		server := httptest.NewServer(rpcHandler(t,
			"0xde0b6b3a7640000", // 1 ETH
			"0x0",               // stale pending returns 0
		))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		bal, err := client.GetNativeBalance(ctx, testAddr)
		require.NoError(t, err)

		assert.Equal(t, big.NewInt(1_000_000_000_000_000_000), bal.Amount)
		assert.Nil(t, bal.Unconfirmed, "stale zero pending should not produce unconfirmed delta")
	})

	t.Run("shows legitimate pending delta", func(t *testing.T) {
		t.Parallel()
		// Simulate: confirmed = 1 ETH, pending = 1.5 ETH (incoming pending tx)
		server := httptest.NewServer(rpcHandler(t,
			"0xde0b6b3a7640000",  // 1 ETH
			"0x14d1120d7b160000", // 1.5 ETH
		))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		bal, err := client.GetNativeBalance(ctx, testAddr)
		require.NoError(t, err)

		assert.NotNil(t, bal.Unconfirmed)
		expected := big.NewInt(500_000_000_000_000_000) // +0.5 ETH
		assert.Equal(t, expected, bal.Unconfirmed)
	})

	t.Run("shows negative delta for outgoing pending tx", func(t *testing.T) {
		t.Parallel()
		// Simulate: confirmed = 1 ETH, pending = 0.5 ETH (outgoing pending tx)
		server := httptest.NewServer(rpcHandler(t,
			"0xde0b6b3a7640000", // 1 ETH
			"0x6f05b59d3b20000", // 0.5 ETH
		))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		bal, err := client.GetNativeBalance(ctx, testAddr)
		require.NoError(t, err)

		assert.NotNil(t, bal.Unconfirmed)
		expected := big.NewInt(-500_000_000_000_000_000) // -0.5 ETH
		assert.Equal(t, expected, bal.Unconfirmed)
	})

	t.Run("no delta when pending equals latest", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(rpcHandler(t,
			"0xde0b6b3a7640000", // 1 ETH
			"0xde0b6b3a7640000", // 1 ETH (same)
		))
		defer server.Close()

		client, err := NewClient(server.URL, nil)
		require.NoError(t, err)
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		bal, err := client.GetNativeBalance(ctx, testAddr)
		require.NoError(t, err)

		assert.Nil(t, bal.Unconfirmed)
	})
}

func TestBalanceStruct(t *testing.T) {
	t.Parallel()

	t.Run("ETH balance without unconfirmed", func(t *testing.T) {
		t.Parallel()
		balance := &Balance{
			Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			Amount:   big.NewInt(1_000_000_000_000_000_000),
			Symbol:   "ETH",
			Decimals: 18,
			Token:    "",
		}
		assert.Equal(t, "ETH", balance.Symbol)
		assert.Equal(t, 18, balance.Decimals)
		assert.Empty(t, balance.Token)
		assert.Nil(t, balance.Unconfirmed)
	})

	t.Run("ETH balance with unconfirmed delta", func(t *testing.T) {
		t.Parallel()
		balance := &Balance{
			Address:     "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			Amount:      big.NewInt(1_000_000_000_000_000_000),
			Unconfirmed: big.NewInt(-500_000_000_000_000_000),
			Symbol:      "ETH",
			Decimals:    18,
		}
		assert.Equal(t, big.NewInt(-500_000_000_000_000_000), balance.Unconfirmed)
	})

	t.Run("USDC balance", func(t *testing.T) {
		t.Parallel()
		balance := &Balance{
			Address:  "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			Amount:   big.NewInt(1_000_000),
			Symbol:   "USDC",
			Decimals: 6,
			Token:    USDCMainnet,
		}
		assert.Equal(t, "USDC", balance.Symbol)
		assert.Equal(t, 6, balance.Decimals)
		assert.Equal(t, USDCMainnet, balance.Token)
	})
}
