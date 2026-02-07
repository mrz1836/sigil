package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	// addressesWallet is the wallet name.
	addressesWallet string
	// addressesChain is the blockchain filter.
	addressesChain string
	// addressesType is the address type filter (receive, change, all).
	addressesType string
	// addressesUsed filters to show only used addresses.
	addressesUsed bool
	// addressesUnused filters to show only unused addresses.
	addressesUnused bool
)

// addressesCmd is the parent command for address operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var addressesCmd = &cobra.Command{
	Use:   "addresses",
	Short: "Manage and view addresses",
	Long:  `View, filter, and manage wallet addresses.`,
}

// addressesListCmd lists all addresses in a wallet.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var addressesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List addresses in a wallet",
	Long: `List all addresses in a wallet with their status and balance.

Examples:
  # List all BSV addresses
  sigil addresses list --wallet main --chain bsv

  # List only unused receiving addresses
  sigil addresses list --wallet main --chain bsv --type receive --unused`,
	RunE: runAddressesList,
}

// addressesLabelCmd sets a label on an address.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var addressesLabelCmd = &cobra.Command{
	Use:   "label <address> <label>",
	Short: "Set a label on an address",
	Long: `Set or update the label for an address.

Examples:
  # Set a label
  sigil addresses label 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa "Savings" --wallet main

  # Clear a label (empty string)
  sigil addresses label 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa "" --wallet main`,
	Args: cobra.ExactArgs(2),
	RunE: runAddressesLabel,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(addressesCmd)
	addressesCmd.AddCommand(addressesListCmd)
	addressesCmd.AddCommand(addressesLabelCmd)

	// List command flags
	addressesListCmd.Flags().StringVarP(&addressesWallet, "wallet", "w", "", "wallet name (required)")
	addressesListCmd.Flags().StringVarP(&addressesChain, "chain", "c", "", "filter by chain (eth, bsv)")
	addressesListCmd.Flags().StringVarP(&addressesType, "type", "t", "all", "filter: receive, change, all")
	addressesListCmd.Flags().BoolVar(&addressesUsed, "used", false, "show only used addresses")
	addressesListCmd.Flags().BoolVar(&addressesUnused, "unused", false, "show only unused addresses")
	_ = addressesListCmd.MarkFlagRequired("wallet")

	// Label command flags
	addressesLabelCmd.Flags().StringVarP(&addressesWallet, "wallet", "w", "", "wallet name (required)")
	_ = addressesLabelCmd.MarkFlagRequired("wallet")
}

// addressInfo holds display information for an address.
type addressInfo struct {
	Type    string // "receive" or "change"
	Index   uint32
	Address string
	Path    string
	Label   string
	Balance uint64
	Used    bool
	ChainID chain.ID
}

//nolint:gocognit,gocyclo // CLI flow involves multiple validation and collection steps
func runAddressesList(cmd *cobra.Command, _ []string) error {
	cmdCtx := GetCmdContext(cmd)

	// Validate type filter
	addressesType = strings.ToLower(addressesType)
	if addressesType != "all" && addressesType != "receive" && addressesType != "change" {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"--type must be: receive, change, or all",
		)
	}

	// Cannot use both --used and --unused
	if addressesUsed && addressesUnused {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"cannot use both --used and --unused flags",
		)
	}

	// Load wallet
	storage := wallet.NewFileStorage(filepath.Join(cmdCtx.Cfg.GetHome(), "wallets"))
	wlt, seed, err := loadWalletWithSession(addressesWallet, storage, cmd)
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(seed)

	// Load UTXO store
	utxoStorePath := filepath.Join(cmdCtx.Cfg.GetHome(), "wallets", addressesWallet)
	store := utxostore.New(utxoStorePath)
	if err := store.Load(); err != nil {
		return fmt.Errorf("loading UTXO store: %w", err)
	}

	// Determine which chains to show
	var chains []chain.ID
	if addressesChain != "" {
		chainID, ok := chain.ParseChainID(addressesChain)
		if !ok || !chainID.IsMVP() {
			return sigilerr.WithSuggestion(
				sigilerr.ErrInvalidInput,
				fmt.Sprintf("invalid chain: %s (use eth or bsv)", addressesChain),
			)
		}
		chains = []chain.ID{chainID}
	} else {
		chains = wlt.EnabledChains
	}

	// Collect all address info
	var allAddresses []addressInfo

	for _, chainID := range chains {
		// Collect receive addresses
		if addressesType == "all" || addressesType == "receive" {
			for _, addr := range wlt.Addresses[chainID] {
				info := buildAddressInfo("receive", &addr, chainID, store)
				if shouldIncludeAddress(info) {
					allAddresses = append(allAddresses, info)
				}
			}
		}

		// Collect change addresses
		if addressesType == "all" || addressesType == "change" {
			if wlt.ChangeAddresses != nil {
				for _, addr := range wlt.ChangeAddresses[chainID] {
					info := buildAddressInfo("change", &addr, chainID, store)
					if shouldIncludeAddress(info) {
						allAddresses = append(allAddresses, info)
					}
				}
			}
		}
	}

	// Sort by chain, type, index
	sort.Slice(allAddresses, func(i, j int) bool {
		if allAddresses[i].ChainID != allAddresses[j].ChainID {
			return allAddresses[i].ChainID < allAddresses[j].ChainID
		}
		if allAddresses[i].Type != allAddresses[j].Type {
			return allAddresses[i].Type < allAddresses[j].Type
		}
		return allAddresses[i].Index < allAddresses[j].Index
	})

	// Display results
	if cmdCtx.Fmt.Format() == output.FormatJSON {
		displayAddressesJSON(cmd, allAddresses)
	} else {
		displayAddressesText(cmd, allAddresses)
	}

	return nil
}

func buildAddressInfo(addrType string, addr *wallet.Address, chainID chain.ID, store *utxostore.Store) addressInfo {
	info := addressInfo{
		Type:    addrType,
		Index:   addr.Index,
		Address: addr.Address,
		Path:    addr.Path,
		ChainID: chainID,
		Balance: store.GetAddressBalance(chainID, addr.Address),
	}

	// Get metadata from UTXO store
	if meta := store.GetAddress(chainID, addr.Address); meta != nil {
		info.Label = meta.Label
		info.Used = meta.HasActivity
	}

	return info
}

func shouldIncludeAddress(info addressInfo) bool {
	if addressesUsed && !info.Used {
		return false
	}
	if addressesUnused && info.Used {
		return false
	}
	return true
}

//nolint:gocognit // Display formatting requires multiple conditional branches
func displayAddressesText(cmd *cobra.Command, addresses []addressInfo) {
	w := cmd.OutOrStdout()

	if len(addresses) == 0 {
		outln(w, "No addresses found matching the criteria.")
		return
	}

	outln(w)
	outln(w, "Addresses:")
	outln(w)

	// Table header
	outln(w, "  Type     Index  Address                                      Label           Balance    Status")
	outln(w, "  ───────  ─────  ───────────────────────────────────────────  ──────────────  ─────────  ──────")

	currentChain := chain.ID("")
	for _, addr := range addresses {
		// Print chain header if changed
		if addr.ChainID != currentChain {
			if currentChain != "" {
				outln(w)
			}
			out(w, "  [%s]\n", strings.ToUpper(string(addr.ChainID)))
			currentChain = addr.ChainID
		}

		// Truncate address for display
		displayAddr := addr.Address
		if len(displayAddr) > 42 {
			displayAddr = displayAddr[:20] + "..." + displayAddr[len(displayAddr)-19:]
		}

		// Format label (truncate if needed)
		label := addr.Label
		if len(label) > 14 {
			label = label[:11] + "..."
		}
		if label == "" {
			label = "-"
		}

		// Format balance
		var balanceStr string
		if addr.Balance > 0 {
			balanceStr = formatSatoshis(addr.Balance)
		} else {
			balanceStr = "-"
		}

		// Status
		status := "unused"
		if addr.Used {
			status = "used"
		}

		out(w, "  %-7s  %5d  %-42s  %-14s  %9s  %s\n",
			addr.Type, addr.Index, displayAddr, label, balanceStr, status)
	}

	outln(w)
}

func displayAddressesJSON(cmd *cobra.Command, addresses []addressInfo) {
	type addressJSON struct {
		Chain   string `json:"chain"`
		Type    string `json:"type"`
		Index   uint32 `json:"index"`
		Address string `json:"address"`
		Path    string `json:"path"`
		Label   string `json:"label"`
		Balance uint64 `json:"balance"`
		Used    bool   `json:"used"`
	}
	type responseJSON struct {
		Addresses []addressJSON `json:"addresses"`
	}

	resp := responseJSON{Addresses: make([]addressJSON, 0, len(addresses))}
	for _, addr := range addresses {
		resp.Addresses = append(resp.Addresses, addressJSON{
			Chain:   string(addr.ChainID),
			Type:    addr.Type,
			Index:   addr.Index,
			Address: addr.Address,
			Path:    addr.Path,
			Label:   addr.Label,
			Balance: addr.Balance,
			Used:    addr.Used,
		})
	}

	_ = writeJSON(cmd.OutOrStdout(), resp)
}

// formatSatoshis formats satoshis as a human-readable string.
func formatSatoshis(sats uint64) string {
	if sats >= 100000000 { // 1 BSV
		return fmt.Sprintf("%.4f", float64(sats)/100000000)
	}
	return fmt.Sprintf("%d sat", sats)
}

func runAddressesLabel(cmd *cobra.Command, args []string) error {
	cmdCtx := GetCmdContext(cmd)
	address := args[0]
	label := args[1]

	// Load UTXO store
	utxoStorePath := filepath.Join(cmdCtx.Cfg.GetHome(), "wallets", addressesWallet)
	store := utxostore.New(utxoStorePath)
	if err := store.Load(); err != nil {
		return fmt.Errorf("loading UTXO store: %w", err)
	}

	// Try to find the address in both chains
	var found bool
	for _, chainID := range []chain.ID{chain.BSV, chain.ETH} {
		if err := store.SetAddressLabel(chainID, address, label); err == nil {
			found = true
			break
		}
	}

	if !found {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			fmt.Sprintf("address not found in wallet: %s", address),
		)
	}

	// Save the store
	if err := store.Save(); err != nil {
		return fmt.Errorf("saving UTXO store: %w", err)
	}

	// Display confirmation
	w := cmd.OutOrStdout()
	if label == "" {
		out(w, "Label cleared for address %s\n", address)
	} else {
		out(w, "Label set to \"%s\" for address %s\n", label, address)
	}

	return nil
}
