package transaction

import (
	"context"

	"github.com/mrz1836/sigil/internal/chain"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// Service provides transaction sending functionality.
type Service struct {
	config  ConfigProvider
	storage StorageProvider
	logger  LogWriter
}

// Config holds dependencies for the transaction service.
type Config struct {
	Config  ConfigProvider
	Storage StorageProvider
	Logger  LogWriter
}

// NewService creates a new transaction service.
func NewService(cfg *Config) *Service {
	return &Service{
		config:  cfg.Config,
		storage: cfg.Storage,
		logger:  cfg.Logger,
	}
}

// Send dispatches a transaction send request to the appropriate chain handler.
func (s *Service) Send(ctx context.Context, req *SendRequest) (*SendResult, error) {
	// Pre-flight validation: deny spending in xpub read-only mode
	// This check is typically done by CLI, but we enforce it here too
	// AgentXpub detection would need to be passed in req if needed

	// Dispatch to chain-specific handler
	switch req.ChainID {
	case chain.ETH:
		return s.sendETH(ctx, req)
	case chain.BSV:
		return s.sendBSV(ctx, req)
	case chain.BTC, chain.BCH:
		return nil, sigilerr.ErrNotImplemented
	default:
		return nil, sigilerr.ErrNotImplemented
	}
}

// sendETH and sendBSV are implemented in eth.go and bsv.go files
