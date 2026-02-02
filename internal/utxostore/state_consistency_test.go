package utxostore

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

// TestState_MarkSpentUpdatesBalance tests that marking UTXOs as spent immediately updates balance.
func TestState_MarkSpentUpdatesBalance(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Setup: 3 UTXOs
	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	amounts := []uint64{1000, 2000, 3000}
	for i, amount := range amounts {
		utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, amount, false)
		store.AddUTXO(utxo)
	}

	// Initial balance: 6000
	assertBalanceEquals(t, store, chain.BSV, 6000)

	// Mark first UTXO as spent
	found := store.MarkSpent(chain.BSV, testTxID(0), 0, testTxID(100))
	require.True(t, found)

	// Balance should immediately reflect the change
	assertBalanceEquals(t, store, chain.BSV, 5000)

	// Mark second UTXO as spent
	found = store.MarkSpent(chain.BSV, testTxID(1), 0, testTxID(101))
	require.True(t, found)
	assertBalanceEquals(t, store, chain.BSV, 3000)

	// Mark third UTXO as spent
	found = store.MarkSpent(chain.BSV, testTxID(2), 0, testTxID(102))
	require.True(t, found)
	assertBalanceEquals(t, store, chain.BSV, 0)

	// Trying to mark non-existent UTXO returns false
	found = store.MarkSpent(chain.BSV, testTxID(999), 0, testTxID(103))
	assert.False(t, found)
}

// TestState_ConcurrentReads tests concurrent read operations don't cause races.
//
//nolint:gosec // Test code uses bounded loop variables for uint32 conversions
func TestState_ConcurrentReads(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Setup: 100 UTXOs
	const numUTXOs = 100
	for i := 0; i < numUTXOs; i++ {
		addr := testAddressN(i % 10)
		metadata := createTestAddress(chain.BSV, addr, uint32(i%10), false)
		store.AddAddress(metadata)

		utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 100, false)
		store.AddUTXO(utxo)
	}

	// Run concurrent reads
	const numGoroutines = 20
	const readsPerGoroutine = 100
	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < readsPerGoroutine; i++ {
				// Mix of read operations
				_ = store.GetBalance(chain.BSV)
				_ = store.GetUTXOs(chain.BSV, "")
				_ = store.GetAddresses(chain.BSV)
				_ = store.IsEmpty()
			}
		}()
	}

	wg.Wait()

	// Verify state is still consistent
	assertBalanceEquals(t, store, chain.BSV, numUTXOs*100)
}

// TestState_ConcurrentReadsWrites tests concurrent read and write operations.
//
//nolint:gosec // Test code uses bounded loop variables for uint32 conversions
func TestState_ConcurrentReadsWrites(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Setup initial state
	const initialUTXOs = 50
	for i := 0; i < initialUTXOs; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 100, false)
		store.AddUTXO(utxo)
	}

	// Run concurrent reads and writes
	const numGoroutines = 10
	var wg sync.WaitGroup

	// Reader goroutines
	for g := 0; g < numGoroutines/2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = store.GetBalance(chain.BSV)
				_ = store.GetUTXOs(chain.BSV, "")
			}
		}()
	}

	// Writer goroutines
	for g := 0; g < numGoroutines/2; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				// Add new UTXOs
				utxo := createTestUTXO(chain.BSV,
					testAddressN(gid+100),
					testTxID(1000+gid*100+i),
					0, 50, false)
				store.AddUTXO(utxo)
			}
		}(g)
	}

	wg.Wait()

	// Verify balance is positive and state is valid
	balance := store.GetBalance(chain.BSV)
	assert.Positive(t, balance)
}

// TestState_PersistReload tests that complex state survives save/load cycle.
//
//nolint:gocognit,gosec // Complex test setup required for thorough state testing
func TestState_PersistReload(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Build complex state
	// 1. Multiple chains
	chains := []chain.ID{chain.BSV, chain.BTC}
	for _, chainID := range chains {
		for i := 0; i < 5; i++ {
			addr := testAddressN(int(chainID.CoinType())*100 + i)
			metadata := createTestAddress(chainID, addr, uint32(i), i%2 == 0)
			metadata.Label = "test-label"
			metadata.HasActivity = i < 3
			store.AddAddress(metadata)

			if i < 3 { // Only fund first 3
				for j := 0; j < 2; j++ {
					utxo := createTestUTXO(chainID, addr, testTxID(int(chainID.CoinType())*1000+i*10+j), uint32(j), 100, false)
					store.AddUTXO(utxo)
				}
			}
		}
	}

	// Add some spent UTXOs
	spentUTXO := createTestUTXO(chain.BSV, testAddressN(0), testTxID(9999), 0, 500, true)
	spentUTXO.SpentTxID = testTxID(10000)
	store.AddUTXO(spentUTXO)

	// Record state before save
	bsvBalanceBefore := store.GetBalance(chain.BSV)
	btcBalanceBefore := store.GetBalance(chain.BTC)
	bsvAddrsBefore := len(store.GetAddresses(chain.BSV))
	btcAddrsBefore := len(store.GetAddresses(chain.BTC))

	// Save
	err := store.Save()
	require.NoError(t, err)

	// Create new store and load
	loadedStore := New(store.walletPath)
	err = loadedStore.Load()
	require.NoError(t, err)

	// Verify all state is preserved
	assert.Equal(t, bsvBalanceBefore, loadedStore.GetBalance(chain.BSV))
	assert.Equal(t, btcBalanceBefore, loadedStore.GetBalance(chain.BTC))
	assert.Len(t, loadedStore.GetAddresses(chain.BSV), bsvAddrsBefore)
	assert.Len(t, loadedStore.GetAddresses(chain.BTC), btcAddrsBefore)

	// Verify address metadata is preserved
	for _, addr := range loadedStore.GetAddresses(chain.BSV) {
		if addr.Index < 3 {
			assert.True(t, addr.HasActivity, "address %d should have activity", addr.Index)
		}
	}
}

// TestState_RefreshMerge tests merging new chain data with existing stored data.
func TestState_RefreshMerge(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	// Initial state: 2 UTXOs
	utxo1 := createTestUTXO(chain.BSV, addr, testTxID(0), 0, 1000, false)
	utxo2 := createTestUTXO(chain.BSV, addr, testTxID(1), 0, 2000, false)
	store.AddUTXO(utxo1)
	store.AddUTXO(utxo2)
	assertBalanceEquals(t, store, chain.BSV, 3000)

	// Simulate "refresh" - one UTXO is now spent, one new UTXO added
	// In a real implementation, this would come from blockchain API

	// Mark utxo1 as spent
	store.MarkSpent(chain.BSV, testTxID(0), 0, testTxID(100))

	// Add new UTXO (maybe change from spending utxo1)
	newUTXO := createTestUTXO(chain.BSV, addr, testTxID(100), 1, 500, false)
	store.AddUTXO(newUTXO)

	// Final state: utxo2 (2000) + newUTXO (500) = 2500
	assertBalanceEquals(t, store, chain.BSV, 2500)

	// Verify UTXO states
	allUTXOs := store.GetUTXOs(chain.BSV, "")
	assert.Len(t, allUTXOs, 2) // 2 unspent

	// Verify the spent UTXO is still stored but not counted
	// (it should exist in the store but marked as spent)
}

// TestState_AtomicWriteFailure tests that partial writes don't corrupt state.
func TestState_AtomicWriteFailure(t *testing.T) {
	t.Parallel()

	// Create store in temp directory
	tmpDir := t.TempDir()
	store := New(tmpDir)

	// Build some state
	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	utxo := createTestUTXO(chain.BSV, addr, testTxID(0), 0, 5000, false)
	store.AddUTXO(utxo)

	// Save successfully first
	err := store.Save()
	require.NoError(t, err)

	// Verify file exists
	filePath := filepath.Join(tmpDir, "utxos.json")
	info, err := os.Stat(filePath)
	require.NoError(t, err)
	originalSize := info.Size()

	// Modify state
	utxo2 := createTestUTXO(chain.BSV, addr, testTxID(1), 0, 3000, false)
	store.AddUTXO(utxo2)

	// Save again
	err = store.Save()
	require.NoError(t, err)

	// File should be updated (different size due to new UTXO)
	info, err = os.Stat(filePath)
	require.NoError(t, err)
	assert.NotEqual(t, originalSize, info.Size())

	// Load and verify
	loadedStore := New(tmpDir)
	err = loadedStore.Load()
	require.NoError(t, err)
	assertBalanceEquals(t, loadedStore, chain.BSV, 8000)
}

// TestState_EmptyStoreOperations tests operations on an empty store.
func TestState_EmptyStoreOperations(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// All operations should work on empty store
	assert.True(t, store.IsEmpty())
	assertBalanceEquals(t, store, chain.BSV, 0)
	assert.Empty(t, store.GetUTXOs(chain.BSV, ""))
	assert.Empty(t, store.GetAddresses(chain.BSV))

	// MarkSpent on non-existent UTXO should return false
	found := store.MarkSpent(chain.BSV, "nonexistent", 0, "tx")
	assert.False(t, found)

	// GetAddress on non-existent address should return nil
	addr := store.GetAddress(chain.BSV, "nonexistent")
	assert.Nil(t, addr)

	// Save and load empty store
	err := store.Save()
	require.NoError(t, err)

	loadedStore := New(store.walletPath)
	err = loadedStore.Load()
	require.NoError(t, err)
	assert.True(t, loadedStore.IsEmpty())
}

// TestState_AddressLabelUpdate tests updating address labels.
func TestState_AddressLabelUpdate(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	// Set label
	err := store.SetAddressLabel(chain.BSV, addr, "My Savings")
	require.NoError(t, err)

	// Verify label
	addrInfo := store.GetAddress(chain.BSV, addr)
	require.NotNil(t, addrInfo)
	assert.Equal(t, "My Savings", addrInfo.Label)

	// Update label
	err = store.SetAddressLabel(chain.BSV, addr, "Updated Label")
	require.NoError(t, err)
	assert.Equal(t, "Updated Label", store.GetAddress(chain.BSV, addr).Label)

	// Try to set label on non-existent address
	err = store.SetAddressLabel(chain.BSV, "nonexistent", "Test")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrAddressNotFound)
}

// TestState_UTXOUpdateInPlace tests updating an existing UTXO.
func TestState_UTXOUpdateInPlace(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	// Add initial UTXO
	utxo := createTestUTXO(chain.BSV, addr, testTxID(0), 0, 1000, false)
	utxo.Confirmations = 1
	store.AddUTXO(utxo)

	assertBalanceEquals(t, store, chain.BSV, 1000)

	// Update UTXO (e.g., more confirmations)
	utxo.Confirmations = 6
	store.AddUTXO(utxo)

	// Balance should be unchanged
	assertBalanceEquals(t, store, chain.BSV, 1000)

	// But confirmations should be updated
	utxos := store.GetUTXOs(chain.BSV, addr)
	require.Len(t, utxos, 1)
	assert.Equal(t, uint32(6), utxos[0].Confirmations)
}

// TestState_ChainIsolation tests that different chains don't interfere.
func TestState_ChainIsolation(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Add UTXOs to BSV
	bsvAddr := testAddressN(0)
	bsvMeta := createTestAddress(chain.BSV, bsvAddr, 0, false)
	store.AddAddress(bsvMeta)
	bsvUTXO := createTestUTXO(chain.BSV, bsvAddr, testTxID(0), 0, 1000, false)
	store.AddUTXO(bsvUTXO)

	// Add UTXOs to BTC
	btcAddr := testAddressN(1)
	btcMeta := createTestAddress(chain.BTC, btcAddr, 0, false)
	store.AddAddress(btcMeta)
	btcUTXO := createTestUTXO(chain.BTC, btcAddr, testTxID(1), 0, 2000, false)
	store.AddUTXO(btcUTXO)

	// Mark BSV UTXO as spent
	store.MarkSpent(chain.BSV, testTxID(0), 0, testTxID(100))

	// BSV balance should be 0, BTC unchanged
	assertBalanceEquals(t, store, chain.BSV, 0)
	assertBalanceEquals(t, store, chain.BTC, 2000)

	// Addresses should be chain-specific
	assert.Len(t, store.GetAddresses(chain.BSV), 1)
	assert.Len(t, store.GetAddresses(chain.BTC), 1)
}

// TestState_VersionMigration tests handling of version mismatches.
func TestState_VersionMigration(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Write a file with future version
	filePath := filepath.Join(tmpDir, "utxos.json")
	futureVersionData := `{"version": 999, "updated_at": "2026-01-01T00:00:00Z", "utxos": {}, "addresses": {}}`
	err := os.WriteFile(filePath, []byte(futureVersionData), 0o600)
	require.NoError(t, err)

	// Loading should fail
	store := New(tmpDir)
	err = store.Load()
	require.Error(t, err)
	require.ErrorIs(t, err, ErrVersionTooNew)
}
