package balance

import (
	"time"

	"github.com/mrz1836/sigil/internal/chain"
)

// ConfigProvider provides configuration access.
// Minimal interface satisfied by cli.ConfigProvider.
type ConfigProvider interface {
	GetETHRPC() string
	GetETHFallbackRPCs() []string
	GetETHProvider() string
	GetETHEtherscanAPIKey() string
}

// CacheProvider provides balance cache operations.
// Adapter for cache.BalanceCache.
type CacheProvider interface {
	Get(chainID chain.ID, address, token string) (*CacheEntry, bool, time.Duration)
	Set(entry CacheEntry)
}

// AddressMetadataProvider provides address metadata for refresh policy decisions.
// Adapter for utxostore.Store.
type AddressMetadataProvider interface {
	GetAddress(chainID chain.ID, address string) *AddressMetadata
}
