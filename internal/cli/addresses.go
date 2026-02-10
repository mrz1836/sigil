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
	"github.com/mrz1836/sigil/internal/chain/bsv"
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
	// addressesRefreshAddresses is a list of specific addresses to refresh.
	addressesRefreshAddresses []string
)

// addressesCmd is the parent command for address operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var addressesCmd = &cobra.Command{
	Use:   "addresses",
	Short: "Manage and view addresses",
	Long: `View, filter, and manage wallet addresses.

List addresses with balances, set labels, and refresh data from the network.
Supports filtering by chain, address type (receive/change), and usage status.`,
}

// addressesListCmd lists all addresses in a wallet.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var addressesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List addresses in a wallet",
	Long: `List all addresses in a wallet with their status and balance.

Balances are fetched live from the network with cache fallback.
Use --refresh to bypass the cache and force a fresh fetch.`,
	Example: `  # List all BSV addresses
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
	Long:  `Set or update the label for an address.`,
	Example: `  # Set a label
  sigil addresses label 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa "Savings" --wallet main

  # Clear a label (empty string)
  sigil addresses label 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa "" --wallet main`,
	Args: cobra.ExactArgs(2),
	RunE: runAddressesLabel,
}

// addressesRefreshCmd refreshes balance and UTXO data from the network.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var addressesRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh address balances from the network",
	Long: `Refresh balance and UTXO data for wallet addresses from the blockchain.

For BSV addresses: re-scans UTXOs via WhatsOnChain and updates the balance cache.
For ETH addresses: fetches fresh balances via the configured provider and updates the balance cache.

By default, refreshes all addresses. Use --address to target specific addresses.
Use --chain to filter by blockchain.`,
	Example: `  # Refresh all addresses
  sigil addresses refresh --wallet main

  # Refresh BSV addresses only
  sigil addresses refresh --wallet main --chain bsv

  # Refresh specific addresses
  sigil addresses refresh --wallet main --address 1ABC... --address 1XYZ...

  # JSON output
  sigil addresses refresh --wallet main -o json`,
	RunE: runAddressesRefresh,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	addressesCmd.GroupID = "wallet"
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

	// Refresh command
	addressesCmd.AddCommand(addressesRefreshCmd)
	addressesRefreshCmd.Flags().StringVarP(&addressesWallet, "wallet", "w", "", "wallet name (required)")
	addressesRefreshCmd.Flags().StringVarP(&addressesChain, "chain", "c", "", "filter by chain (eth, bsv)")
	addressesRefreshCmd.Flags().StringArrayVar(&addressesRefreshAddresses, "address", nil, "specific address(es) to refresh (optional, repeatable)")
	_ = addressesRefreshCmd.MarkFlagRequired("wallet")
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

// refreshTarget identifies a single address to refresh.
type refreshTarget struct {
	address string
	chainID chain.ID
}

//nolint:gocognit,gocyclo // CLI flow involves validation, chain-specific refresh, and display steps
func runAddressesRefresh(cmd *cobra.Command, _ []string) error {
	cmdCtx := GetCmdContext(cmd)
	w := cmd.OutOrStdout()

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
	if loadErr := store.Load(); loadErr != nil {
		return fmt.Errorf("loading UTXO store: %w", loadErr)
	}

	// Create fresh balance cache (refresh always bypasses existing cache)
	cachePath := filepath.Join(cmdCtx.Cfg.GetHome(), "cache", "balances.json")
	cacheStorage := cache.NewFileStorage(cachePath)
	balanceCache := loadOrCreateBalanceCache(cacheStorage, true, cmd, cmdCtx.Log)

	// Determine which chains to refresh
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

	// Build target list
	targets, err := buildRefreshTargets(wlt, chains, addressesRefreshAddresses)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		out(w, "No addresses found to refresh.\n")
		return nil
	}

	ctx, cancel := contextWithTimeout(cmd, 60*time.Second)
	defer cancel()

	out(w, "Refreshing %d address(es) for wallet '%s'...\n", len(targets), addressesWallet)

	// Refresh addresses by chain
	refreshErrors := refreshTargetAddresses(ctx, w, cmdCtx, store, targets, balanceCache)

	// Save balance cache
	if saveErr := cacheStorage.Save(balanceCache); saveErr != nil {
		if cmdCtx.Log != nil {
			cmdCtx.Log.Error("failed to save balance cache: %v", saveErr)
		}
	}

	// Build addressInfo list for display
	targetSet := make(map[string]bool, len(targets))
	for _, t := range targets {
		targetSet[t.address] = true
	}

	var allAddresses []addressInfo
	for _, chainID := range chains {
		for _, addr := range wlt.Addresses[chainID] {
			if targetSet[addr.Address] {
				info := buildAddressInfo("receive", &addr, chainID, store)
				allAddresses = append(allAddresses, info)
			}
		}
		if wlt.ChangeAddresses != nil {
			for _, addr := range wlt.ChangeAddresses[chainID] {
				if targetSet[addr.Address] {
					info := buildAddressInfo("change", &addr, chainID, store)
					allAddresses = append(allAddresses, info)
				}
			}
		}
	}

	// Populate balances from freshly-updated cache
	for i := range allAddresses {
		entry, exists, _ := balanceCache.Get(allAddresses[i].ChainID, allAddresses[i].Address, "")
		if exists {
			allAddresses[i].Balance = entry.Balance
			allAddresses[i].Unconfirmed = entry.Unconfirmed
		}
	}

	// Enrich "Used" status from balance data
	for i := range allAddresses {
		if !allAddresses[i].Used {
			allAddresses[i].Used = isNonZeroBalance(allAddresses[i].Balance) || isNonZeroBalance(allAddresses[i].Unconfirmed)
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
	errorCount := len(refreshErrors)
	outln(w)
	if cmdCtx.Fmt.Format() == output.FormatJSON {
		displayAddressesRefreshJSON(cmd, allAddresses, errorCount)
	} else {
		out(w, "Refreshed %d address(es)", len(targets))
		if errorCount > 0 {
			out(w, " (%d error(s))", errorCount)
		}
		outln(w)
		displayAddressesText(cmd, allAddresses)
		for _, re := range refreshErrors {
			out(w, "  Error refreshing %s: %s\n", truncateAddressDisplay(re.address), re.err)
		}
	}

	return nil
}

// refreshError records a failed refresh for a specific address.
type refreshError struct {
	address string
	err     error
}

// buildRefreshTargets builds the list of addresses to refresh.
// If specific addresses are provided, validates they exist in the wallet.
func buildRefreshTargets(wlt *wallet.Wallet, chains []chain.ID, specificAddrs []string) ([]refreshTarget, error) {
	if len(specificAddrs) > 0 {
		return resolveSpecificTargets(wlt, chains, specificAddrs)
	}

	// All addresses for the selected chains
	var targets []refreshTarget
	for _, chainID := range chains {
		for _, addr := range wlt.Addresses[chainID] {
			targets = append(targets, refreshTarget{address: addr.Address, chainID: chainID})
		}
		if wlt.ChangeAddresses != nil {
			for _, addr := range wlt.ChangeAddresses[chainID] {
				targets = append(targets, refreshTarget{address: addr.Address, chainID: chainID})
			}
		}
	}
	return targets, nil
}

// resolveSpecificTargets validates and resolves specific address strings to targets.
func resolveSpecificTargets(wlt *wallet.Wallet, chains []chain.ID, specificAddrs []string) ([]refreshTarget, error) {
	var targets []refreshTarget
	for _, addr := range specificAddrs {
		found := false
		for _, chainID := range chains {
			if findInAddresses(wlt.Addresses[chainID], addr) || findInAddresses(wlt.ChangeAddresses[chainID], addr) {
				targets = append(targets, refreshTarget{address: addr, chainID: chainID})
				found = true
				break
			}
		}
		if !found {
			return nil, sigilerr.WithSuggestion(
				sigilerr.ErrInvalidInput,
				fmt.Sprintf("address %s not found in wallet for the specified chain(s)", addr),
			)
		}
	}
	return targets, nil
}

// findInAddresses returns true if the address exists in the slice.
func findInAddresses(addresses []wallet.Address, target string) bool {
	for _, a := range addresses {
		if a.Address == target {
			return true
		}
	}
	return false
}

// refreshTargetAddresses performs the actual refresh for all targets.
// Returns any errors encountered during refresh.
func refreshTargetAddresses(ctx context.Context, w io.Writer, cmdCtx *CommandContext, store *utxostore.Store, targets []refreshTarget, balanceCache *cache.BalanceCache) []refreshError {
	var errs []refreshError

	// Group targets by chain
	var bsvTargets, ethTargets []refreshTarget
	for _, t := range targets {
		switch t.chainID {
		case chain.BSV:
			bsvTargets = append(bsvTargets, t)
		case chain.ETH:
			ethTargets = append(ethTargets, t)
		case chain.BTC, chain.BCH:
			// BTC and BCH not supported in MVP
			continue
		}
	}

	// Refresh BSV addresses
	if len(bsvTargets) > 0 {
		bsvErrs := refreshBSVTargets(ctx, w, cmdCtx, store, bsvTargets, balanceCache)
		errs = append(errs, bsvErrs...)
	}

	// Refresh ETH addresses
	ethErrs := refreshETHTargets(ctx, w, cmdCtx, ethTargets, balanceCache)
	errs = append(errs, ethErrs...)

	return errs
}

// refreshBSVTargets refreshes BSV addresses (UTXO refresh + balance cache update).
func refreshBSVTargets(ctx context.Context, w io.Writer, cmdCtx *CommandContext, store *utxostore.Store, targets []refreshTarget, balanceCache *cache.BalanceCache) []refreshError {
	var errs []refreshError

	client := bsv.NewClient(ctx, &bsv.ClientOptions{
		APIKey: cmdCtx.Cfg.GetBSVAPIKey(),
	})
	adapter := &bsvRefreshAdapter{client: client}

	for _, t := range targets {
		if ctx.Err() != nil {
			errs = append(errs, refreshError{address: t.address, err: ctx.Err()})
			break
		}
		out(w, "  Refreshing %s [BSV]...\n", truncateAddressDisplay(t.address))

		// Step 1: Refresh UTXOs in store
		if _, refreshErr := store.RefreshAddress(ctx, t.address, chain.BSV, adapter); refreshErr != nil {
			errs = append(errs, refreshError{address: t.address, err: refreshErr})
			continue
		}

		// Step 2: Update balance cache
		addrCtx, addrCancel := context.WithTimeout(ctx, 30*time.Second)
		_, _, _ = fetchBalancesForAddress(addrCtx, chain.BSV, t.address, balanceCache, cmdCtx.Cfg)
		addrCancel()
	}

	return errs
}

// refreshETHTargets refreshes ETH addresses (balance fetch + cache update).
func refreshETHTargets(ctx context.Context, w io.Writer, cmdCtx *CommandContext, targets []refreshTarget, balanceCache *cache.BalanceCache) []refreshError {
	var errs []refreshError

	for _, t := range targets {
		if ctx.Err() != nil {
			errs = append(errs, refreshError{address: t.address, err: ctx.Err()})
			break
		}
		out(w, "  Refreshing %s [ETH]...\n", truncateAddressDisplay(t.address))

		addrCtx, addrCancel := context.WithTimeout(ctx, 30*time.Second)
		_, _, fetchErr := fetchBalancesForAddress(addrCtx, chain.ETH, t.address, balanceCache, cmdCtx.Cfg)
		addrCancel()

		if fetchErr != nil {
			errs = append(errs, refreshError{address: t.address, err: fetchErr})
		}
	}

	return errs
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

func displayAddressesRefreshJSON(cmd *cobra.Command, addresses []addressInfo, errorCount int) {
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
		Refreshed int           `json:"refreshed"`
		Errors    int           `json:"errors"`
		Addresses []addressJSON `json:"addresses"`
	}

	resp := responseJSON{
		Refreshed: len(addresses),
		Errors:    errorCount,
		Addresses: make([]addressJSON, 0, len(addresses)),
	}
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
