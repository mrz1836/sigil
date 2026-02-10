package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/cache"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/service/balance"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
)

var (
	// ErrCachedAndAsync is returned when both --cached and --async flags are used together.
	ErrCachedAndAsync = errors.New("cannot use --cached and --async together")
	// ErrRefreshAndAsync is returned when both --refresh and --async flags are used together.
	ErrRefreshAndAsync = errors.New("cannot use --refresh and --async together")
	// ErrRefreshAndCached is returned when both --refresh and --cached flags are used together.
	ErrRefreshAndCached = errors.New("cannot use --refresh and --cached together")
	// ErrNoCachedData is returned when no cached data is available in cached-only mode.
	ErrNoCachedData = errors.New("no cached data available")
)

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	// balanceWalletName is the wallet to check balances for.
	balanceWalletName string
	// balanceChainFilter filters by chain (eth, bsv).
	balanceChainFilter string
	// balanceRefresh forces a fresh fetch, ignoring the cache.
	balanceRefresh bool
	// balanceCachedOnly shows cached data only, skipping network calls.
	balanceCachedOnly bool
	// balanceAsync shows cached data immediately and refreshes in background.
	balanceAsync bool
)

// balanceCmd is the parent command for balance operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Check balances",
	Long: `Check cryptocurrency balances across supported chains.

Balances are cached locally and refreshed from the network on demand.
Supports ETH, USDC (on Ethereum), and BSV.`,
}

// balanceShowCmd shows balances for a wallet.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var balanceShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show balances for a wallet",
	Long: `Show balances for all addresses in a wallet across supported chains.

Displays ETH, USDC (on Ethereum), and BSV balances for all addresses.

Balances are cached and refreshed intelligently:
  - Active addresses (with balance): Always refreshed
  - Inactive addresses: Cached for 30 minutes
  - Never-used addresses: Cached for 2 hours

Use --cached for instant display without network calls.
Use --async for instant display with background refresh.
Use --refresh to force fresh network fetch.`,
	Example: `  sigil balance show --wallet main
  sigil balance show --wallet main --cached       # instant, cache only
  sigil balance show --wallet main --async        # instant + background refresh
  sigil balance show --wallet main --refresh      # force fresh fetch
  sigil balance show --wallet main --chain eth    # filter by chain
  sigil balance show --wallet main -o json`,
	RunE: runBalanceShow,
}

// BalanceResult represents a single balance entry.
type BalanceResult struct {
	Chain       string `json:"chain"`
	Address     string `json:"address"`
	Balance     string `json:"balance"`
	Unconfirmed string `json:"unconfirmed,omitempty"`
	Symbol      string `json:"symbol"`
	Token       string `json:"token,omitempty"`
	Decimals    int    `json:"decimals"`
	Stale       bool   `json:"stale,omitempty"`
	CacheAge    string `json:"cache_age,omitempty"`
}

// BalanceShowResponse is the full response for balance show command.
type BalanceShowResponse struct {
	Wallet    string          `json:"wallet"`
	Balances  []BalanceResult `json:"balances"`
	Timestamp string          `json:"timestamp"`
	Warning   string          `json:"warning,omitempty"`
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	balanceCmd.GroupID = "wallet"
	rootCmd.AddCommand(balanceCmd)
	balanceCmd.AddCommand(balanceShowCmd)

	balanceShowCmd.Flags().StringVar(&balanceWalletName, "wallet", "", "wallet name (required)")
	balanceShowCmd.Flags().StringVar(&balanceChainFilter, "chain", "", "filter by chain (eth, bsv)")
	balanceShowCmd.Flags().BoolVar(&balanceRefresh, "refresh", false, "force fresh fetch, ignore cache")
	balanceShowCmd.Flags().BoolVar(&balanceCachedOnly, "cached", false, "show cached data only, skip network")
	balanceShowCmd.Flags().BoolVar(&balanceAsync, "async", false, "show cached data immediately, refresh in background")

	_ = balanceShowCmd.MarkFlagRequired("wallet")
}

//nolint:gocognit,gocyclo,nestif // Complex business logic for balance display with multiple modes (async, cached, normal)
func runBalanceShow(cmd *cobra.Command, _ []string) error {
	cmdCtx := GetCmdContext(cmd)

	// Validate mutually exclusive flags
	if balanceCachedOnly && balanceAsync {
		return ErrCachedAndAsync
	}
	if balanceRefresh && balanceAsync {
		return ErrRefreshAndAsync
	}
	if balanceRefresh && balanceCachedOnly {
		return ErrRefreshAndCached
	}

	// 1. Load wallet
	storage := wallet.NewFileStorage(filepath.Join(cmdCtx.Cfg.GetHome(), "wallets"))
	w, seed, err := loadWalletWithSession(balanceWalletName, storage, cmd)
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(seed)

	// 2. Initialize service dependencies
	utxoStore := loadUTXOStore(cmdCtx, balanceWalletName)
	balanceCache := loadBalanceCache(cmdCtx, cmd.ErrOrStderr())

	balanceService := balance.NewService(&balance.Config{
		ConfigProvider: cmdCtx.Cfg,
		CacheProvider:  balance.NewCacheAdapter(balanceCache),
		Metadata:       balance.NewMetadataAdapter(utxoStore),
		ForceRefresh:   balanceRefresh,
	})

	// 3. Build address list
	addresses := buildAddressList(w, balanceChainFilter)

	ctx, cancel := contextWithTimeout(cmd, 60*time.Second)
	defer cancel()

	var batchResult *balance.FetchBatchResult

	// 4. Fetch balances based on mode
	if balanceAsync {
		// Async mode: show cached data immediately, refresh in background
		batchResult, err = balanceService.FetchCachedBalances(ctx, &balance.FetchBatchRequest{
			Addresses: addresses,
		})
		if err != nil {
			return err
		}

		// Show cached data (even if incomplete)
		if len(batchResult.Results) > 0 {
			response := convertToBalanceResponse(balanceWalletName, batchResult)

			// Add async refresh indicator
			if response.Warning == "" {
				response.Warning = "Showing cached data. Refreshing in background..."
			} else {
				response.Warning += " Refreshing in background..."
			}

			if outErr := outputBalanceResponse(cmd, cmdCtx, response); outErr != nil {
				return outErr
			}
		} else {
			outln(cmd.ErrOrStderr(), "No cached data available. Run without --async to fetch from network.")
		}

		// Spawn background refresh goroutine
		go refreshBalancesAsync(cmdCtx, balanceService, addresses, balanceCache, balanceWalletName)

		return nil
	} else if balanceCachedOnly {
		// Cached-only mode: no network calls
		batchResult, err = balanceService.FetchCachedBalances(ctx, &balance.FetchBatchRequest{
			Addresses: addresses,
		})
		if err != nil {
			return err
		}

		if len(batchResult.Results) == 0 {
			return fmt.Errorf("%w: run without --cached to fetch from network", ErrNoCachedData)
		}
	} else {
		// Normal mode: fetch from network (with smart caching)
		batchResult, err = balanceService.FetchBalances(ctx, &balance.FetchBatchRequest{
			Addresses:     addresses,
			ForceRefresh:  balanceRefresh,
			MaxConcurrent: 8,
			Timeout:       30 * time.Second,
		})
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		// Save cache after network fetch
		saveBalanceCache(cmdCtx, balanceCache)
	}

	// 5. Convert and output results
	response := convertToBalanceResponse(balanceWalletName, batchResult)
	return outputBalanceResponse(cmd, cmdCtx, response)
}

// loadUTXOStore loads the UTXO store for the wallet, logging errors if load fails.
func loadUTXOStore(cmdCtx *CommandContext, walletName string) *utxostore.Store {
	walletDir := filepath.Join(cmdCtx.Cfg.GetHome(), "wallets", walletName)
	utxoStore := utxostore.New(walletDir)
	if err := utxoStore.Load(); err != nil && cmdCtx.Log != nil {
		cmdCtx.Log.Error("failed to load utxo store: %v", err)
	}
	return utxoStore
}

// loadBalanceCache loads or creates the balance cache based on refresh flag.
func loadBalanceCache(cmdCtx *CommandContext, errWriter io.Writer) *cache.BalanceCache {
	if balanceRefresh {
		return cache.NewBalanceCache()
	}

	cachePath := filepath.Join(cmdCtx.Cfg.GetHome(), "cache", "balances.json")
	cacheStorage := cache.NewFileStorage(cachePath)
	balanceCache, err := cacheStorage.Load()
	if err != nil {
		handleCacheLoadError(cmdCtx, errWriter, err)
		return cache.NewBalanceCache()
	}

	return balanceCache
}

// handleCacheLoadError logs and displays cache load errors.
func handleCacheLoadError(cmdCtx *CommandContext, errWriter io.Writer, err error) {
	if errors.Is(err, cache.ErrCorruptCache) {
		if cmdCtx.Log != nil {
			cmdCtx.Log.Error("balance cache file is corrupted: %v", err)
		}
		outln(errWriter, "Warning: balance cache was corrupted and has been reset.")
	} else if cmdCtx.Log != nil {
		cmdCtx.Log.Error("failed to load balance cache: %v", err)
	}
}

// buildAddressList builds the address input list from wallet, applying chain filter if set.
func buildAddressList(w *wallet.Wallet, chainFilter string) []balance.AddressInput {
	var addresses []balance.AddressInput
	for chainID, addrs := range w.Addresses {
		if chainFilter != "" && string(chainID) != chainFilter {
			continue
		}
		for _, addr := range addrs {
			addresses = append(addresses, balance.AddressInput{
				ChainID: chainID,
				Address: addr.Address,
			})
		}
	}
	return addresses
}

// saveBalanceCache saves the balance cache to storage, logging errors.
func saveBalanceCache(cmdCtx *CommandContext, balanceCache *cache.BalanceCache) {
	cachePath := filepath.Join(cmdCtx.Cfg.GetHome(), "cache", "balances.json")
	cacheStorage := cache.NewFileStorage(cachePath)
	if err := cacheStorage.Save(balanceCache); err != nil && cmdCtx.Log != nil {
		cmdCtx.Log.Error("failed to save balance cache: %v", err)
	}
}

// convertToBalanceResponse converts service results to CLI response format.
func convertToBalanceResponse(walletName string, batchResult *balance.FetchBatchResult) BalanceShowResponse {
	response := BalanceShowResponse{
		Wallet:    walletName,
		Balances:  make([]BalanceResult, 0),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	for _, result := range batchResult.Results {
		for _, bal := range result.Balances {
			cliResult := BalanceResult{
				Chain:       string(bal.Chain),
				Address:     bal.Address,
				Balance:     bal.Balance,
				Unconfirmed: bal.Unconfirmed,
				Symbol:      bal.Symbol,
				Token:       bal.Token,
				Decimals:    bal.Decimals,
				Stale:       bal.Stale,
			}
			if bal.Stale {
				cliResult.CacheAge = formatCacheAge(bal.UpdatedAt)
			}
			response.Balances = append(response.Balances, cliResult)
		}
	}

	if len(batchResult.Errors) > 0 {
		if balanceCachedOnly {
			response.Warning = "Some addresses have no cached data. Run without --cached to fetch from network."
		} else {
			response.Warning = "Some balances could not be fetched. Showing cached data where available."
		}
	}

	sortBalanceResults(response.Balances)
	return response
}

// sortBalanceResults sorts balance results by chain, address, and token.
func sortBalanceResults(balances []BalanceResult) {
	sort.Slice(balances, func(i, j int) bool {
		left := balances[i]
		right := balances[j]
		if left.Chain != right.Chain {
			return left.Chain < right.Chain
		}
		if left.Address != right.Address {
			return left.Address < right.Address
		}
		return left.Token < right.Token
	})
}

// outputBalanceResponse outputs the balance response in the requested format.
func outputBalanceResponse(cmd *cobra.Command, cmdCtx *CommandContext, response BalanceShowResponse) error {
	if cmdCtx.Fmt.Format() == output.FormatJSON {
		if err := outputBalanceJSON(cmd.OutOrStdout(), response); err != nil {
			return fmt.Errorf("writing JSON output: %w", err)
		}
	} else {
		outputBalanceText(cmd.OutOrStdout(), response)
	}
	return nil
}

// formatCacheAge formats the age of a cache entry for display.
func formatCacheAge(t time.Time) string {
	age := time.Since(t)
	if age < time.Minute {
		return fmt.Sprintf("%ds ago", int(age.Seconds()))
	} else if age < time.Hour {
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	} else if age < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(age.Hours()/24))
}

// getChainSymbol returns the symbol for a given chain ID.
func getChainSymbol(chainID wallet.ChainID) string {
	switch chainID {
	case wallet.ChainETH:
		return "ETH"
	case wallet.ChainBSV:
		return "BSV"
	case wallet.ChainBTC:
		return "BTC"
	case wallet.ChainBCH:
		return "BCH"
	default:
		return "???"
	}
}

// outputBalanceJSON outputs balances in JSON format.
func outputBalanceJSON(w io.Writer, response BalanceShowResponse) error {
	if response.Balances == nil {
		response.Balances = []BalanceResult{}
	}
	return writeJSON(w, response)
}

// hasUnconfirmedData returns true if any balance entry has non-zero unconfirmed data.
func hasUnconfirmedData(balances []BalanceResult) bool {
	for _, bal := range balances {
		if bal.Unconfirmed != "" {
			return true
		}
	}
	return false
}

// outputBalanceText outputs balances in text table format.
func outputBalanceText(w io.Writer, response BalanceShowResponse) {
	outln(w, fmt.Sprintf("Balances for wallet: %s", response.Wallet))
	outln(w)

	if response.Warning != "" {
		outln(w, fmt.Sprintf("Warning: %s", response.Warning))
		outln(w)
	}

	if len(response.Balances) == 0 {
		outln(w, "No balances found.")
		return
	}

	showUnconfirmed := hasUnconfirmedData(response.Balances)

	if showUnconfirmed {
		outputBalanceTableWide(w, response.Balances)
	} else {
		outputBalanceTableNarrow(w, response.Balances)
	}

	// Show staleness legend if any data is stale
	for _, bal := range response.Balances {
		if bal.Stale {
			outln(w)
			outln(w, "* Cached data (network unavailable)")
			break
		}
	}
}

// outputBalanceTableNarrow renders the 4-column table (no unconfirmed data).
func outputBalanceTableNarrow(w io.Writer, balances []BalanceResult) {
	outln(w, "┌────────┬────────────────────────────────────────────┬──────────────────┬────────┐")
	outln(w, "│ Chain  │ Address                                    │ Balance          │ Symbol │")
	outln(w, "├────────┼────────────────────────────────────────────┼──────────────────┼────────┤")

	for _, bal := range balances {
		addr := truncateAddress(bal.Address)
		balanceStr := bal.Balance
		if bal.Stale {
			balanceStr += " *"
		}

		out(w, "│ %-6s │ %-42s │ %16s │ %-6s │\n",
			strings.ToUpper(bal.Chain),
			addr,
			balanceStr,
			bal.Symbol,
		)
	}

	outln(w, "└────────┴────────────────────────────────────────────┴──────────────────┴────────┘")
}

// outputBalanceTableWide renders the 5-column table with unconfirmed data.
func outputBalanceTableWide(w io.Writer, balances []BalanceResult) {
	outln(w, "┌────────┬────────────────────────────────────────────┬──────────────────┬──────────────────┬────────┐")
	outln(w, "│ Chain  │ Address                                    │ Confirmed        │ Unconfirmed      │ Symbol │")
	outln(w, "├────────┼────────────────────────────────────────────┼──────────────────┼──────────────────┼────────┤")

	for _, bal := range balances {
		addr := truncateAddress(bal.Address)
		balanceStr := bal.Balance
		if bal.Stale {
			balanceStr += " *"
		}

		unconfStr := "-"
		if bal.Unconfirmed != "" {
			unconfStr = bal.Unconfirmed
		}

		out(w, "│ %-6s │ %-42s │ %16s │ %16s │ %-6s │\n",
			strings.ToUpper(bal.Chain),
			addr,
			balanceStr,
			unconfStr,
			bal.Symbol,
		)
	}

	outln(w, "└────────┴────────────────────────────────────────────┴──────────────────┴──────────────────┴────────┘")
}

// truncateAddress shortens an address for table display.
func truncateAddress(addr string) string {
	if len(addr) > 42 {
		return addr[:20] + "..." + addr[len(addr)-17:]
	}
	return addr
}

// refreshBalancesAsync refreshes balances in a background goroutine.
// Updates the cache file when complete. Logs errors but doesn't block.
func refreshBalancesAsync(
	cmdCtx *CommandContext,
	service *balance.Service,
	addresses []balance.AddressInput,
	balanceCache *cache.BalanceCache,
	walletName string,
) {
	// Use background context (don't tie to command context)
	bgCtx := context.Background()
	bgCtx, cancel := context.WithTimeout(bgCtx, 60*time.Second)
	defer cancel()

	// Fetch fresh balances using smart refresh policy
	_, err := service.FetchBalances(bgCtx, &balance.FetchBatchRequest{
		Addresses:     addresses,
		ForceRefresh:  false, // Use smart refresh policy
		MaxConcurrent: 8,
		Timeout:       30 * time.Second,
	})

	if err != nil && cmdCtx.Log != nil {
		cmdCtx.Log.Debug("background balance refresh completed with errors: %v", err)
		// Don't return on error - partial success is OK
	}

	// Save updated cache
	saveBalanceCache(cmdCtx, balanceCache)

	if cmdCtx.Log != nil {
		cmdCtx.Log.Debug("background balance refresh completed for wallet %s", walletName)
	}
}
