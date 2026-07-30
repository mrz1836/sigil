package bsv

import (
	"context"
	"sync"
	"testing"

	whatsonchain "github.com/mrz1836/go-whatsonchain"
	"github.com/stretchr/testify/require"
)

// TestBulkOperationsConcurrentMetrics drives concurrent Bulk* calls against a
// single BulkOperations to exercise the shared-metrics mutex, the global
// metrics sink, and the internal UTXO-fetch goroutine. Run under -race to
// detect data races on the shared metrics.
func TestBulkOperationsConcurrentMetrics(t *testing.T) {
	mock := &mockWOCClient{
		bulkHistoryFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkAddressHistoryResponse, error) {
			return whatsonchain.BulkAddressHistoryResponse{}, nil
		},
		bulkConfirmedUTXOsFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkUnspentResponse, error) {
			return whatsonchain.BulkUnspentResponse{}, nil
		},
		bulkUnconfirmedUTXOsFunc: func(_ context.Context, _ *whatsonchain.AddressList) (whatsonchain.BulkUnspentResponse, error) {
			return whatsonchain.BulkUnspentResponse{}, nil
		},
		bulkSpentOutputsFunc: func(_ context.Context, _ *whatsonchain.BulkSpentOutputRequest) (whatsonchain.BulkSpentOutputResponse, error) {
			return whatsonchain.BulkSpentOutputResponse{}, nil
		},
	}

	// A very high rate limit keeps the limiter from serializing the goroutines.
	bulk := NewBulkOperations(mock, &BulkOperationsOptions{RateLimit: 1e6, RateBurst: 1000})

	ctx := context.Background()
	addresses := []string{"addr-1", "addr-2", "addr-3"}
	utxos := []UTXO{{TxID: "aa", Vout: 0}, {TxID: "bb", Vout: 1}}

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func() {
			defer wg.Done()
			switch i % 4 {
			case 0:
				_, _ = bulk.BulkAddressActivityCheck(ctx, addresses)
			case 1:
				_, _ = bulk.BulkAddressUTXOFetch(ctx, addresses)
			case 2:
				_, _ = bulk.BulkUTXOValidation(ctx, utxos)
			default:
				_ = bulk.GetMetrics()
			}
		}()
	}
	wg.Wait()

	// The request-making goroutines must have recorded requests without racing.
	require.Positive(t, bulk.GetMetrics().TotalRequests)
}
