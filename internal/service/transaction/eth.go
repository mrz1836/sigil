package transaction

import (
	"context"
	"fmt"
	"math/big"
	"path/filepath"
	"runtime"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/eth"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

//nolint:gocognit,gocyclo // Transaction flow is inherently complex (migrated from CLI)
// sendETH handles the complete Ethereum transaction flow.
// Migrated from cli/tx.go lines 183-395
func (s *Service) sendETH(ctx context.Context, req *SendRequest) (*SendResult, error) {
	// Validate ETH address
	if err := eth.ValidateChecksumAddress(req.To); err != nil {
		if !eth.IsValidAddress(req.To) {
			return nil, sigilerr.WithSuggestion(
				sigilerr.ErrInvalidAddress,
				fmt.Sprintf("invalid Ethereum address: %s", req.To),
			)
		}
	}

	// Get RPC URL from config
	rpcURL := s.config.GetETHRPC()
	if rpcURL == "" {
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrConfigInvalid,
			"Ethereum RPC URL not configured. Set it in ~/.sigil/config.yaml or SIGIL_ETH_RPC environment variable",
		)
	}

	// Create ETH client
	client, err := eth.NewClient(rpcURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating ETH client: %w", err)
	}
	defer client.Close()

	// Parse gas speed
	speed, err := eth.ParseGasSpeed(req.GasSpeed)
	if err != nil {
		return nil, sigilerr.WithSuggestion(sigilerr.ErrInvalidInput, err.Error())
	}

	// Resolve token if specified
	var tokenAddress string
	var decimals int
	if req.Token != "" {
		tokenAddress, decimals, err = resolveToken(req.Token)
		if err != nil {
			return nil, err
		}
	}

	// Parse amount (skip for sweep — calculated from balance)
	var amount *big.Int
	if !req.SweepAll() {
		if req.Token != "" {
			amount, err = parseDecimalAmount(req.AmountStr, decimals)
		} else {
			amount, err = client.ParseAmount(req.AmountStr)
		}
		if err != nil {
			return nil, sigilerr.WithSuggestion(
				sigilerr.ErrInvalidInput,
				fmt.Sprintf("invalid amount: %s", req.AmountStr),
			)
		}
	}

	// Estimate gas
	var estimate *eth.GasEstimate
	if tokenAddress != "" {
		estimate, err = client.EstimateGasForERC20Transfer(ctx, speed)
	} else {
		estimate, err = client.EstimateGasForETHTransfer(ctx, speed)
	}
	if err != nil {
		return nil, fmt.Errorf("estimating gas: %w", err)
	}

	// Calculate sweep amount or validate balance
	var displayAmount string
	//nolint:nestif // Sweep calculation branches by token type
	if req.SweepAll() {
		if tokenAddress != "" {
			// ERC-20 sweep: send full token balance (ETH needed for gas only)
			tokenBalance, tokenErr := client.GetTokenBalance(ctx, req.FromAddress, tokenAddress)
			if tokenErr != nil {
				return nil, fmt.Errorf("getting token balance: %w", tokenErr)
			}
			if tokenBalance.Sign() <= 0 {
				return nil, sigilerr.WithDetails(
					sigilerr.ErrInsufficientFunds,
					map[string]string{
						"symbol": req.Token,
						"reason": "zero token balance",
					},
				)
			}
			amount = tokenBalance
			displayAmount = chain.FormatDecimalAmount(amount, decimals) + " (sweep all)"

			// Still need ETH for gas
			ethBalance, ethErr := client.GetBalance(ctx, req.FromAddress)
			if ethErr != nil {
				return nil, fmt.Errorf("getting ETH balance: %w", ethErr)
			}
			if ethBalance.Cmp(estimate.Total) < 0 {
				return nil, sigilerr.WithDetails(
					sigilerr.ErrInsufficientFunds,
					map[string]string{
						"required":  client.FormatAmount(estimate.Total),
						"available": client.FormatAmount(ethBalance),
						"symbol":    "ETH",
						"reason":    "insufficient ETH for gas",
					},
				)
			}
		} else {
			// Native ETH sweep: balance minus gas cost
			ethBalance, ethErr := client.GetBalance(ctx, req.FromAddress)
			if ethErr != nil {
				return nil, fmt.Errorf("getting ETH balance: %w", ethErr)
			}
			amount = new(big.Int).Sub(ethBalance, estimate.Total)
			if amount.Sign() <= 0 {
				return nil, sigilerr.WithDetails(
					sigilerr.ErrInsufficientFunds,
					map[string]string{
						"required":  client.FormatAmount(estimate.Total),
						"available": client.FormatAmount(ethBalance),
						"symbol":    "ETH",
						"reason":    "balance does not cover gas fees",
					},
				)
			}
			displayAmount = client.FormatAmount(amount) + " (sweep all)"
		}
	} else {
		// Normal send: check balance covers amount + fees
		err = checkETHBalance(ctx, client, req.FromAddress, amount, estimate.Total, tokenAddress)
		if err != nil {
			return nil, err
		}
		if tokenAddress != "" {
			displayAmount = chain.FormatDecimalAmount(amount, decimals)
		} else {
			displayAmount = client.FormatAmount(amount)
		}
	}

	// Agent policy enforcement is handled at CLI layer via AgentToken/AgentCounterPath fields

	// Derive private key from seed
	privateKey, err := wallet.DerivePrivateKeyForChain(req.Seed, wallet.ChainETH, 0)
	if err != nil {
		return nil, fmt.Errorf("deriving private key: %w", err)
	}
	defer wallet.ZeroBytes(privateKey)
	defer runtime.KeepAlive(privateKey) // Prevent compiler optimization

	// Build send request
	sendReq := chain.SendRequest{
		From:       req.FromAddress,
		To:         req.To,
		Amount:     amount,
		PrivateKey: privateKey,
		Token:      tokenAddress,
		GasLimit:   estimate.GasLimit,
	}

	// Send transaction
	result, err := client.Send(ctx, sendReq)
	if err != nil {
		return nil, fmt.Errorf("sending transaction: %w", err)
	}

	// Invalidate balance cache
	cachePath := filepath.Join(s.config.GetHome(), "cache", "balances.json")
	cacheProvider := cache.NewFileStorage(cachePath)

	if req.SweepAll() && tokenAddress == "" {
		// Native ETH sweep: balance is now 0
		invalidateBalanceCache(s.logger, cacheProvider, chain.ETH, req.FromAddress, "", "0.0")
	} else if req.SweepAll() && tokenAddress != "" {
		// Token sweep: token balance is 0, ETH balance changed (gas spent)
		invalidateBalanceCache(s.logger, cacheProvider, chain.ETH, req.FromAddress, tokenAddress, "0.0")
		invalidateBalanceCache(s.logger, cacheProvider, chain.ETH, req.FromAddress, "", "")
	} else {
		// Partial send: delete entries to force fresh fetch
		invalidateBalanceCache(s.logger, cacheProvider, chain.ETH, req.FromAddress, "", "")
		if tokenAddress != "" {
			invalidateBalanceCache(s.logger, cacheProvider, chain.ETH, req.FromAddress, tokenAddress, "")
		}
	}

	// Record agent spending (if in agent mode)
	if req.AgentToken != "" && req.AgentCounterPath != "" {
		recordAgentSpend(s.logger, req.AgentCounterPath, req.AgentToken, chain.ETH, amount)
	}

	// Convert to service result
	return &SendResult{
		Hash:     result.Hash,
		From:     result.From,
		To:       result.To,
		Amount:   displayAmount,
		Fee:      result.Fee,
		Token:    req.Token,
		Status:   result.Status,
		ChainID:  chain.ETH,
		GasUsed:  result.GasUsed,
		GasPrice: result.GasPrice,
	}, nil
}
