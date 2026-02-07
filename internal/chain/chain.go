// Package chain provides blockchain interface definitions and common utilities.
package chain

import (
	"context"
	"math/big"
)

// ID represents a supported blockchain.
type ID string

// Supported blockchain identifiers.
const (
	ETH ID = "eth"
	BSV ID = "bsv"
	BTC ID = "btc" // Future: Phase 2
	BCH ID = "bch" // Future: Phase 2
)

// BIP44 coin types for derivation paths.
const (
	CoinTypeETH uint32 = 60
	CoinTypeBSV uint32 = 236
	CoinTypeBTC uint32 = 0
	CoinTypeBCH uint32 = 145
)

// DerivationPath returns the BIP44 derivation path prefix for a chain.
func (id ID) DerivationPath() string {
	switch id {
	case ETH:
		return "m/44'/60'/0'"
	case BSV:
		return "m/44'/236'/0'"
	case BTC:
		return "m/44'/0'/0'"
	case BCH:
		return "m/44'/145'/0'"
	default:
		return ""
	}
}

// CoinType returns the BIP44 coin type for a chain.
func (id ID) CoinType() uint32 {
	switch id {
	case ETH:
		return CoinTypeETH
	case BSV:
		return CoinTypeBSV
	case BTC:
		return CoinTypeBTC
	case BCH:
		return CoinTypeBCH
	default:
		return 0
	}
}

// String returns the chain identifier string.
func (id ID) String() string {
	return string(id)
}

// IsValid returns true if the chain ID is a known chain.
func (id ID) IsValid() bool {
	switch id {
	case ETH, BSV, BTC, BCH:
		return true
	default:
		return false
	}
}

// IsMVP returns true if the chain is supported in MVP (Phase 1).
func (id ID) IsMVP() bool {
	switch id {
	case ETH, BSV:
		return true
	case BTC, BCH:
		return false
	default:
		return false
	}
}

// DustLimit returns the minimum output value in satoshis for UTXO-based chains.
// BSV removed dust limits in 2018, so 1 satoshi is the minimum valid output.
// BTC/BCH use the standard 546 satoshi dust limit.
// ETH uses gas instead of dust limits, so returns 0.
func (id ID) DustLimit() uint64 {
	switch id {
	case BSV:
		return 1 // BSV removed dust limit - 1 sat minimum for safety
	case BTC, BCH:
		return 546 // Standard dust limit
	case ETH:
		return 0 // ETH uses gas, not dust limits
	default:
		return 0
	}
}

// ParseChainID parses a string into a ChainID.
func ParseChainID(s string) (ID, bool) {
	id := ID(s)
	return id, id.IsValid()
}

// Identifier provides chain identification.
type Identifier interface {
	// ID returns the chain identifier.
	ID() ID
}

// BalanceReader provides balance querying capabilities.
type BalanceReader interface {
	// GetBalance retrieves the native token balance for an address.
	// Returns balance as a big.Int in the smallest unit (wei, satoshis).
	GetBalance(ctx context.Context, address string) (*big.Int, error)
}

// AddressValidator provides address validation.
type AddressValidator interface {
	// ValidateAddress checks if an address is valid for this chain.
	ValidateAddress(address string) error
}

// Reader combines read-only chain operations.
type Reader interface {
	Identifier
	BalanceReader
	AddressValidator
}

// FeeEstimator provides fee estimation capabilities.
type FeeEstimator interface {
	// EstimateFee estimates the fee for a transaction.
	// For ETH: returns gas price * estimated gas.
	// For BSV: returns fee rate * estimated tx size.
	EstimateFee(ctx context.Context, from, to string, amount *big.Int) (*big.Int, error)
}

// TransactionSender provides transaction sending capabilities.
type TransactionSender interface {
	FeeEstimator

	// Send builds, signs, and broadcasts a transaction.
	Send(ctx context.Context, req SendRequest) (*TransactionResult, error)
}

// AmountFormatter provides amount formatting and parsing.
// The standalone ParseDecimalAmount and FormatDecimalAmount functions in
// amount.go can be used directly as an alternative to this interface.
type AmountFormatter interface {
	// FormatAmount converts a big.Int to human-readable string with proper decimals.
	FormatAmount(amount *big.Int) string

	// ParseAmount converts a human-readable string to big.Int.
	ParseAmount(amount string) (*big.Int, error)
}

// Chain defines the full interface that all blockchain implementations must satisfy.
// This combines all sub-interfaces for backwards compatibility.
// Prefer using the smaller interfaces (Reader, TransactionSender) when possible.
type Chain interface {
	Reader
	TransactionSender
	AmountFormatter
}

// TokenChain extends Chain with ERC-20 token operations.
// Only Ethereum-like chains implement this interface.
type TokenChain interface {
	Chain

	// GetTokenBalance retrieves an ERC-20 token balance.
	GetTokenBalance(ctx context.Context, address, tokenAddress string) (*big.Int, error)
}

// UTXOChain extends Chain with UTXO-specific operations.
type UTXOChain interface {
	Chain

	// ListUTXOs returns unspent transaction outputs for an address.
	ListUTXOs(ctx context.Context, address string) ([]UTXO, error)

	// SelectUTXOs chooses UTXOs to fund a transaction of the given amount.
	SelectUTXOs(utxos []UTXO, amount, feeRate uint64) (selected []UTXO, change uint64, err error)
}

// SendRequest contains parameters for sending a transaction.
type SendRequest struct {
	From          string   // Sender address
	To            string   // Recipient address
	Amount        *big.Int // Value in smallest units
	PrivateKey    []byte   // Signing key (will be zeroed after use)
	Token         string   // ERC-20 token address (ETH only, empty for native)
	GasLimit      uint64   // Optional gas limit override (ETH only)
	FeeRate       uint64   // Optional fee rate override (satoshis per byte)
	ChangeAddress string   // Optional change address (BSV only, defaults to From)
}

// TransactionResult contains the outcome of a broadcast transaction.
type TransactionResult struct {
	Hash     string `json:"hash"`                // Transaction hash
	From     string `json:"from"`                // Sender address
	To       string `json:"to"`                  // Recipient address
	Amount   string `json:"amount"`              // Transferred amount (human-readable)
	Token    string `json:"token,omitempty"`     // Token symbol if applicable
	Fee      string `json:"fee"`                 // Fee paid (human-readable)
	GasUsed  uint64 `json:"gas_used"`            // ETH-specific gas consumption
	GasPrice string `json:"gas_price,omitempty"` // ETH-specific gas price
	Status   string `json:"status"`              // "pending" after broadcast
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

// SupportedChains returns the list of MVP-supported chain IDs.
func SupportedChains() []ID {
	return []ID{ETH, BSV}
}

// AllChains returns all known chain IDs.
func AllChains() []ID {
	return []ID{ETH, BSV, BTC, BCH}
}
