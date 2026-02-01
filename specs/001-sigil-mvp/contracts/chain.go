// Package contracts defines the interface contracts for Sigil MVP.
// These are design artifacts - not compiled code.
// Actual implementations go in internal/chain/
package contracts

import (
	"context"
	"math/big"
)

// Chain defines the interface that all blockchain implementations must satisfy.
// Each supported chain (ETH, BSV, etc.) implements this interface.
type Chain interface {
	// Name returns the chain identifier (e.g., "eth", "bsv").
	Name() string

	// GetBalance retrieves the native token balance for an address.
	// Returns balance as a big.Int in the smallest unit (wei, satoshis).
	GetBalance(ctx context.Context, address string) (*big.Int, error)

	// GetTokenBalance retrieves an ERC-20 token balance (ETH only).
	// For non-ETH chains, this returns ErrNotSupported.
	GetTokenBalance(ctx context.Context, address, tokenAddress string) (*big.Int, error)

	// EstimateFee estimates the fee for a transaction.
	// For ETH: returns gas price * estimated gas.
	// For BSV: returns fee rate * estimated tx size.
	EstimateFee(ctx context.Context, from, to string, amount *big.Int) (*big.Int, error)

	// Send builds, signs, and broadcasts a transaction.
	// Returns the transaction hash on successful broadcast.
	Send(ctx context.Context, req SendRequest) (*TransactionResult, error)

	// ValidateAddress checks if an address is valid for this chain.
	ValidateAddress(address string) error

	// FormatAmount converts a big.Int to human-readable string with proper decimals.
	FormatAmount(amount *big.Int) string

	// ParseAmount converts a human-readable string to big.Int.
	ParseAmount(amount string) (*big.Int, error)
}

// UTXOChain extends Chain with UTXO-specific operations.
// Implemented by BSV, BTC, BCH.
type UTXOChain interface {
	Chain

	// ListUTXOs returns unspent transaction outputs for an address.
	ListUTXOs(ctx context.Context, address string) ([]UTXO, error)

	// SelectUTXOs chooses UTXOs to fund a transaction of the given amount.
	// Returns selected UTXOs and change amount.
	SelectUTXOs(utxos []UTXO, amount, feeRate uint64) (selected []UTXO, change uint64, err error)
}

// SendRequest contains parameters for sending a transaction.
type SendRequest struct {
	// From is the sender address (must match a wallet address).
	From string

	// To is the recipient address.
	To string

	// Amount is the value to send in smallest units.
	Amount *big.Int

	// PrivateKey is the signing key (will be zeroed after use).
	PrivateKey []byte

	// Token is the ERC-20 token address (ETH only, empty for native).
	Token string

	// GasLimit is optional gas limit override (ETH only).
	GasLimit uint64

	// FeeRate is optional fee rate override (satoshis per byte for UTXO chains).
	FeeRate uint64
}

// TransactionResult contains the outcome of a broadcast transaction.
type TransactionResult struct {
	// Hash is the transaction hash.
	Hash string

	// From is the sender address.
	From string

	// To is the recipient address.
	To string

	// Amount is the transferred amount (human-readable).
	Amount string

	// Token is the token symbol if applicable.
	Token string

	// Fee is the fee paid (human-readable).
	Fee string

	// GasUsed is ETH-specific gas consumption.
	GasUsed uint64

	// GasPrice is ETH-specific gas price.
	GasPrice string

	// Status is "pending" after broadcast.
	Status string
}

// UTXO represents an unspent transaction output.
type UTXO struct {
	TxID          string
	Vout          uint32
	Amount        uint64 // satoshis
	ScriptPubKey  string
	Address       string
	Confirmations uint32
}

// Sentinel errors for chain operations.
var (
	ErrInvalidAddress    = Error{Code: "INVALID_ADDRESS", Message: "invalid address format"}
	ErrInvalidChecksum   = Error{Code: "INVALID_CHECKSUM", Message: "address checksum validation failed"}
	ErrInsufficientFunds = Error{Code: "INSUFFICIENT_FUNDS", Message: "insufficient funds for transaction"}
	ErrNetworkError      = Error{Code: "NETWORK_ERROR", Message: "network communication failed"}
	ErrTxRejected        = Error{Code: "TX_REJECTED", Message: "transaction rejected by network"}
	ErrNotSupported      = Error{Code: "NOT_SUPPORTED", Message: "operation not supported for this chain"}
)

// Error is a structured error with code for programmatic handling.
type Error struct {
	Code    string
	Message string
	Details map[string]string
}

func (e Error) Error() string {
	return e.Message
}
