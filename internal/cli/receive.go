package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/chain/eth"
	"github.com/mrz1836/sigil/internal/chain/eth/etherscan"
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
	// receiveQR displays a QR code for the address.
	receiveQR bool
	// receiveCheck checks for received funds at the address.
	receiveCheck bool
	// receiveAddress is a specific address to check (used with --check).
	receiveAddress string
	// receiveAll checks all receiving addresses (used with --check).
	receiveAll bool
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
  sigil receive --wallet main --chain bsv --new --label "Payment from Alice"

  # Show address with QR code for mobile wallet scanning
  sigil receive --wallet main --chain bsv --qr

  # Check if funds have arrived at your receive address
  sigil receive --wallet main --chain bsv --check

  # Check a specific address for funds
  sigil receive --wallet main --chain bsv --check --address 1ABC...

  # Check all receiving addresses for funds
  sigil receive --wallet main --chain bsv --check --all`,
	RunE: runReceive,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(receiveCmd)

	receiveCmd.Flags().StringVarP(&receiveWallet, "wallet", "w", "", "wallet name (required)")
	receiveCmd.Flags().StringVarP(&receiveChain, "chain", "c", "bsv", "blockchain: eth, bsv")
	receiveCmd.Flags().BoolVar(&receiveNew, "new", false, "force generation of a new address")
	receiveCmd.Flags().StringVarP(&receiveLabel, "label", "l", "", "label for the address")
	receiveCmd.Flags().BoolVar(&receiveQR, "qr", false, "display QR code for the address")
	receiveCmd.Flags().BoolVar(&receiveCheck, "check", false, "check for received funds and refresh local UTXO state")
	receiveCmd.Flags().StringVar(&receiveAddress, "address", "", "specific address to check (use with --check)")
	receiveCmd.Flags().BoolVar(&receiveAll, "all", false, "check all receiving addresses (use with --check)")

	_ = receiveCmd.MarkFlagRequired("wallet")
}

//nolint:gocognit,gocyclo // CLI flow involves multiple validation and setup steps
func runReceive(cmd *cobra.Command, _ []string) error {
	cmdCtx := GetCmdContext(cmd)

	// Validate flag combinations
	if receiveCheck && receiveNew {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"--check and --new cannot be used together (a new address has no funds to check)",
		)
	}
	if receiveAddress != "" && !receiveCheck {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"--address requires --check",
		)
	}
	if receiveAll && !receiveCheck {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"--all requires --check",
		)
	}
	if receiveAddress != "" && receiveAll {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"--address and --all cannot be used together",
		)
	}

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

	// Multi-chain check: when --check --all is used without explicit --chain,
	// check all enabled MVP chains (BSV + ETH).
	if receiveCheck && receiveAll && !cmd.Flags().Changed("chain") {
		return runReceiveCheckAllChains(cmd, cmdCtx, wlt, store)
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

	if isNew {
		if err := storage.UpdateMetadata(wlt); err != nil {
			return fmt.Errorf("persisting wallet metadata: %w", err)
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

	// If --check, refresh the address UTXOs and display status
	if receiveCheck {
		return runReceiveCheck(cmd, cmdCtx, wlt, store, addr, chainID)
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

	// Render QR code if requested and output is a terminal
	if receiveQR && output.CanRenderQR(w) {
		cfg := output.DefaultQRConfig()
		_ = output.RenderQR(w, formatQRData(addr.Address), cfg)
		outln(w)
		outln(w, "  Scan with a mobile wallet to send BSV")
		outln(w)
	}

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

// formatQRData returns the address string formatted for QR code encoding.
// Uses plain address format for maximum wallet compatibility.
func formatQRData(address string) string {
	return address
}

// displayReceiveJSON shows the receiving address in JSON format.
func displayReceiveJSON(cmd *cobra.Command, addr *wallet.Address, chainID chain.ID, label string, isNew bool) {
	payload := struct {
		Chain   string `json:"chain"`
		Address string `json:"address"`
		Path    string `json:"path"`
		Index   uint32 `json:"index"`
		Label   string `json:"label,omitempty"`
		IsNew   bool   `json:"is_new"`
	}{
		Chain:   string(chainID),
		Address: addr.Address,
		Path:    addr.Path,
		Index:   addr.Index,
		Label:   label,
		IsNew:   isNew,
	}

	_ = writeJSON(cmd.OutOrStdout(), payload)
}

// checkTimeout is the network timeout for receive --check operations.
const checkTimeout = 30 * time.Second

// runReceiveCheck refreshes UTXOs/balances for the target address(es) and displays results.
func runReceiveCheck(cmd *cobra.Command, cmdCtx *CommandContext, wlt *wallet.Wallet, store *utxostore.Store, currentAddr *wallet.Address, chainID chain.ID) error {
	ctx, cancel := contextWithTimeout(cmd, checkTimeout)
	defer cancel()

	w := cmd.OutOrStdout()

	// ETH uses account-based balance checking (not UTXO)
	if chainID == chain.ETH {
		return runReceiveCheckETH(ctx, w, cmdCtx, wlt, currentAddr)
	}

	// BSV uses UTXO-based checking
	return runReceiveCheckBSV(ctx, w, cmdCtx, wlt, store, currentAddr, chainID)
}

// runReceiveCheckETH dispatches ETH balance checking for --check mode.
func runReceiveCheckETH(ctx context.Context, w io.Writer, cmdCtx *CommandContext, wlt *wallet.Wallet, currentAddr *wallet.Address) error {
	if receiveAll {
		return runReceiveCheckAllETH(ctx, w, cmdCtx, wlt)
	}
	addr := currentAddr
	if receiveAddress != "" {
		var err error
		addr, err = findWalletAddress(wlt, chain.ETH, receiveAddress)
		if err != nil {
			return err
		}
	}
	return runReceiveCheckSingleETH(ctx, w, cmdCtx, addr)
}

// runReceiveCheckBSV dispatches BSV UTXO checking for --check mode.
func runReceiveCheckBSV(ctx context.Context, w io.Writer, cmdCtx *CommandContext, wlt *wallet.Wallet, store *utxostore.Store, currentAddr *wallet.Address, chainID chain.ID) error {
	client := bsv.NewClient(ctx, &bsv.ClientOptions{
		APIKey: cmdCtx.Cfg.GetBSVAPIKey(),
	})
	adapter := &bsvRefreshAdapter{client: client}

	switch {
	case receiveAll:
		runReceiveCheckAll(ctx, w, cmdCtx, wlt, store, adapter, chainID)
		return nil
	case receiveAddress != "":
		addr, err := findWalletAddress(wlt, chainID, receiveAddress)
		if err != nil {
			return err
		}
		return runReceiveCheckSingle(ctx, w, cmdCtx, store, adapter, addr, chainID)
	default:
		return runReceiveCheckSingle(ctx, w, cmdCtx, store, adapter, currentAddr, chainID)
	}
}

// addressCheckResult holds the result of checking a single address.
type addressCheckResult struct {
	Addr    *wallet.Address
	Label   string
	Balance uint64
	UTXOs   []*utxostore.StoredUTXO
	Err     error
}

// runReceiveCheckSingle checks a single address and displays the result.
func runReceiveCheckSingle(ctx context.Context, w io.Writer, cmdCtx *CommandContext, store *utxostore.Store, adapter *bsvRefreshAdapter, addr *wallet.Address, chainID chain.ID) error {
	_, err := store.RefreshAddress(ctx, addr.Address, chainID, adapter)
	if err != nil {
		return fmt.Errorf("checking address %s: %w", addr.Address, err)
	}

	balance := store.GetAddressBalance(chainID, addr.Address)
	utxos := store.GetUTXOs(chainID, addr.Address)

	label := ""
	if meta := store.GetAddress(chainID, addr.Address); meta != nil {
		label = meta.Label
	}

	if cmdCtx.Fmt.Format() == output.FormatJSON {
		displayReceiveCheckJSON(w, addr, chainID, label, balance, utxos)
	} else {
		displayReceiveCheckText(w, addr, chainID, label, balance, utxos)
	}

	return nil
}

// runReceiveCheckAll checks all receiving addresses and displays a summary.
func runReceiveCheckAll(ctx context.Context, w io.Writer, cmdCtx *CommandContext, wlt *wallet.Wallet, store *utxostore.Store, adapter *bsvRefreshAdapter, chainID chain.ID) {
	addresses, ok := wlt.Addresses[chainID]
	if !ok || len(addresses) == 0 {
		outln(w)
		outln(w, "No receiving addresses found for", chainID)
		return
	}

	results := make([]addressCheckResult, 0, len(addresses))

	for i := range addresses {
		addr := &addresses[i]

		label := ""
		if meta := store.GetAddress(chainID, addr.Address); meta != nil {
			label = meta.Label
		}

		_, err := store.RefreshAddress(ctx, addr.Address, chainID, adapter)
		if err != nil {
			results = append(results, addressCheckResult{
				Addr:  addr,
				Label: label,
				Err:   err,
			})
			continue
		}

		balance := store.GetAddressBalance(chainID, addr.Address)
		utxos := store.GetUTXOs(chainID, addr.Address)

		results = append(results, addressCheckResult{
			Addr:    addr,
			Label:   label,
			Balance: balance,
			UTXOs:   utxos,
		})
	}

	if cmdCtx.Fmt.Format() == output.FormatJSON {
		displayReceiveCheckAllJSON(w, chainID, results)
	} else {
		displayReceiveCheckAllText(w, chainID, results)
	}
}

// ethCheckResult holds the result of checking a single ETH address balance.
type ethCheckResult struct {
	Addr       *wallet.Address
	ETHBalance string
	Err        error
}

// runReceiveCheckAllChains checks all receiving addresses across all enabled MVP chains.
func runReceiveCheckAllChains(cmd *cobra.Command, cmdCtx *CommandContext, wlt *wallet.Wallet, store *utxostore.Store) error {
	ctx, cancel := contextWithTimeout(cmd, checkTimeout)
	defer cancel()

	w := cmd.OutOrStdout()

	// Check BSV addresses (UTXO-based)
	if bsvAddrs, ok := wlt.Addresses[chain.BSV]; ok && len(bsvAddrs) > 0 {
		client := bsv.NewClient(ctx, &bsv.ClientOptions{
			APIKey: cmdCtx.Cfg.GetBSVAPIKey(),
		})
		adapter := &bsvRefreshAdapter{client: client}
		runReceiveCheckAll(ctx, w, cmdCtx, wlt, store, adapter, chain.BSV)
	}

	// Check ETH addresses (account-based balance)
	if ethAddrs, ok := wlt.Addresses[chain.ETH]; ok && len(ethAddrs) > 0 {
		if err := runReceiveCheckAllETH(ctx, w, cmdCtx, wlt); err != nil {
			outln(w)
			out(w, "ETH check error: %s\n", err)
		}
	}

	return nil
}

// runReceiveCheckAllETH checks all ETH receiving addresses and displays balances.
func runReceiveCheckAllETH(ctx context.Context, w io.Writer, cmdCtx *CommandContext, wlt *wallet.Wallet) error {
	addresses, ok := wlt.Addresses[chain.ETH]
	if !ok || len(addresses) == 0 {
		outln(w)
		outln(w, "No receiving addresses found for eth")
		return nil
	}

	apiKey := cmdCtx.Cfg.GetETHEtherscanAPIKey()
	if apiKey == "" {
		return sigilerr.WithSuggestion(
			etherscan.ErrAPIKeyRequired,
			"Set ETHERSCAN_API_KEY environment variable to check ETH balances",
		)
	}

	client, err := etherscan.NewClient(apiKey, nil)
	if err != nil {
		return fmt.Errorf("creating Etherscan client: %w", err)
	}

	results := make([]ethCheckResult, 0, len(addresses))
	for i := range addresses {
		addr := &addresses[i]

		balance, fetchErr := client.GetNativeBalance(ctx, addr.Address)
		if fetchErr != nil {
			results = append(results, ethCheckResult{Addr: addr, Err: fetchErr})
			continue
		}

		results = append(results, ethCheckResult{
			Addr:       addr,
			ETHBalance: eth.FormatBalanceAmount(balance.Amount, balance.Decimals),
		})
	}

	if cmdCtx.Fmt.Format() == output.FormatJSON {
		displayReceiveCheckAllETHJSON(w, results)
	} else {
		displayReceiveCheckAllETHText(w, results)
	}
	return nil
}

// runReceiveCheckSingleETH checks a single ETH address balance.
func runReceiveCheckSingleETH(ctx context.Context, w io.Writer, cmdCtx *CommandContext, addr *wallet.Address) error {
	apiKey := cmdCtx.Cfg.GetETHEtherscanAPIKey()
	if apiKey == "" {
		return sigilerr.WithSuggestion(
			etherscan.ErrAPIKeyRequired,
			"Set ETHERSCAN_API_KEY environment variable to check ETH balances",
		)
	}

	client, err := etherscan.NewClient(apiKey, nil)
	if err != nil {
		return fmt.Errorf("creating Etherscan client: %w", err)
	}

	balance, err := client.GetNativeBalance(ctx, addr.Address)
	if err != nil {
		return fmt.Errorf("checking ETH balance for %s: %w", addr.Address, err)
	}

	outln(w)
	outln(w, "Receive address check:")
	outln(w)
	out(w, "  Chain:   eth\n")
	out(w, "  Address: %s\n", addr.Address)
	out(w, "  Path:    %s\n", addr.Path)
	out(w, "  Index:   %d\n", addr.Index)
	outln(w)
	out(w, "  Balance: %s ETH\n", eth.FormatBalanceAmount(balance.Amount, balance.Decimals))
	outln(w)
	outln(w, "View on Etherscan:")
	out(w, "  https://etherscan.io/address/%s\n", addr.Address)

	return nil
}

// displayReceiveCheckAllETHText displays ETH address check results in text format.
func displayReceiveCheckAllETHText(w io.Writer, results []ethCheckResult) {
	outln(w)
	out(w, "Checking %d receiving address(es) on eth...\n", len(results))
	outln(w)

	var errorCount int
	for _, r := range results {
		if r.Err != nil {
			out(w, "  %-22s %-20s ERROR: %s\n", r.Addr.Path, truncateAddr(r.Addr.Address), r.Err)
			errorCount++
			continue
		}
		out(w, "  %-22s %-20s %s ETH\n", r.Addr.Path, truncateAddr(r.Addr.Address), r.ETHBalance)
	}

	outln(w)
	if errorCount > 0 {
		out(w, "Errors: %d address(es) failed to check\n", errorCount)
	}
}

// displayReceiveCheckAllETHJSON displays ETH address check results in JSON format.
func displayReceiveCheckAllETHJSON(w io.Writer, results []ethCheckResult) {
	type addressJSON struct {
		Address string `json:"address"`
		Path    string `json:"path"`
		Index   uint32 `json:"index"`
		Balance string `json:"balance"`
		Symbol  string `json:"symbol"`
		Error   string `json:"error,omitempty"`
	}

	addrList := make([]addressJSON, 0, len(results))
	for _, r := range results {
		entry := addressJSON{
			Address: r.Addr.Address,
			Path:    r.Addr.Path,
			Index:   r.Addr.Index,
			Symbol:  "ETH",
		}
		if r.Err != nil {
			entry.Error = r.Err.Error()
		} else {
			entry.Balance = r.ETHBalance
		}
		addrList = append(addrList, entry)
	}

	payload := struct {
		Chain            string        `json:"chain"`
		AddressesChecked int           `json:"addresses_checked"`
		Addresses        []addressJSON `json:"addresses"`
	}{
		Chain:            "eth",
		AddressesChecked: len(results),
		Addresses:        addrList,
	}

	_ = writeJSON(w, payload)
}

// findWalletAddress looks up a specific address in the wallet's receiving addresses.
func findWalletAddress(wlt *wallet.Wallet, chainID chain.ID, address string) (*wallet.Address, error) {
	addresses, ok := wlt.Addresses[chainID]
	if ok {
		for i := range addresses {
			if addresses[i].Address == address {
				return &addresses[i], nil
			}
		}
	}
	return nil, sigilerr.WithSuggestion(
		sigilerr.ErrInvalidInput,
		fmt.Sprintf("address %s not found in wallet receive addresses for %s", address, chainID),
	)
}

// displayReceiveCheckText shows the check result for a single address in text format.
func displayReceiveCheckText(w io.Writer, addr *wallet.Address, chainID chain.ID, label string, balance uint64, utxos []*utxostore.StoredUTXO) {
	outln(w)
	outln(w, "Receive address check:")
	outln(w)
	out(w, "  Chain:   %s\n", chainID)
	out(w, "  Address: %s\n", addr.Address)
	out(w, "  Path:    %s\n", addr.Path)
	out(w, "  Index:   %d\n", addr.Index)
	if label != "" {
		out(w, "  Label:   %s\n", label)
	}
	outln(w)

	if len(utxos) == 0 {
		outln(w, "  Status:  No funds received yet")
	} else {
		outln(w, "  Status:  Funds received")
		out(w, "  UTXOs:   %d\n", len(utxos))
		out(w, "  Balance: %d satoshis (%.8f BSV)\n", balance, float64(balance)/1e8)
	}
	outln(w)

	switch chainID {
	case chain.BSV:
		outln(w, "View on WhatsOnChain:")
		out(w, "  https://whatsonchain.com/address/%s\n", addr.Address)
	case chain.ETH:
		outln(w, "View on Etherscan:")
		out(w, "  https://etherscan.io/address/%s\n", addr.Address)
	case chain.BTC, chain.BCH:
		// Future chains
	}
}

// displayReceiveCheckJSON shows the check result for a single address in JSON format.
func displayReceiveCheckJSON(w io.Writer, addr *wallet.Address, chainID chain.ID, label string, balance uint64, utxos []*utxostore.StoredUTXO) {
	type utxoJSON struct {
		TxID          string `json:"txid"`
		Vout          uint32 `json:"vout"`
		Amount        uint64 `json:"amount"`
		Confirmations uint32 `json:"confirmations"`
	}

	utxoList := make([]utxoJSON, 0, len(utxos))
	for _, u := range utxos {
		utxoList = append(utxoList, utxoJSON{
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			Confirmations: u.Confirmations,
		})
	}

	payload := struct {
		Chain      string     `json:"chain"`
		Address    string     `json:"address"`
		Path       string     `json:"path"`
		Index      uint32     `json:"index"`
		Label      string     `json:"label,omitempty"`
		HasFunds   bool       `json:"has_funds"`
		Balance    uint64     `json:"balance"`
		BalanceBSV float64    `json:"balance_bsv"`
		UTXOCount  int        `json:"utxo_count"`
		UTXOs      []utxoJSON `json:"utxos"`
	}{
		Chain:      string(chainID),
		Address:    addr.Address,
		Path:       addr.Path,
		Index:      addr.Index,
		Label:      label,
		HasFunds:   len(utxos) > 0,
		Balance:    balance,
		BalanceBSV: float64(balance) / 1e8,
		UTXOCount:  len(utxos),
		UTXOs:      utxoList,
	}

	_ = writeJSON(w, payload)
}

// formatUTXOCount returns a human-readable UTXO count string for tabular display.
func formatUTXOCount(count int) string {
	switch count {
	case 0:
		return "-"
	case 1:
		return "(1 UTXO)"
	default:
		return fmt.Sprintf("(%d UTXOs)", count)
	}
}

// displayReceiveCheckAllText shows the check results for multiple addresses in text format.
func displayReceiveCheckAllText(w io.Writer, chainID chain.ID, results []addressCheckResult) {
	outln(w)
	out(w, "Checking %d receiving address(es) on %s...\n", len(results), chainID)
	outln(w)

	var totalBalance uint64
	var totalUTXOs int
	var fundedAddresses int
	var errorCount int

	for _, r := range results {
		if r.Err != nil {
			out(w, "  %-22s %-20s ERROR: %s\n", r.Addr.Path, truncateAddr(r.Addr.Address), r.Err)
			errorCount++
			continue
		}

		utxoCount := len(r.UTXOs)
		totalBalance += r.Balance
		totalUTXOs += utxoCount
		if utxoCount > 0 {
			fundedAddresses++
		}

		labelSuffix := ""
		if r.Label != "" {
			labelSuffix = fmt.Sprintf("  [%s]", r.Label)
		}

		out(w, "  %-22s %-20s %.8f BSV  %s%s\n", r.Addr.Path, truncateAddr(r.Addr.Address), float64(r.Balance)/1e8, formatUTXOCount(utxoCount), labelSuffix)
	}

	outln(w)
	out(w, "Total: %.8f BSV (%d UTXOs across %d addresses)\n", float64(totalBalance)/1e8, totalUTXOs, fundedAddresses)
	if errorCount > 0 {
		out(w, "Errors: %d address(es) failed to check\n", errorCount)
	}
}

// displayReceiveCheckAllJSON shows the check results for multiple addresses in JSON format.
func displayReceiveCheckAllJSON(w io.Writer, chainID chain.ID, results []addressCheckResult) {
	type utxoJSON struct {
		TxID          string `json:"txid"`
		Vout          uint32 `json:"vout"`
		Amount        uint64 `json:"amount"`
		Confirmations uint32 `json:"confirmations"`
	}

	type addressJSON struct {
		Address   string     `json:"address"`
		Path      string     `json:"path"`
		Index     uint32     `json:"index"`
		Label     string     `json:"label,omitempty"`
		HasFunds  bool       `json:"has_funds"`
		Balance   uint64     `json:"balance"`
		UTXOCount int        `json:"utxo_count"`
		UTXOs     []utxoJSON `json:"utxos"`
		Error     string     `json:"error,omitempty"`
	}

	var totalBalance uint64
	var totalUTXOs int

	addrList := make([]addressJSON, 0, len(results))
	for _, r := range results {
		entry := addressJSON{
			Address: r.Addr.Address,
			Path:    r.Addr.Path,
			Index:   r.Addr.Index,
			Label:   r.Label,
			UTXOs:   make([]utxoJSON, 0),
		}

		if r.Err != nil {
			entry.Error = r.Err.Error()
		} else {
			entry.HasFunds = len(r.UTXOs) > 0
			entry.Balance = r.Balance
			entry.UTXOCount = len(r.UTXOs)
			totalBalance += r.Balance
			totalUTXOs += len(r.UTXOs)

			for _, u := range r.UTXOs {
				entry.UTXOs = append(entry.UTXOs, utxoJSON{
					TxID:          u.TxID,
					Vout:          u.Vout,
					Amount:        u.Amount,
					Confirmations: u.Confirmations,
				})
			}
		}

		addrList = append(addrList, entry)
	}

	payload := struct {
		Chain            string        `json:"chain"`
		AddressesChecked int           `json:"addresses_checked"`
		TotalBalance     uint64        `json:"total_balance"`
		TotalBalanceBSV  float64       `json:"total_balance_bsv"`
		TotalUTXOCount   int           `json:"total_utxo_count"`
		Addresses        []addressJSON `json:"addresses"`
	}{
		Chain:            string(chainID),
		AddressesChecked: len(results),
		TotalBalance:     totalBalance,
		TotalBalanceBSV:  float64(totalBalance) / 1e8,
		TotalUTXOCount:   totalUTXOs,
		Addresses:        addrList,
	}

	_ = writeJSON(w, payload)
}

// truncateAddr shortens an address for display in tabular output.
func truncateAddr(addr string) string {
	if len(addr) <= 16 {
		return addr
	}
	return addr[:8] + "..." + addr[len(addr)-5:]
}
