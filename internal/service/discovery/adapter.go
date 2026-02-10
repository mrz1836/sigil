// Package discovery provides UTXO scanning and address activity detection services.
package discovery

import (
	"context"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
)

// UTXOStoreAdapter adapts a utxostore.Store to the UTXOProvider interface.
type UTXOStoreAdapter struct {
	store *utxostore.Store
}

// NewUTXOStoreAdapter creates a new UTXO store adapter.
func NewUTXOStoreAdapter(store *utxostore.Store) *UTXOStoreAdapter {
	return &UTXOStoreAdapter{store: store}
}

// RefreshAddress refreshes an address.
func (a *UTXOStoreAdapter) RefreshAddress(ctx context.Context, address string, chainID chain.ID, adapter ChainClient) error {
	// Convert our ChainClient to utxostore.ChainClient
	utxoAdapter := &utxoChainClientAdapter{client: adapter}

	_, err := a.store.RefreshAddress(ctx, address, chainID, utxoAdapter)
	return err
}

// GetAddressBalance gets the balance for an address.
func (a *UTXOStoreAdapter) GetAddressBalance(chainID chain.ID, address string) uint64 {
	return a.store.GetAddressBalance(chainID, address)
}

// GetUTXOs gets UTXOs for an address.
func (a *UTXOStoreAdapter) GetUTXOs(chainID chain.ID, address string) []*utxostore.StoredUTXO {
	return a.store.GetUTXOs(chainID, address)
}

// GetAddress gets address metadata.
func (a *UTXOStoreAdapter) GetAddress(chainID chain.ID, address string) *utxostore.AddressMetadata {
	return a.store.GetAddress(chainID, address)
}

// utxoChainClientAdapter adapts our ChainClient to utxostore.ChainClient.
type utxoChainClientAdapter struct {
	client ChainClient
}

// ListUTXOs implements utxostore.ChainClient.
func (a *utxoChainClientAdapter) ListUTXOs(ctx context.Context, address string) ([]chain.UTXO, error) {
	return a.client.ListUTXOs(ctx, address)
}
