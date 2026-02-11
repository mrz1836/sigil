// Package transaction provides transaction sending functionality for ETH and BSV chains.
package transaction

import (
	"context"
	"fmt"
	"math/big"
	"path/filepath"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// sendBSV handles the complete Bitcoin SV transaction flow.
// Migrated from cli/tx.go lines 398-613
//
//nolint:gocognit,gocyclo,nestif // Transaction flow is inherently complex (migrated from CLI)
func (s *Service) sendBSV(ctx context.Context, req *SendRequest) (*SendResult, error) {
	// Validate BSV address
	if err := bsv.ValidateBase58CheckAddress(req.To); err != nil {
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidAddress,
			fmt.Sprintf("invalid BSV address: %s", req.To),
		)
	}

	// Create BSV client
	opts := &bsv.ClientOptions{
		APIKey:      s.config.GetBSVAPIKey(),
		Logger:      s.logger,
		FeeStrategy: bsv.FeeStrategy(s.config.GetBSVFeeStrategy()),
		MinMiners:   s.config.GetBSVMinMiners(),
	}
	client := bsv.NewClient(ctx, opts)

	// Load local UTXO store for spent-UTXO filtering and post-broadcast marking
	walletPath := filepath.Join(s.config.GetHome(), "wallets", req.Wallet)
	utxoStore := utxostore.New(walletPath)
	if err := utxoStore.Load(); err != nil {
		if s.logger != nil {
			s.logger.Error("bsv send: failed to load utxo store: %v", err)
		}
		// Non-fatal: proceed without local filtering (API-only UTXOs)
		utxoStore = nil
	}

	sweepAll := req.SweepAll()
	if s.logger != nil {
		s.logger.Debug("bsv send: to=%s amount=%s sweep=%v", req.To, req.AmountStr, sweepAll)
	}

	// Parse amount (skip for sweep — amount is calculated from balance minus fees)
	var amount *big.Int
	if !sweepAll {
		var err error
		amount, err = client.ParseAmount(req.AmountStr)
		if err != nil {
			return nil, sigilerr.WithSuggestion(
				sigilerr.ErrInvalidInput,
				fmt.Sprintf("invalid amount: %s", req.AmountStr),
			)
		}
	}

	// Get fee quote
	feeQuote, err := client.GetFeeQuote(ctx)
	if err != nil {
		// Use default if fee quote fails
		feeQuote = &bsv.FeeQuote{StandardRate: bsv.DefaultFeeRate}
	}
	if s.logger != nil {
		s.logger.Debug("bsv send: fee rate=%d sat/KB source=%s", feeQuote.StandardRate, feeQuote.Source)
	}

	// Aggregate UTXOs from ALL wallet addresses for this chain
	allUTXOs, utxoErr := aggregateBSVUTXOs(ctx, client, req.Addresses)
	if utxoErr != nil {
		if s.logger != nil {
			s.logger.Error("bsv send: utxo aggregation failed: %v", utxoErr)
		}
		return nil, fmt.Errorf("listing UTXOs: %w", utxoErr)
	}
	// Filter out UTXOs that are known-spent in the local store (prevents double-spend)
	if utxoStore != nil {
		allUTXOs = filterSpentBSVUTXOs(allUTXOs, utxoStore)
	}

	// Validate UTXOs if requested (for sweep transactions)
	if req.ValidateUTXOs && sweepAll {
		bulkOpts := &bsv.BulkOperationsOptions{
			RateLimit: 3.0,
			RateBurst: 5,
		}
		bulkOps := bsv.NewBulkOperations(client.GetWOCClient(), bulkOpts)

		// Convert to bsv.UTXO format
		bsvUTXOs := make([]bsv.UTXO, len(allUTXOs))
		for i, u := range allUTXOs {
			bsvUTXOs[i] = bsv.UTXO{
				TxID:          u.TxID,
				Vout:          u.Vout,
				Amount:        u.Amount,
				Address:       u.Address,
				Confirmations: u.Confirmations,
			}
		}

		// Validate
		statuses, validateErr := bulkOps.BulkUTXOValidation(ctx, bsvUTXOs)
		if validateErr != nil {
			if s.logger != nil {
				s.logger.Error("bsv send: UTXO validation failed: %v", validateErr)
			}
			// Continue without validation
		} else {
			// Filter out spent UTXOs
			validUTXOs := make([]chain.UTXO, 0, len(allUTXOs))
			spentCount := 0
			for i, status := range statuses {
				if !status.Spent && status.Error == nil {
					validUTXOs = append(validUTXOs, allUTXOs[i])
				} else {
					spentCount++
				}
			}
			allUTXOs = validUTXOs
			if s.logger != nil {
				s.logger.Debug("bsv send: validated %d UTXOs, filtered %d spent", len(allUTXOs), spentCount)
			}
		}
	}

	if s.logger != nil {
		s.logger.Debug("bsv send: %d UTXOs from %d addresses (after filtering)", len(allUTXOs), len(req.Addresses))
	}

	var displayAmount string
	var estimatedFee uint64
	var sendUTXOs []chain.UTXO // UTXOs that will be used in the transaction

	//nolint:nestif // Sweep vs normal send have distinct balance check and fee estimation paths
	if sweepAll {
		// Sweep: use ALL UTXOs from all addresses
		if len(allUTXOs) == 0 {
			return nil, sigilerr.WithSuggestion(sigilerr.ErrInsufficientFunds, "no UTXOs found across any wallet address")
		}

		var totalInputs uint64
		for _, u := range allUTXOs {
			totalInputs += u.Amount
		}

		sweepAmount, sweepErr := bsv.CalculateSweepAmount(totalInputs, len(allUTXOs), feeQuote.StandardRate)
		if sweepErr != nil {
			return nil, sweepErr
		}

		amount = chain.AmountToBigInt(sweepAmount)
		estimatedFee = totalInputs - sweepAmount
		displayAmount = client.FormatAmount(amount) + " (sweep all)"
		sendUTXOs = allUTXOs
	} else {
		// Normal send: select UTXOs across all addresses to cover amount + fee
		if len(allUTXOs) == 0 {
			return nil, sigilerr.WithSuggestion(sigilerr.ErrInsufficientFunds, "no UTXOs found across any wallet address")
		}

		// Convert to bsv.UTXO for SelectUTXOs, preserving address info
		bsvUTXOs := make([]bsv.UTXO, len(allUTXOs))
		for i, u := range allUTXOs {
			bsvUTXOs[i] = bsv.UTXO{
				TxID:          u.TxID,
				Vout:          u.Vout,
				Amount:        u.Amount,
				ScriptPubKey:  u.ScriptPubKey,
				Address:       u.Address,
				Confirmations: u.Confirmations,
			}
		}

		selected, _, selErr := client.SelectUTXOs(bsvUTXOs, amount.Uint64(), feeQuote.StandardRate)
		if selErr != nil {
			return nil, selErr
		}

		// Convert selected back to chain.UTXO
		sendUTXOs = make([]chain.UTXO, len(selected))
		for i, u := range selected {
			sendUTXOs[i] = chain.UTXO{
				TxID:          u.TxID,
				Vout:          u.Vout,
				Amount:        u.Amount,
				ScriptPubKey:  u.ScriptPubKey,
				Address:       u.Address,
				Confirmations: u.Confirmations,
			}
		}

		estimatedFee = bsv.EstimateFeeForTx(len(selected), 2, feeQuote.StandardRate)
		displayAmount = req.AmountStr
	}
	if s.logger != nil {
		s.logger.Debug("bsv send: using %d UTXOs, estimated fee=%d sat", len(sendUTXOs), estimatedFee)
	}

	// Agent policy enforcement is handled at CLI layer via AgentToken/AgentCounterPath fields

	// Derive change address only for non-sweep (sweep has no change output)
	var changeAddress string
	if !sweepAll {
		// Load wallet to derive change address
		storage := wallet.NewFileStorage(filepath.Join(s.config.GetHome(), "wallets"))
		wlt, loadErr := storage.LoadMetadata(req.Wallet)
		if loadErr != nil {
			return nil, fmt.Errorf("loading wallet metadata: %w", loadErr)
		}

		changeAddr, changeErr := wlt.DeriveNextChangeAddress(req.Seed, wallet.ChainBSV)
		if changeErr != nil {
			return nil, fmt.Errorf("deriving change address: %w", changeErr)
		}
		if updateErr := s.storage.UpdateMetadata(wlt); updateErr != nil {
			return nil, fmt.Errorf("persisting wallet metadata: %w", updateErr)
		}
		changeAddress = changeAddr.Address
	}

	// Derive private keys for all addresses that have UTXOs being spent
	privateKeys, keyErr := deriveKeysForUTXOs(sendUTXOs, req.Addresses, req.Seed)
	if keyErr != nil {
		return nil, fmt.Errorf("deriving private keys: %w", keyErr)
	}
	defer func() {
		for _, k := range privateKeys {
			wallet.ZeroBytes(k)
		}
	}()

	// Build send request with multi-address support
	sendReq := chain.SendRequest{
		From:          req.FromAddress,
		To:            req.To,
		Amount:        amount,
		UTXOs:         sendUTXOs,
		PrivateKeys:   privateKeys,
		FeeRate:       feeQuote.StandardRate,
		ChangeAddress: changeAddress,
		SweepAll:      sweepAll,
	}

	// Send transaction
	result, err := client.Send(ctx, sendReq)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("bsv send failed: %v", err)
		}
		return nil, fmt.Errorf("sending transaction: %w", err)
	}
	if s.logger != nil {
		s.logger.Debug("bsv send: success hash=%s", result.Hash)
	}

	// Mark spent UTXOs in the local store to prevent double-spend on subsequent sends
	markSpentBSVUTXOs(s.logger, utxoStore, sendUTXOs, result.Hash)

	// Invalidate balance cache for all addresses that contributed UTXOs
	cachePath := filepath.Join(s.config.GetHome(), "cache", "balances.json")
	cacheProvider := cache.NewFileStorage(cachePath)

	involvedAddrs := uniqueUTXOAddrs(sendUTXOs)
	if sweepAll {
		// Sweep: all addresses are now empty
		for _, addr := range req.Addresses {
			invalidateBalanceCache(s.logger, cacheProvider, chain.BSV, addr.Address, "", "0.0")
		}
	} else {
		// Partial send: invalidate addresses that contributed inputs
		for addr := range involvedAddrs {
			invalidateBalanceCache(s.logger, cacheProvider, chain.BSV, addr, "", "")
		}
	}

	// Record agent spending (if in agent mode)
	if req.AgentToken != "" && req.AgentCounterPath != "" {
		recordAgentSpend(s.logger, req.AgentCounterPath, req.AgentToken, chain.BSV, amount)
	}

	// Convert to service result
	return &SendResult{
		Hash:       result.Hash,
		From:       result.From,
		To:         result.To,
		Amount:     displayAmount,
		Fee:        result.Fee,
		Status:     result.Status,
		ChainID:    chain.BSV,
		UTXOsSpent: len(sendUTXOs),
	}, nil
}
