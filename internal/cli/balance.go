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

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	// balanceWalletName is the wallet to check balances for.
	balanceWalletName string
	// balanceChainFilter filters by chain (eth, bsv).
	balanceChainFilter string
	// balanceRefresh forces a fresh fetch, ignoring the cache.
	balanceRefresh bool
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
Supports filtering by chain with the --chain flag.
Use --refresh to bypass the cache and force a fresh fetch from the network.`,
	Example: `  sigil balance show --wallet main
  sigil balance show --wallet main --chain eth
  sigil balance show --wallet main --refresh
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

	_ = balanceShowCmd.MarkFlagRequired("wallet")
}

func runBalanceShow(cmd *cobra.Command, _ []string) error {
	cmdCtx := GetCmdContext(cmd)

	// 1. Load wallet (CLI concern)
	storage := wallet.NewFileStorage(filepath.Join(cmdCtx.Cfg.GetHome(), "wallets"))
	w, seed, err := loadWalletWithSession(balanceWalletName, storage, cmd)
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(seed)

	// 2. Initialize service dependencies
	walletDir := filepath.Join(cmdCtx.Cfg.GetHome(), "wallets", balanceWalletName)
	utxoStore := utxostore.New(walletDir)
	if err = utxoStore.Load(); err != nil && cmdCtx.Log != nil {
		cmdCtx.Log.Error("failed to load utxo store: %v", err)
		// Continue without metadata (degrades to always-fetch behavior)
	}

	cachePath := filepath.Join(cmdCtx.Cfg.GetHome(), "cache", "balances.json")
	cacheStorage := cache.NewFileStorage(cachePath)
	var balanceCache *cache.BalanceCache
	if balanceRefresh {
		// --refresh: start with a clean cache so every address hits the network.
		balanceCache = cache.NewBalanceCache()
	} else {
		balanceCache, err = cacheStorage.Load()
		if err != nil {
			if errors.Is(err, cache.ErrCorruptCache) {
				if cmdCtx.Log != nil {
					cmdCtx.Log.Error("balance cache file is corrupted: %v", err)
				}
				outln(cmd.ErrOrStderr(), "Warning: balance cache was corrupted and has been reset.")
			} else if cmdCtx.Log != nil {
				cmdCtx.Log.Error("failed to load balance cache: %v", err)
			}
			balanceCache = cache.NewBalanceCache()
		}
	}

	// Create balance service
	balanceService := balance.NewService(&balance.Config{
		ConfigProvider: cmdCtx.Cfg,
		CacheProvider:  balance.NewCacheAdapter(balanceCache),
		Metadata:       balance.NewMetadataAdapter(utxoStore),
		ForceRefresh:   balanceRefresh,
	})

	// 3. Build address list (CLI concern)
	var addresses []balance.AddressInput
	for chainID, addrs := range w.Addresses {
		if balanceChainFilter != "" && string(chainID) != balanceChainFilter {
			continue
		}
		for _, addr := range addrs {
			addresses = append(addresses, balance.AddressInput{
				ChainID: chainID,
				Address: addr.Address,
			})
		}
	}

	// 4. Fetch via service (business logic)
	ctx, cancel := contextWithTimeout(cmd, 60*time.Second)
	defer cancel()

	batchResult, err := balanceService.FetchBalances(ctx, &balance.FetchBatchRequest{
		Addresses:     addresses,
		ForceRefresh:  balanceRefresh,
		MaxConcurrent: 8,
		Timeout:       30 * time.Second,
	})
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	// 5. Save cache (CLI concern)
	if err := cacheStorage.Save(balanceCache); err != nil && cmdCtx.Log != nil {
		cmdCtx.Log.Error("failed to save balance cache: %v", err)
	}

	// 6. Convert to CLI output format (CLI concern)
	response := BalanceShowResponse{
		Wallet:    balanceWalletName,
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

	// Add warning if any fetches failed
	if len(batchResult.Errors) > 0 {
		response.Warning = "Some balances could not be fetched. Showing cached data where available."
	}

	// Sort results
	sort.Slice(response.Balances, func(i, j int) bool {
		left := response.Balances[i]
		right := response.Balances[j]
		if left.Chain != right.Chain {
			return left.Chain < right.Chain
		}
		if left.Address != right.Address {
			return left.Address < right.Address
		}
		return left.Token < right.Token
	})

	// 7. Output results (CLI concern)
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
