package discovery

import (
	"context"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/service/balance"
	"github.com/mrz1836/sigil/internal/utxostore"
)

// UTXOProvider provides UTXO storage and refresh capabilities.
type UTXOProvider interface {
	RefreshAddress(ctx context.Context, address string, chainID chain.ID, adapter ChainClient) error
	GetAddressBalance(chainID chain.ID, address string) uint64
	GetUTXOs(chainID chain.ID, address string) []*utxostore.StoredUTXO
	GetAddress(chainID chain.ID, address string) *utxostore.AddressMetadata
}

// ChainClient provides chain-specific operations for UTXO chains.
type ChainClient interface {
	ListUTXOs(ctx context.Context, address string) ([]chain.UTXO, error)
}

// BalanceProvider provides balance fetching capabilities.
type BalanceProvider interface {
	FetchBalance(ctx context.Context, req *balance.FetchRequest) (*balance.FetchResult, error)
}

// ConfigProvider provides configuration access.
type ConfigProvider interface {
	GetBSVAPIKey() string
	GetETHEtherscanAPIKey() string
}
