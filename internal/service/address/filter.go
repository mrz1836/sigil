package address

import (
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/wallet"
)

// Collect gathers addresses from a wallet based on the provided filters.
// Returns a list of AddressInfo with metadata (labels, activity status).
// Balance and unconfirmed fields are left empty - caller should enrich with balance service.
//
//nolint:gocognit // Complex iteration logic for multi-chain address collection with filtering
func (s *Service) Collect(req *CollectionRequest) []AddressInfo {
	var results []AddressInfo

	// Determine which chains to collect
	chains := req.Wallet.EnabledChains
	if req.ChainFilter != "" {
		chains = []chain.ID{req.ChainFilter}
	}

	// Collect addresses from each chain
	for _, chainID := range chains {
		// Collect receive addresses
		if req.TypeFilter == Receive || req.TypeFilter == AllTypes {
			for _, addr := range req.Wallet.Addresses[chainID] {
				info := s.buildAddressInfo(Receive, &addr, chainID)
				results = append(results, info)
			}
		}

		// Collect change addresses
		if req.TypeFilter == Change || req.TypeFilter == AllTypes {
			if req.Wallet.ChangeAddresses != nil {
				for _, addr := range req.Wallet.ChangeAddresses[chainID] {
					info := s.buildAddressInfo(Change, &addr, chainID)
					results = append(results, info)
				}
			}
		}
	}

	return results
}

// buildAddressInfo creates an AddressInfo from a wallet address.
// Enriches with metadata (label, activity) if metadata provider is available.
func (s *Service) buildAddressInfo(addrType AddressType, addr *wallet.Address, chainID chain.ID) AddressInfo {
	info := AddressInfo{
		Type:    addrType,
		Index:   addr.Index,
		Address: addr.Address,
		Path:    addr.Path,
		ChainID: chainID,
		// Balance and Unconfirmed are populated after network fetch by caller
	}

	// Get metadata from provider (label and HasActivity)
	if s.metadata != nil {
		if meta := s.metadata.GetAddress(chainID, addr.Address); meta != nil {
			info.Label = meta.Label
			info.HasActivity = meta.HasActivity
		}
	}

	return info
}

// FilterUsage filters addresses by usage status.
// Used for implementing --used and --unused CLI flags.
func FilterUsage(addresses []AddressInfo, onlyUsed, onlyUnused bool) []AddressInfo {
	if !onlyUsed && !onlyUnused {
		return addresses // No filter
	}

	var filtered []AddressInfo
	for _, addr := range addresses {
		if onlyUsed && !addr.HasActivity {
			continue
		}
		if onlyUnused && addr.HasActivity {
			continue
		}
		filtered = append(filtered, addr)
	}
	return filtered
}
