package bsv

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"

	"github.com/mrz1836/sigil/internal/chain"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// DustLimit is the minimum output value in satoshis.
	DustLimit = 546

	// TAALBroadcastURL is the URL for TAAL's transaction broadcast API.
	TAALBroadcastURL = "https://merchantapi.taal.com/mapi/tx"
)

var (
	// ErrNoInputs indicates the transaction has no inputs.
	ErrNoInputs = errors.New("transaction has no inputs")

	// ErrNoOutputs indicates the transaction has no outputs.
	ErrNoOutputs = errors.New("transaction has no outputs")

	// ErrDustOutput indicates an output is below the dust limit.
	ErrDustOutput = errors.New("output amount is below dust limit")

	// ErrBroadcastFailed indicates transaction broadcast failed.
	ErrBroadcastFailed = errors.New("transaction broadcast failed")
)

// TxOutput represents a transaction output.
type TxOutput struct {
	Address string
	Amount  uint64
}

// TxBuilder builds BSV transactions.
type TxBuilder struct {
	Inputs  []UTXO
	Outputs []TxOutput
	FeeRate uint64
}

// NewTxBuilder creates a new transaction builder.
func NewTxBuilder() *TxBuilder {
	return &TxBuilder{
		FeeRate: DefaultFeeRate,
	}
}

// AddInput adds a UTXO as an input.
func (b *TxBuilder) AddInput(utxo UTXO) error {
	b.Inputs = append(b.Inputs, utxo)
	return nil
}

// AddOutput adds an output to the transaction.
func (b *TxBuilder) AddOutput(address string, amount uint64) error {
	if err := ValidateBase58CheckAddress(address); err != nil {
		return fmt.Errorf("invalid output address: %w", err)
	}

	if amount < DustLimit {
		return fmt.Errorf("%w: %d satoshis (minimum: %d)", ErrDustOutput, amount, DustLimit)
	}

	b.Outputs = append(b.Outputs, TxOutput{
		Address: address,
		Amount:  amount,
	})

	return nil
}

// TotalInputAmount returns the sum of all input amounts.
func (b *TxBuilder) TotalInputAmount() uint64 {
	var total uint64
	for _, input := range b.Inputs {
		total += input.Amount
	}
	return total
}

// TotalOutputAmount returns the sum of all output amounts.
func (b *TxBuilder) TotalOutputAmount() uint64 {
	var total uint64
	for _, output := range b.Outputs {
		total += output.Amount
	}
	return total
}

// CalculateFee calculates the fee based on transaction size.
func (b *TxBuilder) CalculateFee(feeRate uint64) uint64 {
	size := EstimateTxSize(len(b.Inputs), len(b.Outputs))
	return size * feeRate
}

// Validate checks that the transaction is valid.
func (b *TxBuilder) Validate() error {
	if len(b.Inputs) == 0 {
		return ErrNoInputs
	}

	if len(b.Outputs) == 0 {
		return ErrNoOutputs
	}

	inputTotal := b.TotalInputAmount()
	outputTotal := b.TotalOutputAmount()
	fee := b.CalculateFee(b.FeeRate)

	if inputTotal < outputTotal+fee {
		return fmt.Errorf("%w: have %d, need %d (outputs: %d, fee: %d)",
			ErrInsufficientFunds, inputTotal, outputTotal+fee, outputTotal, fee)
	}

	return nil
}

// SetFeeRate sets the fee rate for the transaction.
func (b *TxBuilder) SetFeeRate(rate uint64) {
	b.FeeRate = ValidateFeeRate(rate)
}

// Send implements the chain.Chain interface for BSV.
//
//nolint:gocognit,gocyclo // Transaction building involves multiple steps
func (c *Client) Send(ctx context.Context, req chain.SendRequest) (*chain.TransactionResult, error) {
	// Validate addresses
	if err := ValidateBase58CheckAddress(req.From); err != nil {
		return nil, fmt.Errorf("invalid from address: %w", err)
	}
	if err := ValidateBase58CheckAddress(req.To); err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}

	// Get amount as uint64
	if req.Amount == nil {
		return nil, sigilerr.ErrAmountRequired
	}
	amount := req.Amount.Uint64()

	// Get UTXOs for the address
	utxos, err := c.ListUTXOs(ctx, req.From)
	if err != nil {
		return nil, fmt.Errorf("listing UTXOs: %w", err)
	}

	// Get fee quote
	feeRate := uint64(DefaultFeeRate)
	if req.FeeRate > 0 {
		feeRate = req.FeeRate
	}

	// Select UTXOs
	selected, change, err := c.SelectUTXOs(utxos, amount, feeRate)
	if err != nil {
		return nil, err
	}

	// Build transaction
	builder := NewTxBuilder()
	builder.SetFeeRate(feeRate)

	for _, utxo := range selected {
		err = builder.AddInput(utxo)
		if err != nil {
			return nil, fmt.Errorf("adding input: %w", err)
		}
	}

	// Add recipient output
	err = builder.AddOutput(req.To, amount)
	if err != nil {
		return nil, fmt.Errorf("adding recipient output: %w", err)
	}

	// Add change output if above dust
	if change >= DustLimit {
		// Use provided change address, or fall back to sender address
		changeAddr := req.From
		if req.ChangeAddress != "" {
			changeAddr = req.ChangeAddress
		}
		err = builder.AddOutput(changeAddr, change)
		if err != nil {
			return nil, fmt.Errorf("adding change output: %w", err)
		}
	}

	// Validate transaction
	err = builder.Validate()
	if err != nil {
		return nil, fmt.Errorf("validating transaction: %w", err)
	}

	// Build and sign raw transaction
	rawTx, err := BuildRawTransaction(builder, req.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("building raw transaction: %w", err)
	}

	// Zero the private key after use
	ZeroPrivateKey(req.PrivateKey)

	// Broadcast transaction
	txHash, err := c.BroadcastTransaction(ctx, rawTx)
	if err != nil {
		return nil, err
	}

	// Calculate fee
	fee := builder.TotalInputAmount() - builder.TotalOutputAmount()

	return &chain.TransactionResult{
		Hash:   txHash,
		From:   req.From,
		To:     req.To,
		Amount: c.FormatAmount(req.Amount),
		Fee:    c.FormatAmount(amountToBigInt(fee)),
		Status: "pending",
	}, nil
}

// BuildRawTransaction builds a raw transaction from the builder.
// This is a simplified implementation - in production, use go-sdk.
func BuildRawTransaction(_ *TxBuilder, _ []byte) ([]byte, error) {
	// TODO: Implement proper transaction building using go-sdk
	// For now, return an error indicating this needs to be implemented
	return nil, sigilerr.ErrNotImplemented
}

// BroadcastTransaction broadcasts a raw transaction to the network.
func (c *Client) BroadcastTransaction(ctx context.Context, rawTx []byte) (string, error) {
	txHex := hex.EncodeToString(rawTx)

	payload := map[string]string{
		"rawtx": txHex,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TAALBroadcastURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", sigilerr.ErrNetworkError, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.debug("failed to close broadcast response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%w: status %d, body: %s", ErrBroadcastFailed, resp.StatusCode, string(body))
	}

	var result struct {
		TxID string `json:"txid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	return result.TxID, nil
}

// ZeroPrivateKey zeros out a private key for security.
func ZeroPrivateKey(key []byte) {
	for i := range key {
		key[i] = 0
	}
}

// amountToBigInt converts uint64 to *big.Int.
func amountToBigInt(amount uint64) *big.Int {
	return new(big.Int).SetUint64(amount)
}

// ErrSweepInsufficientFunds indicates there are not enough funds to cover the fee.
var ErrSweepInsufficientFunds = errors.New("insufficient funds: fee exceeds total balance")

// CalculateSweepAmount calculates the maximum amount that can be sent when sweeping
// all UTXOs from a wallet. It accounts for the transaction fee based on the number
// of inputs and a single output.
//
// Parameters:
//   - totalInputs: total amount in satoshis from all UTXOs
//   - numInputs: number of UTXOs being spent
//   - feeRate: fee rate in satoshis per byte
//
// Returns:
//   - sendAmount: the amount that can be sent after deducting the fee
//   - err: error if fee exceeds available funds
func CalculateSweepAmount(totalInputs uint64, numInputs int, feeRate uint64) (uint64, error) {
	// Validate fee rate
	feeRate = ValidateFeeRate(feeRate)

	// Calculate fee for numInputs -> 1 output transaction
	// No change output since we're sweeping everything
	fee := EstimateFeeForTx(numInputs, 1, feeRate)

	if fee >= totalInputs {
		return 0, fmt.Errorf("%w: total %d satoshis, fee %d satoshis",
			ErrSweepInsufficientFunds, totalInputs, fee)
	}

	sendAmount := totalInputs - fee

	// Verify result is above dust limit
	if sendAmount < DustLimit {
		return 0, fmt.Errorf("%w: remaining %d satoshis is below dust limit %d",
			ErrSweepInsufficientFunds, sendAmount, DustLimit)
	}

	return sendAmount, nil
}
