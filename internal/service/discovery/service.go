package discovery

import (
	"context"
	"errors"
	"fmt"

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

// bsvNetwork returns the effective BSV network: the per-service override if set,
// otherwise the ConfigProvider's value.
func (s *Service) bsvNetwork() string {
	if s.network != "" {
		return s.network
	}
	return s.config.GetBSVNetwork()
}

// createBSVAdapter creates a BSV client adapter for UTXO refresh operations.
func (s *Service) createBSVAdapter(ctx context.Context) *bsvRefreshAdapter {
	apiKey := s.config.GetBSVAPIKey()
	client := bsv.NewClient(ctx, &bsv.ClientOptions{
		APIKey:  apiKey,
		Network: bsv.Network(s.bsvNetwork()),
	})
	return &bsvRefreshAdapter{client: client}
}

// bsvRefreshAdapter adapts a BSV client to the ChainClient interface.
type bsvRefreshAdapter struct {
	client *bsv.Client
}

// ListUTXOs fetches UTXOs for an address from the BSV chain.
func (a *bsvRefreshAdapter) ListUTXOs(ctx context.Context, address string) ([]chain.UTXO, error) {
	utxos, err := a.client.ListUTXOs(ctx, address)
	if err != nil {
		return nil, err
	}

	// Convert BSV UTXOs to generic chain.UTXO format
	result := make([]chain.UTXO, len(utxos))
	for i, u := range utxos {
		result[i] = chain.UTXO{
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			ScriptPubKey:  u.ScriptPubKey,
			Address:       u.Address,
			Confirmations: u.Confirmations,
		}
	}
	return result, nil
}

// btcNetwork returns the effective BTC network: the per-service override if set,
// otherwise the ConfigProvider's value.
func (s *Service) btcNetwork() string {
	if s.network != "" {
		return s.network
	}
	return s.config.GetBTCNetwork()
}

// createBTCAdapter creates a BTC client adapter for UTXO refresh operations.
func (s *Service) createBTCAdapter(ctx context.Context) *btcRefreshAdapter {
	client := btc.NewClient(ctx, &btc.ClientOptions{
		APIKey:  s.config.GetBTCAPIKey(),
		Network: btc.Network(s.btcNetwork()),
	})
	return &btcRefreshAdapter{client: client}
}

// btcRefreshAdapter adapts a BTC client to the ChainClient interface.
type btcRefreshAdapter struct {
	client *btc.Client
}

// ListUTXOs fetches UTXOs for an address from the BTC chain. btc.Client.ListUTXOs
// already returns chain.UTXO, so no conversion is required.
func (a *btcRefreshAdapter) ListUTXOs(ctx context.Context, address string) ([]chain.UTXO, error) {
	return a.client.ListUTXOs(ctx, address)
}

// RefreshAddress performs chain-specific address refresh.
func (s *Service) refreshAddress(ctx context.Context, chainID chain.ID, address string) error {
	switch chainID {
	case chain.BSV:
		return s.refreshBSV(ctx, address)
	case chain.BTC:
		return s.refreshBTC(ctx, address)
	case chain.ETH:
		return s.refreshETH(ctx, address)
	case chain.BCH, chain.LTC:
		return fmt.Errorf("%w: %s", ErrUnsupportedChain, chainID)
	default:
		return fmt.Errorf("%w: %s", ErrUnknownChain, chainID)
	}
}

// refreshBSV refreshes a BSV address (UTXO scan + balance update).
func (s *Service) refreshBSV(ctx context.Context, address string) error {
	// Step 1: Refresh UTXOs in store
	adapter := s.createBSVAdapter(ctx)
	err := s.utxoStore.RefreshAddress(ctx, address, chain.BSV, adapter)
	if err != nil {
		return fmt.Errorf("refreshing BSV UTXOs: %w", err)
	}

	// Step 2: Update balance cache
	_, err = s.balanceService.FetchBalance(ctx, &balance.FetchRequest{
		ChainID:      chain.BSV,
		Address:      address,
		ForceRefresh: true,
	})
	if err != nil {
		return fmt.Errorf("updating BSV balance: %w", err)
	}

	return nil
}

// refreshBTC refreshes a BTC address (UTXO scan + balance update).
func (s *Service) refreshBTC(ctx context.Context, address string) error {
	// Step 1: Refresh UTXOs in store
	adapter := s.createBTCAdapter(ctx)
	if err := s.utxoStore.RefreshAddress(ctx, address, chain.BTC, adapter); err != nil {
		return fmt.Errorf("refreshing BTC UTXOs: %w", err)
	}

	// Step 2: Update balance cache
	_, err := s.balanceService.FetchBalance(ctx, &balance.FetchRequest{
		ChainID:      chain.BTC,
		Address:      address,
		ForceRefresh: true,
	})
	if err != nil {
		return fmt.Errorf("updating BTC balance: %w", err)
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
