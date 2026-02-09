package bsv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

// TestSpend_SingleAddressWithChange tests a basic spend with change output.
func TestSpend_SingleAddressWithChange(t *testing.T) {
	t.Parallel()

	// Single UTXO of 100k sats
	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: 100000, Address: testAddress},
	}

	client := NewClient(nil)

	// Select 50k sats - should have change
	selected, change, err := client.SelectUTXOs(utxos, 50000, DefaultFeeRate)
	require.NoError(t, err)

	assert.Len(t, selected, 1)
	assert.Equal(t, uint64(100000), selected[0].Amount)

	fee := (EstimateTxSize(1, 2)*DefaultFeeRate + 999) / 1000
	expectedChange := 100000 - 50000 - fee
	assert.Equal(t, expectedChange, change)

	// Change should be above dust limit
	dustLimit := chain.BSV.DustLimit()
	assert.Greater(t, change, dustLimit)
}

// TestSpend_Consolidate5AddressesTo1 tests consolidating 5 addresses into 1 output.
func TestSpend_Consolidate5AddressesTo1(t *testing.T) {
	t.Parallel()

	// 5 UTXOs from different addresses, 20k each
	addresses := []string{testAddress, testAddress2, testAddress3, testAddress, testAddress2}
	utxos := make([]UTXO, 5)
	for i := range utxos {
		utxos[i] = UTXO{
			TxID:    testTxID(i),
			Vout:    0,
			Amount:  20000,
			Address: addresses[i],
		}
	}

	// Total: 100k sats
	client := NewClient(nil)

	// Consolidate almost all (leaving room for fee)
	// Fee for 5 inputs, 1 output: 10 + (5*148) + 34 = 784 bytes
	// At 1 sat/byte = 784 satoshis
	feeEstimate := EstimateFeeForTx(5, 1, DefaultFeeRate)

	// Select sweep amount
	targetAmount := 100000 - feeEstimate - 100 // Leave small buffer
	selected, change, err := client.SelectUTXOs(utxos, targetAmount, DefaultFeeRate)
	require.NoError(t, err)

	// Should select all 5 UTXOs
	assert.Len(t, selected, 5)

	// Verify total selected
	var total uint64
	for _, u := range selected {
		total += u.Amount
	}
	assert.Equal(t, uint64(100000), total)

	// Change should be small (just covering fee difference)
	t.Logf("Selected %d UTXOs, total %d, change %d", len(selected), total, change)
}

// TestSpend_ExactAmount_NoChange tests spending exact amount with no change.
func TestSpend_ExactAmount_NoChange(t *testing.T) {
	t.Parallel()

	// UTXO that exactly covers amount + fee
	targetAmount := uint64(50000)
	exactUTXO := targetAmount + (EstimateTxSize(1, 2)*DefaultFeeRate+999)/1000

	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: exactUTXO, Address: testAddress},
	}

	client := NewClient(nil)

	selected, change, err := client.SelectUTXOs(utxos, targetAmount, DefaultFeeRate)
	require.NoError(t, err)

	assert.Len(t, selected, 1)
	assert.Equal(t, uint64(0), change) // No change
}

// TestSpend_ChangeBelowDust_BTC tests change below dust limit for BTC.
func TestSpend_ChangeBelowDust_BTC(t *testing.T) {
	t.Parallel()

	btcDustLimit := chain.BTC.DustLimit()
	require.Equal(t, uint64(546), btcDustLimit)

	// Create UTXO where change would be below BTC dust limit
	// UTXO - amount - fee = change < 546
	// Let's say UTXO = 51000, amount = 50000, fee = EstimateTxSize(1,2)
	// change = 51000 - 50000 - fee (above dust)
	// For change = 400 (below dust): UTXO = 50000 + fee + 400
	targetAmount := uint64(50000)
	utxoAmount := targetAmount + (EstimateTxSize(1, 2)*DefaultFeeRate+999)/1000 + 400 // 400 sats change

	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: utxoAmount, Address: testAddress},
	}

	client := NewClient(nil)

	_, change, err := client.SelectUTXOs(utxos, targetAmount, DefaultFeeRate)
	require.NoError(t, err)

	// Change = 400 sats (below BTC dust limit of 546)
	assert.Equal(t, uint64(400), change)
	assert.Less(t, change, btcDustLimit)

	// In a real BTC transaction, this change would be absorbed into fee
	// The TxBuilder should handle this
}

// TestSpend_ChangeBelowDust_BSV tests that change outputs are created even for tiny amounts on BSV.
func TestSpend_ChangeBelowDust_BSV(t *testing.T) {
	t.Parallel()

	bsvDustLimit := chain.BSV.DustLimit()
	require.Equal(t, uint64(1), bsvDustLimit)

	// Create UTXO where change = 1 satoshi (valid on BSV)
	targetAmount := uint64(50000)
	utxoAmount := targetAmount + (EstimateTxSize(1, 2)*DefaultFeeRate+999)/1000 + 1 // 1 sat change

	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: utxoAmount, Address: testAddress},
	}

	client := NewClient(nil)

	_, change, err := client.SelectUTXOs(utxos, targetAmount, DefaultFeeRate)
	require.NoError(t, err)

	// Change = 1 sat (valid on BSV)
	assert.Equal(t, uint64(1), change)
	assert.GreaterOrEqual(t, change, bsvDustLimit) // At or above BSV dust limit
}

// TestSpend_SequentialDepletion tests spending UTXOs in sequence until depleted.
func TestSpend_SequentialDepletion(t *testing.T) {
	t.Parallel()

	// Start with 3 UTXOs
	initialUTXOs := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: 10000, Address: testAddress},
		{TxID: testTxID(1), Vout: 0, Amount: 10000, Address: testAddress2},
		{TxID: testTxID(2), Vout: 0, Amount: 10000, Address: testAddress3},
	}

	client := NewClient(nil)

	// Spend 1: Uses largest UTXO
	selected1, change1, err := client.SelectUTXOs(initialUTXOs, 5000, DefaultFeeRate)
	require.NoError(t, err)
	assert.Len(t, selected1, 1)
	t.Logf("Spend 1: selected UTXO %s, change %d", selected1[0].TxID[:8], change1)

	// Simulate removing spent UTXO and adding change
	remainingUTXOs := []UTXO{
		initialUTXOs[1], // Unspent
		initialUTXOs[2], // Unspent
		{TxID: "change1", Vout: 1, Amount: change1, Address: testAddress}, // Change from spend 1
	}

	// Spend 2: Uses another UTXO
	selected2, change2, err := client.SelectUTXOs(remainingUTXOs, 5000, DefaultFeeRate)
	require.NoError(t, err)
	assert.NotEmpty(t, selected2)
	t.Logf("Spend 2: selected %d UTXOs, change %d", len(selected2), change2)

	// Calculate remaining balance
	var remaining uint64
	for _, u := range remainingUTXOs {
		found := false
		for _, s := range selected2 {
			if u.TxID == s.TxID && u.Vout == s.Vout {
				found = true
				break
			}
		}
		if !found {
			remaining += u.Amount
		}
	}
	remaining += change2

	t.Logf("Remaining after spend 2: %d", remaining)
}

// TestSpend_MultiInputTransaction tests building a transaction with multiple inputs.
func TestSpend_MultiInputTransaction(t *testing.T) {
	t.Parallel()

	// 3 UTXOs that need to be combined
	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: 5000, Address: testAddress},
		{TxID: testTxID(1), Vout: 0, Amount: 5000, Address: testAddress2},
		{TxID: testTxID(2), Vout: 0, Amount: 5000, Address: testAddress3},
	}

	client := NewClient(nil)

	// Select amount that requires all 3 UTXOs
	// 15000 total, need > 10000 to require 3
	selected, change, err := client.SelectUTXOs(utxos, 12000, DefaultFeeRate)
	require.NoError(t, err)

	// Should select all 3 since fee grows with inputs
	assert.Len(t, selected, 3)

	fee := (EstimateTxSize(3, 2)*DefaultFeeRate + 999) / 1000
	expectedChange := 15000 - 12000 - fee
	assert.Equal(t, expectedChange, change)
}

// TestSpend_FeeEstimation tests that fee estimation works correctly.
func TestSpend_FeeEstimation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		numInputs  int
		numOutputs int
		feeRate    uint64
	}{
		{"1 input, 1 output", 1, 1, 1000},
		{"1 input, 2 outputs", 1, 2, 1000},
		{"3 inputs, 1 output", 3, 1, 1000},
		{"5 inputs, 2 outputs", 5, 2, 1000},
		{"10 inputs, 1 output", 10, 1, 1000},
		{"1 input, 1 output @ 50 sat/byte", 1, 1, 50000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fee := EstimateFeeForTx(tt.numInputs, tt.numOutputs, tt.feeRate)

			// Fee should be positive
			assert.Positive(t, fee)

			// Fee should scale with inputs
			baseFee := EstimateFeeForTx(1, tt.numOutputs, tt.feeRate)
			if tt.numInputs > 1 {
				assert.Greater(t, fee, baseFee)
			}

			// Fee should scale with fee rate
			lowFee := EstimateFeeForTx(tt.numInputs, tt.numOutputs, 1)
			if tt.feeRate > 1 {
				assert.Greater(t, fee, lowFee)
			}

			t.Logf("%d inputs, %d outputs @ %d sat/KB = %d satoshis",
				tt.numInputs, tt.numOutputs, tt.feeRate, fee)
		})
	}
}

// TestSpend_LargeTransaction tests a transaction with many inputs.
func TestSpend_LargeTransaction(t *testing.T) {
	t.Parallel()

	// 100 small UTXOs
	utxos := make([]UTXO, 100)
	for i := range utxos {
		utxos[i] = UTXO{
			TxID:    testTxID(i),
			Vout:    0,
			Amount:  1000,
			Address: testAddress,
		}
	}

	// Total: 100k sats
	client := NewClient(nil)

	// Try to consolidate 80k sats
	selected, change, err := client.SelectUTXOs(utxos, 80000, DefaultFeeRate)
	require.NoError(t, err)

	// Should select many UTXOs
	assert.Greater(t, len(selected), 80) // At least 80 + fee coverage

	// Verify total
	var total uint64
	for _, u := range selected {
		total += u.Amount
	}
	fee := (EstimateTxSize(len(selected), 2)*DefaultFeeRate + 999) / 1000
	assert.GreaterOrEqual(t, total, uint64(80000)+fee)

	t.Logf("Selected %d UTXOs totaling %d, change %d", len(selected), total, change)
}

// TestSpend_MaxOutput tests sending maximum possible amount.
func TestSpend_MaxOutput(t *testing.T) {
	t.Parallel()

	// Single large UTXO
	utxoAmount := uint64(1000000) // 1M sats (0.01 BSV)
	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: utxoAmount, Address: testAddress},
	}

	client := NewClient(nil)

	// Try to send max (UTXO - fee)
	feeEstimate := (EstimateTxSize(1, 2)*DefaultFeeRate + 999) / 1000
	maxSendAmount := utxoAmount - feeEstimate

	selected, change, err := client.SelectUTXOs(utxos, maxSendAmount, DefaultFeeRate)
	require.NoError(t, err)

	assert.Len(t, selected, 1)
	assert.Equal(t, uint64(0), change) // No change when sending max
}

// TestSpend_InsufficientFunds tests various insufficient funds scenarios.
func TestSpend_InsufficientFunds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		utxos  []UTXO
		amount uint64
	}{
		{
			name:   "empty UTXOs",
			utxos:  []UTXO{},
			amount: 1000,
		},
		{
			name: "amount exceeds total",
			utxos: []UTXO{
				{TxID: testTxID(0), Vout: 0, Amount: 5000, Address: testAddress},
			},
			amount: 10000,
		},
		{
			name: "cannot cover fee",
			utxos: []UTXO{
				{TxID: testTxID(0), Vout: 0, Amount: 110, Address: testAddress},
			},
			amount: 100, // 110 - 100 = 10, but fee exceeds available
		},
	}

	client := NewClient(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := client.SelectUTXOs(tt.utxos, tt.amount, DefaultFeeRate)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "insufficient")
		})
	}
}

// TestSpend_ChangeAddressSelection tests that change goes to correct address.
// Actual change address handling is implemented in Send().
func TestSpend_ChangeAddressSelection(t *testing.T) {
	t.Parallel()

	utxos := []UTXO{
		{TxID: testTxID(0), Vout: 0, Amount: 100000, Address: testAddress},
	}

	client := NewClient(nil)

	selected, change, err := client.SelectUTXOs(utxos, 50000, DefaultFeeRate)
	require.NoError(t, err)

	// SelectUTXOs doesn't determine change address, just calculates change amount
	// The change address is determined in Send() based on ChangeAddress in request
	assert.Len(t, selected, 1)
	assert.Positive(t, change)

	// In a real transaction:
	// - If ChangeAddress is specified, change goes there
	// - Otherwise, change goes back to From address
	// - Change address should be a new derived address for privacy
}
