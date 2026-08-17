package chain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/testutil"
)

// errBenchInsufficientFunds is the sentinel passed to the selector under
// benchmark; the benchmark inputs always fund the target, so it is never hit.
var errBenchInsufficientFunds = errors.New("insufficient funds")

// benchFeeFor is a representative P2PKH sat/vByte fee function for the selector
// benchmark (148 vbytes/input, 34/output, 10 overhead).
func benchFeeFor(numInputs, numOutputs int, feeRate uint64) uint64 {
	vbytes := numInputs*148 + numOutputs*34 + 10
	return uint64(vbytes) * feeRate //nolint:gosec // G115: vbytes is small and non-negative
}

// BenchmarkSelectBitcoinUTXOs measures the shared largest-first coin selector
// across growing UTXO-set sizes (its stable sort dominates at scale).
func BenchmarkSelectBitcoinUTXOs(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		utxos := make([]chain.UTXO, size)
		var total uint64
		for i := range utxos {
			// Amounts in the ~1M-sat range keep the input-count-dependent fee
			// negligible against the target, so the target is always fundable
			// (the largest-first walk, not funding, is what we measure).
			amount := uint64(1_000_000 + (i*37)%500_000)
			utxos[i] = testutil.NewTestUTXO(
				testutil.WithUTXOAmount(amount),
				testutil.WithUTXOTxID(fmt.Sprintf("%064x", i)),
				testutil.WithUTXOVout(uint32(i)),
			)
			total += amount
		}
		target := total / 2 // walk roughly half the (largest-first) set

		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := chain.SelectBitcoinUTXOs(utxos, target, 5, 546, benchFeeFor, errBenchInsufficientFunds); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
