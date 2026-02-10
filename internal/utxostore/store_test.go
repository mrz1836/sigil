package utxostore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

func TestNew(t *testing.T) {
	t.Parallel()
	store := New("/tmp/test-wallet")

	assert.NotNil(t, store)
	assert.Equal(t, "/tmp/test-wallet", store.walletPath)
	assert.NotNil(t, store.data)
	assert.Equal(t, currentVersion, store.data.Version)
	assert.NotNil(t, store.data.UTXOs)
	assert.NotNil(t, store.data.Addresses)
	assert.True(t, store.IsEmpty())
}

func TestStoredUTXOKey(t *testing.T) {
	t.Parallel()
	utxo := &StoredUTXO{
		ChainID: chain.BSV,
		TxID:    "abc123",
		Vout:    1,
	}

	assert.Equal(t, "bsv:abc123:1", utxo.Key())
}

func TestAddressMetadataKey(t *testing.T) {
	t.Parallel()
	addr := &AddressMetadata{
		ChainID: chain.BSV,
		Address: "1ABC123",
	}

	assert.Equal(t, "bsv:1ABC123", addr.Key())
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	// Create temp directory
	tmpDir := t.TempDir()

	store := New(tmpDir)
	err := store.Load()

	// Should not error on missing file (fresh wallet)
	require.NoError(t, err)
	assert.True(t, store.IsEmpty())
}

func TestLoadSave(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create store and add data
	store := New(tmpDir)

	utxo := &StoredUTXO{
		ChainID:       chain.BSV,
		TxID:          "txid123",
		Vout:          0,
		Amount:        100000,
		ScriptPubKey:  "76a914...",
		Address:       "1Address",
		Confirmations: 10,
		Spent:         false,
	}
	store.AddUTXO(utxo)

	addr := &AddressMetadata{
		ChainID:        chain.BSV,
		Address:        "1Address",
		DerivationPath: "m/44'/236'/0'/0/0",
		Index:          0,
		Label:          "Main",
		HasActivity:    true,
	}
	store.AddAddress(addr)

	// Save
	err := store.Save()
	require.NoError(t, err)

	// Verify file exists
	filePath := filepath.Join(tmpDir, utxoFileName)
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	// Load in new store
	store2 := New(tmpDir)
	err = store2.Load()
	require.NoError(t, err)

	// Verify data
	assert.False(t, store2.IsEmpty())
	assert.Equal(t, uint64(100000), store2.GetBalance(chain.BSV))

	utxos := store2.GetUTXOs(chain.BSV, "")
	require.Len(t, utxos, 1)
	assert.Equal(t, "txid123", utxos[0].TxID)
	assert.Equal(t, uint32(0), utxos[0].Vout)
	assert.Equal(t, uint64(100000), utxos[0].Amount)

	addrs := store2.GetAddresses(chain.BSV)
	require.Len(t, addrs, 1)
	assert.Equal(t, "1Address", addrs[0].Address)
	assert.Equal(t, "Main", addrs[0].Label)
}

func TestLoadVersionTooNew(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Write file with future version
	filePath := filepath.Join(tmpDir, utxoFileName)
	data := `{"version": 999, "updated_at": "2026-01-01T00:00:00Z", "utxos": {}, "addresses": {}}`
	err := os.WriteFile(filePath, []byte(data), filePermissions)
	require.NoError(t, err)

	store := New(tmpDir)
	err = store.Load()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrVersionTooNew)
}

func TestGetUTXOs(t *testing.T) {
	t.Parallel()
	store := New("/tmp/test")

	// Add multiple UTXOs
	store.AddUTXO(&StoredUTXO{
		ChainID: chain.BSV,
		TxID:    "tx1",
		Vout:    0,
		Amount:  1000,
		Address: "addr1",
		Spent:   false,
	})
	store.AddUTXO(&StoredUTXO{
		ChainID: chain.BSV,
		TxID:    "tx2",
		Vout:    0,
		Amount:  2000,
		Address: "addr2",
		Spent:   false,
	})
	store.AddUTXO(&StoredUTXO{
		ChainID: chain.BSV,
		TxID:    "tx3",
		Vout:    0,
		Amount:  3000,
		Address: "addr1",
		Spent:   true, // spent
	})
	store.AddUTXO(&StoredUTXO{
		ChainID: chain.BTC,
		TxID:    "tx4",
		Vout:    0,
		Amount:  4000,
		Address: "btcaddr",
		Spent:   false,
	})

	// Get all BSV UTXOs
	utxos := store.GetUTXOs(chain.BSV, "")
	assert.Len(t, utxos, 2) // excludes spent

	// Get UTXOs for specific address
	utxos = store.GetUTXOs(chain.BSV, "addr1")
	assert.Len(t, utxos, 1) // only unspent for addr1

	// Get BTC UTXOs
	utxos = store.GetUTXOs(chain.BTC, "")
	assert.Len(t, utxos, 1)
}

func TestGetBalance(t *testing.T) {
	t.Parallel()
	store := New("/tmp/test")

	// Add UTXOs
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
	store.AddUTXO(&StoredUTXO{
		ChainID: chain.BSV,
		TxID:    "tx3",
		Vout:    0,
		Amount:  3000,
		Spent:   true, // should not count
	})

	balance := store.GetBalance(chain.BSV)
	assert.Equal(t, uint64(3000), balance) // 1000 + 2000

	// No BTC UTXOs
	balance = store.GetBalance(chain.BTC)
	assert.Equal(t, uint64(0), balance)
}

func TestMarkSpent(t *testing.T) {
	t.Parallel()
	store := New("/tmp/test")

	// Add UTXO
	store.AddUTXO(&StoredUTXO{
		ChainID: chain.BSV,
		TxID:    "txid",
		Vout:    0,
		Amount:  5000,
		Spent:   false,
	})

	// Verify initial balance
	assert.Equal(t, uint64(5000), store.GetBalance(chain.BSV))

	// Mark as spent
	found := store.MarkSpent(chain.BSV, "txid", 0, "spending-tx")
	assert.True(t, found)

	// Balance should be 0 now
	assert.Equal(t, uint64(0), store.GetBalance(chain.BSV))

	// UTXO should still exist (preserved for history)
	assert.False(t, store.IsEmpty())

	// Try to mark non-existent UTXO
	found = store.MarkSpent(chain.BSV, "nonexistent", 0, "tx")
	assert.False(t, found)
}

func TestAddUTXO(t *testing.T) {
	t.Parallel()
	store := New("/tmp/test")

	utxo := &StoredUTXO{
		ChainID: chain.BSV,
		TxID:    "txid",
		Vout:    0,
		Amount:  1000,
	}

	// Add UTXO
	store.AddUTXO(utxo)
	assert.False(t, store.IsEmpty())

	// FirstSeen should be set
	assert.False(t, utxo.FirstSeen.IsZero())

	// Update same UTXO
	utxo.Amount = 2000
	store.AddUTXO(utxo)

	// Should still be only one UTXO
	utxos := store.GetUTXOs(chain.BSV, "")
	assert.Len(t, utxos, 1)
	assert.Equal(t, uint64(2000), utxos[0].Amount)
}

func TestAddAddress(t *testing.T) {
	t.Parallel()
	store := New("/tmp/test")

	addr := &AddressMetadata{
		ChainID:        chain.BSV,
		Address:        "1Addr",
		DerivationPath: "m/44'/236'/0'/0/0",
		Index:          0,
	}

	store.AddAddress(addr)

	addrs := store.GetAddresses(chain.BSV)
	require.Len(t, addrs, 1)
	assert.Equal(t, "1Addr", addrs[0].Address)

	// Update address
	addr.Label = "Updated"
	store.AddAddress(addr)

	addrs = store.GetAddresses(chain.BSV)
	assert.Len(t, addrs, 1)
	assert.Equal(t, "Updated", addrs[0].Label)
}

func TestIsEmpty(t *testing.T) {
	t.Parallel()
	store := New("/tmp/test")

	assert.True(t, store.IsEmpty())

	store.AddUTXO(&StoredUTXO{
		ChainID: chain.BSV,
		TxID:    "tx",
		Vout:    0,
		Amount:  100,
	})

	assert.False(t, store.IsEmpty())
}

func TestIsSpent(t *testing.T) {
	t.Parallel()

	t.Run("returns true for spent UTXO", func(t *testing.T) {
		t.Parallel()
		store := New("/tmp/test")
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    0,
			Amount:  5000,
			Spent:   false,
		})
		store.MarkSpent(chain.BSV, "tx1", 0, "spending-tx")

		assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))
	})

	t.Run("returns false for unspent UTXO", func(t *testing.T) {
		t.Parallel()
		store := New("/tmp/test")
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    0,
			Amount:  5000,
			Spent:   false,
		})

		assert.False(t, store.IsSpent(chain.BSV, "tx1", 0))
	})

	t.Run("returns false for unknown UTXO", func(t *testing.T) {
		t.Parallel()
		store := New("/tmp/test")

		assert.False(t, store.IsSpent(chain.BSV, "nonexistent", 0))
	})

	t.Run("different vout is independent", func(t *testing.T) {
		t.Parallel()
		store := New("/tmp/test")
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    0,
			Amount:  1000,
			Spent:   false,
		})
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    1,
			Amount:  2000,
			Spent:   false,
		})
		store.MarkSpent(chain.BSV, "tx1", 0, "spending-tx")

		assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))
		assert.False(t, store.IsSpent(chain.BSV, "tx1", 1))
	})

	t.Run("different chain is independent", func(t *testing.T) {
		t.Parallel()
		store := New("/tmp/test")
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    0,
			Amount:  1000,
			Spent:   true,
		})
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BTC,
			TxID:    "tx1",
			Vout:    0,
			Amount:  1000,
			Spent:   false,
		})

		assert.True(t, store.IsSpent(chain.BSV, "tx1", 0))
		assert.False(t, store.IsSpent(chain.BTC, "tx1", 0))
	})

	t.Run("round trip through save and load", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		store := New(tmpDir)
		store.AddUTXO(&StoredUTXO{
			ChainID: chain.BSV,
			TxID:    "tx1",
			Vout:    0,
			Amount:  5000,
			Spent:   false,
		})
		store.MarkSpent(chain.BSV, "tx1", 0, "spending-tx")
		require.NoError(t, store.Save())

		store2 := New(tmpDir)
		require.NoError(t, store2.Load())

		assert.True(t, store2.IsSpent(chain.BSV, "tx1", 0))
	})
}

func TestGetAddresses(t *testing.T) {
	t.Parallel()
	store := New("/tmp/test")

	// Add addresses for different chains
	store.AddAddress(&AddressMetadata{
		ChainID: chain.BSV,
		Address: "bsv1",
	})
	store.AddAddress(&AddressMetadata{
		ChainID: chain.BSV,
		Address: "bsv2",
	})
	store.AddAddress(&AddressMetadata{
		ChainID: chain.BTC,
		Address: "btc1",
	})

	bsvAddrs := store.GetAddresses(chain.BSV)
	assert.Len(t, bsvAddrs, 2)

	btcAddrs := store.GetAddresses(chain.BTC)
	assert.Len(t, btcAddrs, 1)

	bchAddrs := store.GetAddresses(chain.BCH)
	assert.Empty(t, bchAddrs)
}
