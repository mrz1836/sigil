package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	// utxoWallet is the wallet name for UTXO operations.
	utxoWallet string
	// utxoChain is the chain to list UTXOs for.
	utxoChain string
	// utxoAddresses is a list of specific addresses to refresh.
	utxoAddresses []string
)

// utxoCmd is the parent command for UTXO operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var utxoCmd = &cobra.Command{
	Use:   "utxo",
	Short: "Manage UTXOs",
	Long:  `List and manage unspent transaction outputs (UTXOs) for BSV wallets.`,
}

// utxoListCmd lists UTXOs for a wallet.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var utxoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List UTXOs for a wallet",
	Long: `List all unspent transaction outputs (UTXOs) for a BSV wallet address.

Example:
  sigil utxo list --wallet main
  sigil utxo list --wallet main -o json`,
	RunE: runUTXOList,
}

// utxoRefreshCmd refreshes UTXOs from chain.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var utxoRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh UTXOs from chain",
	Long: `Re-scan all known addresses and update stored UTXOs.
New UTXOs are added; spent UTXOs are marked as spent.

Use --address to refresh specific addresses instead of all.

Example:
  sigil utxo refresh --wallet main
  sigil utxo refresh --wallet main --address 1ABC...
  sigil utxo refresh --wallet main --address 1ABC... --address 1XYZ...`,
	RunE: runUTXORefresh,
}

// utxoBalanceCmd shows offline balance from stored UTXOs.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var utxoBalanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Show offline balance from stored UTXOs",
	Long: `Display balance calculated from locally stored UTXOs.
No network connection required after initial scan.

Example:
  sigil utxo balance --wallet main`,
	RunE: runUTXOBalance,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(utxoCmd)
	utxoCmd.AddCommand(utxoListCmd)
	utxoCmd.AddCommand(utxoRefreshCmd)
	utxoCmd.AddCommand(utxoBalanceCmd)

	// utxo list flags
	utxoListCmd.Flags().StringVar(&utxoWallet, "wallet", "", "wallet name (required)")
	utxoListCmd.Flags().StringVar(&utxoChain, "chain", "bsv", "blockchain (only bsv supported)")
	_ = utxoListCmd.MarkFlagRequired("wallet")

	// utxo refresh flags
	utxoRefreshCmd.Flags().StringVar(&utxoWallet, "wallet", "", "wallet name (required)")
	utxoRefreshCmd.Flags().StringArrayVar(&utxoAddresses, "address", nil, "specific address(es) to refresh (optional, repeatable)")
	_ = utxoRefreshCmd.MarkFlagRequired("wallet")

	// utxo balance flags
	utxoBalanceCmd.Flags().StringVar(&utxoWallet, "wallet", "", "wallet name (required)")
	_ = utxoBalanceCmd.MarkFlagRequired("wallet")
}

//nolint:gocognit,gocyclo // Display logic for UTXO list is complex
func runUTXOList(cmd *cobra.Command, _ []string) error {
	cmdCtx := GetCmdContext(cmd) //nolint:govet // shadows package-level cmdCtx; consistent with addresses.go, balance.go
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Only BSV is supported for UTXOs
	if utxoChain != "bsv" {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"UTXO operations only supported for BSV chain",
		)
	}

	// Load wallet
	storage := wallet.NewFileStorage(filepath.Join(cmdCtx.Cfg.GetHome(), "wallets"))
	exists, err := storage.Exists(utxoWallet)
	if err != nil {
		return err
	}
	if !exists {
		return sigilerr.WithSuggestion(
			wallet.ErrWalletNotFound,
			fmt.Sprintf("wallet '%s' not found. List wallets with: sigil wallet list", utxoWallet),
		)
	}

	// Load wallet to get address (no password needed for public addresses)
	password, err := promptPassword("Enter wallet password: ")
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(password)

	wlt, seed, err := storage.Load(utxoWallet, password)
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(seed)

	// Get BSV address
	bsvAddresses, ok := wlt.Addresses[wallet.ChainBSV]
	if !ok || len(bsvAddresses) == 0 {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("wallet '%s' has no BSV addresses", utxoWallet),
		)
	}
	address := bsvAddresses[0].Address

	// Create BSV client
	client := bsv.NewClient(&bsv.ClientOptions{
		APIKey: cmdCtx.Cfg.GetBSVAPIKey(),
	})

	// List UTXOs
	utxos, err := client.ListUTXOs(ctx, address)
	if err != nil {
		return fmt.Errorf("listing UTXOs: %w", err)
	}

	// Display results
	w := cmd.OutOrStdout()
	format := cmdCtx.Fmt.Format()

	if len(utxos) == 0 {
		if format == output.FormatJSON {
			outln(w, "[]")
		} else {
			out(w, "No UTXOs found for address %s\n", address)
		}
		return nil
	}

	if format == output.FormatJSON {
		displayUTXOsJSON(w, utxos)
	} else {
		displayUTXOsText(w, address, utxos)
	}

	return nil
}

// displayUTXOsText shows UTXOs in text format as a table.
func displayUTXOsText(w interface {
	Write(p []byte) (n int, err error)
}, address string, utxos []bsv.UTXO,
) {
	out(w, "UTXOs for %s\n", address)
	outln(w)
	outln(w, "TXID                                                              VOUT    AMOUNT (sats)  CONFIRMATIONS")
	outln(w, "────────────────────────────────────────────────────────────────  ────    ─────────────  ─────────────")

	var total uint64
	for _, utxo := range utxos {
		out(w, "%-64s  %4d    %13d  %13d\n",
			utxo.TxID, utxo.Vout, utxo.Amount, utxo.Confirmations)
		total += utxo.Amount
	}

	outln(w)
	out(w, "Total: %d UTXOs, %d satoshis (%.8f BSV)\n",
		len(utxos), total, float64(total)/100000000)
}

// displayUTXOsJSON shows UTXOs in JSON format.
func displayUTXOsJSON(w interface {
	Write(p []byte) (n int, err error)
}, utxos []bsv.UTXO,
) {
	outln(w, "[")
	for i, utxo := range utxos {
		comma := ","
		if i == len(utxos)-1 {
			comma = ""
		}
		out(w, `  {"txid": "%s", "vout": %d, "amount": %d, "confirmations": %d}%s`+"\n",
			utxo.TxID, utxo.Vout, utxo.Amount, utxo.Confirmations, comma)
	}
	outln(w, "]")
}

// runUTXORefresh re-scans addresses and updates stored UTXOs.
func runUTXORefresh(cmd *cobra.Command, _ []string) error {
	cmdCtx := GetCmdContext(cmd) //nolint:govet // shadows package-level cmdCtx; consistent with addresses.go, balance.go
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Load wallet
	storage := wallet.NewFileStorage(filepath.Join(cmdCtx.Cfg.GetHome(), "wallets"))
	walletPath := filepath.Join(cmdCtx.Cfg.GetHome(), "wallets", utxoWallet)

	exists, err := storage.Exists(utxoWallet)
	if err != nil {
		return err
	}
	if !exists {
		return sigilerr.WithSuggestion(
			wallet.ErrWalletNotFound,
			fmt.Sprintf("wallet '%s' not found. List wallets with: sigil wallet list", utxoWallet),
		)
	}

	// Create UTXO store
	store := utxostore.New(walletPath)
	if loadErr := store.Load(); loadErr != nil {
		return fmt.Errorf("loading UTXO store: %w", loadErr)
	}

	// Create BSV client
	client := bsv.NewClient(&bsv.ClientOptions{
		APIKey: cmdCtx.Cfg.GetBSVAPIKey(),
	})

	// Create adapter for refresh
	adapter := &bsvRefreshAdapter{client: client}

	w := cmd.OutOrStdout()

	// If specific addresses provided, refresh only those
	if len(utxoAddresses) > 0 {
		return refreshSpecificAddresses(ctx, cmd, store, adapter, utxoAddresses)
	}

	// Check if store has addresses to refresh
	addresses := store.GetAddresses(chain.BSV)
	if len(addresses) == 0 {
		out(w, "No addresses found in UTXO store for wallet '%s'.\n", utxoWallet)
		out(w, "Run 'sigil wallet restore --scan' to scan addresses first.\n")
		return nil
	}

	// Run refresh for all addresses
	out(w, "Refreshing UTXOs for wallet '%s'...\n", utxoWallet)

	result, err := store.Refresh(ctx, chain.BSV, adapter)
	if err != nil {
		return fmt.Errorf("refreshing UTXOs: %w", err)
	}

	// Display results
	displayRefreshResults(w, result)
	return nil
}

// refreshSpecificAddresses refreshes UTXOs for specific addresses only.
func refreshSpecificAddresses(ctx context.Context, cmd *cobra.Command, store *utxostore.Store, adapter *bsvRefreshAdapter, addresses []string) error {
	w := cmd.OutOrStdout()

	out(w, "Refreshing %d specific address(es) for wallet '%s'...\n", len(addresses), utxoWallet)

	// Aggregate results from all addresses
	totalResult := &utxostore.ScanResult{}

	for _, addr := range addresses {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		out(w, "  Scanning %s...\n", addr)
		result, err := store.RefreshAddress(ctx, addr, chain.BSV, adapter)
		if err != nil {
			return fmt.Errorf("refreshing address %s: %w", addr, err)
		}

		// Aggregate results
		totalResult.AddressesScanned += result.AddressesScanned
		totalResult.UTXOsFound += result.UTXOsFound
		totalResult.TotalBalance += result.TotalBalance
		totalResult.Errors = append(totalResult.Errors, result.Errors...)
	}

	// Display aggregated results
	displayRefreshResults(w, totalResult)
	return nil
}

// bsvRefreshAdapter adapts bsv.Client to utxostore.ChainClient interface.
type bsvRefreshAdapter struct {
	client *bsv.Client
}

// ListUTXOs implements utxostore.ChainClient.
func (a *bsvRefreshAdapter) ListUTXOs(ctx context.Context, address string) ([]chain.UTXO, error) {
	utxos, err := a.client.ListUTXOs(ctx, address)
	if err != nil {
		return nil, err
	}

	result := make([]chain.UTXO, len(utxos))
	for i, u := range utxos {
		result[i] = chain.UTXO{
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			ScriptPubKey:  u.ScriptPubKey,
			Address:       address,
			Confirmations: u.Confirmations,
		}
	}
	return result, nil
}

// displayRefreshResults shows the results of a UTXO refresh.
func displayRefreshResults(w interface {
	Write(p []byte) (n int, err error)
}, result *utxostore.ScanResult,
) {
	outln(w)
	out(w, "Addresses scanned: %d\n", result.AddressesScanned)
	out(w, "UTXOs found:       %d\n", result.UTXOsFound)
	out(w, "Total balance:     %d satoshis (%.8f BSV)\n",
		result.TotalBalance, float64(result.TotalBalance)/100000000)

	if len(result.Errors) > 0 {
		outln(w)
		out(w, "Errors (%d):\n", len(result.Errors))
		for _, e := range result.Errors {
			out(w, "  - %s\n", e.Error())
		}
	}
}

// runUTXOBalance shows offline balance from stored UTXOs.
func runUTXOBalance(cmd *cobra.Command, _ []string) error {
	cmdCtx := GetCmdContext(cmd) //nolint:govet // shadows package-level cmdCtx; consistent with addresses.go, balance.go

	// Load wallet path
	storage := wallet.NewFileStorage(filepath.Join(cmdCtx.Cfg.GetHome(), "wallets"))
	walletPath := filepath.Join(cmdCtx.Cfg.GetHome(), "wallets", utxoWallet)

	exists, err := storage.Exists(utxoWallet)
	if err != nil {
		return err
	}
	if !exists {
		return sigilerr.WithSuggestion(
			wallet.ErrWalletNotFound,
			fmt.Sprintf("wallet '%s' not found. List wallets with: sigil wallet list", utxoWallet),
		)
	}

	// Load UTXO store
	store := utxostore.New(walletPath)
	if err := store.Load(); err != nil {
		return fmt.Errorf("loading UTXO store: %w", err)
	}

	w := cmd.OutOrStdout()
	format := cmdCtx.Fmt.Format()

	if store.IsEmpty() {
		if format == output.FormatJSON {
			outln(w, `{"balance": 0, "utxos": 0, "note": "no UTXOs stored"}`)
		} else {
			out(w, "No UTXOs stored for wallet '%s'.\n", utxoWallet)
			out(w, "Run 'sigil utxo refresh --wallet %s' to fetch UTXOs from chain.\n", utxoWallet)
		}
		return nil
	}

	// Get balance from stored UTXOs
	balance := store.GetBalance(chain.BSV)
	utxos := store.GetUTXOs(chain.BSV, "")

	if format == output.FormatJSON {
		out(w, `{"balance": %d, "utxos": %d, "bsv": %.8f}`+"\n",
			balance, len(utxos), float64(balance)/100000000)
	} else {
		out(w, "Offline Balance for wallet '%s'\n", utxoWallet)
		outln(w)
		out(w, "UTXOs:   %d\n", len(utxos))
		out(w, "Balance: %d satoshis (%.8f BSV)\n", balance, float64(balance)/100000000)
		outln(w)
		out(w, "Note: This is the locally stored balance. Run 'sigil utxo refresh' to update.\n")
	}

	return nil
}
