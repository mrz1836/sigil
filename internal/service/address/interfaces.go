package address

import (
	"github.com/mrz1836/sigil/internal/chain"
)

// MetadataProvider provides address metadata (labels, activity status).
// Typically backed by a UTXO store or similar persistence layer.
type MetadataProvider interface {
	GetAddress(chainID chain.ID, address string) *AddressMetadata
}

// AddressMetadata contains metadata about an address.
//
//nolint:revive // AddressMetadata name is intentionally explicit for clarity when used as address.AddressMetadata
type AddressMetadata struct {
	HasActivity bool
	Label       string
}
