package utxostore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

// TestMultiAddressBalance_5Addresses tests balance aggregation across 5 addresses.
func TestMultiAddressBalance_5Addresses(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Create 5 addresses with varying UTXOs
	amounts := [][]uint64{
		{1000, 2000},    // addr 0: 3000 sats
		{5000},          // addr 1: 5000 sats
		{100, 200, 300}, // addr 2: 600 sats
		{10000},         // addr 3: 10000 sats
		{500, 500, 500}, // addr 4: 1500 sats
	}

	var expectedTotal uint64
	for i, addrAmounts := range amounts {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		for j, amount := range addrAmounts {
			utxo := createTestUTXO(chain.BSV, addr, testTxID(i*100+j), uint32(j), amount, false)
			store.AddUTXO(utxo)
			expectedTotal += amount
		}
	}

	// Total should be 3000 + 5000 + 600 + 10000 + 1500 = 20100
	assert.Equal(t, uint64(20100), expectedTotal)
	assertBalanceEquals(t, store, chain.BSV, expectedTotal)

	// Verify per-address balances
	for i, addrAmounts := range amounts {
		var addrTotal uint64
		for _, a := range addrAmounts {
			addrTotal += a
		}
		addr := testAddressN(i)
		assert.Equal(t, addrTotal, store.GetAddressBalance(chain.BSV, addr))
	}
}

// TestMultiAddressBalance_50Addresses tests balance with 50 addresses, only 25 funded.
func TestMultiAddressBalance_50Addresses(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	const totalAddresses = 50
	const fundedAddresses = 25
	const amountPerFunded = uint64(1000)

	// Create 50 addresses
	for i := 0; i < totalAddresses; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		// Only fund every other address (25 funded)
		if i%2 == 0 && i < fundedAddresses*2 {
			utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, amountPerFunded, false)
			store.AddUTXO(utxo)
		}
	}

	// Expected: 25 addresses * 1000 sats = 25000
	assertBalanceEquals(t, store, chain.BSV, fundedAddresses*amountPerFunded)
	assertAddressCount(t, store, chain.BSV, totalAddresses)
}

// TestMultiAddressBalance_MixedReceiveChange tests balance with both receive and change addresses.
func TestMultiAddressBalance_MixedReceiveChange(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Add 10 receive addresses with 1000 sats each
	receiveAmount := uint64(1000)
	for i := 0; i < 10; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false) // isChange = false
		store.AddAddress(metadata)

		utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, receiveAmount, false)
		store.AddUTXO(utxo)
	}

	// Add 5 change addresses with 500 sats each
	changeAmount := uint64(500)
	for i := 0; i < 5; i++ {
		addr := testAddressN(100 + i)                                   // Different address range
		metadata := createTestAddress(chain.BSV, addr, uint32(i), true) // isChange = true
		store.AddAddress(metadata)

		utxo := createTestUTXO(chain.BSV, addr, testTxID(100+i), 0, changeAmount, false)
		store.AddUTXO(utxo)
	}

	// Total: 10 * 1000 + 5 * 500 = 12500
	expectedTotal := uint64(10*1000 + 5*500)
	assertBalanceEquals(t, store, chain.BSV, expectedTotal)

	// Verify address counts
	allAddrs := store.GetAddresses(chain.BSV)
	var receiveCount, changeCount int
	for _, addr := range allAddrs {
		if addr.IsChange {
			changeCount++
		} else {
			receiveCount++
		}
	}
	assert.Equal(t, 10, receiveCount)
	assert.Equal(t, 5, changeCount)
}

// TestMultiAddressBalance_PartiallySpent tests balance with some UTXOs spent.
func TestMultiAddressBalance_PartiallySpent(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Create 3 addresses
	addresses := []string{testAddressN(0), testAddressN(1), testAddressN(2)}
	for i, addr := range addresses {
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)
	}

	// Address 0: 2 UTXOs, 1 spent
	utxo0a := createTestUTXO(chain.BSV, addresses[0], testTxID(0), 0, 1000, false)
	utxo0b := createTestUTXO(chain.BSV, addresses[0], testTxID(1), 0, 2000, true) // Spent
	utxo0b.SpentTxID = testTxID(100)
	store.AddUTXO(utxo0a)
	store.AddUTXO(utxo0b)

	// Address 1: 3 UTXOs, 2 spent
	utxo1a := createTestUTXO(chain.BSV, addresses[1], testTxID(10), 0, 500, false)
	utxo1b := createTestUTXO(chain.BSV, addresses[1], testTxID(11), 0, 600, true) // Spent
	utxo1c := createTestUTXO(chain.BSV, addresses[1], testTxID(12), 0, 700, true) // Spent
	store.AddUTXO(utxo1a)
	store.AddUTXO(utxo1b)
	store.AddUTXO(utxo1c)

	// Address 2: 1 UTXO, unspent
	utxo2 := createTestUTXO(chain.BSV, addresses[2], testTxID(20), 0, 3000, false)
	store.AddUTXO(utxo2)

	// Expected balance: 1000 + 500 + 3000 = 4500 (only unspent)
	assertBalanceEquals(t, store, chain.BSV, 4500)

	// Verify per-address balances
	assert.Equal(t, uint64(1000), store.GetAddressBalance(chain.BSV, addresses[0]))
	assert.Equal(t, uint64(500), store.GetAddressBalance(chain.BSV, addresses[1]))
	assert.Equal(t, uint64(3000), store.GetAddressBalance(chain.BSV, addresses[2]))
}

// TestMultiAddressBalance_ZeroBalanceMixed tests balance when some addresses have zero balance.
func TestMultiAddressBalance_ZeroBalanceMixed(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	const totalAddresses = 10
	const fundedCount = 4
	const amountPerFunded = uint64(2500)

	// Create addresses, only fund specific ones
	fundedIndices := map[int]bool{0: true, 3: true, 5: true, 9: true}

	for i := 0; i < totalAddresses; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		if fundedIndices[i] {
			utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, amountPerFunded, false)
			store.AddUTXO(utxo)
		}
	}

	// Expected: 4 * 2500 = 10000
	assertBalanceEquals(t, store, chain.BSV, fundedCount*amountPerFunded)

	// Verify zero balance addresses
	for i := 0; i < totalAddresses; i++ {
		addr := testAddressN(i)
		balance := store.GetAddressBalance(chain.BSV, addr)
		if fundedIndices[i] {
			assert.Equal(t, amountPerFunded, balance, "funded address %d should have balance", i)
		} else {
			assert.Equal(t, uint64(0), balance, "unfunded address %d should have zero balance", i)
		}
	}
}

// TestMultiAddressBalance_SingleSatoshiUTXOs tests balance with many 1-satoshi UTXOs.
// BSV removed dust limits, so 1-sat outputs are valid.
func TestMultiAddressBalance_SingleSatoshiUTXOs(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	const numUTXOs = 100
	const amount = uint64(1) // 1 satoshi each (valid on BSV)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	for i := 0; i < numUTXOs; i++ {
		utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, amount, false)
		store.AddUTXO(utxo)
	}

	// Total: 100 * 1 = 100 satoshis
	assertBalanceEquals(t, store, chain.BSV, numUTXOs*amount)
	assertUTXOCount(t, store, chain.BSV, addr, numUTXOs)
}

// TestMultiAddressBalance_LargeAmounts tests balance with large amounts.
func TestMultiAddressBalance_LargeAmounts(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Test with amounts approaching max BSV supply
	amounts := []uint64{
		100_000_000_000_000, // 1 million BSV
		50_000_000_000_000,  // 500k BSV
		21_000_000_00000000, // 21 million BSV (max supply in sats)
	}

	var expectedTotal uint64
	for i, amount := range amounts {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BSV, addr, uint32(i), false)
		store.AddAddress(metadata)

		utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, amount, false)
		store.AddUTXO(utxo)
		expectedTotal += amount
	}

	assertBalanceEquals(t, store, chain.BSV, expectedTotal)
}

// TestMultiAddressBalance_MultipleChains tests that balances are correctly isolated by chain.
func TestMultiAddressBalance_MultipleChains(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Add BSV UTXOs
	bsvAddr := testAddressN(0)
	bsvMeta := createTestAddress(chain.BSV, bsvAddr, 0, false)
	store.AddAddress(bsvMeta)
	bsvUTXO := createTestUTXO(chain.BSV, bsvAddr, testTxID(0), 0, 10000, false)
	store.AddUTXO(bsvUTXO)

	// Add BTC UTXOs
	btcAddr := testAddressN(1)
	btcMeta := createTestAddress(chain.BTC, btcAddr, 0, false)
	store.AddAddress(btcMeta)
	btcUTXO := createTestUTXO(chain.BTC, btcAddr, testTxID(1), 0, 5000, false)
	store.AddUTXO(btcUTXO)

	// Verify chain isolation
	assertBalanceEquals(t, store, chain.BSV, 10000)
	assertBalanceEquals(t, store, chain.BTC, 5000)
	assertBalanceEquals(t, store, chain.BCH, 0) // No BCH UTXOs
}

// TestMultiAddressBalance_AfterMarkSpent tests that balance updates correctly after marking spent.
func TestMultiAddressBalance_AfterMarkSpent(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Create address with 3 UTXOs
	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	utxo1 := createTestUTXO(chain.BSV, addr, testTxID(0), 0, 1000, false)
	utxo2 := createTestUTXO(chain.BSV, addr, testTxID(1), 0, 2000, false)
	utxo3 := createTestUTXO(chain.BSV, addr, testTxID(2), 0, 3000, false)
	store.AddUTXO(utxo1)
	store.AddUTXO(utxo2)
	store.AddUTXO(utxo3)

	// Initial balance: 6000
	assertBalanceEquals(t, store, chain.BSV, 6000)

	// Mark first UTXO as spent
	found := store.MarkSpent(chain.BSV, testTxID(0), 0, testTxID(100))
	require.True(t, found)
	assertBalanceEquals(t, store, chain.BSV, 5000)

	// Mark second UTXO as spent
	found = store.MarkSpent(chain.BSV, testTxID(1), 0, testTxID(101))
	require.True(t, found)
	assertBalanceEquals(t, store, chain.BSV, 3000)

	// Mark third UTXO as spent
	found = store.MarkSpent(chain.BSV, testTxID(2), 0, testTxID(102))
	require.True(t, found)
	assertBalanceEquals(t, store, chain.BSV, 0)
}

// TestMultiAddressBalance_EmptyStore tests balance on an empty store.
func TestMultiAddressBalance_EmptyStore(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	assertBalanceEquals(t, store, chain.BSV, 0)
	assertBalanceEquals(t, store, chain.BTC, 0)
	assert.True(t, store.IsEmpty())
}

// TestMultiAddressBalance_DustLimitAmounts tests balance with amounts at chain dust limits.
func TestMultiAddressBalance_DustLimitAmounts(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// BSV dust limit is 1 satoshi
	bsvDustLimit := chain.BSV.DustLimit()
	bsvAddr := testAddressN(0)
	bsvMeta := createTestAddress(chain.BSV, bsvAddr, 0, false)
	store.AddAddress(bsvMeta)

	// Add 10 UTXOs at BSV dust limit
	for i := 0; i < 10; i++ {
		utxo := createTestUTXO(chain.BSV, bsvAddr, testTxID(i), 0, bsvDustLimit, false)
		store.AddUTXO(utxo)
	}

	assertBalanceEquals(t, store, chain.BSV, 10*bsvDustLimit)
}
