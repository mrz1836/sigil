package chain

// FeeStrategy selects a fee tier for Bitcoin-family (UTXO) chains. BTC maps the
// tiers onto mempool.space recommended fees; BSV maps them onto miner fee stats.
// The units differ per chain (BTC: sat/vByte, BSV: sat/KB) but the tier names
// are shared.
type FeeStrategy string

// Fee strategy tiers shared by the Bitcoin-family chains.
const (
	// FeeStrategyEconomy targets the cheapest, slowest tier.
	FeeStrategyEconomy FeeStrategy = "economy"
	// FeeStrategyNormal targets the balanced default tier.
	FeeStrategyNormal FeeStrategy = "normal"
	// FeeStrategyPriority targets the fastest, most expensive tier.
	FeeStrategyPriority FeeStrategy = "priority"
)

// P2PKH transaction sizing constants (bytes), shared by the Bitcoin-family
// chains. Sigil only spends legacy P2PKH inputs and creates P2PKH change, so a
// transaction's vsize equals its serialized size (no witness discount).
const (
	// P2PKHInputSize is the size of a legacy P2PKH input
	// (outpoint 36 + scriptSig ~107 + sequence 4 + script-len byte).
	P2PKHInputSize = 148
	// P2PKHOutputSize is the serialized size of a P2PKH output
	// (value 8 + varint 1 + script 25).
	P2PKHOutputSize = 34
	// TxOverhead is the fixed transaction overhead
	// (version 4 + locktime 4 + input-count 1 + output-count 1).
	TxOverhead = 10
)

// EstimateP2PKHTxSize returns the serialized size (== vsize for legacy P2PKH) of
// a transaction with the given number of P2PKH inputs and outputs.
func EstimateP2PKHTxSize(numInputs, numOutputs int) uint64 {
	//nolint:gosec // input/output counts are always small and non-negative
	return uint64(TxOverhead + (numInputs * P2PKHInputSize) + (numOutputs * P2PKHOutputSize))
}

// ClampFeeRate clamps rate to the inclusive [minRate, maxRate] range.
func ClampFeeRate(rate, minRate, maxRate uint64) uint64 {
	if rate < minRate {
		return minRate
	}
	if rate > maxRate {
		return maxRate
	}
	return rate
}
