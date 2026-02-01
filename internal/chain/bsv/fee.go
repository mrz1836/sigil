package bsv

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	// DefaultFeeRate is the default fee rate in satoshis per byte.
	DefaultFeeRate = 1

	// MinFeeRate is the minimum fee rate in satoshis per byte.
	MinFeeRate = 1

	// MaxFeeRate is the maximum reasonable fee rate in satoshis per byte.
	MaxFeeRate = 50

	// TAALMerchantAPIURL is the URL for TAAL's Merchant API.
	TAALMerchantAPIURL = "https://merchantapi.taal.com/mapi/feeQuote"

	// P2PKHInputSize is the size of a P2PKH input in bytes.
	P2PKHInputSize = 148

	// P2PKHOutputSize is the size of a P2PKH output in bytes.
	P2PKHOutputSize = 34

	// TxOverhead is the fixed overhead for a transaction in bytes.
	TxOverhead = 10
)

// FeeQuote represents a fee quote from a miner.
type FeeQuote struct {
	// Standard fee rate for data transactions.
	StandardRate uint64 `json:"standard_rate"`

	// Data fee rate for OP_RETURN transactions.
	DataRate uint64 `json:"data_rate"`

	// Source of the fee quote (e.g., "taal", "gorillapool").
	Source string `json:"source"`

	// Timestamp when the quote was fetched.
	Timestamp time.Time `json:"timestamp"`
}

// TAALFeeQuoteResponse represents the TAAL Merchant API fee quote response.
type TAALFeeQuoteResponse struct {
	Payload string `json:"payload"`
}

// TAALPayload is the parsed payload from TAAL's response.
type TAALPayload struct {
	Fees []struct {
		FeeType   string `json:"feeType"`
		MiningFee struct {
			Satoshis int64 `json:"satoshis"`
			Bytes    int64 `json:"bytes"`
		} `json:"miningFee"`
		RelayFee struct {
			Satoshis int64 `json:"satoshis"`
			Bytes    int64 `json:"bytes"`
		} `json:"relayFee"`
	} `json:"fees"`
}

// GetFeeQuote fetches the current fee quote from TAAL Merchant API.
//
//nolint:gocognit,gocyclo // API response parsing is necessarily complex
func (c *Client) GetFeeQuote(ctx context.Context) (*FeeQuote, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, TAALMerchantAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Fallback to default fee rate on network error
		c.debug("fee API request failed, using default rate: %v", err)
		return defaultFeeQuote(), nil
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.debug("failed to close fee response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		// Fallback to default fee rate on error
		c.debug("fee API returned status %d, using default rate", resp.StatusCode)
		return defaultFeeQuote(), nil
	}

	var taalResp TAALFeeQuoteResponse
	if err := json.NewDecoder(resp.Body).Decode(&taalResp); err != nil {
		c.debug("fee API response decode failed, using default rate: %v", err)
		return defaultFeeQuote(), nil
	}

	// Parse the payload
	var payload TAALPayload
	if err := json.Unmarshal([]byte(taalResp.Payload), &payload); err != nil {
		c.debug("fee API payload parse failed, using default rate: %v", err)
		return defaultFeeQuote(), nil
	}

	// Extract standard fee rate
	var standardRate uint64 = DefaultFeeRate
	for _, fee := range payload.Fees {
		if fee.FeeType == "standard" && fee.MiningFee.Bytes > 0 {
			//nolint:gosec // Safe: satoshis and bytes are always positive from API
			standardRate = uint64(fee.MiningFee.Satoshis) / uint64(fee.MiningFee.Bytes)
			if standardRate == 0 {
				standardRate = 1
			}
			break
		}
	}

	return &FeeQuote{
		StandardRate: standardRate,
		DataRate:     standardRate, // Same for now
		Source:       "taal",
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
func EstimateFeeForTx(numInputs, numOutputs int, feeRate uint64) uint64 {
	size := EstimateTxSize(numInputs, numOutputs)
	return size * feeRate
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
