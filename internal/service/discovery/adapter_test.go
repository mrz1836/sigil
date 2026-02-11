package discovery

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
)

// mockChainClient for testing chain client adapter
type mockChainClient struct {
	listUTXOsCalled bool
	listUTXOsAddr   string
	listUTXOsResult []chain.UTXO
	listUTXOsErr    error
}

func (m *mockChainClient) ListUTXOs(_ context.Context, address string) ([]chain.UTXO, error) {
	m.listUTXOsCalled = true
	m.listUTXOsAddr = address
	if m.listUTXOsErr != nil {
		return nil, m.listUTXOsErr
	}
	return m.listUTXOsResult, nil
}

// TestNewUTXOStoreAdapter tests adapter creation.
func TestNewUTXOStoreAdapter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewUTXOStoreAdapter(store)

	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.store)
}

// TestUTXOStoreAdapter_RefreshAddress tests delegation of RefreshAddress.
func TestUTXOStoreAdapter_RefreshAddress(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewUTXOStoreAdapter(store)

	mockClient := &mockChainClient{
		listUTXOsResult: []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		},
	}

	ctx := context.Background()
	err := adapter.RefreshAddress(ctx, "1ABC", chain.BSV, mockClient)

	require.NoError(t, err)

	// Verify delegation worked by checking store was updated
	metadata := adapter.GetAddress(chain.BSV, "1ABC")
	require.NotNil(t, metadata)
	assert.Equal(t, "1ABC", metadata.Address)
	assert.Equal(t, chain.BSV, metadata.ChainID)
}

// TestUTXOStoreAdapter_GetAddressBalance tests delegation of GetAddressBalance.
func TestUTXOStoreAdapter_GetAddressBalance(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewUTXOStoreAdapter(store)

	// First refresh to populate store
	mockClient := &mockChainClient{
		listUTXOsResult: []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 300000, Address: "1ABC"},
			{TxID: "tx2", Vout: 1, Amount: 200000, Address: "1ABC"},
		},
	}

	ctx := context.Background()
	err := adapter.RefreshAddress(ctx, "1ABC", chain.BSV, mockClient)
	require.NoError(t, err)

	// Get balance
	balance := adapter.GetAddressBalance(chain.BSV, "1ABC")
	assert.Equal(t, uint64(500000), balance)
}

// TestUTXOStoreAdapter_GetUTXOs tests delegation of GetUTXOs.
func TestUTXOStoreAdapter_GetUTXOs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewUTXOStoreAdapter(store)

	// First refresh to populate store
	mockClient := &mockChainClient{
		listUTXOsResult: []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
			{TxID: "tx2", Vout: 1, Amount: 200000, Address: "1ABC"},
		},
	}

	ctx := context.Background()
	err := adapter.RefreshAddress(ctx, "1ABC", chain.BSV, mockClient)
	require.NoError(t, err)

	// Get UTXOs
	utxos := adapter.GetUTXOs(chain.BSV, "1ABC")

	assert.Len(t, utxos, 2)
	assert.Equal(t, "tx1", utxos[0].TxID)
	assert.Equal(t, "tx2", utxos[1].TxID)
}

// TestUTXOStoreAdapter_GetAddress tests delegation of GetAddress.
func TestUTXOStoreAdapter_GetAddress(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewUTXOStoreAdapter(store)

	// First refresh to populate store
	mockClient := &mockChainClient{
		listUTXOsResult: []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		},
	}

	ctx := context.Background()
	err := adapter.RefreshAddress(ctx, "1ABC", chain.BSV, mockClient)
	require.NoError(t, err)

	// Get address metadata
	metadata := adapter.GetAddress(chain.BSV, "1ABC")

	require.NotNil(t, metadata)
	assert.Equal(t, "1ABC", metadata.Address)
	assert.Equal(t, chain.BSV, metadata.ChainID)
	assert.True(t, metadata.HasActivity)
}

// TestUTXOStoreAdapter_GetAddress_Nil tests handling of nil result.
func TestUTXOStoreAdapter_GetAddress_Nil(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewUTXOStoreAdapter(store)

	// Try to get address that was never refreshed
	metadata := adapter.GetAddress(chain.BSV, "1NOTFOUND")

	assert.Nil(t, metadata)
}

// TestUTXOChainClientAdapter_ListUTXOs tests chain client adapter delegation.
func TestUTXOChainClientAdapter_ListUTXOs(t *testing.T) {
	t.Parallel()

	expectedUTXOs := []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
		{TxID: "tx2", Vout: 1, Amount: 200000, Address: "1ABC"},
	}

	mockClient := &mockChainClient{
		listUTXOsResult: expectedUTXOs,
	}

	adapter := &utxoChainClientAdapter{client: mockClient}

	ctx := context.Background()
	utxos, err := adapter.ListUTXOs(ctx, "1ABC")

	require.NoError(t, err)
	assert.Equal(t, expectedUTXOs, utxos)
	assert.True(t, mockClient.listUTXOsCalled, "should delegate to client.ListUTXOs")
	assert.Equal(t, "1ABC", mockClient.listUTXOsAddr)
}

// TestUTXOChainClientAdapter_ListUTXOs_Error tests error propagation.
func TestUTXOChainClientAdapter_ListUTXOs_Error(t *testing.T) {
	t.Parallel()

	mockClient := &mockChainClient{
		listUTXOsErr: assert.AnError,
	}

	adapter := &utxoChainClientAdapter{client: mockClient}

	ctx := context.Background()
	utxos, err := adapter.ListUTXOs(ctx, "1ABC")

	require.Error(t, err)
	assert.Nil(t, utxos)
	assert.True(t, mockClient.listUTXOsCalled)
}

// TestUTXOChainClientAdapter_ListUTXOs_EmptyResult tests empty UTXO list.
func TestUTXOChainClientAdapter_ListUTXOs_EmptyResult(t *testing.T) {
	t.Parallel()

	mockClient := &mockChainClient{
		listUTXOsResult: []chain.UTXO{},
	}

	adapter := &utxoChainClientAdapter{client: mockClient}

	ctx := context.Background()
	utxos, err := adapter.ListUTXOs(ctx, "1EMPTY")

	require.NoError(t, err)
	assert.Empty(t, utxos)
	assert.True(t, mockClient.listUTXOsCalled)
}

// TestUTXOStoreAdapter_RefreshAddress_WithRealStore tests integration with real store.
func TestUTXOStoreAdapter_RefreshAddress_WithRealStore(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewUTXOStoreAdapter(store)

	mockClient := &mockChainClient{
		listUTXOsResult: []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1ABC"},
			{TxID: "tx2", Vout: 1, Amount: 200000, Address: "1ABC"},
		},
	}

	ctx := context.Background()
	err := adapter.RefreshAddress(ctx, "1ABC", chain.BSV, mockClient)

	require.NoError(t, err)

	// Verify UTXOs were stored
	utxos := adapter.GetUTXOs(chain.BSV, "1ABC")
	assert.Len(t, utxos, 2)

	// Verify balance was calculated
	balance := adapter.GetAddressBalance(chain.BSV, "1ABC")
	assert.Equal(t, uint64(300000), balance)

	// Verify metadata was created
	metadata := adapter.GetAddress(chain.BSV, "1ABC")
	require.NotNil(t, metadata)
	assert.Equal(t, "1ABC", metadata.Address)
	assert.Equal(t, chain.BSV, metadata.ChainID)
}

// TestUTXOStoreAdapter_MultipleChains tests adapter works with different chains.
func TestUTXOStoreAdapter_MultipleChains(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	store := utxostore.New(tmpDir)
	adapter := NewUTXOStoreAdapter(store)

	mockClient := &mockChainClient{
		listUTXOsResult: []chain.UTXO{
			{TxID: "tx1", Vout: 0, Amount: 100000, Address: "1BSV"},
		},
	}

	ctx := context.Background()

	// Refresh BSV address
	err := adapter.RefreshAddress(ctx, "1BSV", chain.BSV, mockClient)
	require.NoError(t, err)

	// Verify BSV address stored
	metadata := adapter.GetAddress(chain.BSV, "1BSV")
	require.NotNil(t, metadata)
	assert.Equal(t, chain.BSV, metadata.ChainID)

	// Refresh ETH address (conceptually, though ETH doesn't use UTXOs)
	err = adapter.RefreshAddress(ctx, "0xETH", chain.ETH, mockClient)
	require.NoError(t, err)

	// Verify ETH address stored separately
	ethMetadata := adapter.GetAddress(chain.ETH, "0xETH")
	require.NotNil(t, ethMetadata)
	assert.Equal(t, chain.ETH, ethMetadata.ChainID)
}
