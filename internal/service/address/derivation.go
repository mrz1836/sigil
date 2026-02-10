// Package address provides address derivation, collection, and management services.
package address

import (
	"errors"
	"fmt"

	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// errMaxAddresses is returned when xpub address derivation hits the limit.
var errMaxAddresses = errors.New("would exceed maximum address derivation limit")

// Service provides address derivation and management operations.
type Service struct {
	metadata MetadataProvider
}

// NewService creates a new address service instance.
func NewService(metadata MetadataProvider) *Service {
	return &Service{
		metadata: metadata,
	}
}

// DeriveNext derives the next receive address using either seed or xpub.
// When seed is nil and xpub is available (SIGIL_AGENT_XPUB mode), addresses are
// derived from the xpub without any private key material.
//
// The derived address is automatically added to the wallet's address list.
// The caller must persist the wallet metadata after derivation.
func (s *Service) DeriveNext(req *DerivationRequest) (*wallet.Address, error) {
	// Standard seed-based derivation
	if req.Seed != nil {
		return req.Wallet.DeriveNextReceiveAddress(req.Seed, req.ChainID)
	}

	// xpub read-only derivation
	if req.Xpub != "" {
		nextIndex := req.Wallet.GetReceiveAddressCount(req.ChainID)
		if nextIndex >= wallet.MaxAddressDerivation {
			return nil, fmt.Errorf("%w: %d", errMaxAddresses, wallet.MaxAddressDerivation)
		}
		//nolint:gosec // G115: Safe - validated against MaxAddressDerivation
		addr, err := wallet.DeriveAddressFromXpub(req.Xpub, req.ChainID, wallet.ExternalChain, uint32(nextIndex))
		if err != nil {
			return nil, fmt.Errorf("deriving address from xpub: %w", err)
		}
		req.Wallet.Addresses[req.ChainID] = append(req.Wallet.Addresses[req.ChainID], *addr)
		return addr, nil
	}

	return nil, sigilerr.WithSuggestion(
		sigilerr.ErrAgentXpubInvalid,
		"no seed or xpub available for address derivation",
	)
}
