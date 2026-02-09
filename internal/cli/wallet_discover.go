package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/discovery"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	// discoverInput is the mnemonic phrase for discovery.
	discoverInput string
	// discoverPassphrase indicates whether to prompt for BIP39 passphrase.
	discoverPassphrase bool
	// discoverGap is the gap limit for address scanning.
	discoverGap int
	// discoverPath is a custom derivation path to scan.
	discoverPath string
	// discoverMigrate indicates whether to migrate funds to a sigil wallet.
	discoverMigrate bool
	// discoverWallet is the target wallet name for migration.
	discoverWallet string
	// discoverScheme is a specific path scheme to scan.
	discoverScheme string
)

// walletDiscoverCmd discovers funds across multiple derivation paths.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var walletDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover funds across multiple derivation paths",
	Long: `Discover funds from any BSV wallet by scanning multiple derivation paths.

This command scans common derivation path schemes used by various BSV wallets
(RelayX, MoneyButton, ElectrumSV, HandCash 1.x, etc.) to find funds that may
have been created with different wallet software.

Supported wallet schemes:
  - BSV Standard (m/44'/236'/...): RelayX, RockWallet, Twetch, Trezor, Ledger
  - Bitcoin Legacy (m/44'/0'/...): MoneyButton, ElectrumSV imports
  - Bitcoin Cash (m/44'/145'/...): Exodus, Simply.Cash, BCH fork splits
  - HandCash Legacy (m/0'/...): HandCash 1.x only

Known limitations:
  - HandCash 2.0 uses proprietary non-exportable keys (cannot be imported)
  - Centbee uses 4-digit PIN as BIP39 passphrase (use --passphrase flag)

Examples:
  sigil wallet discover --input "abandon abandon ... about"
  sigil wallet discover --passphrase
  sigil wallet discover --gap 50
  sigil wallet discover -o json
  sigil wallet discover --migrate --wallet main`,
	RunE: runWalletDiscover,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	walletCmd.AddCommand(walletDiscoverCmd)

	walletDiscoverCmd.Flags().StringVar(&discoverInput, "input", "", "mnemonic phrase (or interactive prompt)")
	walletDiscoverCmd.Flags().BoolVar(&discoverPassphrase, "passphrase", false, "prompt for BIP39 passphrase")
	walletDiscoverCmd.Flags().IntVar(&discoverGap, "gap", discovery.DefaultGapLimit, "gap limit for address scanning")
	walletDiscoverCmd.Flags().StringVar(&discoverPath, "path", "", "custom derivation path to scan")
	walletDiscoverCmd.Flags().BoolVar(&discoverMigrate, "migrate", false, "consolidate funds to sigil wallet")
	walletDiscoverCmd.Flags().StringVar(&discoverWallet, "wallet", "", "target wallet name for migration")
	walletDiscoverCmd.Flags().StringVar(&discoverScheme, "scheme", "", "scan only a specific scheme (e.g., 'BSV Standard')")
}

// DiscoverResponse is the JSON response for the discover command.
type DiscoverResponse struct {
	TotalBalance     uint64                     `json:"total_balance"`
	TotalUTXOs       int                        `json:"total_utxos"`
	Addresses        []DiscoverAddressResponse  `json:"addresses"`
	SchemesScanned   []string                   `json:"schemes_scanned"`
	AddressesScanned int                        `json:"addresses_scanned"`
	DurationMs       int64                      `json:"duration_ms"`
	PassphraseUsed   bool                       `json:"passphrase_used,omitempty"`
	Errors           []string                   `json:"errors,omitempty"`
	Migration        *DiscoverMigrationResponse `json:"migration,omitempty"`
}

// DiscoverAddressResponse represents a discovered address in the response.
type DiscoverAddressResponse struct {
	Scheme    string `json:"scheme"`
	Address   string `json:"address"`
	Path      string `json:"path"`
	Balance   uint64 `json:"balance"`
	UTXOCount int    `json:"utxo_count"`
	IsChange  bool   `json:"is_change,omitempty"`
}

// DiscoverMigrationResponse represents migration info in the response.
type DiscoverMigrationResponse struct {
	Destination  string `json:"destination"`
	TotalInput   uint64 `json:"total_input"`
	EstimatedFee uint64 `json:"estimated_fee"`
	NetAmount    uint64 `json:"net_amount"`
	TxID         string `json:"tx_id,omitempty"`
	Warning      string `json:"warning,omitempty"`
}

//nolint:gocognit,gocyclo // CLI command handler with validation, setup, and output - complexity is necessary
func runWalletDiscover(cmd *cobra.Command, _ []string) error {
	cc := GetCmdContext(cmd)

	// Validate flags
	if discoverMigrate && discoverWallet == "" {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"--wallet is required when using --migrate",
		)
	}

	if discoverGap <= 0 {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"gap limit must be positive",
		)
	}

	// Get mnemonic
	mnemonic := discoverInput
	if mnemonic == "" {
		var err error
		mnemonic, err = promptMnemonicForDiscover()
		if err != nil {
			return err
		}
	}

	// Validate mnemonic
	if err := wallet.ValidateMnemonic(mnemonic); err != nil {
		return sigilerr.WithSuggestion(
			err,
			"the mnemonic phrase is not valid. Check for typos or missing words.",
		)
	}

	// Get passphrase if requested
	var passphrase string
	if discoverPassphrase {
		var err error
		passphrase, err = promptPassphraseForDiscover()
		if err != nil {
			return err
		}
	}

	// Convert mnemonic to seed
	seed, err := wallet.MnemonicToSeed(mnemonic, passphrase)
	if err != nil {
		return fmt.Errorf("converting mnemonic to seed: %w", err)
	}
	defer wallet.ZeroBytes(seed)

	// Create BSV client
	client := bsv.NewClient(cmd.Context(), nil)

	// Create key deriver adapter
	deriver := &walletKeyDeriver{}

	// Configure options
	opts := discovery.DefaultOptions()
	opts.GapLimit = discoverGap
	if discoverScheme != "" {
		scheme := discovery.SchemeByName(discoverScheme)
		if scheme == nil {
			return sigilerr.WithSuggestion(
				sigilerr.ErrInvalidInput,
				fmt.Sprintf("unknown scheme: %s. Available: BSV Standard, Bitcoin Legacy, Bitcoin Cash, HandCash Legacy", discoverScheme),
			)
		}
		opts.PathSchemes = []discovery.PathScheme{*scheme}
	}

	// Set up progress callback for text output
	if cc.Fmt.Format() != output.FormatJSON {
		opts.ProgressCallback = createProgressCallback(cmd.OutOrStderr())
	}

	// Create scanner
	scanner := discovery.NewScanner(&discoveryClientAdapter{client}, deriver, opts)

	// Run discovery
	outln(cmd.OutOrStderr(), "\nScanning derivation paths...")
	ctx, cancel := contextWithTimeout(cmd, discovery.DefaultTimeout)
	defer cancel()

	var result *discovery.Result
	if discoverPath != "" {
		// Custom path scan - use BSV coin type by default
		result, err = scanner.ScanCustomPath(ctx, seed, discoverPath, discovery.CoinTypeBSV)
	} else {
		result, err = scanner.Scan(ctx, seed)
	}
	if err != nil {
		return fmt.Errorf("discovery scan failed: %w", err)
	}

	// Build response
	response := buildDiscoverResponse(result)

	// Handle migration if requested
	if discoverMigrate && result.HasFunds() {
		migrationResp, err := executeMigration(ctx, cmd, cc, seed, result, client)
		if err != nil {
			return err
		}
		response.Migration = migrationResp
	}

	// Output results
	if cc.Fmt.Format() == output.FormatJSON {
		outputDiscoverJSON(cmd.OutOrStdout(), response)
	} else {
		outputDiscoverText(cmd.OutOrStdout(), response, discoverMigrate && response.Migration != nil)
	}

	return nil
}

// promptMnemonicForDiscover prompts for a mnemonic phrase.
func promptMnemonicForDiscover() (string, error) {
	outln(os.Stderr, "Enter your mnemonic phrase (12 or 24 words):")
	return promptMnemonicInteractive()
}

// promptPassphraseForDiscover prompts for an optional BIP39 passphrase.
func promptPassphraseForDiscover() (string, error) {
	outln(os.Stderr, "\nBIP39 Passphrase:")
	outln(os.Stderr, "Note: For Centbee wallets, enter your 4-digit PIN here.")

	passphrase, err := promptPasswordFn("Enter passphrase (or press Enter for none): ")
	if err != nil {
		return "", err
	}

	result := string(passphrase)
	wallet.ZeroBytes(passphrase)
	return result, nil
}

// createProgressCallback creates a progress callback for text output.
func createProgressCallback(w io.Writer) discovery.ProgressCallback {
	var lastScheme string
	return func(update discovery.ProgressUpdate) {
		if update.SchemeName != lastScheme {
			out(w, "  %s...", update.SchemeName)
			lastScheme = update.SchemeName
		}
		if update.Phase == "found" {
			out(w, "\n    Found: %s (%d sats)\n", update.CurrentAddress, update.BalanceFound)
		}
	}
}

// buildDiscoverResponse builds the response from scan results.
func buildDiscoverResponse(result *discovery.Result) DiscoverResponse {
	response := DiscoverResponse{
		TotalBalance:     result.TotalBalance,
		TotalUTXOs:       result.TotalUTXOs,
		SchemesScanned:   result.SchemesScanned,
		AddressesScanned: result.AddressesScanned,
		DurationMs:       result.Duration.Milliseconds(),
		PassphraseUsed:   result.PassphraseUsed,
		Errors:           result.Errors,
	}

	for _, addr := range result.AllAddresses() {
		response.Addresses = append(response.Addresses, DiscoverAddressResponse{
			Scheme:    addr.SchemeName,
			Address:   addr.Address,
			Path:      addr.Path,
			Balance:   addr.Balance,
			UTXOCount: addr.UTXOCount,
			IsChange:  addr.IsChange,
		})
	}

	return response
}

// executeMigration executes fund migration to a sigil wallet.
func executeMigration(_ context.Context, cmd *cobra.Command, cmdCtx *CommandContext, _ []byte, result *discovery.Result, _ *bsv.Client) (*DiscoverMigrationResponse, error) {
	// Load target wallet
	storage := wallet.NewFileStorage(filepath.Join(cmdCtx.Cfg.GetHome(), "wallets"))

	targetWallet, targetSeed, err := loadWalletWithSession(discoverWallet, storage, cmd)
	if err != nil {
		return nil, fmt.Errorf("loading target wallet: %w", err)
	}
	defer wallet.ZeroBytes(targetSeed)

	// Get destination address (first BSV address)
	bsvAddrs, ok := targetWallet.Addresses[wallet.ChainBSV]
	if !ok || len(bsvAddrs) == 0 {
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"target wallet has no BSV addresses. Run 'sigil addresses derive' first.",
		)
	}
	destination := bsvAddrs[0].Address

	// Create migration plan
	plan, err := discovery.CreateMigrationPlan(result, destination, discovery.DefaultFeeRate)
	if err != nil {
		return nil, fmt.Errorf("creating migration plan: %w", err)
	}

	// Display plan and confirm
	outln(cmd.OutOrStdout())
	outln(cmd.OutOrStdout(), "═══════════════════════════════════════════════════════════════")
	outln(cmd.OutOrStdout(), "                    MIGRATION PLAN")
	outln(cmd.OutOrStdout(), "═══════════════════════════════════════════════════════════════")
	out(cmd.OutOrStdout(), "  Source Addresses:    %d\n", len(plan.Sources))
	out(cmd.OutOrStdout(), "  Destination:         %s\n", destination)
	out(cmd.OutOrStdout(), "  Total Input:         %s BSV\n", formatSatoshis(plan.TotalInput))
	out(cmd.OutOrStdout(), "  Estimated Fee:       %s BSV\n", formatSatoshis(plan.EstimatedFee))
	out(cmd.OutOrStdout(), "  Net Amount:          %s BSV\n", formatSatoshis(plan.NetAmount))

	if plan.Warning != "" {
		outln(cmd.OutOrStdout())
		out(cmd.OutOrStdout(), "  Warning: %s\n", plan.Warning)
	}
	outln(cmd.OutOrStdout())

	if !promptMigrationConfirmation() {
		outln(cmd.OutOrStdout(), "Migration canceled.")
		// Return empty response to indicate cancellation
		return &DiscoverMigrationResponse{}, nil
	}

	// TODO(migration): Full migration implementation would require transaction building
	// For now, return the plan without executing
	return &DiscoverMigrationResponse{
		Destination:  destination,
		TotalInput:   plan.TotalInput,
		EstimatedFee: plan.EstimatedFee,
		NetAmount:    plan.NetAmount,
		Warning:      plan.Warning,
		// TxID would be set after broadcast
	}, nil
}

// promptMigrationConfirmation asks user to confirm migration.
func promptMigrationConfirmation() bool {
	out(os.Stderr, "Proceed? [y/N]: ")

	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// outputDiscoverJSON outputs discovery results in JSON format.
func outputDiscoverJSON(w io.Writer, response DiscoverResponse) {
	if response.SchemesScanned == nil {
		response.SchemesScanned = []string{}
	}
	if response.Addresses == nil {
		response.Addresses = []DiscoverAddressResponse{}
	}
	_ = writeJSON(w, response)
}

// outputDiscoverText outputs discovery results in text format.
func outputDiscoverText(w io.Writer, response DiscoverResponse, showMigration bool) {
	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w, "                    DISCOVERED FUNDS")
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w)

	if len(response.Addresses) == 0 {
		outln(w, "No funds discovered.")
		outln(w)
		outln(w, "Suggestions:")
		outln(w, "  - Check if you entered the correct mnemonic")
		outln(w, "  - Try with --passphrase if your wallet used one")
		outln(w, "  - Use --gap 50 or higher for wallets with many addresses")
		return
	}

	// Print table header
	outln(w, "Scheme              Address              Path                    Balance")
	outln(w, "----------------    -----------------    --------------------    ----------")

	for _, addr := range response.Addresses {
		// Truncate address for display
		displayAddr := addr.Address
		if len(displayAddr) > 17 {
			displayAddr = displayAddr[:8] + "..." + displayAddr[len(displayAddr)-6:]
		}

		out(w, "%-18s  %-17s  %-22s  %s BSV\n",
			truncateString(addr.Scheme, 18),
			displayAddr,
			truncateString(addr.Path, 22),
			formatSatoshis(addr.Balance),
		)
	}

	outln(w)
	outln(w, "───────────────────────────────────────────────────────────────")
	out(w, "Total: %s BSV (%d addresses, %d UTXOs)\n",
		formatSatoshis(response.TotalBalance),
		len(response.Addresses),
		response.TotalUTXOs,
	)
	out(w, "Scan Time: %.1fs\n", float64(response.DurationMs)/1000.0)
	outln(w, "═══════════════════════════════════════════════════════════════")

	if !showMigration && response.TotalBalance > 0 {
		outln(w)
		outln(w, "Use --migrate --wallet <name> to consolidate funds.")
	}

	if len(response.Errors) > 0 {
		outln(w)
		outln(w, "Warnings:")
		for _, e := range response.Errors {
			out(w, "  - %s\n", e)
		}
	}
}

// truncateString truncates a string to maxLen with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// discoveryClientAdapter adapts the BSV client to the discovery.ChainClient interface.
type discoveryClientAdapter struct {
	client *bsv.Client
}

func (a *discoveryClientAdapter) ListUTXOs(ctx context.Context, address string) ([]discovery.UTXO, error) {
	utxos, err := a.client.ListUTXOs(ctx, address)
	if err != nil {
		return nil, err
	}

	result := make([]discovery.UTXO, len(utxos))
	for i, u := range utxos {
		result[i] = discovery.UTXO{
			TxID:         u.TxID,
			Vout:         u.Vout,
			Amount:       u.Amount,
			ScriptPubKey: u.ScriptPubKey,
			Address:      u.Address,
		}
	}
	return result, nil
}

func (a *discoveryClientAdapter) ValidateAddress(address string) error {
	return a.client.ValidateAddress(address)
}

// walletKeyDeriver adapts wallet derivation to the discovery.KeyDeriver interface.
type walletKeyDeriver struct{}

func (d *walletKeyDeriver) DeriveAddress(seed []byte, coinType, account, change, index uint32) (string, string, error) {
	addr, _, path, err := wallet.DeriveAddressWithCoinType(seed, coinType, account, change, index)
	return addr, path, err
}

func (d *walletKeyDeriver) DeriveLegacyAddress(seed []byte, index uint32) (string, string, error) {
	addr, _, path, err := wallet.DeriveLegacyAddress(seed, index)
	return addr, path, err
}
