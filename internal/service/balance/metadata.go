package balance

import (
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
)

// MetadataAdapter adapts utxostore.Store to the AddressMetadataProvider interface.
// This decouples the service from the concrete utxostore implementation.
type MetadataAdapter struct {
	store *utxostore.Store
}

// NewMetadataAdapter creates a new metadata adapter.
func NewMetadataAdapter(store *utxostore.Store) *MetadataAdapter {
	return &MetadataAdapter{store: store}
}

// GetAddress retrieves address metadata from the UTXO store.
func (a *MetadataAdapter) GetAddress(chainID chain.ID, address string) *AddressMetadata {
	storeMeta := a.store.GetAddress(chainID, address)
	if storeMeta == nil {
		return nil
	}

	// Convert utxostore.AddressMetadata to service AddressMetadata
	return &AddressMetadata{
		ChainID:     storeMeta.ChainID,
		Address:     storeMeta.Address,
		HasActivity: storeMeta.HasActivity,
		LastScanned: storeMeta.LastScanned,
	}
}
