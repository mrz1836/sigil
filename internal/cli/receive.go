package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	// receiveWallet is the wallet name.
	receiveWallet string
	// receiveChain is the blockchain to show address for.
	receiveChain string
	// receiveNew forces generation of a new address.
	receiveNew bool
	// receiveLabel sets a label for the address.
	receiveLabel string
)

// receiveCmd shows a receiving address for a wallet.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Show a receiving address",
	Long: `Display a receiving address for your wallet.

By default, shows the first unused address. Use --new to force generation
of a new address even if the current one hasn't been used yet.

Examples:
  # Show next unused BSV receiving address
  sigil receive --wallet main --chain bsv

  # Generate a new address with a label
  sigil receive --wallet main --chain bsv --new --label "Payment from Alice"`,
	RunE: runReceive,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(receiveCmd)

	receiveCmd.Flags().StringVarP(&receiveWallet, "wallet", "w", "", "wallet name (required)")
	receiveCmd.Flags().StringVarP(&receiveChain, "chain", "c", "bsv", "blockchain: eth, bsv")
	receiveCmd.Flags().BoolVar(&receiveNew, "new", false, "force generation of a new address")
	receiveCmd.Flags().StringVarP(&receiveLabel, "label", "l", "", "label for the address")

	_ = receiveCmd.MarkFlagRequired("wallet")
}

//nolint:gocognit,gocyclo // CLI flow involves multiple validation and setup steps
func runReceive(cmd *cobra.Command, _ []string) error {
	cmdCtx := GetCmdContext(cmd)

	// Validate chain
	chainID, ok := chain.ParseChainID(receiveChain)
	if !ok || !chainID.IsMVP() {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("invalid chain: %s (use eth or bsv)", receiveChain),
		)
	}

	// Load wallet
	storage := wallet.NewFileStorage(filepath.Join(cmdCtx.Cfg.GetHome(), "wallets"))
	wlt, seed, err := loadWalletWithSession(receiveWallet, storage, cmd)
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(seed)

	// Load UTXO store to check address activity
	utxoStorePath := filepath.Join(cmdCtx.Cfg.GetHome(), "wallets", receiveWallet)
	store := utxostore.New(utxoStorePath)
	if loadErr := store.Load(); loadErr != nil {
		return fmt.Errorf("loading UTXO store: %w", loadErr)
	}

	// Find the appropriate address
	var addr *wallet.Address
	var isNew bool

	//nolint:nestif // Address derivation logic requires conditional nesting
	if receiveNew {
		// Force derive a new address
		addr, err = wlt.DeriveNextReceiveAddress(seed, chainID)
		if err != nil {
			return fmt.Errorf("deriving new address: %w", err)
		}
		isNew = true
	} else {
		// Find first unused address
		addr = findUnusedReceiveAddress(wlt, chainID, store)
		if addr == nil {
			// All addresses are used, derive a new one
			addr, err = wlt.DeriveNextReceiveAddress(seed, chainID)
			if err != nil {
				return fmt.Errorf("deriving new address: %w", err)
			}
			isNew = true
		}
	}

	// Register the address in UTXO store if new
	//nolint:nestif // UTXO store operations require conditional nesting
	if isNew {
		store.AddAddress(&utxostore.AddressMetadata{
			Address:        addr.Address,
			ChainID:        chainID,
			DerivationPath: addr.Path,
			Index:          addr.Index,
			Label:          receiveLabel,
			IsChange:       false,
		})
		if err := store.Save(); err != nil {
			return fmt.Errorf("saving UTXO store: %w", err)
		}
	} else if receiveLabel != "" {
		// Update label on existing address
		if err := store.SetAddressLabel(chainID, addr.Address, receiveLabel); err == nil {
			if err := store.Save(); err != nil {
				return fmt.Errorf("saving UTXO store: %w", err)
			}
		}
	}

	// Get label from store if not setting a new one
	label := receiveLabel
	if label == "" {
		if meta := store.GetAddress(chainID, addr.Address); meta != nil {
			label = meta.Label
		}
	}

	// Display result
	if cmdCtx.Fmt.Format() == output.FormatJSON {
		displayReceiveJSON(cmd, addr, chainID, label, isNew)
	} else {
		displayReceiveText(cmd, addr, chainID, label, isNew)
	}

	return nil
}

// findUnusedReceiveAddress returns the first receiving address with no activity.
func findUnusedReceiveAddress(wlt *wallet.Wallet, chainID chain.ID, store *utxostore.Store) *wallet.Address {
	addresses := wlt.Addresses[chainID]
	for i := range addresses {
		addr := &addresses[i]
		meta := store.GetAddress(chainID, addr.Address)
		if meta == nil || !meta.HasActivity {
			return addr
		}
	}
	return nil
}

// displayReceiveText shows the receiving address in text format.
func displayReceiveText(cmd *cobra.Command, addr *wallet.Address, chainID chain.ID, label string, isNew bool) {
	w := cmd.OutOrStdout()

	outln(w)
	if isNew {
		outln(w, "New receiving address generated:")
	} else {
		outln(w, "Receiving address:")
	}
	outln(w)
	out(w, "  Chain:   %s\n", chainID)
	out(w, "  Address: %s\n", addr.Address)
	out(w, "  Path:    %s\n", addr.Path)
	out(w, "  Index:   %d\n", addr.Index)
	if label != "" {
		out(w, "  Label:   %s\n", label)
	}
	outln(w)

	// Show explorer link based on chain
	switch chainID {
	case chain.BSV:
		outln(w, "View on WhatsOnChain:")
		out(w, "  https://whatsonchain.com/address/%s\n", addr.Address)
	case chain.ETH:
		outln(w, "View on Etherscan:")
		out(w, "  https://etherscan.io/address/%s\n", addr.Address)
	case chain.BTC, chain.BCH:
		// Future chains - no explorer link yet
	}
}

// displayReceiveJSON shows the receiving address in JSON format.
func displayReceiveJSON(cmd *cobra.Command, addr *wallet.Address, chainID chain.ID, label string, isNew bool) {
	w := cmd.OutOrStdout()

	outln(w, "{")
	out(w, `  "chain": "%s",`+"\n", chainID)
	out(w, `  "address": "%s",`+"\n", addr.Address)
	out(w, `  "path": "%s",`+"\n", addr.Path)
	out(w, `  "index": %d,`+"\n", addr.Index)
	if label != "" {
		out(w, `  "label": "%s",`+"\n", label)
	}
	out(w, `  "is_new": %t`+"\n", isNew)
	outln(w, "}")
}
