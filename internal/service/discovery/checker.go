package discovery

import (
	"context"
	"fmt"
	"strings"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/utxostore"
)

// CheckAddress checks an address for activity and returns balance/UTXO information.
// For BSV: refreshes UTXOs and returns balance + UTXO list.
// For ETH: fetches balance only (account-based chain has no UTXOs).
func (s *Service) CheckAddress(ctx context.Context, req *CheckRequest) (*CheckResult, error) {
	switch req.ChainID {
	case chain.BSV, chain.BTC:
		return s.checkUTXOChain(ctx, req.ChainID, req.Address)
	case chain.ETH:
		return s.checkETH(ctx, req.Address)
	case chain.BCH, chain.LTC:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedChain, req.ChainID)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownChain, req.ChainID)
	}
}

// checkUTXOChain checks a Bitcoin-family address by refreshing UTXOs into the
// store and returning its balance + UTXO list. Shared core of the BSV and BTC
// activity checks.
func (s *Service) checkUTXOChain(ctx context.Context, chainID chain.ID, address string) (*CheckResult, error) {
	adapter := s.createUTXOAdapter(ctx, chainID)
	if err := s.utxoStore.RefreshAddress(ctx, address, chainID, adapter); err != nil {
		return nil, fmt.Errorf("refreshing %s address: %w", strings.ToUpper(string(chainID)), err)
	}

	balance := s.utxoStore.GetAddressBalance(chainID, address)
	storeUTXOs := s.utxoStore.GetUTXOs(chainID, address)
	meta := s.utxoStore.GetAddress(chainID, address)

	utxos := make([]UTXO, 0, len(storeUTXOs))
	for _, u := range storeUTXOs {
		utxos = append(utxos, UTXO{
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			Confirmations: u.Confirmations,
		})
	}

	return &CheckResult{
		Address:     address,
		ChainID:     chainID,
		Balance:     balance,
		UTXOs:       utxos,
		HasActivity: meta != nil && meta.HasActivity,
		Label:       getLabel(meta),
	}, nil
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
