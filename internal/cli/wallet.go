package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"sigil/internal/output"
	"sigil/internal/wallet"
	sigilerr "sigil/pkg/errors"
)

// out is a helper for CLI output that ignores write errors (standard pattern for CLI tools).
//
//nolint:errcheck // CLI output writes to stdout are intentionally unchecked
func out(w io.Writer, format string, args ...interface{}) {
	fmt.Fprintf(w, format, args...)
}

// outln is a helper for CLI output with newline.
//
//nolint:errcheck // CLI output writes to stdout are intentionally unchecked
func outln(w io.Writer, args ...interface{}) {
	fmt.Fprintln(w, args...)
}

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	// createWords is the number of words for mnemonic generation.
	createWords int
	// createPassphrase indicates whether to prompt for BIP39 passphrase.
	createPassphrase bool
)

// walletCmd is the parent command for wallet operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var walletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Manage wallets",
	Long:  `Create, restore, list, and manage HD wallets.`,
}

// walletCreateCmd creates a new wallet.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var walletCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new HD wallet",
	Long: `Create a new HD wallet with a BIP39 mnemonic phrase.

The mnemonic will be displayed once - write it down and store it securely.
You will be prompted for a password to encrypt the wallet file.

Example:
  sigil wallet create main
  sigil wallet create main --words 24
  sigil wallet create main --passphrase`,
	Args: cobra.ExactArgs(1),
	RunE: runWalletCreate,
}

// walletListCmd lists all wallets.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var walletListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all wallets",
	Long: `List all wallets in the sigil data directory.

Example:
  sigil wallet list
  sigil wallet list -o json`,
	Aliases: []string{"ls"},
	RunE:    runWalletList,
}

// walletShowCmd shows wallet details.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var walletShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show wallet details",
	Long: `Show details for a specific wallet including all derived addresses.

You will be prompted for the wallet password to decrypt and display addresses.

Example:
  sigil wallet show main`,
	Args: cobra.ExactArgs(1),
	RunE: runWalletShow,
}

// validateWalletCreationParams validates inputs for wallet creation.
func validateWalletCreationParams(name string, wordCount int, storage *wallet.FileStorage) error {
	if wordCount != 12 && wordCount != 24 {
		return sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"word count must be 12 or 24",
		)
	}

	exists, err := storage.Exists(name)
	if err != nil {
		return err
	}
	if exists {
		return sigilerr.WithSuggestion(
			wallet.ErrWalletExists,
			fmt.Sprintf("wallet '%s' already exists. Choose a different name or delete existing wallet.", name),
		)
	}

	return nil
}

// generateWalletSeed generates mnemonic and derives seed with optional passphrase.
func generateWalletSeed(wordCount int, usePassphrase bool) (mnemonic string, seed []byte, err error) {
	mnemonic, err = wallet.GenerateMnemonic(wordCount)
	if err != nil {
		return "", nil, err
	}

	var passphrase string
	if usePassphrase {
		passphrase, err = promptPassphrase()
		if err != nil {
			return "", nil, err
		}
	}

	seed, err = wallet.MnemonicToSeed(mnemonic, passphrase)
	if err != nil {
		return "", nil, err
	}

	return mnemonic, seed, nil
}

// createAndSaveWallet creates wallet, derives addresses, and saves to storage.
func createAndSaveWallet(name string, seed []byte, storage *wallet.FileStorage) (*wallet.Wallet, error) {
	w, err := wallet.NewWallet(name, []wallet.Chain{wallet.ChainETH, wallet.ChainBSV})
	if err != nil {
		return nil, err
	}

	err = w.DeriveAddresses(seed, 1)
	if err != nil {
		return nil, err
	}

	password, err := promptNewPassword()
	if err != nil {
		return nil, err
	}

	err = storage.Save(w, seed, password)
	if err != nil {
		return nil, err
	}

	return w, nil
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(walletCmd)
	walletCmd.AddCommand(walletCreateCmd)
	walletCmd.AddCommand(walletListCmd)
	walletCmd.AddCommand(walletShowCmd)

	walletCreateCmd.Flags().IntVar(&createWords, "words", 12, "mnemonic word count (12 or 24)")
	walletCreateCmd.Flags().BoolVar(&createPassphrase, "passphrase", false, "use a BIP39 passphrase")
}

func runWalletCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	storage := wallet.NewFileStorage(filepath.Join(cfg.Home, "wallets"))

	// Validate inputs
	if err := validateWalletCreationParams(name, createWords, storage); err != nil {
		return err
	}

	// Generate mnemonic and seed
	mnemonic, seed, err := generateWalletSeed(createWords, createPassphrase)
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(seed)

	// Create and save wallet
	w, err := createAndSaveWallet(name, seed, storage)
	if err != nil {
		return err
	}

	// Display results
	displayMnemonic(mnemonic, cmd)
	displayWalletAddresses(w, cmd)

	outln(cmd.OutOrStdout())
	out(cmd.OutOrStdout(), "Wallet '%s' created successfully.\n", name)
	outln(cmd.OutOrStdout(), "Wallet file: "+filepath.Join(cfg.Home, "wallets", name+".wallet"))

	return nil
}

// formatEmptyWalletList formats empty wallet list based on output format.
func formatEmptyWalletList(w io.Writer, format output.Format) {
	if format == output.FormatJSON {
		outln(w, "[]")
	} else {
		outln(w, "No wallets found.")
		outln(w, "Create one with: sigil wallet create <name>")
	}
}

// formatWalletListJSON outputs wallet names as JSON array.
func formatWalletListJSON(w io.Writer, names []string) {
	out(w, "[")
	for i, name := range names {
		if i > 0 {
			out(w, ",")
		}
		out(w, `"%s"`, name)
	}
	outln(w, "]")
}

// formatWalletListText outputs wallet names as text list.
func formatWalletListText(w io.Writer, names []string) {
	outln(w, "Wallets:")
	for _, name := range names {
		out(w, "  • %s\n", name)
	}
}

func runWalletList(cmd *cobra.Command, _ []string) error {
	storage := wallet.NewFileStorage(filepath.Join(cfg.Home, "wallets"))

	names, err := storage.List()
	if err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	format := formatter.Format()

	if len(names) == 0 {
		formatEmptyWalletList(w, format)
		return nil
	}

	if format == output.FormatJSON {
		formatWalletListJSON(w, names)
	} else {
		formatWalletListText(w, names)
	}

	return nil
}

func runWalletShow(cmd *cobra.Command, args []string) error {
	name := args[0]

	storage := wallet.NewFileStorage(filepath.Join(cfg.Home, "wallets"))

	// Check if wallet exists
	exists, err := storage.Exists(name)
	if err != nil {
		return err
	}
	if !exists {
		return sigilerr.WithSuggestion(
			wallet.ErrWalletNotFound,
			fmt.Sprintf("wallet '%s' not found. List wallets with: sigil wallet list", name),
		)
	}

	// Prompt for password
	password, err := promptPassword("Enter wallet password: ")
	if err != nil {
		return err
	}

	// Load wallet
	w, seed, err := storage.Load(name, password)
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(seed)

	// Display wallet info
	if formatter.Format() == output.FormatJSON {
		displayWalletJSON(w, cmd)
	} else {
		displayWalletText(w, cmd)
	}

	return nil
}

// displayMnemonic shows the mnemonic phrase with formatting.
func displayMnemonic(mnemonic string, cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w, "                    RECOVERY PHRASE")
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w)
	outln(w, "Write down these words in order and store them securely.")
	outln(w, "This is the ONLY way to recover your wallet.")
	outln(w)

	words := strings.Fields(mnemonic)
	for i, word := range words {
		out(w, "%2d. %s\n", i+1, word)
	}

	outln(w)
	outln(w, "═══════════════════════════════════════════════════════════════")
	outln(w)
}

// displayWalletAddresses shows the derived addresses.
func displayWalletAddresses(wlt *wallet.Wallet, cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	outln(w, "Derived Addresses:")
	for chain, addresses := range wlt.Addresses {
		if len(addresses) > 0 {
			out(w, "  %s: %s\n", strings.ToUpper(string(chain)), addresses[0].Address)
		}
	}
}

// displayWalletText shows wallet details in text format.
func displayWalletText(wlt *wallet.Wallet, cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	out(w, "Wallet: %s\n", wlt.Name)
	out(w, "Created: %s\n", wlt.CreatedAt.Format("2006-01-02 15:04:05"))
	out(w, "Version: %d\n", wlt.Version)
	outln(w)
	outln(w, "Addresses:")
	for chain, addresses := range wlt.Addresses {
		out(w, "  %s:\n", strings.ToUpper(string(chain)))
		for _, addr := range addresses {
			out(w, "    [%d] %s\n", addr.Index, addr.Address)
			out(w, "        Path: %s\n", addr.Path)
		}
	}
}

// displayWalletJSON shows wallet details in JSON format.
func displayWalletJSON(wlt *wallet.Wallet, cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	// Build JSON manually to control format
	outln(w, "{")
	out(w, `  "name": "%s",`+"\n", wlt.Name)
	out(w, `  "created_at": "%s",`+"\n", wlt.CreatedAt.Format("2006-01-02T15:04:05Z"))
	out(w, `  "version": %d,`+"\n", wlt.Version)
	outln(w, `  "addresses": {`)

	chainCount := 0
	for chain, addresses := range wlt.Addresses {
		if chainCount > 0 {
			outln(w, ",")
		}
		out(w, `    "%s": [`, chain)
		for i, addr := range addresses {
			if i > 0 {
				out(w, ",")
			}
			out(w, `{"index": %d, "address": "%s", "path": "%s"}`, addr.Index, addr.Address, addr.Path)
		}
		out(w, "]")
		chainCount++
	}
	outln(w)
	outln(w, "  }")
	outln(w, "}")
}

// promptPassword prompts for a password with hidden input.
func promptPassword(prompt string) (string, error) {
	out(os.Stderr, "%s", prompt)

	password, err := term.ReadPassword(syscall.Stdin)
	outln(os.Stderr) // Add newline after hidden input

	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}

	return string(password), nil
}

// promptNewPassword prompts for a new password with confirmation.
func promptNewPassword() (string, error) {
	password, err := promptPassword("Enter encryption password: ")
	if err != nil {
		return "", err
	}

	if len(password) < 8 {
		return "", sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"password must be at least 8 characters",
		)
	}

	confirm, err := promptPassword("Confirm password: ")
	if err != nil {
		return "", err
	}

	if password != confirm {
		return "", sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"passwords do not match",
		)
	}

	return password, nil
}

// promptPassphrase prompts for an optional BIP39 passphrase.
func promptPassphrase() (string, error) {
	outln(os.Stderr, "\nBIP39 Passphrase (optional extra security layer):")
	outln(os.Stderr, "WARNING: If you lose this passphrase, you cannot recover your wallet!")

	passphrase, err := promptPassword("Enter passphrase: ")
	if err != nil {
		return "", err
	}

	if passphrase == "" {
		return "", nil
	}

	confirm, err := promptPassword("Confirm passphrase: ")
	if err != nil {
		return "", err
	}

	if passphrase != confirm {
		return "", sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"passphrases do not match",
		)
	}

	return passphrase, nil
}
