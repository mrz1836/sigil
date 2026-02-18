package utxostore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

// TestEdge_SingleSatoshiTotal tests balance with a single 1-satoshi UTXO.
// BSV allows 1-satoshi outputs after removing dust limits.
func TestEdge_SingleSatoshiTotal(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	dustLimit := chain.BSV.DustLimit() // 1 satoshi for BSV
	require.Equal(t, uint64(1), dustLimit)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	// Add single 1-satoshi UTXO
	utxo := createTestUTXO(chain.BSV, addr, testTxID(0), 0, 1, false)
	store.AddUTXO(utxo)

	assertBalanceEquals(t, store, chain.BSV, 1)

	// Can mark it as spent
	found := store.MarkSpent(chain.BSV, testTxID(0), 0, testTxID(100))
	assert.True(t, found)
	assertBalanceEquals(t, store, chain.BSV, 0)
}

// TestEdge_MaxFeeEatsChange tests scenarios where fee calculation affects change.
func TestEdge_MaxFeeEatsChange(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Simulate high fee scenario (50 sat/byte)
	// Transaction size estimate: 192 bytes for 1-in-1-out
	// Fee at 50 sat/byte = 9600 satoshis
	const inputAmount = uint64(10000)
	const highFee = uint64(9600)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	utxo := createTestUTXO(chain.BSV, addr, testTxID(0), 0, inputAmount, false)
	store.AddUTXO(utxo)

	assertBalanceEquals(t, store, chain.BSV, inputAmount)

	// After high fee transaction, remaining would be very small
	// This tests that the store correctly tracks small amounts
	store.MarkSpent(chain.BSV, testTxID(0), 0, testTxID(100))

	// Simulate change output (what remains after payment + fee)
	changeAmount := inputAmount - highFee - 300 // 300 sent, rest is change after fee
	changeUTXO := createTestUTXO(chain.BSV, addr, testTxID(100), 1, changeAmount, false)
	store.AddUTXO(changeUTXO)

	assertBalanceEquals(t, store, chain.BSV, changeAmount)
}

// TestEdge_AllAddressesDust tests consolidating many dust-level UTXOs.
// With BTC's 546-satoshi dust limit, consolidation may not be economical.
func TestEdge_AllAddressesDust(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// BTC dust limit is 546 satoshis
	btcDustLimit := chain.BTC.DustLimit()
	assert.Equal(t, uint64(546), btcDustLimit)

	const numAddresses = 50
	amountPerAddr := btcDustLimit // Exactly at dust limit

	var totalAmount uint64
	for i := 0; i < numAddresses; i++ {
		addr := testAddressN(i)
		metadata := createTestAddress(chain.BTC, addr, uint32(i), false)
		store.AddAddress(metadata)

		utxo := createTestUTXO(chain.BTC, addr, testTxID(i), 0, amountPerAddr, false)
		store.AddUTXO(utxo)
		totalAmount += amountPerAddr
	}

	// Total: 50 * 546 = 27300 satoshis
	assertBalanceEquals(t, store, chain.BTC, totalAmount)

	// Fee for consolidating 50 inputs to 1 output:
	// Size: 10 + (50 * 148) + 34 = 7444 bytes
	// At 1 sat/byte = 7444 fee
	// At higher fee rates, consolidation becomes uneconomical
	const consolidationFee1SatByte = uint64(7444)
	const consolidationFee10SatByte = uint64(74440)

	// At 1 sat/byte, consolidation is possible (27300 - 7444 = 19856)
	assert.Greater(t, totalAmount, consolidationFee1SatByte)

	// At 10 sat/byte, consolidation loses money (27300 < 74440)
	assert.Less(t, totalAmount, consolidationFee10SatByte)
}

// TestEdge_LargeScale tests performance with many addresses and UTXOs.
func TestEdge_LargeScale(t *testing.T) {
	t.Parallel()

	const numAddresses = 1000
	const utxosPerAddress = 10
	const amountPerUTXO = uint64(100)

	store, expectedTotal := createLargeScaleStore(t, chain.BSV, numAddresses, utxosPerAddress, amountPerUTXO)

	// Verify total balance
	start := time.Now()
	balance := store.GetBalance(chain.BSV)
	balanceTime := time.Since(start)

	assert.Equal(t, expectedTotal, balance)
	assert.Less(t, balanceTime, 500*time.Millisecond, "balance query should be fast")

	// Verify UTXO retrieval performance
	start = time.Now()
	utxos := store.GetUTXOs(chain.BSV, "")
	utxoTime := time.Since(start)

	assert.Len(t, utxos, numAddresses*utxosPerAddress)
	assert.Less(t, utxoTime, 500*time.Millisecond, "UTXO query should be fast")

	// Verify address retrieval performance
	start = time.Now()
	addrs := store.GetAddresses(chain.BSV)
	addrTime := time.Since(start)

	assert.Len(t, addrs, numAddresses)
	assert.Less(t, addrTime, 100*time.Millisecond, "address query should be fast")
}

// TestEdge_MaxAddressDerivation tests handling near the maximum address index.
func TestEdge_MaxAddressDerivation(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Test with indices near the BIP44 hardened derivation limit
	// Standard limit is 2^31 - 1 for non-hardened derivation
	highIndices := []uint32{
		0,
		1000,
		10000,
		100000,
		1000000, // Reasonable maximum for practical use
	}

	for _, idx := range highIndices {
		addr := testAddressN(int(idx))
		metadata := createTestAddress(chain.BSV, addr, idx, false)
		store.AddAddress(metadata)

		// Add UTXO at this address
		utxo := createTestUTXO(chain.BSV, addr, testTxID(int(idx)), 0, 100, false)
		store.AddUTXO(utxo)
	}

	// Verify all addresses and UTXOs are accessible
	assert.Len(t, store.GetAddresses(chain.BSV), len(highIndices))
	assertBalanceEquals(t, store, chain.BSV, uint64(len(highIndices))*100)
}

// TestEdge_VeryLargeSingleUTXO tests handling of very large amounts.
func TestEdge_VeryLargeSingleUTXO(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Maximum BSV supply: 21 million BSV = 2.1 * 10^15 satoshis
	maxSupply := uint64(2_100_000_000_000_000)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	utxo := createTestUTXO(chain.BSV, addr, testTxID(0), 0, maxSupply, false)
	store.AddUTXO(utxo)

	assertBalanceEquals(t, store, chain.BSV, maxSupply)

	// Can mark as spent
	store.MarkSpent(chain.BSV, testTxID(0), 0, testTxID(100))
	assertBalanceEquals(t, store, chain.BSV, 0)
}

// TestEdge_ZeroAmountUTXO tests that zero-amount UTXOs are handled.
// While zero-value outputs are invalid, the store should handle them gracefully.
func TestEdge_ZeroAmountUTXO(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	// Add valid UTXO
	validUTXO := createTestUTXO(chain.BSV, addr, testTxID(0), 0, 1000, false)
	store.AddUTXO(validUTXO)

	// Add zero-amount UTXO (should be stored but not add to balance)
	zeroUTXO := createTestUTXO(chain.BSV, addr, testTxID(1), 0, 0, false)
	store.AddUTXO(zeroUTXO)

	// Balance should only include valid UTXO
	assertBalanceEquals(t, store, chain.BSV, 1000)

	// Both UTXOs should be stored
	utxos := store.GetUTXOs(chain.BSV, "")
	assert.Len(t, utxos, 2)
}

// TestEdge_SameAddressDifferentChains tests using the same address string on different chains.
func TestEdge_SameAddressDifferentChains(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// In practice, the same address string might theoretically appear on different chains
	// (though this shouldn't happen with properly derived addresses)
	commonAddr := testAddressN(0)

	// Add to BSV
	bsvMeta := createTestAddress(chain.BSV, commonAddr, 0, false)
	store.AddAddress(bsvMeta)
	bsvUTXO := createTestUTXO(chain.BSV, commonAddr, testTxID(0), 0, 1000, false)
	store.AddUTXO(bsvUTXO)

	// Add to BTC (different chain, same address string)
	btcMeta := createTestAddress(chain.BTC, commonAddr, 0, false)
	store.AddAddress(btcMeta)
	btcUTXO := createTestUTXO(chain.BTC, commonAddr, testTxID(1), 0, 2000, false)
	store.AddUTXO(btcUTXO)

	// Balances should be separate
	assertBalanceEquals(t, store, chain.BSV, 1000)
	assertBalanceEquals(t, store, chain.BTC, 2000)

	// Addresses should be separate
	assert.Len(t, store.GetAddresses(chain.BSV), 1)
	assert.Len(t, store.GetAddresses(chain.BTC), 1)
}

// TestEdge_MultipleVoutsFromSameTx tests multiple outputs from the same transaction.
func TestEdge_MultipleVoutsFromSameTx(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	// Same txid, different vouts
	txid := testTxID(0)
	for vout := uint32(0); vout < 5; vout++ {
		utxo := createTestUTXO(chain.BSV, addr, txid, vout, 1000+uint64(vout*100), false)
		store.AddUTXO(utxo)
	}

	// Total: 1000 + 1100 + 1200 + 1300 + 1400 = 6000
	assertBalanceEquals(t, store, chain.BSV, 6000)

	// All 5 UTXOs should exist
	utxos := store.GetUTXOs(chain.BSV, addr)
	assert.Len(t, utxos, 5)

	// Mark specific vout as spent
	store.MarkSpent(chain.BSV, txid, 2, testTxID(100))

	// Balance should decrease by 1200
	assertBalanceEquals(t, store, chain.BSV, 4800)
}

// TestEdge_RapidAddRemove tests rapid addition and removal of UTXOs.
func TestEdge_RapidAddRemove(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	// Rapidly add and spend UTXOs
	const cycles = 100
	for i := 0; i < cycles; i++ {
		// Add UTXO
		utxo := createTestUTXO(chain.BSV, addr, testTxID(i), 0, 100, false)
		store.AddUTXO(utxo)

		// Immediately spend it
		store.MarkSpent(chain.BSV, testTxID(i), 0, testTxID(i+1000))
	}

	// Final balance should be 0
	assertBalanceEquals(t, store, chain.BSV, 0)

	// No unspent UTXOs
	assert.Empty(t, store.GetUTXOs(chain.BSV, ""))
}

// TestEdge_SpecialCharactersInLabel tests address labels with special characters.
//
//nolint:gosmopolitan // Test deliberately uses non-ASCII characters
func TestEdge_SpecialCharactersInLabel(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	specialLabels := []string{
		"Chinese测试",                // Mixed characters
		"Cyrillic-Адрес",           // Cyrillic
		"Address with spaces",      // Spaces
		"Address\twith\ttabs",      // Tabs
		"Address\nwith\nnewlines",  // Newlines
		`Label with "quotes"`,      // Quotes
		"Label with 'apostrophes'", // Apostrophes
		"Label/with/slashes",       // Path-like
		"Label:with:colons",        // Colons
		"emoji 🚀 label",            // Emoji
	}

	for _, label := range specialLabels {
		err := store.SetAddressLabel(chain.BSV, addr, label)
		require.NoError(t, err, "should accept label: %q", label)

		// Verify it was stored correctly
		stored := store.GetAddress(chain.BSV, addr)
		assert.Equal(t, label, stored.Label, "label should be preserved exactly")
	}
}

// TestEdge_EmptyAddressString tests handling of empty address strings.
func TestEdge_EmptyAddressString(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// GetAddress with empty string should return nil
	addr := store.GetAddress(chain.BSV, "")
	assert.Nil(t, addr)

	// GetAddressBalance with empty string should return 0
	balance := store.GetAddressBalance(chain.BSV, "")
	assert.Equal(t, uint64(0), balance)

	// GetUTXOs with empty string should return all UTXOs for the chain
	utxos := store.GetUTXOs(chain.BSV, "")
	assert.Empty(t, utxos) // Empty store
}

// TestEdge_TimestampPreservation tests that timestamps are properly handled.
func TestEdge_TimestampPreservation(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	addr := testAddressN(0)
	metadata := createTestAddress(chain.BSV, addr, 0, false)
	store.AddAddress(metadata)

	// Create UTXO - FirstSeen will be set by AddUTXO
	utxo := &StoredUTXO{
		ChainID:       chain.BSV,
		TxID:          testTxID(0),
		Vout:          0,
		Amount:        1000,
		Address:       addr,
		Confirmations: 10,
		Spent:         false,
	}

	store.AddUTXO(utxo)

	// Retrieve and check timestamps are set
	utxos := store.GetUTXOs(chain.BSV, addr)
	require.Len(t, utxos, 1)
	assert.False(t, utxos[0].FirstSeen.IsZero())
	assert.False(t, utxos[0].LastUpdated.IsZero())
}

// TestEdge_DerivationPathFormat tests proper derivation path formatting.
func TestEdge_DerivationPathFormat(t *testing.T) {
	t.Parallel()
	store := createTestStore(t)

	// Test various derivation path scenarios
	testCases := []struct {
		chainID  chain.ID
		index    uint32
		isChange bool
	}{
		{chain.BSV, 0, false},
		{chain.BSV, 0, true},
		{chain.BSV, 19, false}, // Gap limit edge
		{chain.BTC, 0, false},
		{chain.BCH, 100, true},
	}

	for _, tc := range testCases {
		addr := testAddressN(int(tc.chainID.CoinType())*1000 + int(tc.index))
		metadata := createTestAddress(tc.chainID, addr, tc.index, tc.isChange)
		store.AddAddress(metadata)

		// Verify stored derivation path
		stored := store.GetAddress(tc.chainID, addr)
		require.NotNil(t, stored)
		assert.Contains(t, stored.DerivationPath, "m/44'")
		assert.Equal(t, tc.index, stored.Index)
		assert.Equal(t, tc.isChange, stored.IsChange)
	}
}
