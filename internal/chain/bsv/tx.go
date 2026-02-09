package bsv

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"

	"github.com/mrz1836/sigil/internal/chain"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
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

	// ErrInvalidTxID indicates an invalid transaction ID format.
	ErrInvalidTxID = errors.New("invalid transaction ID")

	// ErrSigningFailed indicates transaction signing failed.
	ErrSigningFailed = errors.New("transaction signing failed")

	// ErrInvalidPrivateKey indicates an invalid private key.
	ErrInvalidPrivateKey = errors.New("invalid private key")

	// ErrMissingLockingScript indicates a UTXO is missing its locking script.
	ErrMissingLockingScript = errors.New("UTXO missing locking script")
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

	dustLimit := chain.BSV.DustLimit()
	if amount < dustLimit {
		return fmt.Errorf("%w: %d satoshis (minimum: %d)", ErrDustOutput, amount, dustLimit)
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
	// Validate addresses: From is required unless pre-fetched UTXOs are provided
	if len(req.UTXOs) == 0 {
		if err := ValidateBase58CheckAddress(req.From); err != nil {
			return nil, fmt.Errorf("invalid from address: %w", err)
		}
	} else if req.From != "" {
		if err := ValidateBase58CheckAddress(req.From); err != nil {
			return nil, fmt.Errorf("invalid from address: %w", err)
		}
	}
	if err := ValidateBase58CheckAddress(req.To); err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}

	// Validate amount for non-sweep requests before any network calls
	if !req.SweepAll && req.Amount == nil {
		return nil, sigilerr.ErrAmountRequired
	}

	// Get UTXOs: use pre-fetched multi-address UTXOs or fetch for single address
	var (
		utxos []UTXO
		err   error
	)
	if len(req.UTXOs) > 0 {
		utxos = convertChainUTXOs(req.UTXOs)
	} else {
		utxos, err = c.ListUTXOs(ctx, req.From)
		if err != nil {
			return nil, fmt.Errorf("listing UTXOs: %w", err)
		}
	}

	// Get fee quote
	feeRate := uint64(DefaultFeeRate)
	if req.FeeRate > 0 {
		feeRate = req.FeeRate
	}

	var selected []UTXO
	var amount uint64
	var change uint64

	//nolint:nestif // Sweep vs normal send have distinct UTXO selection paths
	if req.SweepAll {
		// Sweep: use ALL UTXOs, calculate max send amount, no change output
		if len(utxos) == 0 {
			return nil, ErrInsufficientFunds
		}
		selected = utxos

		var totalInputs uint64
		for _, u := range utxos {
			totalInputs += u.Amount
		}

		sweepAmount, sweepErr := CalculateSweepAmount(totalInputs, len(utxos), feeRate)
		if sweepErr != nil {
			return nil, sweepErr
		}
		amount = sweepAmount
	} else {
		// Normal send: select UTXOs to cover amount + fee
		amount = req.Amount.Uint64()

		selected, change, err = c.SelectUTXOs(utxos, amount, feeRate)
		if err != nil {
			return nil, err
		}
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

	// Add change output if above dust (skipped for sweep since there is no change)
	//nolint:nestif // Change output logic only applies to non-sweep transactions
	if !req.SweepAll {
		dustLimit := chain.BSV.DustLimit()
		if change >= dustLimit {
			changeAddr := req.From
			if req.ChangeAddress != "" {
				changeAddr = req.ChangeAddress
			}
			err = builder.AddOutput(changeAddr, change)
			if err != nil {
				return nil, fmt.Errorf("adding change output: %w", err)
			}
		}
	}

	// Validate transaction
	err = builder.Validate()
	if err != nil {
		return nil, fmt.Errorf("validating transaction: %w", err)
	}

	// Build and sign raw transaction (multi-key when PrivateKeys is provided)
	var rawTx []byte
	if len(req.PrivateKeys) > 0 {
		rawTx, err = BuildRawTransactionMultiKey(builder, req.PrivateKeys)
	} else {
		rawTx, err = BuildRawTransaction(builder, req.PrivateKey)
	}
	if err != nil {
		return nil, fmt.Errorf("building raw transaction: %w", err)
	}

	// Zero private keys after use
	if req.PrivateKey != nil {
		ZeroPrivateKey(req.PrivateKey)
	}
	for addr := range req.PrivateKeys {
		ZeroPrivateKey(req.PrivateKeys[addr])
	}

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
		Amount: c.FormatAmount(amountToBigInt(amount)),
		Fee:    c.FormatAmount(amountToBigInt(fee)),
		Status: "pending",
	}, nil
}

// BuildRawTransaction builds and signs a raw BSV transaction using go-sdk.
// The builder contains the UTXOs to spend and the outputs to create.
// The privateKey is used to sign all inputs (assumes all inputs are from the same key).
//
// This function handles:
// - Transaction structure creation using go-sdk
// - P2PKH locking script generation for outputs
// - P2PKH unlocking script generation with SIGHASH_ALL|SIGHASH_FORKID signing
// - Proper BSV transaction serialization
func BuildRawTransaction(builder *TxBuilder, privateKey []byte) ([]byte, error) {
	if err := validateBuildInputs(builder, privateKey); err != nil {
		return nil, err
	}

	// Create private key and unlocking template
	privKey, _ := ec.PrivateKeyFromBytes(privateKey)
	unlocker, err := p2pkh.Unlock(privKey, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: creating unlocking template: %w", ErrSigningFailed, err)
	}

	// Build the transaction
	tx := transaction.NewTransaction()

	if err := addInputsToTx(tx, builder.Inputs, unlocker); err != nil {
		return nil, err
	}

	if err := addOutputsToTx(tx, builder.Outputs); err != nil {
		return nil, err
	}

	// Sign and verify
	if err := signAndVerifyTx(tx); err != nil {
		return nil, err
	}

	return tx.Bytes(), nil
}

// BuildRawTransactionMultiKey builds and signs a raw BSV transaction where inputs
// may belong to different addresses with different private keys.
// The keyMap maps each address to its 32-byte private key.
// Each input's UTXO.Address is looked up in keyMap to find its signing key.
func BuildRawTransactionMultiKey(builder *TxBuilder, keyMap map[string][]byte) ([]byte, error) {
	if err := validateMultiKeyInputs(builder, keyMap); err != nil {
		return nil, err
	}

	// Build unlockers per address
	unlockers, err := buildUnlockers(keyMap)
	if err != nil {
		return nil, err
	}

	tx := transaction.NewTransaction()

	if err := addInputsToTxMultiKey(tx, builder.Inputs, unlockers); err != nil {
		return nil, err
	}

	if err := addOutputsToTx(tx, builder.Outputs); err != nil {
		return nil, err
	}

	if err := signAndVerifyTx(tx); err != nil {
		return nil, err
	}

	return tx.Bytes(), nil
}

// validateMultiKeyInputs validates builder and keyMap for BuildRawTransactionMultiKey.
func validateMultiKeyInputs(builder *TxBuilder, keyMap map[string][]byte) error {
	if builder == nil || len(builder.Inputs) == 0 {
		return ErrNoInputs
	}
	if len(builder.Outputs) == 0 {
		return ErrNoOutputs
	}
	if len(keyMap) == 0 {
		return fmt.Errorf("%w: no private keys provided", ErrInvalidPrivateKey)
	}
	for addr, keyBytes := range keyMap {
		if len(keyBytes) != 32 {
			return fmt.Errorf("%w: key for %s: expected 32 bytes, got %d",
				ErrInvalidPrivateKey, addr, len(keyBytes))
		}
	}
	return nil
}

// buildUnlockers creates P2PKH unlockers for each address in the key map.
func buildUnlockers(keyMap map[string][]byte) (map[string]*p2pkh.P2PKH, error) {
	unlockers := make(map[string]*p2pkh.P2PKH, len(keyMap))
	for addr, keyBytes := range keyMap {
		privKey, _ := ec.PrivateKeyFromBytes(keyBytes)
		unlocker, err := p2pkh.Unlock(privKey, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: creating unlocking template for %s: %w",
				ErrSigningFailed, addr, err)
		}
		unlockers[addr] = unlocker
	}
	return unlockers, nil
}

// addInputsToTxMultiKey adds inputs with per-address unlockers.
func addInputsToTxMultiKey(tx *transaction.Transaction, utxos []UTXO, unlockers map[string]*p2pkh.P2PKH) error {
	for i, utxo := range utxos {
		unlocker, ok := unlockers[utxo.Address]
		if !ok {
			return fmt.Errorf("%w: input %d: no private key for address %s",
				ErrSigningFailed, i, utxo.Address)
		}

		prevTxID, err := chainhash.NewHashFromHex(utxo.TxID)
		if err != nil {
			return fmt.Errorf("%w: input %d: %w", ErrInvalidTxID, i, err)
		}

		lockingScript, err := getLockingScript(utxo)
		if err != nil {
			return fmt.Errorf("%w: input %d: %w", ErrMissingLockingScript, i, err)
		}

		input := &transaction.TransactionInput{
			SourceTXID:              prevTxID,
			SourceTxOutIndex:        utxo.Vout,
			SequenceNumber:          transaction.DefaultSequenceNumber,
			UnlockingScriptTemplate: unlocker,
		}
		input.SetSourceTxOutput(&transaction.TransactionOutput{
			Satoshis:      utxo.Amount,
			LockingScript: lockingScript,
		})
		tx.AddInput(input)
	}
	return nil
}

// convertChainUTXOs converts chain.UTXO slice to bsv.UTXO slice.
func convertChainUTXOs(utxos []chain.UTXO) []UTXO {
	result := make([]UTXO, len(utxos))
	for i, u := range utxos {
		result[i] = UTXO{
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			ScriptPubKey:  u.ScriptPubKey,
			Address:       u.Address,
			Confirmations: u.Confirmations,
		}
	}
	return result
}

// validateBuildInputs validates the inputs for BuildRawTransaction.
func validateBuildInputs(builder *TxBuilder, privateKey []byte) error {
	if builder == nil || len(builder.Inputs) == 0 {
		return ErrNoInputs
	}
	if len(builder.Outputs) == 0 {
		return ErrNoOutputs
	}
	if len(privateKey) != 32 {
		return fmt.Errorf("%w: expected 32 bytes, got %d", ErrInvalidPrivateKey, len(privateKey))
	}
	return nil
}

// addInputsToTx adds all inputs from UTXOs to the transaction.
func addInputsToTx(tx *transaction.Transaction, utxos []UTXO, unlocker *p2pkh.P2PKH) error {
	for i, utxo := range utxos {
		prevTxID, err := chainhash.NewHashFromHex(utxo.TxID)
		if err != nil {
			return fmt.Errorf("%w: input %d: %w", ErrInvalidTxID, i, err)
		}

		lockingScript, err := getLockingScript(utxo)
		if err != nil {
			return fmt.Errorf("%w: input %d: %w", ErrMissingLockingScript, i, err)
		}

		input := &transaction.TransactionInput{
			SourceTXID:              prevTxID,
			SourceTxOutIndex:        utxo.Vout,
			SequenceNumber:          transaction.DefaultSequenceNumber,
			UnlockingScriptTemplate: unlocker,
		}
		input.SetSourceTxOutput(&transaction.TransactionOutput{
			Satoshis:      utxo.Amount,
			LockingScript: lockingScript,
		})
		tx.AddInput(input)
	}
	return nil
}

// addOutputsToTx adds all outputs to the transaction.
func addOutputsToTx(tx *transaction.Transaction, outputs []TxOutput) error {
	for i, output := range outputs {
		if err := tx.PayToAddress(output.Address, output.Amount); err != nil {
			return fmt.Errorf("adding output %d: %w", i, err)
		}
	}
	return nil
}

// signAndVerifyTx signs all inputs and verifies signatures were created.
func signAndVerifyTx(tx *transaction.Transaction) error {
	if err := tx.Sign(); err != nil {
		return fmt.Errorf("%w: %w", ErrSigningFailed, err)
	}

	for i, input := range tx.Inputs {
		if input.UnlockingScript == nil || len(*input.UnlockingScript) == 0 {
			return fmt.Errorf("%w: input %d: no signature generated", ErrSigningFailed, i)
		}
	}
	return nil
}

// getLockingScript returns the locking script for a UTXO.
// If ScriptPubKey is provided, it's parsed directly.
// Otherwise, the script is derived from the UTXO's address.
func getLockingScript(utxo UTXO) (*script.Script, error) {
	if utxo.ScriptPubKey != "" {
		return script.NewFromHex(utxo.ScriptPubKey)
	}

	if utxo.Address != "" {
		addr, err := script.NewAddressFromString(utxo.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid address: %w", err)
		}
		return p2pkh.Lock(addr)
	}

	return nil, ErrMissingLockingScript
}

// BroadcastTransaction broadcasts a raw transaction to the network.
// It tries each configured broadcaster in order, returning on first success.
func (c *Client) BroadcastTransaction(ctx context.Context, rawTx []byte) (string, error) {
	txHex := hex.EncodeToString(rawTx)

	var lastErr error
	for _, b := range c.broadcasters {
		c.debug("broadcasting via %s", b.Name())
		txid, err := b.Broadcast(ctx, c.httpClient, txHex)
		if err == nil {
			c.debug("broadcast successful via %s: %s", b.Name(), txid)
			return txid, nil
		}
		c.debug("broadcast failed via %s: %v", b.Name(), err)
		lastErr = err
	}

	if lastErr != nil {
		return "", fmt.Errorf("%w: all providers failed: %w", ErrBroadcastFailed, lastErr)
	}
	return "", fmt.Errorf("%w: no broadcast providers configured", ErrBroadcastFailed)
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
	dustLimit := chain.BSV.DustLimit()
	if sendAmount < dustLimit {
		return 0, fmt.Errorf("%w: remaining %d satoshis is below dust limit %d",
			ErrSweepInsufficientFunds, sendAmount, dustLimit)
	}

	return sendAmount, nil
}
