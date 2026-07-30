package discovery

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/chain/btc"
	"github.com/mrz1836/sigil/internal/service/balance"
)

var (
	// ErrUnsupportedChain is returned when a chain is not supported for discovery operations.
	ErrUnsupportedChain = errors.New("unsupported chain")
	// ErrUnknownChain is returned when a chain ID is not recognized.
	ErrUnknownChain = errors.New("unknown chain")
)

// Service provides address discovery, refresh, and activity checking operations.
type Service struct {
	utxoStore      UTXOProvider
	balanceService BalanceProvider
	config         ConfigProvider
	network        string // optional per-wallet override of the config network
}

// Config contains dependencies for creating a discovery service.
type Config struct {
	UTXOStore      UTXOProvider
	BalanceService BalanceProvider
	Config         ConfigProvider
	// Network optionally overrides the ConfigProvider's BSV/BTC network so a
	// wallet's stamped network governs discovery. Empty falls back to config.
	Network string
}

// NewService creates a new discovery service instance.
func NewService(cfg *Config) *Service {
	return &Service{
		utxoStore:      cfg.UTXOStore,
		balanceService: cfg.BalanceService,
		config:         cfg.Config,
		network:        cfg.Network,
	}
}

// networkFor returns the effective network for a Bitcoin-family chain: the
// per-service override if set, otherwise the ConfigProvider's value.
func (s *Service) networkFor(chainID chain.ID) string {
	if s.network != "" {
		return s.network
	}
	if chainID == chain.BTC {
		return s.config.GetBTCNetwork()
	}
	return s.config.GetBSVNetwork()
}

// utxoLister is the ListUTXOs surface shared by the BSV and BTC clients (both
// return []chain.UTXO).
type utxoLister interface {
	ListUTXOs(ctx context.Context, address string) ([]chain.UTXO, error)
}

// refreshAdapter adapts a Bitcoin-family client to the utxostore ChainClient
// interface. The BSV and BTC clients differ only in construction; both list
// UTXOs as []chain.UTXO, so one adapter serves both.
type refreshAdapter struct {
	client utxoLister
}

// ListUTXOs fetches UTXOs for an address from the underlying chain client.
func (a *refreshAdapter) ListUTXOs(ctx context.Context, address string) ([]chain.UTXO, error) {
	return a.client.ListUTXOs(ctx, address)
}

// createUTXOAdapter builds the refresh adapter for a Bitcoin-family chain.
func (s *Service) createUTXOAdapter(ctx context.Context, chainID chain.ID) *refreshAdapter {
	if chainID == chain.BTC {
		return &refreshAdapter{client: btc.NewClient(ctx, &btc.ClientOptions{
			APIKey:  s.config.GetBTCAPIKey(),
			Network: btc.Network(s.networkFor(chainID)),
		})}
	}
	return &refreshAdapter{client: bsv.NewClient(ctx, &bsv.ClientOptions{
		APIKey:  s.config.GetBSVAPIKey(),
		Network: bsv.Network(s.networkFor(chainID)),
	})}
}

// RefreshAddress performs chain-specific address refresh.
func (s *Service) refreshAddress(ctx context.Context, chainID chain.ID, address string) error {
	switch chainID {
	case chain.BSV, chain.BTC:
		return s.refreshUTXOChain(ctx, chainID, address)
	case chain.ETH:
		return s.refreshETH(ctx, address)
	case chain.BCH, chain.LTC:
		return fmt.Errorf("%w: %s", ErrUnsupportedChain, chainID)
	default:
		return fmt.Errorf("%w: %s", ErrUnknownChain, chainID)
	}
}

// refreshUTXOChain refreshes a Bitcoin-family address: it rescans UTXOs into the
// local store and then force-updates the balance cache.
func (s *Service) refreshUTXOChain(ctx context.Context, chainID chain.ID, address string) error {
	// Step 1: Refresh UTXOs in store.
	label := strings.ToUpper(string(chainID))
	adapter := s.createUTXOAdapter(ctx, chainID)
	if err := s.utxoStore.RefreshAddress(ctx, address, chainID, adapter); err != nil {
		return fmt.Errorf("refreshing %s UTXOs: %w", label, err)
	}

	// Step 2: Update balance cache.
	if _, err := s.balanceService.FetchBalance(ctx, &balance.FetchRequest{
		ChainID:      chainID,
		Address:      address,
		ForceRefresh: true,
	}); err != nil {
		return fmt.Errorf("updating %s balance: %w", label, err)
	}

	return nil
}

// refreshETH refreshes an ETH address (balance update only - account-based chain).
func (s *Service) refreshETH(ctx context.Context, address string) error {
	_, err := s.balanceService.FetchBalance(ctx, &balance.FetchRequest{
		ChainID:      chain.ETH,
		Address:      address,
		ForceRefresh: true,
	})
	if err != nil {
		return fmt.Errorf("updating ETH balance: %w", err)
	}

	return nil
}
