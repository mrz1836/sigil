package btc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"

	"github.com/bsv-blockchain/go-sdk/chainhash"
	ec "github.com/bsv-blockchain/go-sdk/primitives/ec"
	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/bsv-blockchain/go-sdk/transaction"
	sighash "github.com/bsv-blockchain/go-sdk/transaction/sighash"
	"github.com/bsv-blockchain/go-sdk/transaction/template/p2pkh"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
	"github.com/mrz1836/sigil/internal/wallet/bitcoin"
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

	// ErrAmountOverflow indicates an arithmetic overflow in amount calculation.
	ErrAmountOverflow = errors.New("amount overflow: uint64 limit exceeded")

	// ErrSigningFailed indicates transaction signing failed.
	ErrSigningFailed = errors.New("transaction signing failed")

	// ErrInvalidPrivateKey indicates an invalid private key.
	ErrInvalidPrivateKey = errors.New("invalid private key")

	// ErrMissingLockingScript indicates a UTXO is missing its locking script.
	ErrMissingLockingScript = errors.New("UTXO missing locking script")

	// ErrMissingOutputScript indicates a built output is missing its locking script.
	ErrMissingOutputScript = errors.New("output missing locking script")

	// ErrSweepInsufficientFunds indicates the fee exceeds the sweepable balance.
	ErrSweepInsufficientFunds = errors.New("insufficient funds: fee exceeds total balance")
)

// p2pkhScriptLen is the byte length of a P2PKH locking script (change outputs).
const p2pkhScriptLen = 25

// legacySighashFlag is SIGHASH_ALL (0x01) WITHOUT the BSV/BCH ForkID bit. go-sdk's
// sigStrat() selects the original pre-fork sighash algorithm (CalcInputPreimage
// Legacy) whenever ForkID is absent, producing a byte-valid legacy Bitcoin P2PKH
// signature. This is the single most load-bearing difference from the BSV package.
func legacySighashFlag() sighash.Flag {
	return sighash.All
}

// checkedAdd returns a + b, or an error if the result overflows uint64.
func checkedAdd(a, b uint64) (uint64, error) {
	if a > math.MaxUint64-b {
		return 0, ErrAmountOverflow
	}
	return a + b, nil
}

// TxOutput represents a transaction output with its pre-built locking script.
type TxOutput struct {
	Address string
	Amount  uint64
	script  *script.Script // locking script for the recipient's address type
}

// scriptLen returns the locking-script byte length of the output.
func (o TxOutput) scriptLen() int {
	if o.script == nil {
		return p2pkhScriptLen
	}
	return len(*o.script)
}

// TxBuilder builds BTC transactions. Inputs are always our own legacy P2PKH
// UTXOs; outputs may be any standard type (P2PKH/P2SH/P2WPKH/P2WSH).
type TxBuilder struct {
	Inputs  []UTXO
	Outputs []TxOutput
	FeeRate uint64
	// network scopes output-address validation and locking-script construction.
	network Network
}

// NewTxBuilder creates a new transaction builder.
func NewTxBuilder() *TxBuilder {
	return &TxBuilder{
		FeeRate: DefaultFeeRate,
	}
}

// SetNetwork sets the network used to validate addresses and build scripts.
func (b *TxBuilder) SetNetwork(network Network) {
	b.network = network
}

// AddInput adds a UTXO as an input.
func (b *TxBuilder) AddInput(utxo UTXO) error {
	b.Inputs = append(b.Inputs, utxo)
	return nil
}

// AddOutput adds an output paying the given address. The locking script is built
// per the address type; the amount must be at or above the 546-satoshi dust limit.
func (b *TxBuilder) AddOutput(address string, amount uint64) error {
	lockingScript, err := AddressToScript(address, b.network)
	if err != nil {
		return fmt.Errorf("invalid output address: %w", err)
	}

	dustLimit := chain.BTC.DustLimit()
	if amount < dustLimit {
		return fmt.Errorf("%w: %d satoshis (minimum: %d)", ErrDustOutput, amount, dustLimit)
	}

	b.Outputs = append(b.Outputs, TxOutput{
		Address: address,
		Amount:  amount,
		script:  lockingScript,
	})
	return nil
}

// TotalInputAmount returns the sum of all input amounts (overflow-checked).
func (b *TxBuilder) TotalInputAmount() (uint64, error) {
	var total uint64
	for _, input := range b.Inputs {
		sum, err := checkedAdd(total, input.Amount)
		if err != nil {
			return 0, fmt.Errorf("input sum: %w", err)
		}
		total = sum
	}
	return total, nil
}

// TotalOutputAmount returns the sum of all output amounts (overflow-checked).
func (b *TxBuilder) TotalOutputAmount() (uint64, error) {
	var total uint64
	for _, output := range b.Outputs {
		sum, err := checkedAdd(total, output.Amount)
		if err != nil {
			return 0, fmt.Errorf("output sum: %w", err)
		}
		total = sum
	}
	return total, nil
}

// CalculateFee returns the exact fee for the transaction as currently built:
// (overhead + inputs + Σ exact output sizes) × sat/vByte. Because only legacy
// P2PKH inputs are spent, vsize equals serialized size.
func (b *TxBuilder) CalculateFee(feeRate uint64) uint64 {
	size := uint64(TxOverhead + len(b.Inputs)*P2PKHInputSize)
	for _, o := range b.Outputs {
		size += outputVBytes(o.scriptLen())
	}
	return size * feeRate
}

// SetFeeRate sets the fee rate (satoshis per vByte), clamped to sane bounds.
func (b *TxBuilder) SetFeeRate(rate uint64) {
	b.FeeRate = ValidateFeeRate(rate)
}

// Validate checks that the transaction is well-formed and fully funded.
func (b *TxBuilder) Validate() error {
	if len(b.Inputs) == 0 {
		return ErrNoInputs
	}
	if len(b.Outputs) == 0 {
		return ErrNoOutputs
	}

	inputTotal, err := b.TotalInputAmount()
	if err != nil {
		return fmt.Errorf("calculating input total: %w", err)
	}
	outputTotal, err := b.TotalOutputAmount()
	if err != nil {
		return fmt.Errorf("calculating output total: %w", err)
	}

	fee := b.CalculateFee(b.FeeRate)
	needed, err := checkedAdd(outputTotal, fee)
	if err != nil {
		return fmt.Errorf("calculating required amount: %w", err)
	}
	if inputTotal < needed {
		return fmt.Errorf("%w: have %d, need %d (outputs: %d, fee: %d)",
			ErrInsufficientFunds, inputTotal, needed, outputTotal, fee)
	}
	return nil
}

// Send builds, signs, and broadcasts a BTC transaction, implementing chain.Chain.
// The recipient may be any standard address type; change is always legacy P2PKH.
//
//nolint:gocognit,gocyclo // Transaction building involves multiple sequential steps
func (c *Client) Send(ctx context.Context, req chain.SendRequest) (*chain.TransactionResult, error) {
	// Network-scoped address validation. From is required unless pre-fetched
	// multi-address UTXOs are provided.
	if len(req.UTXOs) == 0 {
		if err := c.ValidateAddress(req.From); err != nil {
			return nil, fmt.Errorf("invalid from address: %w", err)
		}
	} else if req.From != "" {
		if err := c.ValidateAddress(req.From); err != nil {
			return nil, fmt.Errorf("invalid from address: %w", err)
		}
	}
	if err := c.ValidateAddress(req.To); err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}

	if !req.SweepAll && req.Amount == nil {
		return nil, sigilerr.ErrAmountRequired
	}

	// Gather UTXOs: pre-fetched (multi-address) or fetched for the single From.
	var (
		utxos []UTXO
		err   error
	)
	if len(req.UTXOs) > 0 {
		utxos = req.UTXOs
		c.debug("send: using %d pre-fetched UTXOs", len(utxos))
	} else {
		c.debug("send: fetching UTXOs for %s", req.From)
		utxos, err = c.ListUTXOs(ctx, req.From)
		if err != nil {
			return nil, fmt.Errorf("listing UTXOs: %w", err)
		}
	}

	feeRate := req.FeeRate
	if feeRate == 0 {
		feeRate = c.FeeRate(ctx)
	}
	feeRate = ValidateFeeRate(feeRate)

	// Recipient locking script (any standard type) — its exact size drives the fee.
	recipientScript, err := AddressToScript(req.To, c.network)
	if err != nil {
		return nil, fmt.Errorf("invalid to address: %w", err)
	}
	recipientScriptLen := len(*recipientScript)

	var (
		selected []UTXO
		amount   uint64
		change   uint64
	)

	//nolint:nestif // Sweep vs normal send have distinct selection paths
	if req.SweepAll {
		if len(utxos) == 0 {
			return nil, ErrInsufficientFunds
		}
		selected = utxos

		totalInputs, sumErr := sumUTXOAmounts(selected)
		if sumErr != nil {
			return nil, fmt.Errorf("calculating sweep total: %w", sumErr)
		}
		sweepAmount, sweepErr := CalculateSweepAmount(totalInputs, len(selected), recipientScriptLen, feeRate)
		if sweepErr != nil {
			return nil, sweepErr
		}
		amount = sweepAmount
	} else {
		amount = req.Amount.Uint64()

		selected, _, err = c.SelectUTXOs(utxos, amount, feeRate)
		if err != nil {
			return nil, err
		}

		totalSelected, sumErr := sumUTXOAmounts(selected)
		if sumErr != nil {
			return nil, fmt.Errorf("calculating selected total: %w", sumErr)
		}
		change, err = computeChange(totalSelected, amount, len(selected), recipientScriptLen, feeRate)
		if err != nil {
			return nil, err
		}
	}

	// Build the transaction.
	builder := NewTxBuilder()
	builder.SetNetwork(c.network)
	builder.SetFeeRate(feeRate)

	for _, utxo := range selected {
		if err = builder.AddInput(utxo); err != nil {
			return nil, fmt.Errorf("adding input: %w", err)
		}
	}
	if err = builder.AddOutput(req.To, amount); err != nil {
		return nil, fmt.Errorf("adding recipient output: %w", err)
	}
	if !req.SweepAll && change >= chain.BTC.DustLimit() {
		changeAddr := req.From
		if req.ChangeAddress != "" {
			changeAddr = req.ChangeAddress
		}
		if err = builder.AddOutput(changeAddr, change); err != nil {
			return nil, fmt.Errorf("adding change output: %w", err)
		}
	}

	if err = builder.Validate(); err != nil {
		return nil, fmt.Errorf("validating transaction: %w", err)
	}

	// Build and sign the raw transaction (multi-key when PrivateKeys is provided).
	var rawTx []byte
	if len(req.PrivateKeys) > 0 {
		rawTx, err = BuildRawTransactionMultiKey(builder, req.PrivateKeys)
	} else {
		rawTx, err = BuildRawTransaction(builder, req.PrivateKey)
	}
	if err != nil {
		return nil, fmt.Errorf("building raw transaction: %w", err)
	}
	c.debug("send: raw tx built, %d bytes", len(rawTx))

	// Zero private keys immediately after signing.
	if req.PrivateKey != nil {
		wallet.ZeroBytes(req.PrivateKey)
	}
	for addr := range req.PrivateKeys {
		wallet.ZeroBytes(req.PrivateKeys[addr])
	}

	txHash, err := c.BroadcastTransaction(ctx, rawTx)
	if err != nil {
		return nil, err
	}

	// Fee is the input/output difference (Validate confirmed inputs >= outputs).
	inputTotal, _ := builder.TotalInputAmount()
	outputTotal, _ := builder.TotalOutputAmount()
	fee := inputTotal - outputTotal

	return &chain.TransactionResult{
		Hash:   txHash,
		From:   req.From,
		To:     req.To,
		Amount: c.FormatAmount(chain.AmountToBigInt(amount)),
		Fee:    c.FormatAmount(chain.AmountToBigInt(fee)),
		Status: "pending",
	}, nil
}

// sumUTXOAmounts returns the overflow-checked total of a UTXO set.
func sumUTXOAmounts(utxos []UTXO) (uint64, error) {
	var total uint64
	for _, u := range utxos {
		sum, err := checkedAdd(total, u.Amount)
		if err != nil {
			return 0, err
		}
		total = sum
	}
	return total, nil
}

// computeChange computes the change amount using the EXACT recipient output size
// plus a P2PKH change output. When a change output would leave less than the dust
// limit — or when the exact fee outgrows the selection (e.g. a large P2WSH
// recipient) — change is 0 and the remainder becomes additional fee.
func computeChange(totalSelected, amount uint64, numInputs, recipientScriptLen int, feeRate uint64) (uint64, error) {
	feeWithChange := estimateFeeExact(numInputs, []int{recipientScriptLen, p2pkhScriptLen}, feeRate)
	needWithChange, err := checkedAdd(amount, feeWithChange)
	if err != nil {
		return 0, err
	}
	if totalSelected >= needWithChange {
		change := totalSelected - needWithChange
		if change < chain.BTC.DustLimit() {
			return 0, nil
		}
		return change, nil
	}

	// Fall back to a no-change transaction (remainder absorbed as fee).
	feeNoChange := estimateFeeExact(numInputs, []int{recipientScriptLen}, feeRate)
	needNoChange, err := checkedAdd(amount, feeNoChange)
	if err != nil {
		return 0, err
	}
	if totalSelected >= needNoChange {
		return 0, nil
	}
	return 0, fmt.Errorf("%w: need %d satoshis, have %d", ErrInsufficientFunds, needNoChange, totalSelected)
}

// CalculateSweepAmount returns the maximum sendable amount when sweeping all
// UTXOs to a single recipient of the given locking-script length.
func CalculateSweepAmount(totalInputs uint64, numInputs, recipientScriptLen int, feeRate uint64) (uint64, error) {
	feeRate = ValidateFeeRate(feeRate)
	fee := estimateFeeExact(numInputs, []int{recipientScriptLen}, feeRate)

	if fee >= totalInputs {
		return 0, fmt.Errorf("%w: total %d satoshis, fee %d satoshis",
			ErrSweepInsufficientFunds, totalInputs, fee)
	}
	sendAmount := totalInputs - fee

	dustLimit := chain.BTC.DustLimit()
	if sendAmount < dustLimit {
		return 0, fmt.Errorf("%w: remaining %d satoshis is below dust limit %d",
			ErrSweepInsufficientFunds, sendAmount, dustLimit)
	}
	return sendAmount, nil
}

// BuildRawTransaction builds and signs a raw BTC transaction. All inputs are
// signed with the same key using the legacy (non-ForkID) SIGHASH_ALL algorithm.
func BuildRawTransaction(builder *TxBuilder, privateKey []byte) ([]byte, error) {
	tx, err := buildSignedTx(builder, privateKey)
	if err != nil {
		return nil, err
	}
	return tx.Bytes(), nil
}

// buildSignedTx builds and signs the transaction, returning the go-sdk object
// (with source outputs still attached, so callers/tests can recompute sighashes).
func buildSignedTx(builder *TxBuilder, privateKey []byte) (*transaction.Transaction, error) {
	if err := validateBuildInputs(builder, privateKey); err != nil {
		return nil, err
	}

	privKey, _ := ec.PrivateKeyFromBytes(privateKey)
	shf := legacySighashFlag()
	unlocker, err := p2pkh.Unlock(privKey, &shf)
	if err != nil {
		return nil, fmt.Errorf("%w: creating unlocking template: %w", ErrSigningFailed, err)
	}

	tx := transaction.NewTransaction()
	if err = addInputsToTx(tx, builder.Inputs, builder.network, unlocker); err != nil {
		return nil, err
	}
	if err = addOutputsToTx(tx, builder.Outputs); err != nil {
		return nil, err
	}
	if err = signAndVerifyTx(tx); err != nil {
		return nil, err
	}

	return tx, nil
}

// BuildRawTransactionMultiKey builds and signs a raw BTC transaction whose inputs
// may belong to different addresses/keys. keyMap maps each input's address to its
// 32-byte private key. Signing uses the legacy SIGHASH_ALL algorithm.
func BuildRawTransactionMultiKey(builder *TxBuilder, keyMap map[string][]byte) ([]byte, error) {
	tx, err := buildSignedTxMultiKey(builder, keyMap)
	if err != nil {
		return nil, err
	}
	return tx.Bytes(), nil
}

// buildSignedTxMultiKey is the multi-key form of buildSignedTx.
func buildSignedTxMultiKey(builder *TxBuilder, keyMap map[string][]byte) (*transaction.Transaction, error) {
	if err := validateMultiKeyInputs(builder, keyMap); err != nil {
		return nil, err
	}

	unlockers, err := buildUnlockers(keyMap)
	if err != nil {
		return nil, err
	}

	tx := transaction.NewTransaction()
	if err = addInputsToTxMultiKey(tx, builder.Inputs, builder.network, unlockers); err != nil {
		return nil, err
	}
	if err = addOutputsToTx(tx, builder.Outputs); err != nil {
		return nil, err
	}
	if err = signAndVerifyTx(tx); err != nil {
		return nil, err
	}

	return tx, nil
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

// validateMultiKeyInputs validates the inputs for BuildRawTransactionMultiKey.
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

// buildUnlockers creates legacy-SIGHASH_ALL P2PKH unlockers for each key.
func buildUnlockers(keyMap map[string][]byte) (map[string]*p2pkh.P2PKH, error) {
	unlockers := make(map[string]*p2pkh.P2PKH, len(keyMap))
	for addr, keyBytes := range keyMap {
		privKey, _ := ec.PrivateKeyFromBytes(keyBytes)
		shf := legacySighashFlag()
		unlocker, err := p2pkh.Unlock(privKey, &shf)
		if err != nil {
			return nil, fmt.Errorf("%w: creating unlocking template for %s: %w",
				ErrSigningFailed, addr, err)
		}
		unlockers[addr] = unlocker
	}
	return unlockers, nil
}

// addInputsToTx adds all inputs with a single shared unlocker.
func addInputsToTx(tx *transaction.Transaction, utxos []UTXO, network Network, unlocker *p2pkh.P2PKH) error {
	for i, utxo := range utxos {
		if err := addOneInput(tx, utxo, i, network, unlocker); err != nil {
			return err
		}
	}
	return nil
}

// addInputsToTxMultiKey adds inputs with per-address unlockers.
func addInputsToTxMultiKey(tx *transaction.Transaction, utxos []UTXO, network Network, unlockers map[string]*p2pkh.P2PKH) error {
	for i, utxo := range utxos {
		unlocker, ok := unlockers[utxo.Address]
		if !ok {
			return fmt.Errorf("%w: input %d: no private key for address %s",
				ErrSigningFailed, i, utxo.Address)
		}
		if err := addOneInput(tx, utxo, i, network, unlocker); err != nil {
			return err
		}
	}
	return nil
}

// addOneInput appends a single input, rebuilding its P2PKH locking script.
func addOneInput(tx *transaction.Transaction, utxo UTXO, index int, network Network, unlocker *p2pkh.P2PKH) error {
	prevTxID, err := chainhash.NewHashFromHex(utxo.TxID)
	if err != nil {
		return fmt.Errorf("%w: input %d: %w", ErrInvalidTxID, index, err)
	}

	lockingScript, err := getLockingScript(utxo, network)
	if err != nil {
		return fmt.Errorf("%w: input %d: %w", ErrMissingLockingScript, index, err)
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
	return nil
}

// addOutputsToTx adds all pre-built outputs directly (supporting any script type).
func addOutputsToTx(tx *transaction.Transaction, outputs []TxOutput) error {
	for i, output := range outputs {
		lockingScript := output.script
		if lockingScript == nil {
			return fmt.Errorf("%w: output %d", ErrMissingOutputScript, i)
		}
		tx.AddOutput(&transaction.TransactionOutput{
			Satoshis:      output.Amount,
			LockingScript: lockingScript,
		})
	}
	return nil
}

// signAndVerifyTx signs all inputs and verifies signatures were produced.
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

// getLockingScript returns the locking script for a UTXO. Sigil only spends its
// own legacy P2PKH outputs, so absent an explicit scriptPubKey the P2PKH script
// is rebuilt from the (network-scoped) address.
func getLockingScript(utxo UTXO, network Network) (*script.Script, error) {
	if utxo.ScriptPubKey != "" {
		return script.NewFromHex(utxo.ScriptPubKey)
	}
	if utxo.Address != "" {
		return AddressToScript(utxo.Address, network)
	}
	return nil, ErrMissingLockingScript
}

// BroadcastTransaction broadcasts a raw transaction, trying each broadcaster in
// order. A broadcaster reporting the tx is already known is treated as success,
// returning the locally-computed txid.
func (c *Client) BroadcastTransaction(ctx context.Context, rawTx []byte) (string, error) {
	txHex := hex.EncodeToString(rawTx)
	computed := computeTxID(rawTx)

	var lastErr error
	for _, b := range c.broadcasters {
		c.debug("broadcasting via %s", b.Name())
		txid, err := b.Broadcast(ctx, txHex)
		if err == nil {
			c.debug("broadcast successful via %s: %s", b.Name(), txid)
			return txid, nil
		}
		if errors.Is(err, errAlreadyKnown) {
			c.debug("broadcast via %s: already known, using computed txid %s", b.Name(), computed)
			return computed, nil
		}
		c.logError("broadcast failed via %s: %v", b.Name(), err)
		lastErr = err
	}

	if lastErr != nil {
		c.logError("all broadcast providers failed, last error: %v", lastErr)
		return "", fmt.Errorf("%w: all providers failed: %w", ErrBroadcastFailed, lastErr)
	}
	return "", fmt.Errorf("%w: no broadcast providers configured", ErrBroadcastFailed)
}

// computeTxID returns the display (big-endian) txid of a non-witness transaction:
// the byte-reversed double-SHA256 of the full serialization.
func computeTxID(rawTx []byte) string {
	h := bitcoin.DoubleSHA256(rawTx)
	for i, j := 0, len(h)-1; i < j; i, j = i+1, j-1 {
		h[i], h[j] = h[j], h[i]
	}
	return hex.EncodeToString(h)
}
