package discovery

import (
	"context"
	"fmt"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
)

// CheckAddress checks an address for activity and returns balance/UTXO information.
// For BSV: refreshes UTXOs and returns balance + UTXO list.
// For ETH: fetches balance only (account-based chain has no UTXOs).
func (s *Service) CheckAddress(ctx context.Context, req *CheckRequest) (*CheckResult, error) {
	switch req.ChainID {
	case chain.BSV:
		return s.checkBSV(ctx, req.Address)
	case chain.ETH:
		return s.checkETH(ctx, req.Address)
	case chain.BTC, chain.BCH:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedChain, req.ChainID)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownChain, req.ChainID)
	}
}

// checkBSV checks a BSV address by refreshing UTXOs and returning results.
func (s *Service) checkBSV(ctx context.Context, address string) (*CheckResult, error) {
	// Refresh UTXOs
	adapter := s.createBSVAdapter(ctx)
	err := s.utxoStore.RefreshAddress(ctx, address, chain.BSV, adapter)
	if err != nil {
		return nil, fmt.Errorf("refreshing BSV address: %w", err)
	}

	// Get balance and UTXOs from store
	balance := s.utxoStore.GetAddressBalance(chain.BSV, address)
	storeUTXOs := s.utxoStore.GetUTXOs(chain.BSV, address)
	meta := s.utxoStore.GetAddress(chain.BSV, address)

	// Convert UTXOs to service type
	utxos := make([]UTXO, 0, len(storeUTXOs))
	for _, u := range storeUTXOs {
		utxos = append(utxos, UTXO{
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			Confirmations: u.Confirmations,
		})
	}

	result := &CheckResult{
		Address:     address,
		ChainID:     chain.BSV,
		Balance:     balance,
		UTXOs:       utxos,
		HasActivity: meta != nil && meta.HasActivity,
		Label:       getLabel(meta),
	}

	return result, nil
}

// checkETH checks an ETH address by fetching balance (no UTXOs for account-based chains).
func (s *Service) checkETH(_ context.Context, address string) (*CheckResult, error) {
	// ETH is account-based, no UTXO refresh needed
	// Balance check is handled separately in CLI via balance service

	result := &CheckResult{
		Address:     address,
		ChainID:     chain.ETH,
		Balance:     0, // Populated by caller via balance service
		UTXOs:       nil,
		HasActivity: false, // Determined by balance check
		Label:       "",
	}

	return result, nil
}

// getLabel extracts label from metadata, returning empty string if nil.
func getLabel(meta *utxostore.AddressMetadata) string {
	if meta == nil {
		return ""
	}
	return meta.Label
}
