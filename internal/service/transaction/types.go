package transaction

import (
	"math/big"
	"time"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
)

// SendRequest represents a transaction send request with chain-specific fields.
type SendRequest struct {
	// Common fields
	ChainID     chain.ID
	To          string
	AmountStr   string // Raw amount string from user (e.g., "1.5" or "all")
	Wallet      string
	FromAddress string

	// ETH-specific
	Token    string // ERC-20 token symbol (e.g., "USDC")
	GasSpeed string // "slow", "medium", "fast"

	// BSV-specific (populated by service layer)
	Addresses []wallet.Address // All wallet addresses for BSV multi-address support

	// Flags
	Confirm       bool // If false, prompt user for confirmation
	ValidateUTXOs bool // If true, validate UTXOs before sweep (BSV only)

	// Agent mode fields (optional)
	AgentCredID      string
	AgentToken       string
	AgentCounterPath string

	// Internal (populated by CLI layer)
	Seed []byte
}

// SweepAll returns true if the amount is "all".
func (r *SendRequest) SweepAll() bool {
	return IsAmountAll(r.AmountStr)
}

// SendResult represents the outcome of a transaction send operation.
type SendResult struct {
	Hash    string
	From    string
	To      string
	Amount  string // Formatted amount
	Fee     string // Formatted fee
	Token   string // Empty for native currency
	Status  string
	ChainID chain.ID

	// ETH-specific
	GasUsed  uint64
	GasPrice string

	// BSV-specific
	UTXOsSpent int
}

// ValidationError represents a validation error with context.
type ValidationError struct {
	Field   string
	Message string
	Details map[string]string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if len(e.Details) > 0 {
		return e.Message + " (details available)"
	}
	return e.Message
}

// AmountMode represents how the amount should be interpreted.
type AmountMode int

const (
	// AmountFixed represents a fixed amount specified by the user.
	AmountFixed AmountMode = iota
	// AmountSweepAll represents sending the entire balance.
	AmountSweepAll
)

// PrepareContext holds context needed for transaction preparation.
type PrepareContext struct {
	FromAddress string
	Amount      *big.Int
	AmountMode  AmountMode
	Token       string // Empty for native currency
	Timestamp   time.Time
}

// ExecuteContext holds context needed for transaction execution.
type ExecuteContext struct {
	PrepareContext

	Seed        []byte
	PrivateKeys map[string][]byte // For BSV multi-address
	UTXOs       []chain.UTXO      // For BSV
}
