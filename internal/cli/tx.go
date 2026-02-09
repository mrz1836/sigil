package cli

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/cache"
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

Use --amount all to send the entire balance (fees are deducted automatically).

Examples:
  # Send ETH
  sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount 0.1 --chain eth

  # Send all ETH
  sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount all --chain eth

  # Send USDC
  sigil tx send --wallet main --to 0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0 --amount 100 --chain eth --token USDC

  # Send BSV
  sigil tx send --wallet main --to 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount 0.001 --chain bsv

  # Send all BSV
  sigil tx send --wallet main --to 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa --amount all --chain bsv`,
	RunE: runTxSend,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(txCmd)
	txCmd.AddCommand(txSendCmd)

	txSendCmd.Flags().StringVar(&txWallet, "wallet", "", "wallet name (required)")
	txSendCmd.Flags().StringVar(&txTo, "to", "", "recipient address (required)")
	txSendCmd.Flags().StringVar(&txAmount, "amount", "", "amount to send, or 'all' for entire balance (required)")
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
	cc := GetCmdContext(cmd)
	ctx, cancel := contextWithTimeout(cmd, 60*time.Second)
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
	storage := wallet.NewFileStorage(filepath.Join(cc.Cfg.GetHome(), "wallets"))
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
		return runBSVSend(ctx, cmd, wlt, storage, fromAddress, seed)
	case chain.BTC, chain.BCH:
		return sigilerr.ErrNotImplemented
	default:
		return sigilerr.ErrNotImplemented
	}
}

//nolint:gocognit,gocyclo // Transaction flow involves multiple validation and setup steps
func runETHSend(ctx context.Context, cmd *cobra.Command, fromAddress string, seed []byte) error {
	cc := GetCmdContext(cmd)

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
	rpcURL := cc.Cfg.GetETHRPC()
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
	sweepAll := isAmountAll(txAmount)
	var amount *big.Int
	var tokenAddress string
	var decimals int

	//nolint:nestif // Amount parsing branches by token type and sweep mode
	if txToken != "" {
		// ERC-20 transfer
		tokenAddress, decimals, err = resolveToken(txToken)
		if err != nil {
			return err
		}
		if !sweepAll {
			amount, err = parseDecimalAmount(txAmount, decimals)
			if err != nil {
				return sigilerr.WithSuggestion(
					sigilerr.ErrInvalidInput,
					fmt.Sprintf("invalid amount: %s", txAmount),
				)
			}
		}
	} else {
		// Native ETH transfer
		if !sweepAll {
			amount, err = client.ParseAmount(txAmount)
			if err != nil {
				return sigilerr.WithSuggestion(
					sigilerr.ErrInvalidInput,
					fmt.Sprintf("invalid amount: %s", txAmount),
				)
			}
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
		return fmt.Errorf("estimating gas: %w", err)
	}

	// For sweep, calculate the actual amount from balance minus fees
	//nolint:nestif // Sweep calculation branches by token type with balance/gas checks
	if sweepAll {
		if tokenAddress != "" {
			// ERC-20 sweep: send full token balance (ETH needed for gas only)
			tokenBalance, tokenErr := client.GetTokenBalance(ctx, fromAddress, tokenAddress)
			if tokenErr != nil {
				return fmt.Errorf("getting token balance: %w", tokenErr)
			}
			if tokenBalance.Sign() <= 0 {
				return sigilerr.WithDetails(
					sigilerr.ErrInsufficientFunds,
					map[string]string{
						"symbol": txToken,
						"reason": "zero token balance",
					},
				)
			}
			amount = tokenBalance

			// Still need ETH for gas
			ethBalance, ethErr := client.GetBalance(ctx, fromAddress)
			if ethErr != nil {
				return fmt.Errorf("getting ETH balance: %w", ethErr)
			}
			if ethBalance.Cmp(estimate.Total) < 0 {
				return sigilerr.WithDetails(
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
			ethBalance, ethErr := client.GetBalance(ctx, fromAddress)
			if ethErr != nil {
				return fmt.Errorf("getting ETH balance: %w", ethErr)
			}
			amount = new(big.Int).Sub(ethBalance, estimate.Total)
			if amount.Sign() <= 0 {
				return sigilerr.WithDetails(
					sigilerr.ErrInsufficientFunds,
					map[string]string{
						"required":  client.FormatAmount(estimate.Total),
						"available": client.FormatAmount(ethBalance),
						"symbol":    "ETH",
						"reason":    "balance does not cover gas fees",
					},
				)
			}
		}
	} else {
		// Normal send: check balance covers amount + fees
		err = checkETHBalance(ctx, client, fromAddress, amount, estimate.Total, tokenAddress)
		if err != nil {
			return err
		}
	}

	// Display transaction details and confirm
	displayAmount := txAmount
	if sweepAll {
		if tokenAddress != "" {
			displayAmount = eth.FormatBalanceAmount(amount, decimals) + " (sweep all)"
		} else {
			displayAmount = client.FormatAmount(amount) + " (sweep all)"
		}
	}
	if !txConfirm {
		displayTxDetails(cmd, fromAddress, txTo, displayAmount, txToken, estimate)
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

	// Invalidate balance cache so next "balance show" reflects the send.
	if sweepAll && tokenAddress == "" {
		// Native ETH sweep: balance is now 0
		invalidateBalanceCache(cc, chain.ETH, fromAddress, "", "0.0")
	} else if sweepAll && tokenAddress != "" {
		// Token sweep: token balance is 0, ETH balance changed (gas spent)
		invalidateBalanceCache(cc, chain.ETH, fromAddress, tokenAddress, "0.0")
		invalidateBalanceCache(cc, chain.ETH, fromAddress, "", "")
	} else {
		// Partial send: delete entries to force fresh fetch
		invalidateBalanceCache(cc, chain.ETH, fromAddress, "", "")
		if tokenAddress != "" {
			invalidateBalanceCache(cc, chain.ETH, fromAddress, tokenAddress, "")
		}
	}

	// Display result
	displayTxResult(cmd, result)

	return nil
}

//nolint:gocognit,gocyclo // Transaction flow involves multiple validation and setup steps
func runBSVSend(ctx context.Context, cmd *cobra.Command, wlt *wallet.Wallet, storage *wallet.FileStorage, fromAddress string, seed []byte) error {
	cc := GetCmdContext(cmd)

	// Validate BSV address
	if err := bsv.ValidateBase58CheckAddress(txTo); err != nil {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidAddress,
			fmt.Sprintf("invalid BSV address: %s", txTo),
		)
	}

	// Create BSV client
	opts := &bsv.ClientOptions{
		APIKey: cc.Cfg.GetBSVAPIKey(),
	}
	// Pass custom broadcast URL if configured (non-default value).
	if b := cc.Cfg.GetBSVBroadcast(); b != "" && b != "taal" && b != "whatsonchain" {
		opts.BroadcastURL = b
	}
	client := bsv.NewClient(opts)

	sweepAll := isAmountAll(txAmount)

	// Parse amount (skip for sweep — amount is calculated from balance minus fees)
	var amount *big.Int
	if !sweepAll {
		var err error
		amount, err = client.ParseAmount(txAmount)
		if err != nil {
			return sigilerr.WithSuggestion(
				sigilerr.ErrInvalidInput,
				fmt.Sprintf("invalid amount: %s", txAmount),
			)
		}
	}

	// Get fee quote
	feeQuote, err := client.GetFeeQuote(ctx)
	if err != nil {
		// Use default if fee quote fails
		feeQuote = &bsv.FeeQuote{StandardRate: bsv.DefaultFeeRate}
	}

	var displayAmount string
	var estimatedFee uint64

	//nolint:nestif // Sweep vs normal send have distinct balance check and fee estimation paths
	if sweepAll {
		// Sweep: calculate max send amount from UTXOs
		utxos, utxoErr := client.ListUTXOs(ctx, fromAddress)
		if utxoErr != nil {
			return fmt.Errorf("listing UTXOs: %w", utxoErr)
		}
		if len(utxos) == 0 {
			return sigilerr.WithSuggestion(sigilerr.ErrInsufficientFunds, "no UTXOs found for address")
		}

		var totalInputs uint64
		for _, u := range utxos {
			totalInputs += u.Amount
		}

		sweepAmount, sweepErr := bsv.CalculateSweepAmount(totalInputs, len(utxos), feeQuote.StandardRate)
		if sweepErr != nil {
			return sweepErr
		}

		amount = amountToBigInt(sweepAmount)
		estimatedFee = totalInputs - sweepAmount
		displayAmount = client.FormatAmount(amount) + " (sweep all)"
	} else {
		// Normal: check balance covers amount + fee
		estimatedFee = bsv.EstimateFeeForTx(1, 2, feeQuote.StandardRate)

		balance, balErr := client.GetBalance(ctx, fromAddress)
		if balErr != nil {
			return fmt.Errorf("getting balance: %w", balErr)
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
		displayAmount = txAmount
	}

	// Display transaction details and confirm
	if !txConfirm {
		displayBSVTxDetails(cmd, fromAddress, txTo, displayAmount, estimatedFee, feeQuote.StandardRate)
		if !promptConfirmation() {
			outln(cmd.OutOrStdout(), "Transaction canceled.")
			return nil
		}
	}

	// Derive change address only for non-sweep (sweep has no change output)
	var changeAddress string
	if !sweepAll {
		changeAddr, changeErr := wlt.DeriveNextChangeAddress(seed, wallet.ChainBSV)
		if changeErr != nil {
			return fmt.Errorf("deriving change address: %w", changeErr)
		}
		if updateErr := storage.UpdateMetadata(wlt); updateErr != nil {
			return fmt.Errorf("persisting wallet metadata: %w", updateErr)
		}
		changeAddress = changeAddr.Address
	}

	// Derive private key from seed for the sending address
	// Find the index of the from address
	fromIndex := uint32(0)
	for _, addr := range wlt.Addresses[wallet.ChainBSV] {
		if addr.Address == fromAddress {
			fromIndex = addr.Index
			break
		}
	}
	privateKey, err := wallet.DerivePrivateKeyForChain(seed, wallet.ChainBSV, fromIndex)
	if err != nil {
		return fmt.Errorf("deriving private key: %w", err)
	}
	defer wallet.ZeroBytes(privateKey)

	// Build send request
	req := chain.SendRequest{
		From:          fromAddress,
		To:            txTo,
		Amount:        amount,
		PrivateKey:    privateKey,
		FeeRate:       feeQuote.StandardRate,
		ChangeAddress: changeAddress,
		SweepAll:      sweepAll,
	}

	// Send transaction
	result, err := client.Send(ctx, req)
	if err != nil {
		return fmt.Errorf("sending transaction: %w", err)
	}

	// Invalidate balance cache so next "balance show" reflects the send.
	if sweepAll {
		invalidateBalanceCache(cc, chain.BSV, fromAddress, "", "0.0")
	} else {
		invalidateBalanceCache(cc, chain.BSV, fromAddress, "", "")
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
	cc := GetCmdContext(cmd)
	w := cmd.OutOrStdout()
	format := cc.Fmt.Format()

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
	payload := struct {
		Hash   string `json:"hash"`
		From   string `json:"from"`
		To     string `json:"to"`
		Amount string `json:"amount"`
		Fee    string `json:"fee"`
		Status string `json:"status"`
	}{
		Hash:   result.Hash,
		From:   result.From,
		To:     result.To,
		Amount: result.Amount,
		Fee:    result.Fee,
		Status: result.Status,
	}

	_ = writeJSON(w, payload)
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

// amountAll is the special value for sending the entire balance.
const amountAll = "all"

// isAmountAll returns true if the user specified "all" as the amount.
func isAmountAll(amount string) bool {
	return strings.EqualFold(strings.TrimSpace(amount), amountAll)
}

// SanitizeAmount trims whitespace from the amount string without altering content.
// Amount parsing performs strict validation and rejects non-numeric characters.
func SanitizeAmount(amount string) string {
	return strings.TrimSpace(amount)
}

// parseDecimalAmount parses a decimal string to big.Int with specified decimals.
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
	cc := GetCmdContext(cmd)
	w := cmd.OutOrStdout()
	format := cc.Fmt.Format()

	if format == output.FormatJSON {
		displayTxResultJSON(w, result)
	} else {
		displayTxResultText(w, result)
	}
}

// displayTxResultText shows transaction result in text format.
func displayTxResultText(w io.Writer, result *chain.TransactionResult) {
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
func displayTxResultJSON(w io.Writer, result *chain.TransactionResult) {
	payload := struct {
		Hash     string `json:"hash"`
		From     string `json:"from"`
		To       string `json:"to"`
		Amount   string `json:"amount"`
		Token    string `json:"token,omitempty"`
		Fee      string `json:"fee"`
		GasUsed  uint64 `json:"gas_used"`
		GasPrice string `json:"gas_price"`
		Status   string `json:"status"`
	}{
		Hash:     result.Hash,
		From:     result.From,
		To:       result.To,
		Amount:   result.Amount,
		Token:    result.Token,
		Fee:      result.Fee,
		GasUsed:  result.GasUsed,
		GasPrice: result.GasPrice,
		Status:   result.Status,
	}

	_ = writeJSON(w, payload)
}

// invalidateBalanceCache updates the on-disk balance cache after a successful
// transaction broadcast. If expectedBalance is non-empty, the cached entry is
// updated with that value (e.g., "0.0" for sweep-all). Otherwise the entry is
// deleted, forcing the next balance query to fetch from the network.
// Errors are logged but never returned — cache invalidation is best-effort.
func invalidateBalanceCache(cc *CommandContext, chainID chain.ID, address, token, expectedBalance string) {
	cachePath := filepath.Join(cc.Cfg.GetHome(), "cache", "balances.json")
	cacheStorage := cache.NewFileStorage(cachePath)

	balanceCache, err := cacheStorage.Load()
	if err != nil {
		logCacheError(cc, "failed to load balance cache for post-send update: %v", err)
		return
	}

	if expectedBalance == "" {
		// Unknown expected balance — delete to force a fresh network fetch.
		balanceCache.Delete(chainID, address, token)
	} else {
		// Known expected balance (e.g., sweep-all → "0.0").
		// Preserve symbol/decimals from the existing entry if available.
		entry := buildPostSendEntry(balanceCache, chainID, address, token, expectedBalance)
		balanceCache.Set(entry)
	}

	if err := cacheStorage.Save(balanceCache); err != nil {
		logCacheError(cc, "failed to save balance cache after send: %v", err)
	}
}

// buildPostSendEntry creates a cache entry with the expected post-send balance,
// preserving symbol and decimals from any existing entry.
func buildPostSendEntry(bc *cache.BalanceCache, chainID chain.ID, address, token, balance string) cache.BalanceCacheEntry {
	if existing, exists, _ := bc.Get(chainID, address, token); exists {
		existing.Balance = balance
		existing.Unconfirmed = "" // Clear stale unconfirmed data after send
		return *existing
	}
	return cache.BalanceCacheEntry{
		Chain:   chainID,
		Address: address,
		Token:   token,
		Balance: balance,
	}
}

func logCacheError(cc *CommandContext, format string, args ...any) {
	if cc.Log != nil {
		cc.Log.Error(format, args...)
	}
}
