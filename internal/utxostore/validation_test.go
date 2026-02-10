package utxostore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
)

// mockBulkOperationsClient implements BulkOperationsClient for testing.
type mockBulkOperationsClient struct {
	validateFunc func(ctx context.Context, utxos []bsv.UTXO) ([]bsv.UTXOSpentStatus, error)
	fetchFunc    func(ctx context.Context, addresses []string) ([]bsv.BulkUTXOResult, error)
}

func (m *mockBulkOperationsClient) BulkUTXOValidation(ctx context.Context, utxos []bsv.UTXO) ([]bsv.UTXOSpentStatus, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, utxos)
	}
	// Default: all unspent
	results := make([]bsv.UTXOSpentStatus, len(utxos))
	for i, u := range utxos {
		results[i] = bsv.UTXOSpentStatus{TxID: u.TxID, Vout: u.Vout, Spent: false}
	}
	return results, nil
}

func (m *mockBulkOperationsClient) BulkAddressUTXOFetch(ctx context.Context, addresses []string) ([]bsv.BulkUTXOResult, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, addresses)
	}
	// Default: no UTXOs
	results := make([]bsv.BulkUTXOResult, len(addresses))
	for i, addr := range addresses {
		results[i] = bsv.BulkUTXOResult{Address: addr}
	}
	return results, nil
}

func TestStore_ValidateUTXOs(t *testing.T) {
	t.Run("all UTXOs unspent", func(t *testing.T) {
		store := New(t.TempDir())

		// Add some UTXOs
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    0,
			Amount:  1000,
			Spent:   false,
		})
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx2",
			Vout:    0,
			Amount:  2000,
			Spent:   false,
		})

		mock := &mockBulkOperationsClient{}
		ctx := context.Background()

		report, err := store.ValidateUTXOs(ctx, chain.BSV, mock)

		require.NoError(t, err)
		assert.Equal(t, 2, report.TotalChecked)
		assert.Equal(t, 2, report.StillUnspent)
		assert.Equal(t, 0, report.NowSpent)
		assert.Empty(t, report.SpentUTXOs)
	})

	t.Run("some UTXOs spent", func(t *testing.T) {
		store := New(t.TempDir())

		// Add some UTXOs
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    0,
			Amount:  1000,
			Spent:   false,
		})
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx2",
			Vout:    0,
			Amount:  2000,
			Spent:   false,
		})

		mock := &mockBulkOperationsClient{
			validateFunc: func(_ context.Context, _ []bsv.UTXO) ([]bsv.UTXOSpentStatus, error) {
				// First UTXO is spent
				return []bsv.UTXOSpentStatus{
					{TxID: "tx1", Vout: 0, Spent: true},
					{TxID: "tx2", Vout: 0, Spent: false},
				}, nil
			},
		}
		ctx := context.Background()

		report, err := store.ValidateUTXOs(ctx, chain.BSV, mock)

		require.NoError(t, err)
		assert.Equal(t, 2, report.TotalChecked)
		assert.Equal(t, 1, report.StillUnspent)
		assert.Equal(t, 1, report.NowSpent)
		require.Len(t, report.SpentUTXOs, 1)
		assert.Equal(t, "tx1", report.SpentUTXOs[0].TxID)
	})

	t.Run("empty store", func(t *testing.T) {
		store := New(t.TempDir())
		mock := &mockBulkOperationsClient{}
		ctx := context.Background()

		report, err := store.ValidateUTXOs(ctx, chain.BSV, mock)

		require.NoError(t, err)
		assert.Equal(t, 0, report.TotalChecked)
	})
}

func TestStore_ReconcileWithChain(t *testing.T) {
	t.Run("discover new UTXOs", func(t *testing.T) {
		store := New(t.TempDir())

		// Add an address
		store.AddAddress(&AddressMetadata{
			Address: "addr1",
			ChainID: chain.BSV,
		})

		mock := &mockBulkOperationsClient{
			fetchFunc: func(_ context.Context, _ []string) ([]bsv.BulkUTXOResult, error) {
				return []bsv.BulkUTXOResult{
					{
						Address: "addr1",
						ConfirmedUTXOs: []bsv.UTXO{
							{TxID: "tx1", Vout: 0, Amount: 1000},
							{TxID: "tx2", Vout: 0, Amount: 2000},
						},
					},
				}, nil
			},
		}
		ctx := context.Background()

		report, err := store.ReconcileWithChain(ctx, chain.BSV, mock)

		require.NoError(t, err)
		assert.Equal(t, 1, report.AddressesScanned)
		assert.Equal(t, 2, report.NewUTXOs)
		assert.Equal(t, 0, report.RemovedUTXOs)
		assert.Equal(t, int64(3000), report.UpdatedBalance)
	})

	t.Run("mark missing UTXOs as spent", func(t *testing.T) {
		store := New(t.TempDir())

		// Add an address and UTXO
		store.AddAddress(&AddressMetadata{
			Address: "addr1",
			ChainID: chain.BSV,
		})
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    0,
			Amount:  1000,
			Address: "addr1",
			Spent:   false,
		})

		mock := &mockBulkOperationsClient{
			fetchFunc: func(_ context.Context, _ []string) ([]bsv.BulkUTXOResult, error) {
				// Return different UTXO (original one is spent)
				return []bsv.BulkUTXOResult{
					{
						Address: "addr1",
						ConfirmedUTXOs: []bsv.UTXO{
							{TxID: "tx2", Vout: 0, Amount: 2000},
						},
					},
				}, nil
			},
		}
		ctx := context.Background()

		report, err := store.ReconcileWithChain(ctx, chain.BSV, mock)

		require.NoError(t, err)
		assert.Equal(t, 1, report.NewUTXOs)
		assert.Equal(t, 1, report.RemovedUTXOs)
		assert.Equal(t, int64(1000), report.UpdatedBalance) // +2000 -1000

		// Verify original UTXO is marked as spent
		key := "bsv:tx1:0"
		store.mu.RLock()
		utxo, exists := store.data.UTXOs[key]
		store.mu.RUnlock()
		require.True(t, exists)
		assert.True(t, utxo.Spent)
	})

	t.Run("update confirmations", func(t *testing.T) {
		store := New(t.TempDir())

		// Add an address and unconfirmed UTXO
		store.AddAddress(&AddressMetadata{
			Address: "addr1",
			ChainID: chain.BSV,
		})
		store.AddUTXO(&StoredUTXO{
			ChainID:       chain.BSV,
			TxID:          "tx1",
			Vout:          0,
			Amount:        1000,
			Address:       "addr1",
			Confirmations: 0,
			Spent:         false,
		})

		mock := &mockBulkOperationsClient{
			fetchFunc: func(_ context.Context, _ []string) ([]bsv.BulkUTXOResult, error) {
				// Return same UTXO but confirmed
				return []bsv.BulkUTXOResult{
					{
						Address: "addr1",
						ConfirmedUTXOs: []bsv.UTXO{
							{TxID: "tx1", Vout: 0, Amount: 1000, Confirmations: 6},
						},
					},
				}, nil
			},
		}
		ctx := context.Background()

		report, err := store.ReconcileWithChain(ctx, chain.BSV, mock)

		require.NoError(t, err)
		assert.Equal(t, 0, report.NewUTXOs) // Not new, just updated
		assert.Equal(t, int64(0), report.UpdatedBalance)

		// Verify confirmations updated
		key := "bsv:tx1:0"
		store.mu.RLock()
		utxo, exists := store.data.UTXOs[key]
		store.mu.RUnlock()
		require.True(t, exists)
		assert.Equal(t, uint32(6), utxo.Confirmations)
	})
}

func TestValidationReport_Duration(t *testing.T) {
	store := New(t.TempDir())

	// Add some UTXOs
	for i := 0; i < 10; i++ {
		//nolint:gosec // Test data, no overflow risk with small index values
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx",
			Vout:    uint32(i),
			Amount:  1000,
			Spent:   false,
		})
	}

	mock := &mockBulkOperationsClient{}
	ctx := context.Background()

	start := time.Now()
	report, err := store.ValidateUTXOs(ctx, chain.BSV, mock)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Greater(t, report.Duration, time.Duration(0))
	assert.LessOrEqual(t, report.Duration, elapsed+10*time.Millisecond)
}
