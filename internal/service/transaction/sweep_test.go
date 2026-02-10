package transaction

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/wallet"
)

// mockBSVClientForSweep implements a minimal BSV client for sweep testing.
type mockBSVClientForSweep struct {
	listUTXOsFunc    func(ctx context.Context, address string) ([]bsv.UTXO, error)
	validateAddrFunc func(address string) error
	sendFunc         func(ctx context.Context, req chain.SendRequest) (*chain.TransactionResult, error)
}

func (m *mockBSVClientForSweep) ListUTXOs(ctx context.Context, address string) ([]bsv.UTXO, error) {
	if m.listUTXOsFunc != nil {
		return m.listUTXOsFunc(ctx, address)
	}
	return []bsv.UTXO{}, nil
}

func (m *mockBSVClientForSweep) ValidateAddress(address string) error {
	if m.validateAddrFunc != nil {
		return m.validateAddrFunc(address)
	}
	return nil
}

func (m *mockBSVClientForSweep) Send(ctx context.Context, req chain.SendRequest) (*chain.TransactionResult, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, req)
	}
	return &chain.TransactionResult{Hash: "tx123"}, nil
}

// mockBulkOpsForSweep implements BulkOperations for sweep testing.
type mockBulkOpsForSweep struct {
	fetchFunc    func(ctx context.Context, addresses []string) ([]bsv.BulkUTXOResult, error)
	validateFunc func(ctx context.Context, utxos []bsv.UTXO) ([]bsv.UTXOSpentStatus, error)
}

func (m *mockBulkOpsForSweep) BulkAddressUTXOFetch(ctx context.Context, addresses []string) ([]bsv.BulkUTXOResult, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, addresses)
	}
	return []bsv.BulkUTXOResult{}, nil
}

func (m *mockBulkOpsForSweep) BulkUTXOValidation(ctx context.Context, utxos []bsv.UTXO) ([]bsv.UTXOSpentStatus, error) {
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

func TestSweepService_Sweep_DryRun(t *testing.T) {
	client := &mockBSVClientForSweep{}
	bulkOps := &mockBulkOpsForSweep{
		fetchFunc: func(_ context.Context, _ []string) ([]bsv.BulkUTXOResult, error) {
			return []bsv.BulkUTXOResult{
				{
					Address: "addr1",
					ConfirmedUTXOs: []bsv.UTXO{
						{TxID: "tx1", Vout: 0, Amount: 10000, Address: "addr1"},
						{TxID: "tx2", Vout: 0, Amount: 20000, Address: "addr1"},
					},
				},
				{
					Address: "addr2",
					ConfirmedUTXOs: []bsv.UTXO{
						{TxID: "tx3", Vout: 0, Amount: 30000, Address: "addr2"},
					},
				},
			}, nil
		},
	}

	service := NewSweepService(client, bulkOps, nil)
	ctx := context.Background()

	opts := &SweepOptions{
		Destination: "dest_addr",
		Addresses: []wallet.Address{
			{Address: "addr1"},
			{Address: "addr2"},
		},
		DryRun:  true,
		FeeRate: 250, // 250 sat/KB
	}

	result, err := service.Sweep(ctx, opts)

	require.NoError(t, err)
	assert.True(t, result.DryRun)
	assert.Equal(t, uint64(60000), result.TotalInput)
	assert.Equal(t, 3, result.UTXOsUsed)
	assert.Equal(t, 2, result.AddressesUsed)
	assert.Positive(t, result.Fee)
	assert.Equal(t, result.TotalInput-result.Fee, result.NetAmount)
	assert.Empty(t, result.TxID) // Dry run doesn't produce TxID
}

func TestSweepService_Sweep_WithValidation(t *testing.T) {
	client := &mockBSVClientForSweep{
		sendFunc: func(_ context.Context, _ chain.SendRequest) (*chain.TransactionResult, error) {
			return &chain.TransactionResult{Hash: "sweep_tx_123"}, nil
		},
	}

	bulkOps := &mockBulkOpsForSweep{
		fetchFunc: func(_ context.Context, _ []string) ([]bsv.BulkUTXOResult, error) {
			return []bsv.BulkUTXOResult{
				{
					Address: "addr1",
					ConfirmedUTXOs: []bsv.UTXO{
						{TxID: "tx1", Vout: 0, Amount: 10000, Address: "addr1"},
						{TxID: "tx2", Vout: 0, Amount: 20000, Address: "addr1"},
						{TxID: "tx3", Vout: 0, Amount: 30000, Address: "addr1"},
					},
				},
			}, nil
		},
		validateFunc: func(_ context.Context, _ []bsv.UTXO) ([]bsv.UTXOSpentStatus, error) {
			// First UTXO is spent
			return []bsv.UTXOSpentStatus{
				{TxID: "tx1", Vout: 0, Spent: true},
				{TxID: "tx2", Vout: 0, Spent: false},
				{TxID: "tx3", Vout: 0, Spent: false},
			}, nil
		},
	}

	service := NewSweepService(client, bulkOps, nil)
	ctx := context.Background()

	seed := make([]byte, 32) // Dummy seed
	opts := &SweepOptions{
		Destination:   "dest_addr",
		Addresses:     []wallet.Address{{Address: "addr1"}},
		Seed:          seed,
		ValidateUTXOs: true,
		FeeRate:       250,
	}

	result, err := service.Sweep(ctx, opts)

	require.NoError(t, err)
	assert.Equal(t, 3, result.ValidatedUTXOs)
	assert.Equal(t, 1, result.SpentUTXOs)
	assert.Equal(t, 2, result.UTXOsUsed) // Only 2 unspent UTXOs used
	assert.Equal(t, "sweep_tx_123", result.TxID)
	assert.Equal(t, uint64(50000), result.TotalInput) // tx2 + tx3
}

func TestSweepService_Sweep_InsufficientFunds(t *testing.T) {
	client := &mockBSVClientForSweep{}
	bulkOps := &mockBulkOpsForSweep{
		fetchFunc: func(_ context.Context, _ []string) ([]bsv.BulkUTXOResult, error) {
			return []bsv.BulkUTXOResult{
				{Address: "addr1"}, // No UTXOs
			}, nil
		},
	}

	service := NewSweepService(client, bulkOps, nil)
	ctx := context.Background()

	opts := &SweepOptions{
		Destination: "dest_addr",
		Addresses:   []wallet.Address{{Address: "addr1"}},
		DryRun:      true,
	}

	_, err := service.Sweep(ctx, opts)

	require.Error(t, err)
	// The actual error could be "no UTXOs found" or "insufficient funds" depending on the flow
	assert.Error(t, err)
}

func TestSweepService_Sweep_BelowDustThreshold(t *testing.T) {
	client := &mockBSVClientForSweep{}
	bulkOps := &mockBulkOpsForSweep{
		fetchFunc: func(_ context.Context, _ []string) ([]bsv.BulkUTXOResult, error) {
			return []bsv.BulkUTXOResult{
				{
					Address: "addr1",
					ConfirmedUTXOs: []bsv.UTXO{
						// Very small amount that won't cover fee
						{TxID: "tx1", Vout: 0, Amount: 10, Address: "addr1"},
					},
				},
			}, nil
		},
	}

	service := NewSweepService(client, bulkOps, nil)
	ctx := context.Background()

	opts := &SweepOptions{
		Destination: "dest_addr",
		Addresses:   []wallet.Address{{Address: "addr1"}},
		DryRun:      true,
		FeeRate:     250,
	}

	_, err := service.Sweep(ctx, opts)

	require.Error(t, err)
	// Should fail due to insufficient funds or dust threshold
}

func TestSweepService_Sweep_MultipleAddresses(t *testing.T) {
	client := &mockBSVClientForSweep{}
	bulkOps := &mockBulkOpsForSweep{
		fetchFunc: func(_ context.Context, _ []string) ([]bsv.BulkUTXOResult, error) {
			return []bsv.BulkUTXOResult{
				{
					Address:        "addr1",
					ConfirmedUTXOs: []bsv.UTXO{{TxID: "tx1", Vout: 0, Amount: 10000, Address: "addr1"}},
				},
				{
					Address:        "addr2",
					ConfirmedUTXOs: []bsv.UTXO{{TxID: "tx2", Vout: 0, Amount: 20000, Address: "addr2"}},
				},
				{
					Address:        "addr3",
					ConfirmedUTXOs: []bsv.UTXO{{TxID: "tx3", Vout: 0, Amount: 30000, Address: "addr3"}},
				},
			}, nil
		},
	}

	service := NewSweepService(client, bulkOps, nil)
	ctx := context.Background()

	opts := &SweepOptions{
		Destination: "dest_addr",
		Addresses: []wallet.Address{
			{Address: "addr1"},
			{Address: "addr2"},
			{Address: "addr3"},
		},
		DryRun:  true,
		FeeRate: 250,
	}

	result, err := service.Sweep(ctx, opts)

	require.NoError(t, err)
	assert.Equal(t, 3, result.UTXOsUsed)
	assert.Equal(t, 3, result.AddressesUsed)
	assert.Equal(t, uint64(60000), result.TotalInput)
}

func TestSweepService_Sweep_ValidationOptions(t *testing.T) {
	t.Run("missing destination", func(t *testing.T) {
		service := NewSweepService(nil, nil, nil)
		_, err := service.Sweep(context.Background(), &SweepOptions{
			Addresses: []wallet.Address{{Address: "addr1"}},
		})
		require.Error(t, err)
	})

	t.Run("no addresses", func(t *testing.T) {
		service := NewSweepService(nil, nil, nil)
		_, err := service.Sweep(context.Background(), &SweepOptions{
			Destination: "dest_addr",
			Addresses:   []wallet.Address{},
		})
		require.Error(t, err)
	})

	t.Run("missing seed for non-dry-run", func(t *testing.T) {
		service := NewSweepService(nil, nil, nil)
		_, err := service.Sweep(context.Background(), &SweepOptions{
			Destination: "dest_addr",
			Addresses:   []wallet.Address{{Address: "addr1"}},
			DryRun:      false,
			// Missing Seed
		})
		require.Error(t, err)
	})
}
