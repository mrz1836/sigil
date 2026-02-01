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

// ParseChainID parses a string into a ChainID.
func ParseChainID(s string) (ID, bool) {
	id := ID(s)
	return id, id.IsValid()
}

// Chain defines the interface that all blockchain implementations must satisfy.
type Chain interface {
	// ID returns the chain identifier.
	ID() ID

	// GetBalance retrieves the native token balance for an address.
	// Returns balance as a big.Int in the smallest unit (wei, satoshis).
	GetBalance(ctx context.Context, address string) (*big.Int, error)

	// EstimateFee estimates the fee for a transaction.
	// For ETH: returns gas price * estimated gas.
	// For BSV: returns fee rate * estimated tx size.
	EstimateFee(ctx context.Context, from, to string, amount *big.Int) (*big.Int, error)

	// Send builds, signs, and broadcasts a transaction.
	Send(ctx context.Context, req SendRequest) (*TransactionResult, error)

	// ValidateAddress checks if an address is valid for this chain.
	ValidateAddress(address string) error

	// FormatAmount converts a big.Int to human-readable string with proper decimals.
	FormatAmount(amount *big.Int) string

	// ParseAmount converts a human-readable string to big.Int.
	ParseAmount(amount string) (*big.Int, error)
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
	From       string   // Sender address
	To         string   // Recipient address
	Amount     *big.Int // Value in smallest units
	PrivateKey []byte   // Signing key (will be zeroed after use)
	Token      string   // ERC-20 token address (ETH only, empty for native)
	GasLimit   uint64   // Optional gas limit override (ETH only)
	FeeRate    uint64   // Optional fee rate override (satoshis per byte)
}

// TransactionResult contains the outcome of a broadcast transaction.
type TransactionResult struct {
	Hash     string // Transaction hash
	From     string // Sender address
	To       string // Recipient address
	Amount   string // Transferred amount (human-readable)
	Token    string // Token symbol if applicable
	Fee      string // Fee paid (human-readable)
	GasUsed  uint64 // ETH-specific gas consumption
	GasPrice string // ETH-specific gas price
	Status   string // "pending" after broadcast
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
