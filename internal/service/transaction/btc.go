package transaction

import (
	"context"
	"fmt"
	"math/big"
	"path/filepath"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/btc"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// sendBTC builds, signs, and broadcasts a BTC transaction. It mirrors sendBSV
// with BTC-specific differences: sat/vByte fees from mempool.space, a recipient
// of any standard address type (change is legacy P2PKH), coin-type-0 key
// derivation, and a bounded (rate-limited) UTXO aggregation fan-out.
//
//nolint:gocognit,gocyclo // Transaction building involves multiple sequential steps
func (s *Service) sendBTC(ctx context.Context, req *SendRequest) (*SendResult, error) {
	// Resolve the network (wallet-stamped value wins, else config) and validate
	// the recipient against it so a mainnet address cannot be paid from a testnet
	// wallet (and vice versa). The recipient may be any standard address type.
	network := req.Network
	if network == "" {
		network = s.config.GetBTCNetwork()
	}
	if err := btc.ValidateBTCAddressForNetwork(req.To, btc.Network(network)); err != nil {
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidAddress,
			fmt.Sprintf("invalid BTC %s address: %s", network, req.To),
		)
	}

	client := btc.NewClient(ctx, &btc.ClientOptions{
		APIKey:      s.config.GetBTCAPIKey(),
		Network:     btc.Network(network),
		Logger:      s.logger,
		FeeStrategy: btc.FeeStrategy(s.config.GetBTCFeeStrategy()),
	})

	// Load local UTXO store for spent-UTXO filtering and post-broadcast marking.
	walletPath := filepath.Join(s.config.GetHome(), "wallets", req.Wallet)
	utxoStore := utxostore.New(walletPath)
	if err := utxoStore.Load(); err != nil {
		if s.logger != nil {
			s.logger.Error("btc send: failed to load utxo store: %v", err)
		}
		utxoStore = nil // Non-fatal: proceed without local filtering
	}

	sweepAll := req.SweepAll()
	if s.logger != nil {
		s.logger.Debug("btc send: to=%s amount=%s sweep=%v", req.To, req.AmountStr, sweepAll)
	}

	// Parse amount (skip for sweep — amount is calculated from balance minus fees).
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

	feeRate := client.FeeRate(ctx)
	if s.logger != nil {
		s.logger.Debug("btc send: fee rate=%d sat/vB", feeRate)
	}

	// Aggregate UTXOs from ALL wallet addresses (bounded/rate-limited fan-out).
	allUTXOs, utxoErr := aggregateBTCUTXOs(ctx, client, req.Addresses)
	if utxoErr != nil {
		if s.logger != nil {
			s.logger.Error("btc send: utxo aggregation failed: %v", utxoErr)
		}
		return nil, fmt.Errorf("listing UTXOs: %w", utxoErr)
	}
	if utxoStore != nil {
		allUTXOs = filterSpentUTXOs(chain.BTC, allUTXOs, utxoStore)
	}
	if s.logger != nil {
		s.logger.Debug("btc send: %d UTXOs from %d addresses (after filtering)", len(allUTXOs), len(req.Addresses))
	}

	var (
		displayAmount string
		estimatedFee  uint64
		sendUTXOs     []chain.UTXO
	)

	//nolint:nestif // Sweep vs normal send have distinct balance-check paths
	if sweepAll {
		if len(allUTXOs) == 0 {
			return nil, sigilerr.WithSuggestion(sigilerr.ErrInsufficientFunds, "no UTXOs found across any wallet address")
		}

		var totalInputs uint64
		for _, u := range allUTXOs {
			totalInputs += u.Amount
		}

		recipientScriptLen, scriptErr := recipientScriptLength(req.To, network)
		if scriptErr != nil {
			return nil, scriptErr
		}
		sweepAmount, sweepErr := btc.CalculateSweepAmount(totalInputs, len(allUTXOs), recipientScriptLen, feeRate)
		if sweepErr != nil {
			return nil, sweepErr
		}

		amount = chain.AmountToBigInt(sweepAmount)
		estimatedFee = totalInputs - sweepAmount
		displayAmount = client.FormatAmount(amount) + " (sweep all)"
		sendUTXOs = allUTXOs
	} else {
		if len(allUTXOs) == 0 {
			return nil, sigilerr.WithSuggestion(sigilerr.ErrInsufficientFunds, "no UTXOs found across any wallet address")
		}

		selected, _, selErr := client.SelectUTXOs(allUTXOs, amount.Uint64(), feeRate)
		if selErr != nil {
			return nil, selErr
		}
		sendUTXOs = selected
		estimatedFee = btc.EstimateFeeForTx(len(selected), 2, feeRate)
		displayAmount = req.AmountStr
	}
	if s.logger != nil {
		s.logger.Debug("btc send: using %d UTXOs, estimated fee=%d sat", len(sendUTXOs), estimatedFee)
	}

	// Derive change address (legacy P2PKH) only for non-sweep transactions.
	var changeAddress string
	if !sweepAll {
		storage := wallet.NewFileStorage(filepath.Join(s.config.GetHome(), "wallets"))
		wlt, loadErr := storage.LoadMetadata(req.Wallet)
		if loadErr != nil {
			return nil, fmt.Errorf("loading wallet metadata: %w", loadErr)
		}
		changeAddr, changeErr := wlt.DeriveNextChangeAddress(req.Seed, wallet.ChainBTC)
		if changeErr != nil {
			return nil, fmt.Errorf("deriving change address: %w", changeErr)
		}
		if updateErr := s.storage.UpdateMetadata(wlt); updateErr != nil {
			return nil, fmt.Errorf("persisting wallet metadata: %w", updateErr)
		}
		changeAddress = changeAddr.Address
	}

	// Derive private keys for all addresses that have UTXOs being spent
	// (coin-type-0 for BTC — distinct from BSV's coin-type-236).
	privateKeys, keyErr := deriveKeysForUTXOs(chain.BTC, sendUTXOs, req.Addresses, req.Seed)
	if keyErr != nil {
		return nil, fmt.Errorf("deriving private keys: %w", keyErr)
	}
	defer func() {
		for _, k := range privateKeys {
			wallet.ZeroBytes(k)
		}
	}()

	sendReq := chain.SendRequest{
		From:          req.FromAddress,
		To:            req.To,
		Amount:        amount,
		UTXOs:         sendUTXOs,
		PrivateKeys:   privateKeys,
		FeeRate:       feeRate,
		ChangeAddress: changeAddress,
		SweepAll:      sweepAll,
	}

	result, err := client.Send(ctx, sendReq)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("btc send failed: %v", err)
		}
		return nil, fmt.Errorf("sending transaction: %w", err)
	}
	if s.logger != nil {
		s.logger.Debug("btc send: success hash=%s", result.Hash)
	}

	// Mark spent UTXOs locally to prevent double-spend on subsequent sends.
	markSpentUTXOs(s.logger, utxoStore, chain.BTC, sendUTXOs, result.Hash)

	// Invalidate balance cache for all addresses that contributed UTXOs.
	cachePath := filepath.Join(s.config.GetHome(), "cache", "balances.json")
	cacheProvider := cache.NewFileStorage(cachePath)

	involvedAddrs := uniqueUTXOAddrs(sendUTXOs)
	if sweepAll {
		for _, addr := range req.Addresses {
			invalidateBalanceCache(s.logger, cacheProvider, chain.BTC, addr.Address, "", "0.0")
		}
	} else {
		for addr := range involvedAddrs {
			invalidateBalanceCache(s.logger, cacheProvider, chain.BTC, addr, "", "")
		}
	}

	if req.AgentToken != "" && req.AgentCounterPath != "" {
		recordAgentSpend(s.logger, req.AgentCounterPath, req.AgentToken, chain.BTC, amount)
	}

	return &SendResult{
		Hash:       result.Hash,
		From:       result.From,
		To:         result.To,
		Amount:     displayAmount,
		Fee:        result.Fee,
		Status:     result.Status,
		ChainID:    chain.BTC,
		UTXOsSpent: len(sendUTXOs),
	}, nil
}

// recipientScriptLength returns the locking-script byte length for the recipient
// address, used to size the sweep fee exactly (a P2WSH recipient is larger than
// the flat P2PKH assumption).
func recipientScriptLength(to, network string) (int, error) {
	s, err := btc.AddressToScript(to, btc.Network(network))
	if err != nil {
		return 0, fmt.Errorf("building recipient script: %w", err)
	}
	return len(*s), nil
}
