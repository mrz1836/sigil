package bsv

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
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

// wocMinerFeeEntry represents a single entry from the WhatsOnChain miner fees API.
type wocMinerFeeEntry struct {
	Timestamp int64   `json:"timestamp"`
	Name      string  `json:"name"`
	FeeRate   float64 `json:"fee_rate"` // sat/KB
}

// GetFeeQuote fetches the current fee quote from WhatsOnChain's miner fees API.
// Falls back to the default fee rate on any error.
func (c *Client) GetFeeQuote(ctx context.Context) (*FeeQuote, error) {
	now := time.Now().Unix()
	from := now - feeWindowSeconds
	url := fmt.Sprintf("%s/miner/fees?from=%d&to=%d", c.baseURL, from, now)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.debug("fee API request failed, using default rate: %v", err)
		return defaultFeeQuote(), nil
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.debug("failed to close fee response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		c.debug("fee API returned status %d, using default rate", resp.StatusCode)
		return defaultFeeQuote(), nil
	}

	var entries []wocMinerFeeEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		c.debug("fee API response decode failed, using default rate: %v", err)
		return defaultFeeQuote(), nil
	}

	if len(entries) == 0 {
		c.debug("fee API returned no entries, using default rate")
		return defaultFeeQuote(), nil
	}

	// Calculate average fee rate across all miners
	var total float64
	for _, entry := range entries {
		total += entry.FeeRate
	}
	avgRate := uint64(math.Ceil(total / float64(len(entries))))

	if avgRate < MinFeeRate {
		avgRate = MinFeeRate
	}

	return &FeeQuote{
		StandardRate: avgRate,
		DataRate:     avgRate,
		Source:       "whatsonchain",
		Timestamp:    time.Now(),
	}, nil
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
