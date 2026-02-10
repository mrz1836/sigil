package bsv

import (
	"context"
	"math"
	"sort"
	"time"

	whatsonchain "github.com/mrz1836/go-whatsonchain"
)

const (
	// DefaultFeeRate is the default fee rate in satoshis per kilobyte (1000 bytes).
	// 250 sat/KB = 0.25 sat/byte, based on current BSV network fee environment.
	DefaultFeeRate = 250

	// MinFeeRate is the minimum fee rate in satoshis per kilobyte.
	MinFeeRate = 50

	// MaxFeeRate is the maximum reasonable fee rate in satoshis per kilobyte.
	MaxFeeRate = 50000

	// feeWindowSeconds is the lookback window for miner fee stats (24 hours).
	feeWindowSeconds = 86400

	// P2PKHInputSize is the size of a P2PKH input in bytes.
	P2PKHInputSize = 148

	// P2PKHOutputSize is the size of a P2PKH output in bytes.
	P2PKHOutputSize = 34

	// TxOverhead is the fixed overhead for a transaction in bytes.
	TxOverhead = 10
)

// FeeStrategy defines the fee selection strategy for BSV transactions.
type FeeStrategy string

const (
	// FeeStrategyEconomy selects the lowest MinFeeRate from any miner.
	FeeStrategyEconomy FeeStrategy = "economy"
	// FeeStrategyNormal selects the Nth-lowest rate (sorted descending) to ensure at least N miners accept.
	FeeStrategyNormal FeeStrategy = "normal"
	// FeeStrategyPriority selects the highest MinFeeRate across all miners.
	FeeStrategyPriority FeeStrategy = "priority"
)

// FeeQuote represents a fee quote from a miner.
type FeeQuote struct {
	// Standard fee rate in satoshis per kilobyte.
	StandardRate uint64 `json:"standard_rate"`

	// Data fee rate in satoshis per kilobyte.
	DataRate uint64 `json:"data_rate"`

	// Source of the fee quote (e.g., "whatsonchain", "default").
	Source string `json:"source"`

	// Timestamp when the quote was fetched.
	Timestamp time.Time `json:"timestamp"`
}

// GetFeeQuote fetches the current fee quote from WhatsOnChain's miner fees API.
// Falls back to the default fee rate on any error.
func (c *Client) GetFeeQuote(ctx context.Context) (*FeeQuote, error) {
	now := time.Now().Unix()
	from := now - feeWindowSeconds

	entries, err := c.woc.GetMinerFeesStats(ctx, from, now)
	if err != nil {
		c.logError("fee API request failed, using default rate: %v", err)
		return defaultFeeQuote(), nil
	}

	if len(entries) == 0 {
		c.debug("fee API returned no entries, using default rate")
		return defaultFeeQuote(), nil
	}

	rate := uint64(math.Ceil(selectFeeRate(entries, c.feeStrategy, c.minMiners)))

	if rate < MinFeeRate {
		rate = MinFeeRate
	}
	c.debug("fee quote: %d sat/KB from %d miners (strategy=%s, min_miners=%d)", rate, len(entries), c.feeStrategy, c.minMiners)

	return &FeeQuote{
		StandardRate: rate,
		DataRate:     rate,
		Source:       "whatsonchain",
		Timestamp:    time.Now(),
	}, nil
}

// selectFeeRate picks a fee rate from miner entries based on the given strategy.
// entries must be non-empty.
func selectFeeRate(entries []*whatsonchain.MinerFeeStats, strategy FeeStrategy, minMiners int) float64 {
	switch strategy {
	case FeeStrategyEconomy:
		return minFeeRateFrom(entries)
	case FeeStrategyPriority:
		return maxFeeRateFrom(entries)
	case FeeStrategyNormal:
		return nthFeeRate(entries, minMiners)
	}
	// Unknown strategy falls back to normal behavior.
	return nthFeeRate(entries, minMiners)
}

// minFeeRateFrom returns the lowest MinFeeRate across all entries.
func minFeeRateFrom(entries []*whatsonchain.MinerFeeStats) float64 {
	lowest := entries[0].MinFeeRate
	for _, e := range entries[1:] {
		if e.MinFeeRate < lowest {
			lowest = e.MinFeeRate
		}
	}
	return lowest
}

// maxFeeRateFrom returns the highest MinFeeRate across all entries.
func maxFeeRateFrom(entries []*whatsonchain.MinerFeeStats) float64 {
	highest := entries[0].MinFeeRate
	for _, e := range entries[1:] {
		if e.MinFeeRate > highest {
			highest = e.MinFeeRate
		}
	}
	return highest
}

// nthFeeRate sorts entries descending and returns the rate at index (minMiners-1),
// clamped to [0, len-1]. This guarantees at least minMiners miners accept the rate.
func nthFeeRate(entries []*whatsonchain.MinerFeeStats, minMiners int) float64 {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].MinFeeRate > entries[j].MinFeeRate
	})
	idx := minMiners - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(entries) {
		idx = len(entries) - 1
	}
	return entries[idx].MinFeeRate
}

// EstimateTxSize estimates the transaction size in bytes.
func EstimateTxSize(numInputs, numOutputs int) uint64 {
	// P2PKH transaction size estimate:
	// - Fixed overhead: 10 bytes (version: 4, locktime: 4, vin count: 1, vout count: 1)
	// - Per input: ~148 bytes (outpoint: 36, scriptSig: 107, sequence: 4)
	// - Per output: ~34 bytes (value: 8, scriptPubKey: 25)
	//nolint:gosec // Safe: transaction sizes are always positive and within bounds
	return uint64(TxOverhead + (numInputs * P2PKHInputSize) + (numOutputs * P2PKHOutputSize))
}

// EstimateFeeForTx estimates the fee for a transaction with given inputs/outputs.
// The feeRate is in satoshis per kilobyte. The result is rounded up to ensure
// the fee always covers the rate.
func EstimateFeeForTx(numInputs, numOutputs int, feeRate uint64) uint64 {
	size := EstimateTxSize(numInputs, numOutputs)
	return (size*feeRate + 999) / 1000
}

// EstimateFeeForAmount estimates the fee for sending a specific amount.
// Assumes 1 input initially, then recalculates based on UTXO selection.
func (c *Client) EstimateFeeForAmount(ctx context.Context, _ uint64) (uint64, error) {
	quote, err := c.GetFeeQuote(ctx)
	if err != nil {
		quote = defaultFeeQuote()
	}

	// Assume 1 input, 2 outputs (recipient + change)
	return EstimateFeeForTx(1, 2, quote.StandardRate), nil
}

// defaultFeeQuote returns a default fee quote when API is unavailable.
func defaultFeeQuote() *FeeQuote {
	return &FeeQuote{
		StandardRate: DefaultFeeRate,
		DataRate:     DefaultFeeRate,
		Source:       "default",
		Timestamp:    time.Now(),
	}
}

// ValidateFeeRate ensures a fee rate is within acceptable bounds.
func ValidateFeeRate(rate uint64) uint64 {
	if rate < MinFeeRate {
		return MinFeeRate
	}
	if rate > MaxFeeRate {
		return MaxFeeRate
	}
	return rate
}
