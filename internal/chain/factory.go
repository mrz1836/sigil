package chain

import (
	"context"
	"fmt"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// Factory errors.
var (
	// ErrUseDirectClient indicates that chain-specific NewClient should be used.
	// This is a transitional pattern - the CLI currently creates clients directly.
	ErrUseDirectClient = &sigilerr.SigilError{
		Code:     "USE_DIRECT_CLIENT",
		Message:  "use chain-specific NewClient functions directly",
		ExitCode: sigilerr.ExitGeneral,
	}

	// ErrUnsupportedChain indicates the chain is not supported.
	ErrUnsupportedChain = &sigilerr.SigilError{
		Code:     "UNSUPPORTED_CHAIN",
		Message:  "unsupported chain",
		ExitCode: sigilerr.ExitInput,
	}
)

// Factory creates chain clients.
type Factory interface {
	// NewChain creates a chain client for the given ID.
	NewChain(ctx context.Context, id ID, rpcURL string) (Chain, error)
}

// ClientCloser is implemented by clients that need cleanup.
type ClientCloser interface {
	Close()
}

// DefaultFactory is the production chain factory.
type DefaultFactory struct{}

// NewDefaultFactory creates a new default factory.
func NewDefaultFactory() *DefaultFactory {
	return &DefaultFactory{}
}

// NewChain creates a chain client for the given ID.
// The actual client creation is delegated to the specific chain packages
// to avoid import cycles. This factory provides a consistent interface.
func (f *DefaultFactory) NewChain(_ context.Context, id ID, _ string) (Chain, error) {
	switch id {
	case ETH, BSV, BTC, BCH:
		// The actual client creation happens in the CLI layer where we have
		// access to both the chain package and specific implementations.
		// This factory validates the chain ID and provides the interface.
		return nil, ErrUseDirectClient
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedChain, id)
	}
}

// Compile-time interface check
var _ Factory = (*DefaultFactory)(nil)
