package bsv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSelectUTXOsStableTieBreak verifies coin selection is deterministic: a
// stable sort keeps equal-amount UTXOs in input order, so selection is
// reproducible for identical UTXO sets (largest-first, ties by input order).
func TestSelectUTXOsStableTieBreak(t *testing.T) {
	c := &Client{}

	// Deliberately unsorted, with two 300k and four 100k tie groups.
	utxos := []UTXO{
		{TxID: "c300", Amount: 300000},
		{TxID: "a100", Amount: 100000},
		{TxID: "b100", Amount: 100000},
		{TxID: "c100", Amount: 100000},
		{TxID: "d200", Amount: 200000},
		{TxID: "e100", Amount: 100000},
		{TxID: "f300", Amount: 300000},
	}

	// amount+fee falls between the 4th (900k) and 5th (1.0M) cumulative totals,
	// so exactly five UTXOs are selected, reaching into the 100k tie group.
	const amount, feeRate = 950000, 1
	want := []string{"c300", "f300", "d200", "a100", "b100"}

	// Two runs must yield identical order (determinism).
	for range 2 {
		selected, _, err := c.SelectUTXOs(utxos, amount, feeRate)
		require.NoError(t, err)

		got := make([]string, len(selected))
		for i, u := range selected {
			got[i] = u.TxID
		}
		require.Equal(t, want, got)
	}
}
