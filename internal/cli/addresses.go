package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/cache"
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
	// addressesRefresh forces a fresh fetch, ignoring the cache.
	addressesRefresh bool
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

Balances are fetched live from the network with cache fallback.
Use --refresh to bypass the cache and force a fresh fetch.

Examples:
  # List all BSV addresses
  sigil addresses list --wallet main --chain bsv

  # List only unused receiving addresses
  sigil addresses list --wallet main --chain bsv --type receive --unused

  # Force fresh balance fetch
  sigil addresses list --wallet main --refresh`,
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
	addressesListCmd.Flags().BoolVar(&addressesRefresh, "refresh", false, "force fresh fetch, ignore cache")
	_ = addressesListCmd.MarkFlagRequired("wallet")

	// Label command flags
	addressesLabelCmd.Flags().StringVarP(&addressesWallet, "wallet", "w", "", "wallet name (required)")
	_ = addressesLabelCmd.MarkFlagRequired("wallet")
}

// addressInfo holds display information for an address.
type addressInfo struct {
	Type        string // "receive" or "change"
	Index       uint32
	Address     string
	Path        string
	Label       string
	Balance     string // formatted confirmed balance (e.g. "0.00070422") or ""
	Unconfirmed string // formatted unconfirmed delta (e.g. "-0.00070422") or ""
	Used        bool
	ChainID     chain.ID
}

//nolint:gocognit,gocyclo // CLI flow involves multiple validation, collection, and fetch steps
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

	// Load UTXO store (for address metadata: labels and HasActivity)
	utxoStorePath := filepath.Join(cmdCtx.Cfg.GetHome(), "wallets", addressesWallet)
	store := utxostore.New(utxoStorePath)
	if loadErr := store.Load(); loadErr != nil {
		return fmt.Errorf("loading UTXO store: %w", loadErr)
	}

	// Load or create balance cache
	cachePath := filepath.Join(cmdCtx.Cfg.GetHome(), "cache", "balances.json")
	cacheStorage := cache.NewFileStorage(cachePath)
	balanceCache := loadOrCreateBalanceCache(cacheStorage, addressesRefresh, cmd, cmdCtx.Log)

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

	// Collect all address info (filter applied after balance enrichment)
	var allAddresses []addressInfo

	for _, chainID := range chains {
		// Collect receive addresses
		if addressesType == "all" || addressesType == "receive" {
			for _, addr := range wlt.Addresses[chainID] {
				info := buildAddressInfo("receive", &addr, chainID, store)
				allAddresses = append(allAddresses, info)
			}
		}

		// Collect change addresses
		if addressesType == "all" || addressesType == "change" {
			if wlt.ChangeAddresses != nil {
				for _, addr := range wlt.ChangeAddresses[chainID] {
					info := buildAddressInfo("change", &addr, chainID, store)
					allAddresses = append(allAddresses, info)
				}
			}
		}
	}

	// Fetch live balances concurrently
	fetchAddressBalances(cmd, allAddresses, balanceCache, cmdCtx.Cfg)

	// Enrich "Used" status from fetched balance data
	for i := range allAddresses {
		if !allAddresses[i].Used {
			allAddresses[i].Used = isNonZeroBalance(allAddresses[i].Balance) || isNonZeroBalance(allAddresses[i].Unconfirmed)
		}
	}

	// Apply --used/--unused filter after balance enrichment
	var filtered []addressInfo
	for _, addr := range allAddresses {
		if shouldIncludeAddress(addr) {
			filtered = append(filtered, addr)
		}
	}
	allAddresses = filtered

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

	// Save updated cache
	if saveErr := cacheStorage.Save(balanceCache); saveErr != nil {
		if cmdCtx.Log != nil {
			cmdCtx.Log.Error("failed to save balance cache: %v", saveErr)
		}
	}

	// Display results
	if cmdCtx.Fmt.Format() == output.FormatJSON {
		displayAddressesJSON(cmd, allAddresses)
	} else {
		displayAddressesText(cmd, allAddresses)
	}

	return nil
}

// loadOrCreateBalanceCache loads the balance cache from storage, or creates a fresh one.
func loadOrCreateBalanceCache(storage *cache.FileStorage, refresh bool, cmd *cobra.Command, log LogWriter) *cache.BalanceCache {
	if refresh {
		return cache.NewBalanceCache()
	}

	balanceCache, err := storage.Load()
	if err == nil {
		return balanceCache
	}

	if errors.Is(err, cache.ErrCorruptCache) {
		if log != nil {
			log.Error("balance cache file is corrupted: %v", err)
		}
		outln(cmd.ErrOrStderr(), "Warning: balance cache was corrupted and has been reset.")
	} else if log != nil {
		log.Error("failed to load balance cache: %v", err)
	}

	return cache.NewBalanceCache()
}

// fetchAddressBalances fetches live balances for all addresses concurrently.
//
//nolint:gocognit // Concurrent fetch logic requires nested control flow
func fetchAddressBalances(cmd *cobra.Command, addresses []addressInfo, balanceCache *cache.BalanceCache, cfg ConfigProvider) {
	if len(addresses) == 0 {
		return
	}

	const perAddressTimeout = 30 * time.Second
	const maxConcurrent = 8

	ctx, cancel := contextWithTimeout(cmd, 60*time.Second)
	defer cancel()

	type balanceTask struct {
		chainID chain.ID
		address string
	}

	// Deduplicate addresses per chain
	seen := make(map[string]bool)
	var tasks []balanceTask
	for _, addr := range addresses {
		key := string(addr.ChainID) + ":" + addr.Address
		if !seen[key] {
			seen[key] = true
			tasks = append(tasks, balanceTask{chainID: addr.ChainID, address: addr.Address})
		}
	}

	type fetchResult struct {
		balance     string
		unconfirmed string
	}
	results := make(map[string]*fetchResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)

	for _, task := range tasks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			addrCtx, addrCancel := context.WithTimeout(ctx, perAddressTimeout)
			entries, _, _ := fetchBalancesForAddress(addrCtx, task.chainID, task.address, balanceCache, cfg)
			addrCancel()

			mu.Lock()
			defer mu.Unlock()
			key := string(task.chainID) + ":" + task.address
			if len(entries) > 0 {
				results[key] = &fetchResult{
					balance:     entries[0].Balance,
					unconfirmed: entries[0].Unconfirmed,
				}
			}
		}()
	}
	wg.Wait()

	// Populate balance fields on addresses from results
	for i := range addresses {
		key := string(addresses[i].ChainID) + ":" + addresses[i].Address
		if r, exists := results[key]; exists {
			addresses[i].Balance = r.balance
			addresses[i].Unconfirmed = r.unconfirmed
		}
	}
}

func buildAddressInfo(addrType string, addr *wallet.Address, chainID chain.ID, store *utxostore.Store) addressInfo {
	info := addressInfo{
		Type:    addrType,
		Index:   addr.Index,
		Address: addr.Address,
		Path:    addr.Path,
		ChainID: chainID,
		// Balance and Unconfirmed are populated after network fetch
	}

	// Get metadata from UTXO store (label and HasActivity)
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

// isNonZeroBalance returns true if the balance string represents a non-zero amount.
func isNonZeroBalance(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c != '0' && c != '.' && c != '-' {
			return true
		}
	}
	return false
}

// hasUnconfirmedAddressData returns true if any address has non-empty unconfirmed data.
func hasUnconfirmedAddressData(addresses []addressInfo) bool {
	for _, addr := range addresses {
		if addr.Unconfirmed != "" {
			return true
		}
	}
	return false
}

func displayAddressesText(cmd *cobra.Command, addresses []addressInfo) {
	w := cmd.OutOrStdout()

	if len(addresses) == 0 {
		outln(w, "No addresses found matching the criteria.")
		return
	}

	outln(w)
	outln(w, "Addresses:")
	outln(w)

	if hasUnconfirmedAddressData(addresses) {
		displayAddressesTextWide(w, addresses)
	} else {
		displayAddressesTextNarrow(w, addresses)
	}

	outln(w)
}

// displayAddressesTextNarrow renders the address table without unconfirmed column.
func displayAddressesTextNarrow(w io.Writer, addresses []addressInfo) {
	outln(w, "  Type     Index  Address                                      Label           Balance          Status")
	outln(w, "  ───────  ─────  ───────────────────────────────────────────  ──────────────  ───────────────  ──────")

	currentChain := chain.ID("")
	for _, addr := range addresses {
		if addr.ChainID != currentChain {
			if currentChain != "" {
				outln(w)
			}
			out(w, "  [%s]\n", strings.ToUpper(string(addr.ChainID)))
			currentChain = addr.ChainID
		}

		out(w, "  %-7s  %5d  %-42s  %-14s  %15s  %s\n",
			addr.Type, addr.Index, truncateAddressDisplay(addr.Address),
			formatLabel(addr.Label), formatBalanceDisplay(addr.Balance), formatStatus(addr.Used))
	}
}

// displayAddressesTextWide renders the address table with confirmed and unconfirmed columns.
func displayAddressesTextWide(w io.Writer, addresses []addressInfo) {
	outln(w, "  Type     Index  Address                                      Label           Confirmed        Unconfirmed      Status")
	outln(w, "  ───────  ─────  ───────────────────────────────────────────  ──────────────  ───────────────  ───────────────  ──────")

	currentChain := chain.ID("")
	for _, addr := range addresses {
		if addr.ChainID != currentChain {
			if currentChain != "" {
				outln(w)
			}
			out(w, "  [%s]\n", strings.ToUpper(string(addr.ChainID)))
			currentChain = addr.ChainID
		}

		out(w, "  %-7s  %5d  %-42s  %-14s  %15s  %15s  %s\n",
			addr.Type, addr.Index, truncateAddressDisplay(addr.Address),
			formatLabel(addr.Label), formatBalanceDisplay(addr.Balance),
			formatBalanceDisplay(addr.Unconfirmed), formatStatus(addr.Used))
	}
}

// truncateAddressDisplay shortens an address for table display.
func truncateAddressDisplay(addr string) string {
	if len(addr) > 42 {
		return addr[:20] + "..." + addr[len(addr)-19:]
	}
	return addr
}

// formatLabel formats a label for display, truncating if needed.
func formatLabel(label string) string {
	if len(label) > 14 {
		label = label[:11] + "..."
	}
	if label == "" {
		label = "-"
	}
	return label
}

// formatBalanceDisplay formats a balance string for display.
func formatBalanceDisplay(balance string) string {
	if balance == "" {
		return "-"
	}
	return balance
}

// formatStatus returns "used" or "unused" for display.
func formatStatus(used bool) string {
	if used {
		return "used"
	}
	return "unused"
}

func displayAddressesJSON(cmd *cobra.Command, addresses []addressInfo) {
	type addressJSON struct {
		Chain       string `json:"chain"`
		Type        string `json:"type"`
		Index       uint32 `json:"index"`
		Address     string `json:"address"`
		Path        string `json:"path"`
		Label       string `json:"label"`
		Balance     string `json:"balance"`
		Unconfirmed string `json:"unconfirmed,omitempty"`
		Used        bool   `json:"used"`
	}
	type responseJSON struct {
		Addresses []addressJSON `json:"addresses"`
	}

	resp := responseJSON{Addresses: make([]addressJSON, 0, len(addresses))}
	for _, addr := range addresses {
		resp.Addresses = append(resp.Addresses, addressJSON{
			Chain:       string(addr.ChainID),
			Type:        addr.Type,
			Index:       addr.Index,
			Address:     addr.Address,
			Path:        addr.Path,
			Label:       addr.Label,
			Balance:     addr.Balance,
			Unconfirmed: addr.Unconfirmed,
			Used:        addr.Used,
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
