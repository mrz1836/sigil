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

	ethtypes "github.com/mrz1836/sigil/internal/chain/eth/types"
)

func TestBuildTransaction(t *testing.T) {
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
				"result":  "0x1", // Chain ID 1
			}
		case "eth_getTransactionCount":
			resp = map[string]any{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result":  "0x5", // Nonce 5
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

	params := &TxParams{
		From:     "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		To:       "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		Value:    big.NewInt(1000000000000000000), // 1 ETH
		GasLimit: 21000,
		GasPrice: big.NewInt(20000000000), // 20 Gwei
	}

	tx, err := client.BuildTransaction(ctx, params)
	require.NoError(t, err)
	assert.NotNil(t, tx)

	assert.Equal(t, uint64(5), tx.Nonce)
	assert.Equal(t, uint64(21000), tx.GasLimit)
	assert.Equal(t, big.NewInt(20000000000), tx.GasPrice)
	assert.Equal(t, big.NewInt(1000000000000000000), tx.Value)
}

func TestBroadcastTransaction(t *testing.T) {
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
				"result":  "0x1",
			}
		case "eth_sendRawTransaction":
			resp = map[string]any{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result":  "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", // Mock tx hash
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

	// Create a dummy signed transaction
	tx := ethtypes.NewLegacyTx(
		1,
		[]byte{0x01}, // to
		big.NewInt(1),
		21000,
		big.NewInt(10),
		[]byte{},
	)
	// Manually set signature values to simulate a signed tx
	tx.V = big.NewInt(37)
	tx.R = big.NewInt(1)
	tx.S = big.NewInt(2)

	hash, err := client.BroadcastTransaction(ctx, tx)
	require.NoError(t, err)
	assert.Equal(t, "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", hash)
}
