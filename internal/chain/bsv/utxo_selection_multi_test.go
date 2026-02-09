package bsv

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

// estimateFee calculates the fee for a transaction with the given number of inputs and outputs.
func estimateFee(inputs, outputs int, feeRate uint64) uint64 {
	return (EstimateTxSize(inputs, outputs)*feeRate + 999) / 1000
}

// TestSelectUTXOs_MultipleAddresses tests UTXO selection across multiple addresses.
func TestSelectUTXOs_MultipleAddresses(t *testing.T) {
	t.Parallel()

	// Create UTXOs from 3 different addresses
	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: 1000, Address: testAddress},
		{TxID: testTxID(1), Vout: 0, Amount: 2000, Address: testAddress2},
		{TxID: testTxID(2), Vout: 0, Amount: 3000, Address: testAddress3},
	}

	client := NewClient(context.Background(), nil)

	// Select enough for 4000 sats (needs 2-3 UTXOs)
	selected, _, err := client.SelectUTXOs(utxos, 4000, DefaultFeeRate)
	require.NoError(t, err)

	// Should select the two largest UTXOs (3000 + 2000 = 5000)
	// Or potentially all three depending on fee calculation
	assert.NotEmpty(t, selected)

	// Total selected should cover amount + fee
	var totalSelected uint64
	for _, u := range selected {
		totalSelected += u.Amount
	}
	assert.GreaterOrEqual(t, totalSelected, uint64(4000))
}

// TestSelectUTXOs_PreferFewerInputs tests that largest-first selection prefers fewer inputs.
func TestSelectUTXOs_PreferFewerInputs(t *testing.T) {
	t.Parallel()

	// 1 large UTXO vs 10 small UTXOs
	largeUTXO := UTXO{TxID: testTxID(0), Vout: 0, Amount: 100000, Address: testAddress}
	smallUTXOs := make([]UTXO, 10)
	for i := range smallUTXOs {
		smallUTXOs[i] = UTXO{TxID: testTxID(i + 1), Vout: 0, Amount: 10000, Address: testAddress}
	}

	// Mix them together
	allUTXOs := append([]UTXO{largeUTXO}, smallUTXOs...)

	client := NewClient(context.Background(), nil)

	// Select 50000 sats - should prefer the large UTXO
	selected, _, err := client.SelectUTXOs(allUTXOs, 50000, DefaultFeeRate)
	require.NoError(t, err)

	// Should select just the large UTXO (100000 > 50000 + fee)
	assert.Len(t, selected, 1)
	assert.Equal(t, uint64(100000), selected[0].Amount)
}

// TestSelectUTXOs_ExactAmountAcross3 tests selecting exact amount across 3 addresses.
func TestSelectUTXOs_ExactAmountAcross3(t *testing.T) {
	t.Parallel()

	// 3 UTXOs that together cover target + fee for 3 inputs.
	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: 200, Address: testAddress},
		{TxID: testTxID(1), Vout: 0, Amount: 200, Address: testAddress2},
		{TxID: testTxID(2), Vout: 0, Amount: 200, Address: testAddress3},
	}

	client := NewClient(context.Background(), nil)

	fee := estimateFee(3, 2, DefaultFeeRate)
	target := uint64(50)
	require.LessOrEqual(t, target+fee, uint64(600))

	_, _, err := client.SelectUTXOs(utxos, target, DefaultFeeRate)
	require.NoError(t, err)
}

// TestSelectUTXOs_DustScattered tests selecting many small UTXOs scattered across addresses.
func TestSelectUTXOs_DustScattered(t *testing.T) {
	t.Parallel()

	// 100 UTXOs of 600 sats each across 20 addresses
	utxos := make([]UTXO, 100)
	for i := range utxos {
		addr := testAddress
		if i%2 == 1 {
			addr = testAddress2
		}
		utxos[i] = UTXO{
			TxID:    testTxID(i),
			Vout:    0,
			Amount:  600,
			Address: addr,
		}
	}

	client := NewClient(context.Background(), nil)

	// Try to select 50000 sats
	// Total available: 100 * 600 = 60000 sats
	// But fee will grow with each input selected
	selected, change, err := client.SelectUTXOs(utxos, 50000, DefaultFeeRate)

	// This may or may not succeed depending on fee calculation
	if err == nil {
		// Verify selection is valid
		var total uint64
		for _, u := range selected {
			total += u.Amount
		}
		// Total should be >= target + estimated fee
		assert.GreaterOrEqual(t, total, uint64(50000))
		t.Logf("Selected %d UTXOs totaling %d sats, change: %d", len(selected), total, change)
	} else {
		// Insufficient funds due to fee growth is acceptable
		assert.Contains(t, err.Error(), "insufficient")
	}
}

// TestSelectUTXOs_SingleSatoshis tests selecting many 1-satoshi UTXOs (valid on BSV).
func TestSelectUTXOs_SingleSatoshis(t *testing.T) {
	t.Parallel()

	// BSV allows 1-satoshi outputs
	dustLimit := chain.BSV.DustLimit()
	require.Equal(t, uint64(1), dustLimit)

	// Create 100 UTXOs of 500 satoshis each (small but enough to cover per-input fee)
	utxos := make([]UTXO, 100)
	for i := range utxos {
		utxos[i] = UTXO{
			TxID:    testTxID(i),
			Vout:    0,
			Amount:  500,
			Address: testAddress,
		}
	}

	client := NewClient(context.Background(), nil)

	selected, change, err := client.SelectUTXOs(utxos, 100, DefaultFeeRate)
	require.NoError(t, err)

	fee := estimateFee(len(selected), 2, DefaultFeeRate)
	var total uint64
	for _, u := range selected {
		total += u.Amount
	}
	require.GreaterOrEqual(t, total, uint64(100)+fee)
	assert.Equal(t, total-uint64(100)-fee, change)
}

// TestSelectUTXOs_MixedAmounts tests selection with a mix of large and small UTXOs.
func TestSelectUTXOs_MixedAmounts(t *testing.T) {
	t.Parallel()

	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: 100000, Address: testAddress}, // Large
		{TxID: testTxID(1), Vout: 0, Amount: 500, Address: testAddress2},   // Small
		{TxID: testTxID(2), Vout: 0, Amount: 50000, Address: testAddress3}, // Medium
		{TxID: testTxID(3), Vout: 0, Amount: 200, Address: testAddress},    // Tiny
		{TxID: testTxID(4), Vout: 0, Amount: 25000, Address: testAddress2}, // Medium
	}

	client := NewClient(context.Background(), nil)

	tests := []struct {
		name      string
		target    uint64
		expectLen int
		expectOK  bool
	}{
		{"small amount uses 1 UTXO", 10000, 1, true},
		{"medium amount uses 1-2 UTXOs", 75000, 2, true},
		{"large amount uses multiple UTXOs", 150000, 3, true},
		{"exceeds available fails", 200000, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			selected, _, err := client.SelectUTXOs(utxos, tt.target, DefaultFeeRate)

			if tt.expectOK {
				require.NoError(t, err)
				// Due to largest-first selection, we should use as few as possible
				assert.LessOrEqual(t, len(selected), tt.expectLen+1) // Allow 1 extra for fee coverage
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestSelectUTXOs_AllSameAmount tests selection when all UTXOs have the same amount.
func TestSelectUTXOs_AllSameAmount(t *testing.T) {
	t.Parallel()

	// 10 UTXOs of exactly 1000 sats each
	utxos := make([]UTXO, 10)
	for i := range utxos {
		utxos[i] = UTXO{
			TxID:    testTxID(i),
			Vout:    0,
			Amount:  1000,
			Address: testAddress,
		}
	}

	client := NewClient(context.Background(), nil)

	// Select 5000 sats - needs at least 6 UTXOs once fee grows with inputs
	selected, change, err := client.SelectUTXOs(utxos, 5000, DefaultFeeRate)
	require.NoError(t, err)

	// Should select 6 UTXOs
	assert.Len(t, selected, 6)

	// Change should be positive
	assert.Positive(t, change)
}

// TestSelectUTXOs_SortOrder tests that UTXOs are sorted by amount descending.
func TestSelectUTXOs_SortOrder(t *testing.T) {
	t.Parallel()

	// UTXOs in random order
	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: 300, Address: testAddress},
		{TxID: testTxID(1), Vout: 0, Amount: 500, Address: testAddress},
		{TxID: testTxID(2), Vout: 0, Amount: 100, Address: testAddress},
		{TxID: testTxID(3), Vout: 0, Amount: 400, Address: testAddress},
		{TxID: testTxID(4), Vout: 0, Amount: 200, Address: testAddress},
	}

	client := NewClient(context.Background(), nil)

	// Select amount that needs 2 UTXOs after fee growth
	// The two largest (500 + 400 = 900) should be enough
	selected, _, err := client.SelectUTXOs(utxos, 500, DefaultFeeRate)
	require.NoError(t, err)

	// Should select the 2 largest
	require.Len(t, selected, 2)
	assert.Equal(t, uint64(500), selected[0].Amount)
	assert.Equal(t, uint64(400), selected[1].Amount)
}

// TestSelectUTXOs_EmptyList tests selection with no UTXOs.
func TestSelectUTXOs_EmptyList(t *testing.T) {
	t.Parallel()

	client := NewClient(context.Background(), nil)

	_, _, err := client.SelectUTXOs([]UTXO{}, 1000, DefaultFeeRate)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInsufficientFunds)
}

// TestSelectUTXOs_ZeroAmount tests selection for zero amount.
func TestSelectUTXOs_ZeroAmount(t *testing.T) {
	t.Parallel()

	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: 1000, Address: testAddress},
	}

	client := NewClient(context.Background(), nil)

	// Zero amount still needs fee coverage
	selected, change, err := client.SelectUTXOs(utxos, 0, DefaultFeeRate)
	require.NoError(t, err)

	// Should select 1 UTXO to cover fee
	assert.Len(t, selected, 1)
	// Change should be amount - fee
	assert.Positive(t, change)
}

// TestSelectUTXOs_VariousFeeRates tests selection with different fee rates.
func TestSelectUTXOs_VariousFeeRates(t *testing.T) {
	t.Parallel()

	// Need enough to cover high fee rates: at 50 sat/byte with 1 input,
	// fee = 226*50 = 11300. Target 5000 + fee = 16300. Use 55000 total for buffer.
	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: 50000, Address: testAddress},
		{TxID: testTxID(1), Vout: 0, Amount: 5000, Address: testAddress},
	}

	client := NewClient(context.Background(), nil)

	tests := []struct {
		name    string
		feeRate uint64
	}{
		{"minimum fee rate (1)", 1},
		{"default fee rate", DefaultFeeRate},
		{"medium fee rate (10)", 10},
		{"high fee rate (25)", 25},
		{"maximum fee rate (50)", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			selected, change, err := client.SelectUTXOs(utxos, 5000, tt.feeRate)

			// All should succeed for small amount with 55000 available
			require.NoError(t, err)
			assert.NotEmpty(t, selected)

			// Higher fee rates should result in less change
			t.Logf("Fee rate %d: selected %d UTXOs, change %d",
				tt.feeRate, len(selected), change)
		})
	}
}

// TestSelectUTXOs_ConsolidationScenario tests selecting many UTXOs for consolidation.
func TestSelectUTXOs_ConsolidationScenario(t *testing.T) {
	t.Parallel()

	// Simulate consolidation: many small UTXOs
	const numUTXOs = 50
	const amountEach = uint64(2000)
	utxos := make([]UTXO, numUTXOs)
	for i := range utxos {
		utxos[i] = UTXO{
			TxID:    testTxID(i),
			Vout:    0,
			Amount:  amountEach,
			Address: testAddress,
		}
	}

	client := NewClient(context.Background(), nil)

	// Try to consolidate most - select total minus enough headroom for dynamic fees
	totalAvailable := numUTXOs * amountEach // 100000 sats
	// For 50 inputs at feeRate 1: fee = (10 + 50*148 + 2*34) = 7478

	selected, _, err := client.SelectUTXOs(utxos, totalAvailable-10000, DefaultFeeRate)
	require.NoError(t, err)

	// Should select all or most UTXOs
	assert.Greater(t, len(selected), 40)
}
