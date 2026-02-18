package utxostore

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

// TestAIAgent_HighFrequencyTx simulates rapid transaction creation by an AI agent.
func TestAIAgent_HighFrequencyTx(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Setup: 1 BSV (100M sats) in initial UTXO
	initialAmount := uint64(100_000_000)
	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	initialUTXO := createTestUTXO(chain.BSV, addr, testTxID(0), 0, initialAmount, false)
	store.AddUTXO(initialUTXO)

	// Simulate 100 rapid micro-transactions
	// Each tx: spend one UTXO, create payment + change
	const numTx = 100
	const paymentAmount = uint64(10_000) // 10k sats per payment
	const feePerTx = uint64(200)         // Simplified fee

	currentBalance := initialAmount
	currentUTXOTxID := testTxID(0)
	currentUTXOVout := uint32(0)

	for i := 0; i < numTx; i++ {
		// Mark current UTXO as spent
		newTxID := testTxID(i + 1)
		found := store.MarkSpent(chain.BSV, currentUTXOTxID, currentUTXOVout, newTxID)
		require.True(t, found, "UTXO should exist at tx %d", i)

		// Calculate change
		change := currentBalance - paymentAmount - feePerTx
		require.Positive(t, change, "should have change at tx %d", i)

		// Add change UTXO
		changeUTXO := createTestUTXO(chain.BSV, addr, newTxID, 1, change, false)
		store.AddUTXO(changeUTXO)

		// Update for next iteration
		currentBalance = change
		currentUTXOTxID = newTxID
		currentUTXOVout = 1
	}

	// Verify final balance
	expectedFinal := initialAmount - (numTx * (paymentAmount + feePerTx))
	assertBalanceEquals(t, store, chain.BSV, expectedFinal)

	// Verify no double-spends (only 1 unspent UTXO should exist)
	unspent := store.GetUTXOs(chain.BSV, "")
	assert.Len(t, unspent, 1, "should have exactly 1 unspent UTXO")
}

// TestAIAgent_ManyPendingTx tests balance calculation with pending transactions.
func TestAIAgent_ManyPendingTx(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Create 10 UTXOs
	const numUTXOs = 10
	const amountEach = uint64(1000)
	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	for i := 0; i < numUTXOs; i++ {
		utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, amountEach, false)
		store.AddUTXO(utxo)
	}

	// Initial balance: 10 * 1000 = 10000
	assertBalanceEquals(t, store, chain.BSV, numUTXOs*amountEach)

	// Simulate 5 pending transactions (UTXOs marked as spent)
	const pendingCount = 5
	for i := 0; i < pendingCount; i++ {
		pendingTxID := testTxID(100 + i)
		store.MarkSpent(chain.BSV, testTxID(i), 0, pendingTxID)
	}

	// Balance should exclude pending (spent) UTXOs
	expectedBalance := (numUTXOs - pendingCount) * amountEach
	assertBalanceEquals(t, store, chain.BSV, expectedBalance)

	// Verify correct number of unspent UTXOs
	unspent := store.GetUTXOs(chain.BSV, "")
	assert.Len(t, unspent, numUTXOs-pendingCount)
}

// TestAIAgent_ReceiveWhileSpending tests receiving new UTXOs during spending operations.
func TestAIAgent_ReceiveWhileSpending(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Initial setup
	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	// Initial UTXO
	utxo1 := createTestUTXO(chain.BSV, addr, testTxID(0), 0, 5000, false)
	store.AddUTXO(utxo1)
	assertBalanceEquals(t, store, chain.BSV, 5000)

	// Simulate receiving new UTXO while preparing a spend
	// (this could happen in real scenarios with async operations)
	utxo2 := createTestUTXO(chain.BSV, addr, testTxID(1), 0, 3000, false)
	store.AddUTXO(utxo2)
	assertBalanceEquals(t, store, chain.BSV, 8000)

	// Now spend the original UTXO
	store.MarkSpent(chain.BSV, testTxID(0), 0, testTxID(100))
	assertBalanceEquals(t, store, chain.BSV, 3000)

	// The new UTXO should still be available
	unspent := store.GetUTXOs(chain.BSV, "")
	require.Len(t, unspent, 1)
	assert.Equal(t, testTxID(1), unspent[0].TxID)
}

// TestAIAgent_RapidAddressGeneration tests generating many addresses quickly.
func TestAIAgent_RapidAddressGeneration(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Generate 150 addresses rapidly
	const numAddresses = 150
	addresses := make(map[string]bool)

	for i := 0; i < numAddresses; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), i%5 == 0) // Every 5th is change

		// Verify uniqueness
		require.False(t, addresses[addr], "address should be unique: %s", addr)
		addresses[addr] = true

		store.AddAddress(metadata)
	}

	// Verify all addresses were stored
	storedAddrs := store.GetAddresses(chain.BSV)
	assert.Len(t, storedAddrs, numAddresses)

	// Verify derivation paths are unique and correct
	paths := make(map[string]bool)
	for _, addr := range storedAddrs {
		require.False(t, paths[addr.DerivationPath], "path should be unique: %s", addr.DerivationPath)
		paths[addr.DerivationPath] = true
	}
}

// TestAIAgent_ConcurrentOperations tests thread safety of store operations.
//
//nolint:gocognit // Complex test setup for concurrent operations
func TestAIAgent_ConcurrentOperations(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Setup initial state
	const numAddresses = 100
	const utxosPerAddress = 10

	for i := 0; i < numAddresses; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		for j := 0; j < utxosPerAddress; j++ {
			utxo := createTestUTXO(chain.BSV, addr, testTxID(i*100+j), uint32(j), 100, false)
			store.AddUTXO(utxo)
		}
	}

	// Run concurrent operations
	const numGoroutines = 10
	var wg sync.WaitGroup
	var readCount, writeCount atomic.Int64

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()

			for i := 0; i < 100; i++ {
				switch i % 4 {
				case 0:
					// Read balance
					_ = store.GetBalance(chain.BSV)
					readCount.Add(1)
				case 1:
					// Read UTXOs
					_ = store.GetUTXOs(chain.BSV, "")
					readCount.Add(1)
				case 2:
					// Read addresses
					_ = store.GetAddresses(chain.BSV)
					readCount.Add(1)
				case 3:
					// Write: add new UTXO
					utxo := createTestUTXO(chain.BSV,
						testAddressN(gid),
						testTxID(10000+gid*1000+i),
						0, 50, false)
					store.AddUTXO(utxo)
					writeCount.Add(1)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify no panics occurred and state is consistent
	assert.Positive(t, readCount.Load())
	assert.Positive(t, writeCount.Load())

	// State should be valid
	balance := store.GetBalance(chain.BSV)
	assert.Positive(t, balance)
}

// TestAIAgent_BatchPayments tests creating multiple outputs in batch payments.
func TestAIAgent_BatchPayments(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// AI agents often make batch payments to multiple recipients
	// Setup: 1 large UTXO
	senderAddr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, senderAddr, 0, false)
	store.AddAddress(metadata)

	largeUTXO := createTestUTXO(chain.BSV, senderAddr, testTxID(0), 0, 1_000_000, false)
	store.AddUTXO(largeUTXO)

	// Simulate batch payment: spend large UTXO, multiple recipients + change
	store.MarkSpent(chain.BSV, testTxID(0), 0, testTxID(1))

	// Add change UTXO (remaining after batch payment)
	changeAmount := uint64(900_000) // After paying 10 recipients 10k each
	changeAddr := testAddressN(1)
	changeMeta := createTestAddress(chain.BSV, changeAddr, 0, true)
	store.AddAddress(changeMeta)

	changeUTXO := createTestUTXO(chain.BSV, changeAddr, testTxID(1), 10, changeAmount, false)
	store.AddUTXO(changeUTXO)

	// Verify final balance is the change
	assertBalanceEquals(t, store, chain.BSV, changeAmount)
}

// TestAIAgent_UTXOConsolidation tests consolidating many small UTXOs into fewer large ones.
func TestAIAgent_UTXOConsolidation(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Setup: many small UTXOs across multiple addresses
	const numAddresses = 20
	const utxosPerAddr = 5
	const smallAmount = uint64(1000)

	totalBefore := uint64(0)
	for i := 0; i < numAddresses; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		for j := 0; j < utxosPerAddr; j++ {
			utxo := createTestUTXO(chain.BSV, addr, testTxID(i*100+j), uint32(j), smallAmount, false)
			store.AddUTXO(utxo)
			totalBefore += smallAmount
		}
	}

	assertBalanceEquals(t, store, chain.BSV, totalBefore)

	// Simulate consolidation: mark all as spent
	consolidationTxID := testTxID(9999)
	for i := 0; i < numAddresses; i++ {
		for j := 0; j < utxosPerAddr; j++ {
			store.MarkSpent(chain.BSV, testTxID(i*100+j), uint32(j), consolidationTxID)
		}
	}

	// Add single consolidated UTXO (minus fee)
	const fee = uint64(2000)
	consolidatedAmount := totalBefore - fee
	consolidatedUTXO := createTestUTXO(chain.BSV, testAddressN(0), consolidationTxID, 0, consolidatedAmount, false)
	store.AddUTXO(consolidatedUTXO)

	// Balance should be total minus fee
	assertBalanceEquals(t, store, chain.BSV, consolidatedAmount)

	// Should have only 1 unspent UTXO now
	unspent := store.GetUTXOs(chain.BSV, "")
	assert.Len(t, unspent, 1)
}

// TestAIAgent_FrequentBalanceChecks tests rapid balance queries.
func TestAIAgent_FrequentBalanceChecks(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Setup some UTXOs
	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	for i := 0; i < 100; i++ {
		utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 100, false)
		store.AddUTXO(utxo)
	}

	expectedBalance := uint64(10000)

	// AI agents often check balance very frequently
	// Verify consistency across 1000 rapid queries
	for i := 0; i < 1000; i++ {
		balance := store.GetBalance(chain.BSV)
		assert.Equal(t, expectedBalance, balance, "balance should be consistent on query %d", i)
	}
}

// TestAIAgent_AddressReuseTracking tests tracking address reuse patterns.
func TestAIAgent_AddressReuseTracking(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Create address
	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	// First receive
	utxo1 := createTestUTXO(chain.BSV, addr, testTxID(0), 0, 1000, false)
	store.AddUTXO(utxo1)
	store.MarkAddressUsed(chain.BSV, addr)

	// Check activity flag
	addrInfo := store.GetAddress(chain.BSV, addr)
	require.NotNil(t, addrInfo)
	assert.True(t, addrInfo.HasActivity)

	// Second receive to same address (address reuse)
	utxo2 := createTestUTXO(chain.BSV, addr, testTxID(1), 0, 2000, false)
	store.AddUTXO(utxo2)

	// Balance should include both
	assertBalanceEquals(t, store, chain.BSV, 3000)

	// Address should still show as used
	assert.True(t, store.GetAddress(chain.BSV, addr).HasActivity)
}

// TestAIAgent_RecoveryAfterCrash simulates state recovery after a crash.
func TestAIAgent_RecoveryAfterCrash(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Build up some state
	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	for i := 0; i < 50; i++ {
		utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 100, false)
		store.AddUTXO(utxo)
	}

	expectedBalance := uint64(5000)
	assertBalanceEquals(t, store, chain.BSV, expectedBalance)

	// Save state
	err := store.Save()
	require.NoError(t, err)

	// Simulate crash by creating new store instance
	recoveredStore := New(store.walletPath)
	err = recoveredStore.Load()
	require.NoError(t, err)

	// Verify recovered state matches
	assertBalanceEquals(t, recoveredStore, chain.BSV, expectedBalance)
	assertUTXOCount(t, recoveredStore, chain.BSV, addr, 50)
}
