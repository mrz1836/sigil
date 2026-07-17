package bsv

import (
	"context"
	"testing"

	whatsonchain "github.com/mrz1836/go-whatsonchain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The WhatsOnChain /balance endpoint can report 0 unconfirmed while a mempool
// UTXO exists (notably on testnet). These tests verify GetNativeBalance and
// GetBulkNativeBalance reconcile the unconfirmed amount from the unspent set.

func TestGetNativeBalanceReconcilesMempool(t *testing.T) {
	t.Parallel()

	mock := &mockWOCClient{
		// Balance endpoint reports nothing (the testnet quirk).
		balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
			return &whatsonchain.AddressBalance{Confirmed: 0, Unconfirmed: 0}, nil
		},
		// But a pending (height 0) UTXO exists.
		utxoFunc: func(_ context.Context, _ string) (whatsonchain.AddressHistory, error) {
			return whatsonchain.AddressHistory{
				{TxHash: "abc", TxPos: 0, Value: 6457248, Height: 0},
			}, nil
		},
	}
	client := NewClient(context.Background(), &ClientOptions{WOCClient: mock, Network: NetworkTestnet})

	bal, err := client.GetNativeBalance(context.Background(), testnetP2PKH)
	require.NoError(t, err)
	assert.Equal(t, int64(0), bal.Amount.Int64())
	require.NotNil(t, bal.Unconfirmed, "pending mempool funds should surface as unconfirmed")
	assert.Equal(t, int64(6457248), bal.Unconfirmed.Int64())
}

func TestGetNativeBalanceDoesNotOverrideEndpointUnconfirmed(t *testing.T) {
	t.Parallel()

	mock := &mockWOCClient{
		balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
			return &whatsonchain.AddressBalance{Confirmed: 100, Unconfirmed: 50}, nil
		},
		// UTXO fetch would return a large value, but it must NOT be used because
		// the endpoint already provided a non-zero unconfirmed delta.
		utxoFunc: func(_ context.Context, _ string) (whatsonchain.AddressHistory, error) {
			return whatsonchain.AddressHistory{{Value: 999999, Height: 0}}, nil
		},
	}
	client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

	bal, err := client.GetNativeBalance(context.Background(), mainnetP2PKH)
	require.NoError(t, err)
	require.NotNil(t, bal.Unconfirmed)
	assert.Equal(t, int64(50), bal.Unconfirmed.Int64(), "endpoint unconfirmed must be preserved")
}

func TestGetNativeBalanceIgnoresConfirmedUTXOsInReconcile(t *testing.T) {
	t.Parallel()

	mock := &mockWOCClient{
		balanceFunc: func(_ context.Context, _ string) (*whatsonchain.AddressBalance, error) {
			return &whatsonchain.AddressBalance{Confirmed: 0, Unconfirmed: 0}, nil
		},
		// Only a confirmed (height > 0) UTXO — must not be counted as unconfirmed.
		utxoFunc: func(_ context.Context, _ string) (whatsonchain.AddressHistory, error) {
			return whatsonchain.AddressHistory{{Value: 1000, Height: 100}}, nil
		},
	}
	client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

	bal, err := client.GetNativeBalance(context.Background(), mainnetP2PKH)
	require.NoError(t, err)
	assert.Nil(t, bal.Unconfirmed, "confirmed UTXOs must not be counted as unconfirmed")
}

func TestGetBulkNativeBalanceReconcilesMempool(t *testing.T) {
	t.Parallel()

	const addr = "muGmcYARsqLbfvwLWDcnGMhcCyN9rbbfnM"
	mock := &mockWOCClient{
		bulkConfirmedFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
			return whatsonchain.AddressBalances{{Address: addr, Balance: &whatsonchain.AddressBalance{Confirmed: 0}}}, nil
		},
		bulkUnconfirmedFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.AddressBalances, error) {
			return whatsonchain.AddressBalances{{Address: addr, Balance: &whatsonchain.AddressBalance{Unconfirmed: 0}}}, nil
		},
		bulkUnconfirmedUTXOsFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkUnspentResponse, error) {
			return whatsonchain.BulkUnspentResponse{
				{Address: addr, Utxos: []*whatsonchain.HistoryRecord{{Value: 6457248, Height: 0}}},
			}, nil
		},
	}
	client := NewClient(context.Background(), &ClientOptions{WOCClient: mock})

	results, err := client.GetBulkNativeBalance(context.Background(), []string{addr})
	require.NoError(t, err)
	bal, ok := results[addr]
	require.True(t, ok, "address should be present in bulk results")
	require.NotNil(t, bal.Unconfirmed, "pending mempool funds should surface as unconfirmed")
	assert.Equal(t, int64(6457248), bal.Unconfirmed.Int64())
}
