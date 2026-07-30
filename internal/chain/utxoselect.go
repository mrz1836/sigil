package chain

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
)

// ErrAmountOverflow indicates a uint64 addition would overflow while summing
// UTXO amounts or a target. It should be unreachable with realistic amounts.
var ErrAmountOverflow = errors.New("amount overflow: uint64 limit exceeded")

// addChecked returns a+b, or ErrAmountOverflow if the sum would exceed uint64.
func addChecked(a, b uint64) (uint64, error) {
	if a > math.MaxUint64-b {
		return 0, ErrAmountOverflow
	}
	return a + b, nil
}

// SelectBitcoinUTXOs chooses UTXOs (largest-first) to fund a transaction for a
// Bitcoin-family chain. It is the shared core of the BTC and BSV coin selectors;
// the only per-chain differences are injected: feeFor computes the fee for a
// given input/output count and rate (BTC: sat/vByte, BSV: sat/KB), dustLimit is
// the chain's dust threshold, and insufficientFundsErr is the chain's sentinel.
//
// A stable sort keeps equal-amount UTXOs in input order, giving deterministic,
// reproducible selection. Change below dustLimit is dropped (added to the fee).
//
//nolint:gocognit // Overflow checks add necessary complexity for fund safety
func SelectBitcoinUTXOs(
	utxos []UTXO,
	amount, feeRate, dustLimit uint64,
	feeFor func(numInputs, numOutputs int, feeRate uint64) uint64,
	insufficientFundsErr error,
) (selected []UTXO, change uint64, err error) {
	if len(utxos) == 0 {
		return nil, 0, insufficientFundsErr
	}

	sorted := slices.Clone(utxos)
	slices.SortStableFunc(sorted, func(a, b UTXO) int {
		return cmp.Compare(b.Amount, a.Amount) // descending
	})

	var total uint64
	var estimatedFee uint64
	for _, utxo := range sorted {
		selected = append(selected, utxo)

		sum, addErr := addChecked(total, utxo.Amount)
		if addErr != nil {
			return nil, 0, fmt.Errorf("UTXO sum: %w", addErr)
		}
		total = sum

		// Assume 2 outputs (recipient + change) while selecting.
		estimatedFee = feeFor(len(selected), 2, feeRate)
		target, targetErr := addChecked(amount, estimatedFee)
		if targetErr != nil {
			return nil, 0, fmt.Errorf("target amount: %w", targetErr)
		}
		if total >= target {
			change = total - target
			if change < dustLimit {
				change = 0
			}
			return selected, change, nil
		}
	}

	target, _ := addChecked(amount, estimatedFee)
	return nil, 0, fmt.Errorf("%w: need %d satoshis, have %d", insufficientFundsErr, target, total)
}
