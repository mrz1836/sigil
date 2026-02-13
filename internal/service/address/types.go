package address

import (
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
)

// AddressInfo holds display information for an address.
// This is a framework-agnostic representation suitable for CLI, API, or TUI display.
type AddressInfo struct {
	ChainID     chain.ID
	Address     string
	Type        AddressType // Receive or Change
	Index       uint32
	Path        string
	Label       string
	Balance     string // Formatted confirmed balance (e.g. "0.00070422") or ""
	Unconfirmed string // Formatted unconfirmed delta (e.g. "-0.00070422") or ""
	Stale       bool   // True if balance data is stale
	HasActivity bool   // True if address has been used on-chain
}

// AddressType represents the type of address (receive or change).
type AddressType int

const (
	// Receive is an external (receiving) address.
	Receive AddressType = iota
	// Change is an internal (change) address.
	Change
	// AllTypes is a filter that includes both receive and change addresses.
	AllTypes
)

// String returns the string representation of the address type.
func (a AddressType) String() string {
	switch a {
	case Receive:
		return "receive"
	case Change:
		return "change"
	case AllTypes:
		return "all"
	default:
		return "unknown"
	}
}

// CollectionRequest specifies parameters for collecting addresses from a wallet.
type CollectionRequest struct {
	Wallet      *wallet.Wallet
	ChainFilter chain.ID    // Empty = all chains
	TypeFilter  AddressType // Receive, Change, or AllTypes
}

// DerivationRequest specifies parameters for deriving a new address.
type DerivationRequest struct {
	Wallet  *wallet.Wallet
	Seed    []byte // May be nil for xpub mode
	ChainID chain.ID
	Xpub    string // Optional: for xpub-based derivation (read-only mode)
}

// FindRequest specifies parameters for finding an unused address.
type FindRequest struct {
	Wallet  *wallet.Wallet
	ChainID chain.ID
}
