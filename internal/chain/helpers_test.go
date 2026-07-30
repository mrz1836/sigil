package chain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// flatFee returns a fee equal to feeRate regardless of input/output counts, so
// selection math in the tests is easy to reason about.
func flatFee(_, _ int, feeRate uint64) uint64 { return feeRate }

func TestSelectBitcoinUTXOs(t *testing.T) {
	t.Parallel()

	utxos := []chain.UTXO{
		{TxID: "a", Amount: 100},
		{TxID: "b", Amount: 200},
		{TxID: "c", Amount: 50},
	}

	t.Run("empty returns the insufficient-funds sentinel", func(t *testing.T) {
		t.Parallel()
		selected, change, err := chain.SelectBitcoinUTXOs(nil, 100, 10, 5, flatFee, sigilerr.ErrInsufficientFunds)
		require.ErrorIs(t, err, sigilerr.ErrInsufficientFunds)
		assert.Nil(t, selected)
		assert.Zero(t, change)
	})

	t.Run("selects largest-first and returns change", func(t *testing.T) {
		t.Parallel()
		selected, change, err := chain.SelectBitcoinUTXOs(utxos, 120, 10, 5, flatFee, sigilerr.ErrInsufficientFunds)
		require.NoError(t, err)
		require.Len(t, selected, 1)
		assert.Equal(t, "b", selected[0].TxID) // 200 is largest
		assert.Equal(t, uint64(70), change)    // 200 - (120 + 10)
	})

	t.Run("drops change below the dust limit", func(t *testing.T) {
		t.Parallel()
		selected, change, err := chain.SelectBitcoinUTXOs(utxos, 195, 5, 100, flatFee, sigilerr.ErrInsufficientFunds)
		require.NoError(t, err)
		require.Len(t, selected, 1)
		assert.Zero(t, change) // 200 - 200 = 0 (and would-be change < dust anyway)
	})

	t.Run("insufficient total returns the sentinel", func(t *testing.T) {
		t.Parallel()
		selected, _, err := chain.SelectBitcoinUTXOs(utxos, 1000, 10, 5, flatFee, sigilerr.ErrInsufficientFunds)
		require.ErrorIs(t, err, sigilerr.ErrInsufficientFunds)
		assert.Nil(t, selected)
	})

	t.Run("stable tie-break keeps equal amounts in input order", func(t *testing.T) {
		t.Parallel()
		tied := []chain.UTXO{
			{TxID: "x", Amount: 100},
			{TxID: "y", Amount: 100},
			{TxID: "z", Amount: 300},
		}
		// Need z (300) plus two 100s; ties resolve to input order x before y.
		selected, _, err := chain.SelectBitcoinUTXOs(tied, 450, 10, 5, flatFee, sigilerr.ErrInsufficientFunds)
		require.NoError(t, err)
		require.Len(t, selected, 3)
		assert.Equal(t, []string{"z", "x", "y"}, []string{selected[0].TxID, selected[1].TxID, selected[2].TxID})
	})
}

func TestEstimateP2PKHTxSize(t *testing.T) {
	t.Parallel()
	// Overhead 10 + 1 input (148) + 2 outputs (2*34) = 226.
	assert.Equal(t, uint64(226), chain.EstimateP2PKHTxSize(1, 2))
	assert.Equal(t, uint64(chain.TxOverhead), chain.EstimateP2PKHTxSize(0, 0))
}

func TestClampFeeRate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		rate, minR, maxR uint64
		want             uint64
	}{
		{name: "below min clamps up", rate: 1, minR: 50, maxR: 5000, want: 50},
		{name: "above max clamps down", rate: 9999, minR: 50, maxR: 5000, want: 5000},
		{name: "within range unchanged", rate: 250, minR: 50, maxR: 5000, want: 250},
		{name: "at min unchanged", rate: 50, minR: 50, maxR: 5000, want: 50},
		{name: "at max unchanged", rate: 5000, minR: 50, maxR: 5000, want: 5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, chain.ClampFeeRate(tt.rate, tt.minR, tt.maxR))
		})
	}
}

func TestIsValidTxID(t *testing.T) {
	t.Parallel()
	valid := "6dfb16dd580698242bcfd8e433d557ed8c642272a368894de27292a8844a4e75"
	assert.True(t, chain.IsValidTxID(valid))
	assert.False(t, chain.IsValidTxID(""))
	assert.False(t, chain.IsValidTxID(valid[:63]))    // too short
	assert.False(t, chain.IsValidTxID(valid+"aa"))    // too long
	assert.False(t, chain.IsValidTxID("z"+valid[1:])) // non-hex char
}

func TestExtractTxID(t *testing.T) {
	t.Parallel()
	id := "6dfb16dd580698242bcfd8e433d557ed8c642272a368894de27292a8844a4e75"
	assert.Equal(t, id, chain.ExtractTxID("txid: "+id+" (already known)"))
	assert.Empty(t, chain.ExtractTxID("no hex here"))
}

func TestFormatFixedDecimal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		amount   int64
		decimals int
		isNil    bool
		want     string
	}{
		{name: "one BTC", amount: 100000000, decimals: 8, want: "1.00000000"},
		{name: "sub-unit keeps trailing zeros", amount: 1, decimals: 8, want: "0.00000001"},
		{name: "zero", amount: 0, decimals: 8, want: "0.00000000"},
		{name: "nil treated as zero", isNil: true, decimals: 8, want: "0.00000000"},
		{name: "eth wei", amount: 1500000000000000000, decimals: 18, want: "1.500000000000000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			amt := chain.AmountToBigInt(uint64(tt.amount)) //nolint:gosec // test values are non-negative
			if tt.isNil {
				amt = nil
			}
			assert.Equal(t, tt.want, chain.FormatFixedDecimal(amt, tt.decimals))
		})
	}
}
