package transaction

import (
	"math/big"

	"github.com/mrz1836/sigil/internal/agent"
	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
)

// ConfigProvider provides configuration values needed for transactions.
type ConfigProvider interface {
	GetHome() string
	GetETHRPC() string
	GetETHFallbackRPCs() []string
	GetBSVAPIKey() string
	GetBSVFeeStrategy() string
	GetBSVMinMiners() int
}

// CacheProvider provides balance cache operations.
type CacheProvider interface {
	Load() (*cache.BalanceCache, error)
	Save(bc *cache.BalanceCache) error
}

// UTXOProvider provides UTXO store operations.
type UTXOProvider interface {
	Load() error
	Save() error
	IsSpent(chainID chain.ID, txid string, vout uint32) bool
	AddUTXO(utxo *utxostore.StoredUTXO)
	MarkSpent(chainID chain.ID, txid string, vout uint32, spentTxID string) bool
}

// StorageProvider provides wallet metadata access.
type StorageProvider interface {
	UpdateMetadata(w *wallet.Wallet) error
}

// LogWriter provides logging operations.
type LogWriter interface {
	Debug(format string, args ...any)
	Error(format string, args ...any)
}

// AgentCredential represents agent mode credentials for policy enforcement.
type AgentCredential interface {
	HasChain(chainID chain.ID) bool
}

// AgentPolicy provides agent policy enforcement operations.
type AgentPolicy interface {
	ValidateTransaction(cred *agent.Credential, chainID chain.ID, to string, amount *big.Int) error
	CheckDailyLimit(counterPath, token string, cred *agent.Credential, chainID chain.ID, amount *big.Int) error
	RecordSpend(counterPath, token string, chainID chain.ID, amount *big.Int) error
}
