package wallet

import (
	"fmt"
	"os"

	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// Service provides wallet loading and management operations without CLI dependencies.
type Service struct {
	storage    StorageProvider
	sessionMgr SessionManager
	config     ConfigProvider
	logger     LogWriter
}

// Config contains dependencies for creating a wallet service.
type Config struct {
	Storage    StorageProvider
	SessionMgr SessionManager
	Config     ConfigProvider
	Logger     LogWriter
}

// NewService creates a new wallet service instance.
func NewService(cfg *Config) *Service {
	return &Service{
		storage:    cfg.Storage,
		sessionMgr: cfg.SessionMgr,
		config:     cfg.Config,
		logger:     cfg.Logger,
	}
}

// ValidateExists checks if a wallet exists in storage.
// Returns an error with helpful suggestion if wallet is not found.
func (s *Service) ValidateExists(name string) error {
	exists, existsErr := s.storage.Exists(name)
	if existsErr != nil {
		return existsErr
	}
	if !exists {
		return sigilerr.WithSuggestion(
			wallet.ErrWalletNotFound,
			fmt.Sprintf("wallet '%s' not found. List wallets with: sigil wallet list", name),
		)
	}
	return nil
}

// List returns all wallet names from storage.
func (s *Service) List() ([]string, error) {
	return s.storage.List()
}

// LoadMetadata loads wallet metadata without requiring authentication.
// This is useful for read-only operations that don't need the seed.
func (s *Service) LoadMetadata(name string) (*wallet.Wallet, error) {
	if err := s.ValidateExists(name); err != nil {
		return nil, err
	}
	return s.storage.LoadMetadata(name)
}

// CheckAgentToken checks if an agent token is configured in the environment.
func CheckAgentToken() string {
	return os.Getenv(config.EnvAgentToken)
}

// CheckAgentXpub checks if an agent xpub is configured in the environment.
func CheckAgentXpub() string {
	return os.Getenv(config.EnvAgentXpub)
}
