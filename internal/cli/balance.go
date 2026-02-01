package cli

import (
	"context"
	"fmt"
	"io"
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
	// balanceWalletName is the wallet to check balances for.
	balanceWalletName string
	// balanceChainFilter filters by chain (eth, bsv).
	balanceChainFilter string
)

// balanceCmd is the parent command for balance operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Check balances",
	Long:  `Check cryptocurrency balances across chains.`,
}

// balanceShowCmd shows balances for a wallet.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var balanceShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show balances for a wallet",
	Long: `Show balances for all addresses in a wallet across supported chains.

Displays ETH, USDC (on Ethereum), and BSV balances for all addresses.
Supports filtering by chain with the --chain flag.

Example:
  sigil balance show --wallet main
  sigil balance show --wallet main --chain eth
  sigil balance show --wallet main -o json`,
	RunE: runBalanceShow,
}

// BalanceResult represents a single balance entry.
type BalanceResult struct {
	Chain    string `json:"chain"`
	Address  string `json:"address"`
	Balance  string `json:"balance"`
	Symbol   string `json:"symbol"`
	Token    string `json:"token,omitempty"`
	Decimals int    `json:"decimals"`
	Stale    bool   `json:"stale,omitempty"`
	CacheAge string `json:"cache_age,omitempty"`
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
	rootCmd.AddCommand(balanceCmd)
	balanceCmd.AddCommand(balanceShowCmd)

	balanceShowCmd.Flags().StringVar(&balanceWalletName, "wallet", "", "wallet name (required)")
	balanceShowCmd.Flags().StringVar(&balanceChainFilter, "chain", "", "filter by chain (eth, bsv)")

	_ = balanceShowCmd.MarkFlagRequired("wallet")
}

//nolint:gocognit,gocyclo // CLI entry point has inherent complexity
func runBalanceShow(cmd *cobra.Command, _ []string) error {
	// Load wallet to get addresses (using session if available)
	storage := wallet.NewFileStorage(filepath.Join(cfg.Home, "wallets"))

	w, seed, err := loadWalletWithSession(balanceWalletName, storage, cmd)
	if err != nil {
		return err
	}
	wallet.ZeroBytes(seed)

	// Load or create cache
	cachePath := filepath.Join(cfg.Home, "cache", "balances.json")
	cacheStorage := cache.NewFileStorage(cachePath)
	balanceCache, err := cacheStorage.Load()
	if err != nil {
		balanceCache = cache.NewBalanceCache()
	}

	// Fetch balances with overall timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	response := BalanceShowResponse{
		Wallet:    balanceWalletName,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	var fetchErrors []string

	// Per-address timeout for individual balance fetches
	const perAddressTimeout = 30 * time.Second

	// Get balances per chain
	for chainID, addresses := range w.Addresses {
		// Apply chain filter if specified
		if balanceChainFilter != "" && string(chainID) != balanceChainFilter {
			continue
		}

		for _, addr := range addresses {
			// Create per-address context with its own timeout
			addrCtx, addrCancel := context.WithTimeout(ctx, perAddressTimeout)
			balances, stale, fetchErr := fetchBalancesForAddress(addrCtx, chainID, addr.Address, balanceCache)
			addrCancel()

			if fetchErr != nil {
				fetchErrors = append(fetchErrors, fetchErr.Error())
			}

			for _, bal := range balances {
				result := BalanceResult{
					Chain:    string(bal.Chain),
					Address:  bal.Address,
					Balance:  bal.Balance,
					Symbol:   bal.Symbol,
					Token:    bal.Token,
					Decimals: bal.Decimals,
					Stale:    stale,
				}
				if stale {
					result.CacheAge = formatCacheAge(bal.UpdatedAt)
				}
				response.Balances = append(response.Balances, result)
			}

			// Add placeholder for failed fetches with no cache data
			if fetchErr != nil && len(balances) == 0 {
				result := BalanceResult{
					Chain:   string(chainID),
					Address: addr.Address,
					Balance: "N/A",
					Symbol:  getChainSymbol(chainID),
					Stale:   true,
				}
				response.Balances = append(response.Balances, result)
			}
		}
	}

	// Save updated cache
	if err := cacheStorage.Save(balanceCache); err != nil {
		logger.Error("failed to save balance cache: %v", err)
	}

	// Add warning if any fetches failed and using stale data
	if len(fetchErrors) > 0 {
		response.Warning = "Some balances could not be fetched. Showing cached data where available."
	}

	// Output results
	if formatter.Format() == output.FormatJSON {
		outputBalanceJSON(cmd.OutOrStdout(), response)
	} else {
		outputBalanceText(cmd.OutOrStdout(), response)
	}

	return nil
}

// fetchBalancesForAddress fetches balances for a single address.
// Returns balances, whether data is stale, and any error.
func fetchBalancesForAddress(ctx context.Context, chainID wallet.ChainID, address string, balanceCache *cache.BalanceCache) ([]cache.BalanceCacheEntry, bool, error) {
	var entries []cache.BalanceCacheEntry
	var stale bool
	var fetchErr error

	switch chainID {
	case wallet.ChainETH:
		entries, stale, fetchErr = fetchETHBalances(ctx, address, balanceCache)
	case wallet.ChainBSV:
		entries, stale, fetchErr = fetchBSVBalances(ctx, address, balanceCache)
	case wallet.ChainBTC, wallet.ChainBCH:
		// BTC and BCH not supported in MVP
		return nil, false, nil
	}

	return entries, stale, fetchErr
}

// fetchETHBalances fetches ETH and USDC balances.
func fetchETHBalances(ctx context.Context, address string, balanceCache *cache.BalanceCache) ([]cache.BalanceCacheEntry, bool, error) {
	var entries []cache.BalanceCacheEntry
	var stale bool

	// Get ETH RPC URL from config
	rpcURL := cfg.Networks.ETH.RPC
	if rpcURL == "" {
		// RPC explicitly disabled - try cache first
		cached, isStale, cacheErr := getCachedETHBalances(address, balanceCache)
		if cacheErr == nil {
			return cached, isStale, nil
		}
		// No cache, return clear error
		return nil, true, sigilerr.WithSuggestion(
			sigilerr.ErrNetworkError,
			"ETH RPC not configured. Set SIGIL_ETH_RPC or configure networks.eth.rpc in config.yaml",
		)
	}

	client, err := eth.NewClient(rpcURL, nil)
	if err != nil {
		return getCachedETHBalances(address, balanceCache)
	}
	defer client.Close()

	// Fetch ETH balance with retry logic for transient failures
	ethBalance, err := chain.Retry(ctx, func() (*eth.Balance, error) {
		bal, fetchErr := client.GetNativeBalance(ctx, address)
		if fetchErr != nil {
			// Wrap network errors as retryable
			return nil, chain.WrapRetryable(fetchErr)
		}
		return bal, nil
	})
	if err != nil {
		// Fall back to cache
		cachedEntries, isStale, _ := getCachedETHBalances(address, balanceCache)
		return cachedEntries, isStale, err
	}

	// Store in cache
	ethEntry := cache.BalanceCacheEntry{
		Chain:    chain.ETH,
		Address:  address,
		Balance:  eth.FormatBalanceAmount(ethBalance.Amount, ethBalance.Decimals),
		Symbol:   ethBalance.Symbol,
		Decimals: ethBalance.Decimals,
	}
	balanceCache.Set(ethEntry)
	entries = append(entries, ethEntry)

	// Fetch USDC balance
	usdcBalance, err := client.GetUSDCBalance(ctx, address)
	if err == nil {
		usdcEntry := cache.BalanceCacheEntry{
			Chain:    chain.ETH,
			Address:  address,
			Balance:  eth.FormatBalanceAmount(usdcBalance.Amount, usdcBalance.Decimals),
			Symbol:   usdcBalance.Symbol,
			Token:    usdcBalance.Token,
			Decimals: usdcBalance.Decimals,
		}
		balanceCache.Set(usdcEntry)
		entries = append(entries, usdcEntry)
	}

	return entries, stale, nil
}

// getCachedETHBalances returns cached ETH balances if available.
func getCachedETHBalances(address string, balanceCache *cache.BalanceCache) ([]cache.BalanceCacheEntry, bool, error) {
	entries := make([]cache.BalanceCacheEntry, 0, 2)
	stale := false

	// Check for ETH
	entry, exists, age := balanceCache.Get(chain.ETH, address, "")
	if exists {
		entries = append(entries, *entry)
		if age > cache.DefaultStaleness {
			stale = true
		}
	}

	// Check for USDC
	usdcEntry, exists, age := balanceCache.Get(chain.ETH, address, eth.USDCMainnet)
	if exists {
		entries = append(entries, *usdcEntry)
		if age > cache.DefaultStaleness {
			stale = true
		}
	}

	if len(entries) == 0 {
		return nil, true, sigilerr.ErrCacheNotFound
	}

	return entries, stale, nil
}

// fetchBSVBalances fetches BSV balances.
func fetchBSVBalances(ctx context.Context, address string, balanceCache *cache.BalanceCache) ([]cache.BalanceCacheEntry, bool, error) {
	entries := make([]cache.BalanceCacheEntry, 0, 1)
	var stale bool

	client := bsv.NewClient(nil)

	// Fetch BSV balance
	bsvBalance, err := client.GetNativeBalance(ctx, address)
	if err != nil {
		// Fall back to cache
		return getCachedBSVBalances(address, balanceCache)
	}

	// Store in cache
	entry := cache.BalanceCacheEntry{
		Chain:    chain.BSV,
		Address:  address,
		Balance:  bsv.FormatBalanceAmount(bsvBalance.Amount, bsvBalance.Decimals),
		Symbol:   bsvBalance.Symbol,
		Decimals: bsvBalance.Decimals,
	}
	balanceCache.Set(entry)
	entries = append(entries, entry)

	return entries, stale, nil
}

// getCachedBSVBalances returns cached BSV balances if available.
func getCachedBSVBalances(address string, balanceCache *cache.BalanceCache) ([]cache.BalanceCacheEntry, bool, error) {
	entry, exists, age := balanceCache.Get(chain.BSV, address, "")
	if !exists {
		return nil, true, sigilerr.ErrCacheNotFound
	}

	stale := age > cache.DefaultStaleness
	return []cache.BalanceCacheEntry{*entry}, stale, nil
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
//
//nolint:gocognit // JSON formatting has inherent conditional complexity
func outputBalanceJSON(w io.Writer, response BalanceShowResponse) {
	out(w, "{\n")
	out(w, `  "wallet": "%s",`+"\n", response.Wallet)
	out(w, `  "timestamp": "%s",`+"\n", response.Timestamp)

	if response.Warning != "" {
		out(w, `  "warning": "%s",`+"\n", response.Warning)
	}

	out(w, `  "balances": [`+"\n")
	for i, bal := range response.Balances {
		out(w, "    {\n")
		out(w, `      "chain": "%s",`+"\n", bal.Chain)
		out(w, `      "address": "%s",`+"\n", bal.Address)
		out(w, `      "balance": "%s",`+"\n", bal.Balance)
		out(w, `      "symbol": "%s",`+"\n", bal.Symbol)
		out(w, `      "decimals": %d`, bal.Decimals)

		if bal.Token != "" {
			out(w, `,`+"\n")
			out(w, `      "token": "%s"`, bal.Token)
		}
		if bal.Stale {
			out(w, `,`+"\n")
			out(w, `      "stale": true`)
			if bal.CacheAge != "" {
				out(w, `,`+"\n")
				out(w, `      "cache_age": "%s"`, bal.CacheAge)
			}
		}
		out(w, "\n    }")

		if i < len(response.Balances)-1 {
			out(w, ",")
		}
		out(w, "\n")
	}
	out(w, "  ]\n")
	out(w, "}\n")
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

	// Print table header
	outln(w, "┌────────┬────────────────────────────────────────────┬──────────────────┬────────┐")
	outln(w, "│ Chain  │ Address                                    │ Balance          │ Symbol │")
	outln(w, "├────────┼────────────────────────────────────────────┼──────────────────┼────────┤")

	for _, bal := range response.Balances {
		// Truncate address if too long
		addr := bal.Address
		if len(addr) > 42 {
			addr = addr[:20] + "..." + addr[len(addr)-17:]
		}

		// Format balance with staleness indicator
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

	// Show staleness legend if any data is stale
	for _, bal := range response.Balances {
		if bal.Stale {
			outln(w)
			outln(w, "* Cached data (network unavailable)")
			break
		}
	}
}
