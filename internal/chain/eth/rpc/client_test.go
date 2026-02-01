package rpc

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

func TestChainID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "eth_chainId", req["method"])

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  "0x1", // Mainnet
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(1), chainID)
}

func TestGetBalance(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "eth_getBalance", req["method"])

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  "0xde0b6b3a7640000", // 1 ETH
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	balance, err := client.GetBalance(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", "latest")
	require.NoError(t, err)

	expected := new(big.Int)
	expected.SetString("1000000000000000000", 10)
	assert.Equal(t, expected, balance)
}

func TestGetTransactionCount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "eth_getTransactionCount", req["method"])

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  "0xa", // 10
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nonce, err := client.GetTransactionCount(ctx, "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", "pending")
	require.NoError(t, err)
	assert.Equal(t, uint64(10), nonce)
}

func TestGasPrice(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "eth_gasPrice", req["method"])

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  "0x4a817c800", // 20 Gwei
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gasPrice, err := client.GasPrice(ctx)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(20000000000), gasPrice)
}

func TestEthCall(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "eth_call", req["method"])

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  "0x000000000000000000000000000000000000000000000000000000001dcd6500", // 500 USDC
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg := CallMsg{
		To:   "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		Data: []byte{0x70, 0xa0, 0x82, 0x31}, // balanceOf selector
	}

	result, err := client.EthCall(ctx, msg, "latest")
	require.NoError(t, err)
	assert.Len(t, result, 32)
}

func TestSendRawTransaction(t *testing.T) {
	t.Parallel()

	expectedHash := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "eth_sendRawTransaction", req["method"])

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  expectedHash,
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	txHash, err := client.SendRawTransaction(ctx, []byte{0x01, 0x02, 0x03})
	require.NoError(t, err)
	assert.Equal(t, expectedHash, txHash)
}

func TestRPCError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)

		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"error": map[string]any{
				"code":    -32600,
				"message": "Invalid Request",
			},
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.ChainID(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid Request")
}

func TestCallMsgMarshalJSON(t *testing.T) {
	t.Parallel()

	msg := CallMsg{
		From:  "0x1234567890123456789012345678901234567890",
		To:    "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Gas:   21000,
		Value: big.NewInt(1000000000000000000),
		Data:  []byte{0x70, 0xa0, 0x82, 0x31},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var result map[string]string
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.Equal(t, "0x1234567890123456789012345678901234567890", result["from"])
	assert.Equal(t, "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd", result["to"])
	assert.Equal(t, "0x5208", result["gas"])
	assert.Equal(t, "0xde0b6b3a7640000", result["value"])
	assert.Equal(t, "0x70a08231", result["data"])
}
