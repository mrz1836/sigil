package discovery

import (
	"context"
	"fmt"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// Migration-specific errors.
var (
	// ErrNoAddressesToMigrate indicates no discovered addresses to migrate.
	ErrNoAddressesToMigrate = &sigilerr.SigilError{
		Code:     "NO_ADDRESSES_TO_MIGRATE",
		Message:  "no addresses with funds to migrate",
		ExitCode: sigilerr.ExitInput,
	}

	// ErrDustAmount indicates the total is too small to migrate.
	ErrDustAmount = &sigilerr.SigilError{
		Code:     "DUST_AMOUNT",
		Message:  "total amount is too small to migrate (would be consumed by fees)",
		ExitCode: sigilerr.ExitInput,
	}

	// ErrMigrationFailed indicates the migration transaction failed.
	ErrMigrationFailed = &sigilerr.SigilError{
		Code:     "MIGRATION_FAILED",
		Message:  "migration transaction failed",
		ExitCode: sigilerr.ExitGeneral,
	}
)

// Fee calculation constants.
const (
	// DefaultFeeRate is the default fee rate in satoshis per kilobyte.
	// 50 sat/KB = 0.05 sat/byte, matching the current BSV network status quo.
	DefaultFeeRate uint64 = 50

	// InputSize is the estimated size of a P2PKH input in bytes.
	InputSize uint64 = 148

	// OutputSize is the estimated size of a P2PKH output in bytes.
	OutputSize uint64 = 34

	// OverheadSize is the base transaction overhead in bytes.
	OverheadSize uint64 = 10

	// DustWarningThreshold is the percentage of fees vs total where we warn.
	// If fees > 10% of total, warn the user.
	DustWarningThreshold = 0.10
)

// MigrationSource represents a source address for migration.
type MigrationSource struct {
	// Address is the source BSV address.
	Address string

	// Path is the derivation path used.
	Path string

	// Balance is the balance in satoshis.
	Balance uint64

	// UTXOCount is the number of UTXOs at this address.
	UTXOCount int

	// PrivateKey is the derived private key for signing.
	// This is populated during migration execution.
	PrivateKey []byte
}

// MigrationPlan describes a consolidation transaction.
type MigrationPlan struct {
	// Sources are the addresses to consolidate from.
	Sources []MigrationSource

	// Destination is the target address for consolidated funds.
	Destination string

	// TotalInput is the sum of all source balances in satoshis.
	TotalInput uint64

	// EstimatedFee is the estimated transaction fee in satoshis.
	EstimatedFee uint64

	// NetAmount is TotalInput - EstimatedFee.
	NetAmount uint64

	// FeeRate is the fee rate used in satoshis per byte.
	FeeRate uint64

	// EstimatedSize is the estimated transaction size in bytes.
	EstimatedSize uint64

	// TotalUTXOs is the total number of UTXOs being consolidated.
	TotalUTXOs int

	// Warning is set if fees are a significant portion of total.
	Warning string
}

// CreateMigrationPlan creates a plan for consolidating discovered funds.
func CreateMigrationPlan(result *Result, destination string, feeRate uint64) (*MigrationPlan, error) {
	if result == nil || !result.HasFunds() {
		return nil, ErrNoAddressesToMigrate
	}

	if destination == "" {
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidAddress,
			"destination address is required for migration",
		)
	}

	if feeRate == 0 {
		feeRate = DefaultFeeRate
	}

	plan := &MigrationPlan{
		Destination: destination,
		FeeRate:     feeRate,
	}

	// Collect all sources
	for _, addresses := range result.FoundAddresses {
		for _, addr := range addresses {
			source := MigrationSource{
				Address:   addr.Address,
				Path:      addr.Path,
				Balance:   addr.Balance,
				UTXOCount: addr.UTXOCount,
			}
			plan.Sources = append(plan.Sources, source)
			plan.TotalInput += addr.Balance
			plan.TotalUTXOs += addr.UTXOCount
		}
	}

	if len(plan.Sources) == 0 {
		return nil, ErrNoAddressesToMigrate
	}

	// Calculate estimated transaction size
	// Size = overhead + (inputs * input_size) + (outputs * output_size)
	// For consolidation, we typically have 1 output (destination)
	//nolint:gosec // TotalUTXOs is bounded by number of addresses scanned, not user input
	plan.EstimatedSize = OverheadSize + (uint64(plan.TotalUTXOs) * InputSize) + OutputSize

	// Calculate fee (feeRate is in sat/KB, round up)
	plan.EstimatedFee = (plan.EstimatedSize*feeRate + 999) / 1000

	// Check if fee exceeds total
	if plan.EstimatedFee >= plan.TotalInput {
		return nil, sigilerr.WithSuggestion(
			ErrDustAmount,
			fmt.Sprintf("estimated fee (%d sats) exceeds total balance (%d sats)",
				plan.EstimatedFee, plan.TotalInput),
		)
	}

	plan.NetAmount = plan.TotalInput - plan.EstimatedFee

	// Warn if fees are a significant portion
	feeRatio := float64(plan.EstimatedFee) / float64(plan.TotalInput)
	if feeRatio > DustWarningThreshold {
		plan.Warning = fmt.Sprintf(
			"fees are %.1f%% of total balance - consider waiting for lower fees",
			feeRatio*100,
		)
	}

	return plan, nil
}

// TransactionBuilder defines the interface for building migration transactions.
type TransactionBuilder interface {
	// BuildConsolidationTx builds a transaction consolidating multiple inputs to one output.
	BuildConsolidationTx(ctx context.Context, inputs []TxInput, destination string, amount uint64) ([]byte, error)

	// SignInput signs a specific input of the transaction.
	SignInput(tx []byte, inputIndex int, privateKey []byte, sigHashType uint32) ([]byte, error)

	// BroadcastTx broadcasts a signed transaction and returns the txid.
	BroadcastTx(ctx context.Context, tx []byte) (string, error)
}

// TxInput represents an input for the consolidation transaction.
type TxInput struct {
	TxID         string
	Vout         uint32
	Amount       uint64
	ScriptPubKey string
	Address      string
	PrivateKey   []byte
}

// MigrationResult contains the outcome of a migration operation.
type MigrationResult struct {
	// TxID is the transaction ID of the consolidation transaction.
	TxID string

	// TotalMigrated is the amount consolidated in satoshis.
	TotalMigrated uint64

	// Fee is the actual fee paid in satoshis.
	Fee uint64

	// InputCount is the number of inputs consolidated.
	InputCount int

	// SourceAddresses lists the addresses that were consolidated.
	SourceAddresses []string

	// Destination is the target address.
	Destination string
}

// Migrator handles the execution of migration plans.
type Migrator struct {
	client  ChainClient
	builder TransactionBuilder
	deriver KeyDeriver
}

// NewMigrator creates a new migrator.
func NewMigrator(client ChainClient, builder TransactionBuilder, deriver KeyDeriver) *Migrator {
	return &Migrator{
		client:  client,
		builder: builder,
		deriver: deriver,
	}
}

// Execute executes a migration plan.
//
//nolint:gocognit,gocyclo // Transaction building requires multiple validation and processing steps
func (m *Migrator) Execute(ctx context.Context, seed []byte, plan *MigrationPlan) (*MigrationResult, error) {
	if plan == nil {
		return nil, ErrNoAddressesToMigrate
	}

	if len(seed) == 0 {
		return nil, ErrInvalidSeed
	}

	// Collect all UTXOs from source addresses
	var inputs []TxInput
	var sourceAddresses []string

	for _, source := range plan.Sources {
		utxos, err := m.client.ListUTXOs(ctx, source.Address)
		if err != nil {
			return nil, fmt.Errorf("fetching UTXOs for %s: %w", source.Address, err)
		}

		for _, utxo := range utxos {
			input := TxInput{
				TxID:         utxo.TxID,
				Vout:         utxo.Vout,
				Amount:       utxo.Amount,
				ScriptPubKey: utxo.ScriptPubKey,
				Address:      source.Address,
				// PrivateKey will be derived during signing
			}
			inputs = append(inputs, input)
		}

		sourceAddresses = append(sourceAddresses, source.Address)
	}

	if len(inputs) == 0 {
		return nil, ErrNoAddressesToMigrate
	}

	// Build the consolidation transaction
	rawTx, err := m.builder.BuildConsolidationTx(ctx, inputs, plan.Destination, plan.NetAmount)
	if err != nil {
		return nil, fmt.Errorf("building transaction: %w", err)
	}

	// Sign each input
	// In a real implementation, we would need to derive the private key
	// for each source address. This is a simplified version.
	signedTx := rawTx
	for i := range inputs {
		// The actual private key derivation would happen here based on the path
		// For now, we assume the builder handles signing internally
		signedTx, err = m.builder.SignInput(signedTx, i, nil, 0x41) // SIGHASH_ALL | SIGHASH_FORKID
		if err != nil {
			return nil, fmt.Errorf("signing input %d: %w", i, err)
		}
	}

	// Broadcast the transaction
	txid, err := m.builder.BroadcastTx(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("broadcasting transaction: %w", err)
	}

	result := &MigrationResult{
		TxID:            txid,
		TotalMigrated:   plan.NetAmount,
		Fee:             plan.EstimatedFee,
		InputCount:      len(inputs),
		SourceAddresses: sourceAddresses,
		Destination:     plan.Destination,
	}

	return result, nil
}

// ValidatePlan checks if a migration plan is still valid.
// This should be called before execution to ensure UTXOs haven't been spent.
func (m *Migrator) ValidatePlan(ctx context.Context, plan *MigrationPlan) error {
	if plan == nil {
		return ErrNoAddressesToMigrate
	}

	// Verify destination address is valid
	if err := m.client.ValidateAddress(plan.Destination); err != nil {
		return fmt.Errorf("invalid destination: %w", err)
	}

	// Verify each source still has the expected balance
	var actualTotal uint64
	for _, source := range plan.Sources {
		utxos, err := m.client.ListUTXOs(ctx, source.Address)
		if err != nil {
			return fmt.Errorf("validating %s: %w", source.Address, err)
		}

		var addressBalance uint64
		for _, utxo := range utxos {
			addressBalance += utxo.Amount
		}

		if addressBalance != source.Balance {
			return sigilerr.WithDetails(
				sigilerr.ErrInvalidValue,
				map[string]string{
					"address":  source.Address,
					"expected": fmt.Sprintf("%d", source.Balance),
					"actual":   fmt.Sprintf("%d", addressBalance),
				},
			)
		}

		actualTotal += addressBalance
	}

	if actualTotal != plan.TotalInput {
		return sigilerr.WithDetails(
			sigilerr.ErrInvalidValue,
			map[string]string{
				"expected": fmt.Sprintf("%d", plan.TotalInput),
				"actual":   fmt.Sprintf("%d", actualTotal),
			},
		)
	}

	return nil
}
