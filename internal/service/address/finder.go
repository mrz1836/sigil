package address

import (
	"github.com/mrz1836/sigil/internal/wallet"
)

// FindUnused returns the first receiving address with no activity.
// Returns nil if all addresses have been used.
func (s *Service) FindUnused(req *FindRequest) *wallet.Address {
	addresses := req.Wallet.Addresses[req.ChainID]
	for i := range addresses {
		addr := &addresses[i]
		if s.metadata == nil {
			// No metadata available, return first address
			return addr
		}
		meta := s.metadata.GetAddress(req.ChainID, addr.Address)
		if meta == nil || !meta.HasActivity {
			return addr
		}
	}
	return nil
}
