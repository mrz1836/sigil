package btc

import (
	"context"
	"math"
)

const (
	// DefaultFeeRate is the fallback fee rate in satoshis per vByte, used only
	// when the fee API is unavailable.
	DefaultFeeRate = 2

	// MinFeeRate is the minimum fee rate floor in satoshis per vByte (min-relay).
	MinFeeRate = 1

	// MaxFeeRate is the sanity ceiling in satoshis per vByte, guarding against
	// absurd API values.
	MaxFeeRate = 5000

	// P2PKHInputSize is the size of a legacy P2PKH input in bytes
	// (outpoint 36 + scriptSig ~107 + sequence 4 + script-len byte).
	P2PKHInputSize = 148

	// P2PKHOutputSize is the serialized size of a P2PKH output in bytes
	// (value 8 + varint 1 + script 25). Change outputs are always P2PKH.
	P2PKHOutputSize = 34

	// TxOverhead is the fixed transaction overhead in bytes
	// (version 4 + locktime 4 + input-count 1 + output-count 1).
	TxOverhead = 10
)

// FeeStrategy selects which mempool.space fee tier to use.
type FeeStrategy string

// Fee strategy constants map to mempool.space recommended-fee tiers.
const (
	// FeeStrategyEconomy targets the economyFee tier (cheapest, slowest).
	FeeStrategyEconomy FeeStrategy = "economy"
	// FeeStrategyNormal targets the halfHourFee tier (balanced default).
	FeeStrategyNormal FeeStrategy = "normal"
	// FeeStrategyPriority targets the fastestFee tier (next-block).
	FeeStrategyPriority FeeStrategy = "priority"
)

// ValidateFeeRate clamps a fee rate (satoshis per vByte) to sane bounds.
func ValidateFeeRate(rate uint64) uint64 {
	if rate < MinFeeRate {
		return MinFeeRate
	}
	if rate > MaxFeeRate {
		return MaxFeeRate
	}
	return rate
}

// EstimateTxSize estimates the transaction vsize in bytes assuming legacy P2PKH
// inputs and P2PKH outputs. Since Sigil only spends legacy P2PKH inputs, vsize
// equals the serialized size (there is no witness discount to apply).
func EstimateTxSize(numInputs, numOutputs int) uint64 {
	//nolint:gosec // Safe: input/output counts are always small and non-negative
	return uint64(TxOverhead + (numInputs * P2PKHInputSize) + (numOutputs * P2PKHOutputSize))
}

// EstimateFeeForTx estimates the fee for a P2PKH transaction with the given
// input/output counts. Fee = vsize × sat/vByte (no per-kilobyte division).
func EstimateFeeForTx(numInputs, numOutputs int, feeRate uint64) uint64 {
	return EstimateTxSize(numInputs, numOutputs) * feeRate
}

// varIntLen returns the byte length of a Bitcoin CompactSize (varint) for n.
func varIntLen(n uint64) uint64 {
	switch {
	case n < 0xfd:
		return 1
	case n <= 0xffff:
		return 3
	case n <= 0xffffffff:
		return 5
	default:
		return 9
	}
}

// outputVBytes returns the exact serialized size of a transaction output whose
// locking script is scriptLen bytes: value(8) + varint(scriptLen) + scriptLen.
// This corrects the flat-34-byte assumption for non-P2PKH recipients — notably
// P2WSH (43 bytes), which the flat estimate under-sizes by 9 bytes.
func outputVBytes(scriptLen int) uint64 {
	//nolint:gosec // scriptLen is a small, non-negative script length
	l := uint64(scriptLen)
	return 8 + varIntLen(l) + l
}

// estimateFeeExact computes the fee for a transaction with the given number of
// P2PKH inputs and the exact locking-script lengths of each output.
func estimateFeeExact(numInputs int, outputScriptLens []int, feeRate uint64) uint64 {
	//nolint:gosec // counts are small and non-negative
	size := uint64(TxOverhead + numInputs*P2PKHInputSize)
	for _, l := range outputScriptLens {
		size += outputVBytes(l)
	}
	return size * feeRate
}

// FeeRate returns the current fee rate in satoshis per vByte for the client's
// strategy, floored at max(minimumFee, 1) and clamped to MaxFeeRate. It falls
// back to DefaultFeeRate when the fee API is unavailable.
func (c *Client) FeeRate(ctx context.Context) uint64 {
	est, err := c.provider.FeeEstimates(ctx)
	if err != nil || est == nil {
		c.debug("fee estimates unavailable, using default %d sat/vB: %v", DefaultFeeRate, err)
		return DefaultFeeRate
	}

	var raw float64
	switch c.feeStrategy {
	case FeeStrategyEconomy:
		raw = est.EconomyFee
	case FeeStrategyPriority:
		raw = est.FastestFee
	case FeeStrategyNormal:
		raw = est.HalfHourFee
	default:
		raw = est.HalfHourFee
	}

	rate := uint64(math.Ceil(raw))

	floor := max(uint64(math.Ceil(est.MinimumFee)), MinFeeRate)
	if rate < floor {
		rate = floor
	}
	if rate > MaxFeeRate {
		rate = MaxFeeRate
	}

	c.debug("fee rate: %d sat/vB (strategy=%s)", rate, c.feeStrategy)
	return rate
}
