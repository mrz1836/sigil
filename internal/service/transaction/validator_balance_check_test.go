package transaction

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain/eth"
)

func TestValidateETHBalance_Native(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ethBalance *big.Int
		amount     *big.Int
		gasCost    *big.Int
		wantErr    bool
	}{
		{
			name:       "sufficient native ETH",
			ethBalance: mustBigInt("2000000000000000000"),
			amount:     mustBigInt("1000000000000000000"),
			gasCost:    mustBigInt("21000000000000000"),
			wantErr:    false,
		},
		{
			name:       "insufficient native ETH",
			ethBalance: mustBigInt("100000000000000000"),
			amount:     mustBigInt("1000000000000000000"),
			gasCost:    mustBigInt("21000000000000000"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newETHBalanceRPCServer(t, tt.ethBalance, nil)
			defer server.Close()

			client, err := eth.NewClient(server.URL, nil)
			require.NoError(t, err)
			defer client.Close()

			err = ValidateETHBalance(
				context.Background(),
				client,
				validETHAddress,
				tt.amount,
				tt.gasCost,
				"",
			)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateETHBalance_Token(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		ethBalance   *big.Int
		tokenBalance *big.Int
		amount       *big.Int
		gasCost      *big.Int
		wantErr      bool
	}{
		{
			name:         "sufficient ETH gas and token balance",
			ethBalance:   mustBigInt("1000000000000000000"),
			tokenBalance: mustBigInt("2000000"),
			amount:       mustBigInt("1000000"),
			gasCost:      mustBigInt("21000000000000000"),
			wantErr:      false,
		},
		{
			name:         "insufficient ETH for gas",
			ethBalance:   mustBigInt("10000000000000000"),
			tokenBalance: mustBigInt("2000000"),
			amount:       mustBigInt("1000000"),
			gasCost:      mustBigInt("21000000000000000"),
			wantErr:      true,
		},
		{
			name:         "insufficient token",
			ethBalance:   mustBigInt("1000000000000000000"),
			tokenBalance: mustBigInt("500000"),
			amount:       mustBigInt("1000000"),
			gasCost:      mustBigInt("21000000000000000"),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newETHBalanceRPCServer(t, tt.ethBalance, tt.tokenBalance)
			defer server.Close()

			client, err := eth.NewClient(server.URL, nil)
			require.NoError(t, err)
			defer client.Close()

			err = ValidateETHBalance(
				context.Background(),
				client,
				validETHAddress,
				tt.amount,
				tt.gasCost,
				eth.USDCMainnet,
			)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newETHBalanceRPCServer(t *testing.T, latestBalance, tokenBalance *big.Int) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			assert.NoError(t, err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		method, _ := req["method"].(string)
		id := req["id"]

		var response map[string]any
		switch method {
		case "eth_chainId":
			response = map[string]any{"jsonrpc": "2.0", "id": id, "result": "0x1"}
		case "eth_getBalance":
			response = map[string]any{"jsonrpc": "2.0", "id": id, "result": fmt.Sprintf("0x%x", latestBalance)}
		case "eth_call":
			balance := tokenBalance
			if balance == nil {
				balance = big.NewInt(0)
			}
			response = map[string]any{"jsonrpc": "2.0", "id": id, "result": fmt.Sprintf("0x%064x", balance)}
		default:
			response = map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"error": map[string]any{
					"code":    -32601,
					"message": "method not found",
				},
			}
		}

		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
}

func mustBigInt(value string) *big.Int {
	result, ok := new(big.Int).SetString(value, 10)
	if !ok {
		panic("invalid big integer in test")
	}
	return result
}
