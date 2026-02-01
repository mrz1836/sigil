package chain

import (
	"context"
	"fmt"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// Factory errors.
var (
	// ErrUnsupportedChain indicates the chain is not supported.
	ErrUnsupportedChain = &sigilerr.SigilError{
		Code:     "UNSUPPORTED_CHAIN",
		Message:  "unsupported chain",
		ExitCode: sigilerr.ExitInput,
	}
)

// Factory creates chain clients.
// Due to Go's import cycle restrictions, the base chain package cannot
// directly import eth/bsv packages. Use ConfigurableFactory in the CLI layer
// or create clients directly with eth.NewClient()/bsv.NewClient().
type Factory interface {
	// NewChain creates a chain client for the given ID.
	NewChain(ctx context.Context, id ID, rpcURL string) (Chain, error)
}

// ClientCloser is implemented by clients that need cleanup.
type ClientCloser interface {
	Close()
}

// Creator is a function type that creates a Chain client.
// This allows registering chain-specific client constructors without import cycles.
type Creator func(ctx context.Context, rpcURL string) (Chain, error)

// ConfigurableFactory is a factory that can have chain creators registered.
type ConfigurableFactory struct {
	creators map[ID]Creator
}

// NewConfigurableFactory creates a new configurable factory.
func NewConfigurableFactory() *ConfigurableFactory {
	return &ConfigurableFactory{
		creators: make(map[ID]Creator),
	}
}

// Register adds a chain creator for the given ID.
func (f *ConfigurableFactory) Register(id ID, creator Creator) {
	f.creators[id] = creator
}

// NewChain creates a chain client using the registered creator.
func (f *ConfigurableFactory) NewChain(ctx context.Context, id ID, rpcURL string) (Chain, error) {
	creator, ok := f.creators[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedChain, id)
	}
	return creator(ctx, rpcURL)
}

// IsSupported returns true if the chain ID has a registered creator.
func (f *ConfigurableFactory) IsSupported(id ID) bool {
	_, ok := f.creators[id]
	return ok
}

// SupportedChains returns all registered chain IDs.
func (f *ConfigurableFactory) SupportedChains() []ID {
	chains := make([]ID, 0, len(f.creators))
	for id := range f.creators {
		chains = append(chains, id)
	}
	return chains
}

// Compile-time interface check
var _ Factory = (*ConfigurableFactory)(nil)

// DefaultFactory is a simple factory that validates chain IDs.
// For actual client creation, use ConfigurableFactory with registered creators,
// or create clients directly using eth.NewClient()/bsv.NewClient().
type DefaultFactory struct{}

// NewDefaultFactory creates a new default factory.
func NewDefaultFactory() *DefaultFactory {
	return &DefaultFactory{}
}

// ErrValidationOnly is returned when DefaultFactory is used.
// DefaultFactory validates chain IDs but does not create clients.
var ErrValidationOnly = fmt.Errorf("chain ID valid; use ConfigurableFactory or direct client creation")

// NewChain validates the chain ID but does not create clients.
// Use ConfigurableFactory or create clients directly in the CLI layer.
func (f *DefaultFactory) NewChain(_ context.Context, id ID, _ string) (Chain, error) {
	if !IsSupportedChain(id) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedChain, id)
	}
	// Return sentinel error indicating validation passed but no client created
	return nil, ErrValidationOnly
}

// IsSupportedChain returns true if the chain ID is supported by sigil.
func IsSupportedChain(id ID) bool {
	switch id {
	case ETH, BSV:
		return true
	case BTC, BCH:
		// Planned but not yet implemented
		return false
	default:
		return false
	}
}

// Compile-time interface check
var _ Factory = (*DefaultFactory)(nil)
