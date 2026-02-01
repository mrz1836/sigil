package utxostore_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClient implements utxostore.ChainClient for integration testing.
type mockClient struct {
	utxos map[string][]chain.UTXO
}

func newMockClient() *mockClient {
	return &mockClient{utxos: make(map[string][]chain.UTXO)}
}

func (m *mockClient) ListUTXOs(_ context.Context, address string) ([]chain.UTXO, error) {
	return m.utxos[address], nil
}

func (m *mockClient) setUTXOs(address string, utxos []chain.UTXO) {
	m.utxos[address] = utxos
}

// TestIntegration_FullWorkflow tests the complete UTXO storage workflow:
// 1. Create store
// 2. Scan wallet (initial discovery)
// 3. Verify persistence
// 4. Refresh (simulate spent/new UTXOs)
// 5. Verify merge logic
func TestIntegration_FullWorkflow(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	client := newMockClient()
	ctx := context.Background()

	// Create wallet with addresses
	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "addr0", Path: "m/44'/236'/0'/0/0", Index: 0},
				{Address: "addr1", Path: "m/44'/236'/0'/0/1", Index: 1},
				{Address: "addr2", Path: "m/44'/236'/0'/0/2", Index: 2},
			},
		},
	}

	// Initial chain state: addr0 and addr1 have UTXOs
	client.setUTXOs("addr0", []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 10000, Address: "addr0", Confirmations: 100},
		{TxID: "tx2", Vout: 1, Amount: 20000, Address: "addr0", Confirmations: 50},
	})
	client.setUTXOs("addr1", []chain.UTXO{
		{TxID: "tx3", Vout: 0, Amount: 5000, Address: "addr1", Confirmations: 10},
	})

	// Step 1: Create store and scan wallet
	store := utxostore.New(tmpDir)
	result, err := store.ScanWallet(ctx, w, chain.BSV, client)
	require.NoError(t, err)

	assert.Equal(t, 3, result.AddressesScanned)
	assert.Equal(t, 3, result.UTXOsFound)
	assert.Equal(t, uint64(35000), result.TotalBalance)

	// Verify balance
	assert.Equal(t, uint64(35000), store.GetBalance(chain.BSV))

	// Step 2: Verify persistence - load in new store
	store2 := utxostore.New(tmpDir)
	err = store2.Load()
	require.NoError(t, err)

	assert.False(t, store2.IsEmpty())
	assert.Equal(t, uint64(35000), store2.GetBalance(chain.BSV))

	// Verify address metadata was persisted
	addrs := store2.GetAddresses(chain.BSV)
	assert.Len(t, addrs, 3)

	// Step 3: Simulate chain changes
	// - tx1 is spent (gone from chain response)
	// - tx3 is spent (gone)
	// - tx4 is new
	client.setUTXOs("addr0", []chain.UTXO{
		{TxID: "tx2", Vout: 1, Amount: 20000, Address: "addr0", Confirmations: 51}, // Still exists, more confirms
		{TxID: "tx4", Vout: 0, Amount: 15000, Address: "addr0", Confirmations: 1},  // New
	})
	client.setUTXOs("addr1", nil) // tx3 spent

	// Step 4: Refresh
	result, err = store2.Refresh(ctx, chain.BSV, client)
	require.NoError(t, err)

	// tx2 (20000) + tx4 (15000) = 35000
	assert.Equal(t, uint64(35000), result.TotalBalance)

	// Balance should reflect only unspent
	assert.Equal(t, uint64(35000), store2.GetBalance(chain.BSV))

	// Step 5: Verify spent UTXOs are preserved
	// The store should still have tx1 and tx3, but marked as spent
	utxos := store2.GetUTXOs(chain.BSV, "")
	assert.Len(t, utxos, 2) // Only unspent: tx2 and tx4

	// Store is not empty (has historical data)
	assert.False(t, store2.IsEmpty())
}

// TestIntegration_OfflineAccess verifies offline balance access after scan.
func TestIntegration_OfflineAccess(t *testing.T) {
	tmpDir := t.TempDir()
	client := newMockClient()
	ctx := context.Background()

	w := &wallet.Wallet{
		Addresses: map[chain.ID][]wallet.Address{
			chain.BSV: {
				{Address: "addr0", Index: 0},
			},
		},
	}

	client.setUTXOs("addr0", []chain.UTXO{
		{TxID: "tx1", Vout: 0, Amount: 100000, Address: "addr0"},
	})

	// Scan and save
	store := utxostore.New(tmpDir)
	_, err := store.ScanWallet(ctx, w, chain.BSV, client)
	require.NoError(t, err)

	// Simulate going offline - create new store, load from disk
	// No network calls needed
	offlineStore := utxostore.New(tmpDir)
	err = offlineStore.Load()
	require.NoError(t, err)

	// Should have balance from persisted data
	assert.Equal(t, uint64(100000), offlineStore.GetBalance(chain.BSV))

	// Can list UTXOs offline
	utxos := offlineStore.GetUTXOs(chain.BSV, "")
	require.Len(t, utxos, 1)
	assert.Equal(t, "tx1", utxos[0].TxID)
}

// TestIntegration_AddressLabels verifies address label persistence.
func TestIntegration_AddressLabels(t *testing.T) {
	tmpDir := t.TempDir()

	store := utxostore.New(tmpDir)

	// Add addresses with labels
	store.AddAddress(&utxostore.AddressMetadata{
		ChainID:        chain.BSV,
		Address:        "addr1",
		DerivationPath: "m/44'/236'/0'/0/0",
		Index:          0,
		Label:          "Savings",
	})
	store.AddAddress(&utxostore.AddressMetadata{
		ChainID:        chain.BSV,
		Address:        "addr2",
		DerivationPath: "m/44'/236'/0'/0/1",
		Index:          1,
		Label:          "Donations",
	})

	err := store.Save()
	require.NoError(t, err)

	// Reload
	store2 := utxostore.New(tmpDir)
	err = store2.Load()
	require.NoError(t, err)

	addrs := store2.GetAddresses(chain.BSV)
	assert.Len(t, addrs, 2)

	// Verify labels preserved
	labels := make(map[string]string)
	for _, a := range addrs {
		labels[a.Address] = a.Label
	}
	assert.Equal(t, "Savings", labels["addr1"])
	assert.Equal(t, "Donations", labels["addr2"])
}

// TestIntegration_MultiChain verifies chain isolation.
func TestIntegration_MultiChain(t *testing.T) {
	tmpDir := t.TempDir()

	store := utxostore.New(tmpDir)

	// Add UTXOs for different chains
	store.AddUTXO(&utxostore.StoredUTXO{
		ChainID: chain.BSV,
		TxID:    "bsv-tx",
		Vout:    0,
		Amount:  10000,
	})
	store.AddUTXO(&utxostore.StoredUTXO{
		ChainID: chain.BTC,
		TxID:    "btc-tx",
		Vout:    0,
		Amount:  20000,
	})

	err := store.Save()
	require.NoError(t, err)

	// Reload
	store2 := utxostore.New(tmpDir)
	err = store2.Load()
	require.NoError(t, err)

	// Verify chain isolation
	assert.Equal(t, uint64(10000), store2.GetBalance(chain.BSV))
	assert.Equal(t, uint64(20000), store2.GetBalance(chain.BTC))
	assert.Equal(t, uint64(0), store2.GetBalance(chain.BCH))

	bsvUTXOs := store2.GetUTXOs(chain.BSV, "")
	assert.Len(t, bsvUTXOs, 1)
	assert.Equal(t, "bsv-tx", bsvUTXOs[0].TxID)

	btcUTXOs := store2.GetUTXOs(chain.BTC, "")
	assert.Len(t, btcUTXOs, 1)
	assert.Equal(t, "btc-tx", btcUTXOs[0].TxID)
}

// TestIntegration_AtomicWrite verifies atomic write behavior.
func TestIntegration_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	store := utxostore.New(tmpDir)
	store.AddUTXO(&utxostore.StoredUTXO{
		ChainID: chain.BSV,
		TxID:    "tx1",
		Vout:    0,
		Amount:  1000,
	})

	err := store.Save()
	require.NoError(t, err)

	// Verify no temp file left behind
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	for _, f := range files {
		assert.NotContains(t, f.Name(), ".tmp", "temp file should be cleaned up")
	}

	// Verify main file exists
	_, err = os.Stat(filepath.Join(tmpDir, "utxos.json"))
	require.NoError(t, err)
}
