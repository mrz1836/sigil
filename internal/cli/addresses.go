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
	"github.com/mrz1836/sigil/internal/service/address"
	"github.com/mrz1836/sigil/internal/service/balance"
	"github.com/mrz1836/sigil/internal/service/discovery"
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

	// Parse chain filter
	var chainFilter chain.ID
	if addressesChain != "" {
		parsed, ok := chain.ParseChainID(addressesChain)
		if !ok || !parsed.IsMVP() {
			return sigilerr.WithSuggestion(
				sigilerr.ErrInvalidInput,
				fmt.Sprintf("invalid chain: %s (use eth or bsv)", addressesChain),
			)
		}
		chainFilter = parsed
	}

	// Parse type filter
	var typeFilter address.AddressType
	switch addressesType {
	case "all":
		typeFilter = address.AllTypes
	case "receive":
		typeFilter = address.Receive
	case "change":
		typeFilter = address.Change
	}

	// Create address service and collect addresses
	addressService := address.NewService(address.NewMetadataAdapter(store))
	allAddresses := addressService.Collect(&address.CollectionRequest{
		Wallet:      wlt,
		ChainFilter: chainFilter,
		TypeFilter:  typeFilter,
	})

	// Fetch live balances concurrently
	fetchAddressBalances(cmd, allAddresses, balanceCache, cmdCtx.Cfg)

	// Enrich "Used" status from fetched balance data
	for i := range allAddresses {
		if !allAddresses[i].HasActivity {
			allAddresses[i].HasActivity = isNonZeroBalance(allAddresses[i].Balance) || isNonZeroBalance(allAddresses[i].Unconfirmed)
		}
	}

	// Apply --used/--unused filter after balance enrichment
	allAddresses = address.FilterUsage(allAddresses, addressesUsed, addressesUnused)

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

	// Build addressInfo list for display using address service
	targetSet := make(map[string]bool, len(targets))
	for _, t := range targets {
		targetSet[t.address] = true
	}

	addressService := address.NewService(address.NewMetadataAdapter(store))
	allAddressesUnfiltered := addressService.Collect(&address.CollectionRequest{
		Wallet:      wlt,
		ChainFilter: "", // All chains
		TypeFilter:  address.AllTypes,
	})

	// Filter to only target addresses
	var allAddresses []address.AddressInfo
	for _, addr := range allAddressesUnfiltered {
		if targetSet[addr.Address] {
			allAddresses = append(allAddresses, addr)
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
		if !allAddresses[i].HasActivity {
			allAddresses[i].HasActivity = isNonZeroBalance(allAddresses[i].Balance) || isNonZeroBalance(allAddresses[i].Unconfirmed)
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
	targetsByChain := groupTargetsByChain(targets)

	// Create discovery service with balance service
	discoverySvc := createDiscoveryService(cmdCtx, store, balanceCache)

	// Refresh each chain's addresses
	errs := make([]refreshError, 0, len(targets))
	for chainID, chainTargets := range targetsByChain {
		addresses := extractAddresses(chainTargets)
		displayRefreshProgress(w, addresses, chainID)

		results, _ := discoverySvc.RefreshBatch(ctx, &discovery.RefreshRequest{
			ChainID:   chainID,
			Addresses: addresses,
			Timeout:   30 * time.Second,
		})

		errs = append(errs, convertRefreshResults(results)...)
	}

	return errs
}

// groupTargetsByChain groups refresh targets by chain ID, excluding unsupported chains.
func groupTargetsByChain(targets []refreshTarget) map[chain.ID][]refreshTarget {
	targetsByChain := make(map[chain.ID][]refreshTarget)
	for _, t := range targets {
		if t.chainID == chain.BTC || t.chainID == chain.BCH {
			// BTC and BCH not supported in MVP
			continue
		}
		targetsByChain[t.chainID] = append(targetsByChain[t.chainID], t)
	}
	return targetsByChain
}

// createDiscoveryService creates a discovery service with balance service integration.
func createDiscoveryService(cmdCtx *CommandContext, store *utxostore.Store, balanceCache *cache.BalanceCache) *discovery.Service {
	balanceSvc := balance.NewService(&balance.Config{
		ConfigProvider: cmdCtx.Cfg,
		CacheProvider:  balance.NewCacheAdapter(balanceCache),
		Metadata:       nil,
		ForceRefresh:   true,
	})

	return discovery.NewService(&discovery.Config{
		UTXOStore:      discovery.NewUTXOStoreAdapter(store),
		BalanceService: balanceSvc,
		Config:         cmdCtx.Cfg,
	})
}

// extractAddresses extracts address strings from refresh targets.
func extractAddresses(targets []refreshTarget) []string {
	addresses := make([]string, len(targets))
	for i, t := range targets {
		addresses[i] = t.address
	}
	return addresses
}

// displayRefreshProgress shows progress messages for addresses being refreshed.
func displayRefreshProgress(w io.Writer, addresses []string, chainID chain.ID) {
	for _, addr := range addresses {
		out(w, "  Refreshing %s [%s]...\n", truncateAddressDisplay(addr), strings.ToUpper(string(chainID)))
	}
}

// convertRefreshResults converts discovery results to refreshError format.
func convertRefreshResults(results []discovery.RefreshResult) []refreshError {
	var errs []refreshError
	for _, result := range results {
		if !result.Success {
			errs = append(errs, refreshError{address: result.Address, err: result.Error})
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
//nolint:gocognit,gocyclo // Concurrent fetch logic requires nested control flow
func fetchAddressBalances(cmd *cobra.Command, addresses []address.AddressInfo, balanceCache *cache.BalanceCache, cfg ConfigProvider) {
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

			balanceSvc := balance.NewService(&balance.Config{
				ConfigProvider: cfg,
				CacheProvider:  balance.NewCacheAdapter(balanceCache),
				Metadata:       nil,
				ForceRefresh:   false,
			})

			addrCtx, addrCancel := context.WithTimeout(ctx, perAddressTimeout)
			result, _ := balanceSvc.FetchBalance(addrCtx, &balance.FetchRequest{
				ChainID:      task.chainID,
				Address:      task.address,
				ForceRefresh: false,
			})
			addrCancel()

			mu.Lock()
			defer mu.Unlock()
			key := string(task.chainID) + ":" + task.address
			if result != nil && len(result.Balances) > 0 {
				results[key] = &fetchResult{
					balance:     result.Balances[0].Balance,
					unconfirmed: result.Balances[0].Unconfirmed,
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
func hasUnconfirmedAddressData(addresses []address.AddressInfo) bool {
	for _, addr := range addresses {
		if addr.Unconfirmed != "" {
			return true
		}
	}
	return false
}

func displayAddressesText(cmd *cobra.Command, addresses []address.AddressInfo) {
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
func displayAddressesTextNarrow(w io.Writer, addresses []address.AddressInfo) {
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
			addr.Type.String(), addr.Index, truncateAddressDisplay(addr.Address),
			formatLabel(addr.Label), formatBalanceDisplay(addr.Balance), formatStatus(addr.HasActivity))
	}
}

// displayAddressesTextWide renders the address table with confirmed and unconfirmed columns.
func displayAddressesTextWide(w io.Writer, addresses []address.AddressInfo) {
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
			addr.Type.String(), addr.Index, truncateAddressDisplay(addr.Address),
			formatLabel(addr.Label), formatBalanceDisplay(addr.Balance),
			formatBalanceDisplay(addr.Unconfirmed), formatStatus(addr.HasActivity))
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

func displayAddressesJSON(cmd *cobra.Command, addresses []address.AddressInfo) {
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
			Type:        addr.Type.String(),
			Index:       addr.Index,
			Address:     addr.Address,
			Path:        addr.Path,
			Label:       addr.Label,
			Balance:     addr.Balance,
			Unconfirmed: addr.Unconfirmed,
			Used:        addr.HasActivity,
		})
	}

	_ = writeJSON(cmd.OutOrStdout(), resp)
}

func displayAddressesRefreshJSON(cmd *cobra.Command, addresses []address.AddressInfo, errorCount int) {
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
			Type:        addr.Type.String(),
			Index:       addr.Index,
			Address:     addr.Address,
			Path:        addr.Path,
			Label:       addr.Label,
			Balance:     addr.Balance,
			Unconfirmed: addr.Unconfirmed,
			Used:        addr.HasActivity,
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
