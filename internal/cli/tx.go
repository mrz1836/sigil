package cli

import (
	"context"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/chain/eth"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	// txWallet is the wallet name for transactions.
	txWallet string
	// txTo is the recipient address.
	txTo string
	// txAmount is the amount to send.
	txAmount string
	// txChain is the blockchain to use.
	txChain string
	// txToken is the ERC-20 token to transfer (e.g., "USDC").
	txToken string
	// txGasSpeed is the gas speed preference (slow/medium/fast).
	txGasSpeed string
	// txConfirm skips confirmation prompt if false.
	txConfirm bool
)

// txCmd is the parent command for transaction operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var txCmd = &cobra.Command{
	Use:   "tx",
	Short: "Manage transactions",
	Long:  `Send transactions and manage transaction history.`,
}

// txSendCmd sends a transaction.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var txSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a transaction",
	Long: `Send ETH, USDC, or BSV to an address.

For Ethereum transactions, you can send native ETH or ERC-20 tokens like USDC.
For BSV transactions, only native BSV is supported.

Examples:
  # Send ETH
  sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount 0.1 --chain eth

  # Send USDC
  sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount 100 --chain eth --token USDC

  # Send BSV
  sigil tx send --wallet main --to 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount 0.001 --chain bsv`,
	RunE: runTxSend,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(txCmd)
	txCmd.AddCommand(txSendCmd)

	txSendCmd.Flags().StringVar(&txWallet, "wallet", "", "wallet name (required)")
	txSendCmd.Flags().StringVar(&txTo, "to", "", "recipient address (required)")
	txSendCmd.Flags().StringVar(&txAmount, "amount", "", "amount to send (required)")
	txSendCmd.Flags().StringVar(&txChain, "chain", "eth", "blockchain: eth, bsv")
	txSendCmd.Flags().StringVar(&txToken, "token", "", "ERC-20 token symbol (e.g., USDC) - ETH only")
	txSendCmd.Flags().StringVar(&txGasSpeed, "gas", "medium", "gas speed: slow, medium, fast")
	txSendCmd.Flags().BoolVar(&txConfirm, "yes", false, "skip confirmation prompt")

	_ = txSendCmd.MarkFlagRequired("wallet")
	_ = txSendCmd.MarkFlagRequired("to")
	_ = txSendCmd.MarkFlagRequired("amount")
}

//nolint:gocyclo // CLI flow involves validation and routing
func runTxSend(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Validate chain
	chainID, ok := chain.ParseChainID(txChain)
	if !ok || !chainID.IsMVP() {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("invalid chain: %s (use eth or bsv)", txChain),
		)
	}

	// Token validation
	if txToken != "" && chainID != chain.ETH {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"--token flag is only supported for ETH chain",
		)
	}

	// Load wallet and get private key (using session if available)
	storage := wallet.NewFileStorage(filepath.Join(cfg.Home, "wallets"))
	wlt, seed, err := loadWalletWithSession(txWallet, storage, cmd)
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(seed)

	// Get the address for this chain
	addresses, ok := wlt.Addresses[chainID]
	if !ok || len(addresses) == 0 {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("wallet '%s' has no addresses for chain %s", txWallet, chainID),
		)
	}
	fromAddress := addresses[0].Address

	// Execute chain-specific send
	switch chainID {
	case chain.ETH:
		return runETHSend(ctx, cmd, fromAddress, seed)
	case chain.BSV:
		return runBSVSend(ctx, cmd, fromAddress, seed)
	case chain.BTC, chain.BCH:
		return sigilerr.ErrNotImplemented
	default:
		return sigilerr.ErrNotImplemented
	}
}

//nolint:gocognit,gocyclo // Transaction flow involves multiple validation and setup steps
func runETHSend(ctx context.Context, cmd *cobra.Command, fromAddress string, seed []byte) error {
	// Validate ETH address
	if err := eth.ValidateChecksumAddress(txTo); err != nil {
		if !eth.IsValidAddress(txTo) {
			return sigilerr.WithSuggestion(
				sigilerr.ErrInvalidAddress,
				fmt.Sprintf("invalid Ethereum address: %s", txTo),
			)
		}
	}

	// Get RPC URL from config
	rpcURL := cfg.Networks.ETH.RPC
	if rpcURL == "" {
		return sigilerr.WithSuggestion(
			sigilerr.ErrConfigInvalid,
			"Ethereum RPC URL not configured. Set it in ~/.sigil/config.yaml or SIGIL_ETH_RPC environment variable",
		)
	}

	// Create ETH client
	client, err := eth.NewClient(rpcURL, nil)
	if err != nil {
		return fmt.Errorf("creating ETH client: %w", err)
	}
	defer client.Close()

	// Parse gas speed
	speed, err := eth.ParseGasSpeed(txGasSpeed)
	if err != nil {
		return sigilerr.WithSuggestion(sigilerr.ErrInvalidInput, err.Error())
	}

	// Parse amount and get token address
	var amount *big.Int
	var tokenAddress string
	var decimals int

	if txToken != "" {
		// ERC-20 transfer
		tokenAddress, decimals, err = resolveToken(txToken)
		if err != nil {
			return err
		}
		amount, err = parseDecimalAmount(txAmount, decimals)
	} else {
		// Native ETH transfer
		amount, err = client.ParseAmount(txAmount)
	}
	if err != nil {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("invalid amount: %s", txAmount),
		)
	}

	// Estimate gas
	var estimate *eth.GasEstimate
	if tokenAddress != "" {
		estimate, err = client.EstimateGasForERC20Transfer(ctx, speed)
	} else {
		estimate, err = client.EstimateGasForETHTransfer(ctx, speed)
	}
	if err != nil {
		return fmt.Errorf("estimating gas: %w", err)
	}

	// Check balance
	err = checkETHBalance(ctx, client, fromAddress, amount, estimate.Total, tokenAddress)
	if err != nil {
		return err
	}

	// Display transaction details and confirm
	if !txConfirm {
		displayTxDetails(cmd, fromAddress, txTo, txAmount, txToken, estimate)
		if !promptConfirmation() {
			outln(cmd.OutOrStdout(), "Transaction canceled.")
			return nil
		}
	}

	// Derive private key from seed
	privateKey, err := wallet.DerivePrivateKeyForChain(seed, wallet.ChainETH, 0)
	if err != nil {
		return fmt.Errorf("deriving private key: %w", err)
	}
	defer wallet.ZeroBytes(privateKey)

	// Build send request
	req := chain.SendRequest{
		From:       fromAddress,
		To:         txTo,
		Amount:     amount,
		PrivateKey: privateKey,
		Token:      tokenAddress,
		GasLimit:   estimate.GasLimit,
	}

	// Send transaction
	result, err := client.Send(ctx, req)
	if err != nil {
		return fmt.Errorf("sending transaction: %w", err)
	}

	// Display result
	displayTxResult(cmd, result)

	return nil
}

func runBSVSend(ctx context.Context, cmd *cobra.Command, fromAddress string, seed []byte) error {
	// Validate BSV address
	if err := bsv.ValidateBase58CheckAddress(txTo); err != nil {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidAddress,
			fmt.Sprintf("invalid BSV address: %s", txTo),
		)
	}

	// Create BSV client
	client := bsv.NewClient(&bsv.ClientOptions{
		APIKey: cfg.Networks.BSV.APIKey,
	})

	// Parse amount
	amount, err := client.ParseAmount(txAmount)
	if err != nil {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("invalid amount: %s", txAmount),
		)
	}

	// Get fee quote
	feeQuote, err := client.GetFeeQuote(ctx)
	if err != nil {
		// Use default if fee quote fails
		feeQuote = &bsv.FeeQuote{StandardRate: bsv.DefaultFeeRate}
	}

	// Estimate fee
	estimatedFee := bsv.EstimateFeeForTx(1, 2, feeQuote.StandardRate)

	// Check balance
	balance, err := client.GetBalance(ctx, fromAddress)
	if err != nil {
		return fmt.Errorf("getting balance: %w", err)
	}

	totalRequired := amount.Uint64() + estimatedFee
	if balance.Uint64() < totalRequired {
		return sigilerr.WithDetails(
			sigilerr.ErrInsufficientFunds,
			map[string]string{
				"required":  client.FormatAmount(amountToBigInt(totalRequired)),
				"available": client.FormatAmount(balance),
				"symbol":    "BSV",
			},
		)
	}

	// Display transaction details and confirm
	if !txConfirm {
		displayBSVTxDetails(cmd, fromAddress, txTo, txAmount, estimatedFee, feeQuote.StandardRate)
		if !promptConfirmation() {
			outln(cmd.OutOrStdout(), "Transaction canceled.")
			return nil
		}
	}

	// Derive private key from seed
	privateKey, err := wallet.DerivePrivateKeyForChain(seed, wallet.ChainBSV, 0)
	if err != nil {
		return fmt.Errorf("deriving private key: %w", err)
	}
	defer wallet.ZeroBytes(privateKey)

	// Build send request
	req := chain.SendRequest{
		From:       fromAddress,
		To:         txTo,
		Amount:     amount,
		PrivateKey: privateKey,
		FeeRate:    feeQuote.StandardRate,
	}

	// Send transaction
	result, err := client.Send(ctx, req)
	if err != nil {
		return fmt.Errorf("sending transaction: %w", err)
	}

	// Display result
	displayBSVTxResult(cmd, result)

	return nil
}

// displayBSVTxDetails shows BSV transaction details before confirmation.
func displayBSVTxDetails(cmd *cobra.Command, from, to, amount string, fee, feeRate uint64) {
	w := cmd.OutOrStdout()
	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w, "                    TRANSACTION DETAILS")
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w)

	out(w, "  From:      %s\n", from)
	out(w, "  To:        %s\n", to)
	out(w, "  Amount:    %s BSV\n", amount)
	out(w, "  Fee Rate:  %d sat/byte\n", feeRate)
	out(w, "  Est. Fee:  %d satoshis\n", fee)

	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
}

// displayBSVTxResult shows the BSV transaction result.
func displayBSVTxResult(cmd *cobra.Command, result *chain.TransactionResult) {
	w := cmd.OutOrStdout()
	format := formatter.Format()

	if format == output.FormatJSON {
		displayBSVTxResultJSON(w, result)
	} else {
		displayBSVTxResultText(w, result)
	}
}

// displayBSVTxResultText shows BSV transaction result in text format.
func displayBSVTxResultText(w interface {
	Write(p []byte) (n int, err error)
}, result *chain.TransactionResult,
) {
	outln(w, "\nTransaction broadcast successfully!")
	outln(w)
	out(w, "  Hash:   %s\n", result.Hash)
	out(w, "  Status: %s\n", result.Status)
	out(w, "  Amount: %s BSV\n", result.Amount)
	out(w, "  Fee:    %s BSV\n", result.Fee)
	outln(w)
	outln(w, "Track your transaction on WhatsOnChain:")
	out(w, "  https://whatsonchain.com/tx/%s\n", result.Hash)
}

// displayBSVTxResultJSON shows BSV transaction result in JSON format.
func displayBSVTxResultJSON(w interface {
	Write(p []byte) (n int, err error)
}, result *chain.TransactionResult,
) {
	outln(w, "{")
	out(w, `  "hash": "%s",`+"\n", result.Hash)
	out(w, `  "from": "%s",`+"\n", result.From)
	out(w, `  "to": "%s",`+"\n", result.To)
	out(w, `  "amount": "%s",`+"\n", result.Amount)
	out(w, `  "fee": "%s",`+"\n", result.Fee)
	out(w, `  "status": "%s"`+"\n", result.Status)
	outln(w, "}")
}

// amountToBigInt converts uint64 to *big.Int.
func amountToBigInt(amount uint64) *big.Int {
	return new(big.Int).SetUint64(amount)
}

// resolveToken resolves a token symbol to its contract address and decimals.
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

// parseDecimalAmount parses a decimal string to big.Int with specified decimals.
//
//nolint:gocognit,gocyclo // Decimal parsing with precision requires multiple checks
func parseDecimalAmount(amount string, decimals int) (*big.Int, error) {
	if amount == "" {
		return nil, sigilerr.ErrAmountRequired
	}

	// Parse as float and multiply by 10^decimals
	parts := strings.Split(amount, ".")
	if len(parts) > 2 {
		return nil, sigilerr.WithDetails(
			sigilerr.ErrInvalidAmount,
			map[string]string{"amount": amount},
		)
	}

	intPart := parts[0]
	decPart := ""
	if len(parts) == 2 {
		decPart = parts[1]
	}

	// Validate integer part
	if intPart == "" {
		intPart = "0"
	}
	intVal, ok := new(big.Int).SetString(intPart, 10)
	if !ok {
		return nil, sigilerr.WithDetails(
			sigilerr.ErrInvalidAmount,
			map[string]string{"amount": amount},
		)
	}

	// Scale integer part
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	result := new(big.Int).Mul(intVal, multiplier)

	// Handle decimal part
	if decPart != "" {
		// Validate decimal characters
		for _, c := range decPart {
			if c < '0' || c > '9' {
				return nil, sigilerr.WithDetails(
					sigilerr.ErrInvalidAmount,
					map[string]string{"amount": amount},
				)
			}
		}

		// Pad or truncate decimal part
		for len(decPart) < decimals {
			decPart += "0"
		}
		decPart = decPart[:decimals]

		decVal, ok := new(big.Int).SetString(decPart, 10)
		if !ok {
			return nil, sigilerr.WithDetails(
				sigilerr.ErrInvalidAmount,
				map[string]string{"amount": amount},
			)
		}

		result = result.Add(result, decVal)
	}

	return result, nil
}

// checkETHBalance verifies sufficient balance for the transaction.
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
					"required":  eth.FormatBalanceAmount(amount, eth.USDCDecimals),
					"available": eth.FormatBalanceAmount(tokenBalance, eth.USDCDecimals),
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

// displayTxDetails shows transaction details before confirmation.
func displayTxDetails(cmd *cobra.Command, from, to, amount, token string, estimate *eth.GasEstimate) {
	w := cmd.OutOrStdout()
	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w, "                    TRANSACTION DETAILS")
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w)

	out(w, "  From:      %s\n", from)
	out(w, "  To:        %s\n", to)

	if token != "" {
		out(w, "  Amount:    %s %s\n", amount, token)
	} else {
		out(w, "  Amount:    %s ETH\n", amount)
	}

	out(w, "  Gas Limit: %d\n", estimate.GasLimit)
	out(w, "  Gas Price: %s\n", eth.FormatGasPrice(estimate.GasPrice))
	out(w, "  Est. Fee:  %s ETH\n", eth.FormatBalanceAmount(estimate.Total, 18))

	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
}

// displayTxResult shows the transaction result.
func displayTxResult(cmd *cobra.Command, result *chain.TransactionResult) {
	w := cmd.OutOrStdout()
	format := formatter.Format()

	if format == output.FormatJSON {
		displayTxResultJSON(w, result)
	} else {
		displayTxResultText(w, result)
	}
}

// displayTxResultText shows transaction result in text format.
func displayTxResultText(w interface {
	Write(p []byte) (n int, err error)
}, result *chain.TransactionResult,
) {
	outln(w, "\nTransaction broadcast successfully!")
	outln(w)
	out(w, "  Hash:   %s\n", result.Hash)
	out(w, "  Status: %s\n", result.Status)

	if result.Token != "" {
		out(w, "  Amount: %s %s\n", result.Amount, result.Token)
	} else {
		out(w, "  Amount: %s ETH\n", result.Amount)
	}

	out(w, "  Fee:    %s\n", result.Fee)
	outln(w)
	outln(w, "Track your transaction on Etherscan:")
	out(w, "  https://etherscan.io/tx/%s\n", result.Hash)
}

// displayTxResultJSON shows transaction result in JSON format.
func displayTxResultJSON(w interface {
	Write(p []byte) (n int, err error)
}, result *chain.TransactionResult,
) {
	outln(w, "{")
	out(w, `  "hash": "%s",`+"\n", result.Hash)
	out(w, `  "from": "%s",`+"\n", result.From)
	out(w, `  "to": "%s",`+"\n", result.To)
	out(w, `  "amount": "%s",`+"\n", result.Amount)
	if result.Token != "" {
		out(w, `  "token": "%s",`+"\n", result.Token)
	}
	out(w, `  "fee": "%s",`+"\n", result.Fee)
	out(w, `  "gas_used": %d,`+"\n", result.GasUsed)
	out(w, `  "gas_price": "%s",`+"\n", result.GasPrice)
	out(w, `  "status": "%s"`+"\n", result.Status)
	outln(w, "}")
}
