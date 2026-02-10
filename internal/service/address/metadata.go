package address

import (
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
)

// MetadataAdapter adapts a UTXO store to the MetadataProvider interface.
// This decouples the address service from the concrete UTXO store implementation.
type MetadataAdapter struct {
	store *utxostore.Store
}

// NewMetadataAdapter creates a new metadata adapter wrapping a UTXO store.
func NewMetadataAdapter(store *utxostore.Store) *MetadataAdapter {
	return &MetadataAdapter{store: store}
}

// GetAddress retrieves address metadata from the UTXO store.
func (a *MetadataAdapter) GetAddress(chainID chain.ID, address string) *AddressMetadata {
	if a.store == nil {
		return nil
	}

	storeMeta := a.store.GetAddress(chainID, address)
	if storeMeta == nil {
		return nil
	}

	return &AddressMetadata{
		HasActivity: storeMeta.HasActivity,
		Label:       storeMeta.Label,
	}
}
