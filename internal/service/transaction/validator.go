package transaction

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/eth"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// resolveToken resolves a token symbol to its contract address and decimals.
// Migrated from cli/tx.go lines 729-740
func resolveToken(symbol string) (address string, decimals int, err error) {
	switch strings.ToUpper(symbol) {
	case "USDC":
		return eth.USDCMainnet, eth.USDCDecimals, nil
	default:
		return "", 0, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("unsupported token: %s (only USDC is supported)", symbol),
		)
	}
}

// amountAll is the special value for sending the entire balance.
const amountAll = "all"

// isAmountAll returns true if the user specified "all" as the amount.
// Migrated from cli/tx.go lines 745-748
func isAmountAll(amount string) bool {
	return strings.EqualFold(strings.TrimSpace(amount), amountAll)
}

// IsAmountAll is the exported version for external use.
func IsAmountAll(amount string) bool {
	return isAmountAll(amount)
}

// SanitizeAmount trims whitespace from the amount string without altering content.
// Amount parsing performs strict validation and rejects non-numeric characters.
// Migrated from cli/tx.go lines 750-754
func SanitizeAmount(amount string) string {
	return strings.TrimSpace(amount)
}

// parseDecimalAmount parses a decimal string to big.Int with specified decimals.
// Migrated from cli/tx.go lines 756-790
func parseDecimalAmount(amount string, decimals int) (*big.Int, error) {
	amount = SanitizeAmount(amount)
	if amount == "" {
		return nil, sigilerr.ErrAmountRequired
	}

	if amount == "." {
		return nil, sigilerr.WithDetails(
			sigilerr.ErrInvalidAmount,
			map[string]string{"amount": amount},
		)
	}

	dotSeen := false
	for _, c := range amount {
		switch {
		case c == '.':
			if dotSeen {
				return nil, sigilerr.WithDetails(
					sigilerr.ErrInvalidAmount,
					map[string]string{"amount": amount},
				)
			}
			dotSeen = true
		case c < '0' || c > '9':
			return nil, sigilerr.WithDetails(
				sigilerr.ErrInvalidAmount,
				map[string]string{"amount": amount},
			)
		}
	}

	return chain.ParseDecimalAmount(amount, decimals, sigilerr.ErrInvalidAmount)
}

// ParseDecimalAmount is the exported version for external use.
func ParseDecimalAmount(amount string, decimals int) (*big.Int, error) {
	return parseDecimalAmount(amount, decimals)
}

// checkETHBalance verifies sufficient balance for the transaction.
// Migrated from cli/tx.go lines 792-847
func checkETHBalance(ctx context.Context, client *eth.Client, address string, amount, gasCost *big.Int, tokenAddress string) error {
	// Check ETH balance for gas
	ethBalance, err := client.GetBalance(ctx, address)
	if err != nil {
		return fmt.Errorf("getting ETH balance: %w", err)
	}

	//nolint:nestif // Balance checking logic is necessarily complex
	if tokenAddress != "" {
		// For ERC-20: need ETH for gas only
		if ethBalance.Cmp(gasCost) < 0 {
			return sigilerr.WithDetails(
				sigilerr.ErrInsufficientFunds,
				map[string]string{
					"required":  client.FormatAmount(gasCost),
					"available": client.FormatAmount(ethBalance),
					"symbol":    "ETH",
					"reason":    "insufficient ETH for gas",
				},
			)
		}

		// Check token balance
		tokenBalance, err := client.GetTokenBalance(ctx, address, tokenAddress)
		if err != nil {
			return fmt.Errorf("getting token balance: %w", err)
		}

		if tokenBalance.Cmp(amount) < 0 {
			return sigilerr.WithDetails(
				sigilerr.ErrInsufficientFunds,
				map[string]string{
					"required":  chain.FormatDecimalAmount(amount, eth.USDCDecimals),
					"available": chain.FormatDecimalAmount(tokenBalance, eth.USDCDecimals),
					"symbol":    "USDC",
				},
			)
		}
	} else {
		// For native ETH: need amount + gas
		totalRequired := new(big.Int).Add(amount, gasCost)
		if ethBalance.Cmp(totalRequired) < 0 {
			return sigilerr.WithDetails(
				sigilerr.ErrInsufficientFunds,
				map[string]string{
					"required":  client.FormatAmount(totalRequired),
					"available": client.FormatAmount(ethBalance),
					"symbol":    "ETH",
				},
			)
		}
	}

	return nil
}

// ValidateETHBalance is the exported version for external use.
func ValidateETHBalance(ctx context.Context, client *eth.Client, address string, amount, gasCost *big.Int, tokenAddress string) error {
	return checkETHBalance(ctx, client, address, amount, gasCost, tokenAddress)
}
