package bsv

import (
	"context"
	"testing"
	"time"

	whatsonchain "github.com/mrz1836/go-whatsonchain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBulkOperations_BulkAddressActivityCheck(t *testing.T) {
	t.Run("single batch", func(t *testing.T) {
		mock := &mockWOCClient{
			bulkHistoryFunc: func(_ context.Context, list *whatsonchain.AddressList) (whatsonchain.BulkAddressHistoryResponse, error) {
				// Return history for first address only
				return whatsonchain.BulkAddressHistoryResponse{
					{
						Address: list.Addresses[0],
						History: whatsonchain.AddressHistory{&whatsonchain.HistoryRecord{TxHash: "abc"}},
					},
				}, nil
			},
		}

		bulkOps := NewBulkOperations(mock, nil)
		ctx := context.Background()

		addresses := []string{"addr1", "addr2", "addr3"}
		results, err := bulkOps.BulkAddressActivityCheck(ctx, addresses)

		require.NoError(t, err)
		assert.Len(t, results, 3)
		assert.True(t, results[0].HasHistory)
		assert.False(t, results[1].HasHistory)
		assert.False(t, results[2].HasHistory)
	})

	t.Run("multiple batches", func(t *testing.T) {
		mock := &mockWOCClient{
			bulkHistoryFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkAddressHistoryResponse, error) {
				// Return empty history
				return whatsonchain.BulkAddressHistoryResponse{}, nil
			},
		}

		bulkOps := NewBulkOperations(mock, nil)
		ctx := context.Background()

		// 25 addresses = 2 batches (20 + 5)
		addresses := make([]string, 25)
		for i := range addresses {
			addresses[i] = "addr" + string(rune(i))
		}

		results, err := bulkOps.BulkAddressActivityCheck(ctx, addresses)

		require.NoError(t, err)
		assert.Len(t, results, 25)
	})

	t.Run("empty addresses", func(t *testing.T) {
		mock := &mockWOCClient{}
		bulkOps := NewBulkOperations(mock, nil)
		ctx := context.Background()

		results, err := bulkOps.BulkAddressActivityCheck(ctx, []string{})

		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestBulkOperations_BulkAddressUTXOFetch(t *testing.T) {
	t.Run("confirmed and unconfirmed UTXOs", func(t *testing.T) {
		mock := &mockWOCClient{
			bulkConfirmedUTXOsFunc: func(_ context.Context, list *whatsonchain.AddressList) (whatsonchain.BulkUnspentResponse, error) {
				return whatsonchain.BulkUnspentResponse{
					{
						Address: list.Addresses[0],
						Utxos: []*whatsonchain.HistoryRecord{
							{TxHash: "tx1", TxPos: 0, Value: 1000},
						},
					},
				}, nil
			},
			bulkUnconfirmedUTXOsFunc: func(_ context.Context, list *whatsonchain.AddressList) (whatsonchain.BulkUnspentResponse, error) {
				return whatsonchain.BulkUnspentResponse{
					{
						Address: list.Addresses[0],
						Utxos: []*whatsonchain.HistoryRecord{
							{TxHash: "tx2", TxPos: 1, Value: 500},
						},
					},
				}, nil
			},
		}

		bulkOps := NewBulkOperations(mock, nil)
		ctx := context.Background()

		addresses := []string{"addr1"}
		results, err := bulkOps.BulkAddressUTXOFetch(ctx, addresses)

		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Len(t, results[0].ConfirmedUTXOs, 1)
		assert.Len(t, results[0].UnconfirmedUTXOs, 1)
		assert.Equal(t, uint64(1000), results[0].ConfirmedUTXOs[0].Amount)
		assert.Equal(t, uint64(500), results[0].UnconfirmedUTXOs[0].Amount)
	})

	t.Run("multiple batches", func(t *testing.T) {
		mock := &mockWOCClient{
			bulkConfirmedUTXOsFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkUnspentResponse, error) {
				return whatsonchain.BulkUnspentResponse{}, nil
			},
			bulkUnconfirmedUTXOsFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkUnspentResponse, error) {
				return whatsonchain.BulkUnspentResponse{}, nil
			},
		}

		bulkOps := NewBulkOperations(mock, nil)
		ctx := context.Background()

		// 30 addresses = 2 batches
		addresses := make([]string, 30)
		for i := range addresses {
			addresses[i] = "addr" + string(rune(i))
		}

		results, err := bulkOps.BulkAddressUTXOFetch(ctx, addresses)

		require.NoError(t, err)
		assert.Len(t, results, 30)
	})
}

func TestBulkOperations_BulkUTXOValidation(t *testing.T) {
	t.Run("mixed spent and unspent", func(t *testing.T) {
		mock := &mockWOCClient{
			bulkSpentOutputsFunc: func(_ context.Context, _ *whatsonchain.BulkSpentOutputRequest) (whatsonchain.BulkSpentOutputResponse, error) {
				// First UTXO is spent, second is unspent
				return whatsonchain.BulkSpentOutputResponse{
					{TxID: "tx1", Vout: 0, Spent: &whatsonchain.SpentOutput{}}, // Spent
					{TxID: "tx2", Vout: 1, Spent: nil},                         // Unspent
				}, nil
			},
		}

		bulkOps := NewBulkOperations(mock, nil)
		ctx := context.Background()

		utxos := []UTXO{
			{TxID: "tx1", Vout: 0, Amount: 1000},
			{TxID: "tx2", Vout: 1, Amount: 2000},
		}

		results, err := bulkOps.BulkUTXOValidation(ctx, utxos)

		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.True(t, results[0].Spent)
		assert.False(t, results[1].Spent)
	})

	t.Run("large batch validation", func(t *testing.T) {
		mock := &mockWOCClient{
			bulkSpentOutputsFunc: func(_ context.Context, req *whatsonchain.BulkSpentOutputRequest) (whatsonchain.BulkSpentOutputResponse, error) {
				// All unspent
				response := make(whatsonchain.BulkSpentOutputResponse, len(req.UTXOs))
				for i, utxo := range req.UTXOs {
					response[i] = whatsonchain.BulkSpentOutputResult{
						TxID:  utxo.TxID,
						Vout:  utxo.Vout,
						Spent: nil,
					}
				}
				return response, nil
			},
		}

		bulkOps := NewBulkOperations(mock, nil)
		ctx := context.Background()

		// 50 UTXOs = 3 batches (20 + 20 + 10)
		utxos := make([]UTXO, 50)
		for i := range utxos {
			utxos[i] = UTXO{TxID: "tx", Vout: uint32(i), Amount: 1000}
		}

		results, err := bulkOps.BulkUTXOValidation(ctx, utxos)

		require.NoError(t, err)
		assert.Len(t, results, 50)

		// All should be unspent
		for _, r := range results {
			assert.False(t, r.Spent)
		}
	})
}

func TestBulkOperations_RateLimiting(t *testing.T) {
	mock := &mockWOCClient{
		bulkHistoryFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkAddressHistoryResponse, error) {
			return whatsonchain.BulkAddressHistoryResponse{}, nil
		},
	}

	bulkOps := NewBulkOperations(mock, &BulkOperationsOptions{
		RateLimit: 100.0, // 100 req/sec - fast enough for tests
		RateBurst: 1,     // No burst
	})

	ctx := context.Background()

	// Make 3 requests and measure time
	start := time.Now()
	addresses := []string{"addr1"}

	for i := 0; i < 3; i++ {
		_, err := bulkOps.BulkAddressActivityCheck(ctx, addresses)
		require.NoError(t, err)
	}

	duration := time.Since(start)

	// 3 requests at 100/sec = minimum 20ms
	// Allow some tolerance for test execution overhead
	assert.GreaterOrEqual(t, duration, 10*time.Millisecond)
}

func TestBulkOperations_Metrics(t *testing.T) {
	mock := &mockWOCClient{
		bulkHistoryFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkAddressHistoryResponse, error) {
			return whatsonchain.BulkAddressHistoryResponse{}, nil
		},
	}

	bulkOps := NewBulkOperations(mock, nil)
	ctx := context.Background()

	addresses := []string{"addr1", "addr2", "addr3"}
	_, err := bulkOps.BulkAddressActivityCheck(ctx, addresses)
	require.NoError(t, err)

	metrics := bulkOps.GetMetrics()
	assert.Equal(t, 1, metrics.TotalRequests)
	assert.Equal(t, 0, metrics.FailedRequests)
	assert.Greater(t, metrics.AverageLatency, time.Duration(0))
}
