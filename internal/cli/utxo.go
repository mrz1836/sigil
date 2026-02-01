package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	// utxoWallet is the wallet name for UTXO operations.
	utxoWallet string
	// utxoChain is the chain to list UTXOs for.
	utxoChain string
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

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(utxoCmd)
	utxoCmd.AddCommand(utxoListCmd)

	utxoListCmd.Flags().StringVar(&utxoWallet, "wallet", "", "wallet name (required)")
	utxoListCmd.Flags().StringVar(&utxoChain, "chain", "bsv", "blockchain (only bsv supported)")

	_ = utxoListCmd.MarkFlagRequired("wallet")
}

//nolint:gocognit,gocyclo // Display logic for UTXO list is complex
func runUTXOList(cmd *cobra.Command, _ []string) error {
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
	storage := wallet.NewFileStorage(filepath.Join(cfg.Home, "wallets"))
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
		APIKey: cfg.Networks.BSV.APIKey,
	})

	// List UTXOs
	utxos, err := client.ListUTXOs(ctx, address)
	if err != nil {
		return fmt.Errorf("listing UTXOs: %w", err)
	}

	// Display results
	w := cmd.OutOrStdout()
	format := formatter.Format()

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
