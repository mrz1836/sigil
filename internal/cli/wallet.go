package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
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
	// createScan indicates whether to scan for existing UTXOs after creation.
	createScan bool
	// restoreInput is the seed material for wallet restoration.
	restoreInput string
	// restorePassphrase indicates whether to prompt for BIP39 passphrase during restore.
	restorePassphrase bool
	// restoreScan indicates whether to scan for existing UTXOs after restore.
	restoreScan bool
)

// walletCmd is the parent command for wallet operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var walletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Manage wallets",
	Long: `Create, restore, list, and manage HD wallets.

Sigil uses BIP39 mnemonics and BIP32/BIP44 hierarchical deterministic key derivation.
Wallets support multiple chains (BSV, ETH) from a single seed phrase.`,
}

// walletCreateCmd creates a new wallet.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var walletCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new HD wallet",
	Long: `Create a new HD wallet with a BIP39 mnemonic phrase.

The mnemonic will be displayed once - write it down and store it securely.
You will be prompted for a password to encrypt the wallet file.`,
	Example: `  sigil wallet create main
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
	Long:  `List all wallets in the sigil data directory.`,
	Example: `  sigil wallet list
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

You will be prompted for the wallet password to decrypt and display addresses.`,
	Example: `  sigil wallet show main`,
	Args:    cobra.ExactArgs(1),
	RunE:    runWalletShow,
}

// walletRestoreCmd restores a wallet from a mnemonic, WIF, or hex key.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var walletRestoreCmd = &cobra.Command{
	Use:   "restore <name>",
	Short: "Restore a wallet from mnemonic, WIF, or hex key",
	Long: `Restore a wallet from a BIP39 mnemonic phrase, WIF private key, or hex private key.

The input format is automatically detected. You can provide the seed material
via the --input flag or be guided through interactive prompts.`,
	Example: `  sigil wallet restore backup --input "abandon abandon ... about"
  sigil wallet restore imported --input "5HueCGU8rMjxEXxiPuD5BDku..."
  sigil wallet restore backup  # Interactive mode`,
	Args: cobra.ExactArgs(1),
	RunE: runWalletRestore,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	walletCmd.GroupID = "wallet"
	rootCmd.AddCommand(walletCmd)
	walletCmd.AddCommand(walletCreateCmd)
	walletCmd.AddCommand(walletListCmd)
	walletCmd.AddCommand(walletShowCmd)
	walletCmd.AddCommand(walletRestoreCmd)

	walletCreateCmd.Flags().IntVar(&createWords, "words", 12, "mnemonic word count (12 or 24)")
	walletCreateCmd.Flags().BoolVar(&createPassphrase, "passphrase", false, "use a BIP39 passphrase")
	walletCreateCmd.Flags().BoolVar(&createScan, "scan", false, "scan for existing UTXOs after creation")

	walletRestoreCmd.Flags().StringVar(&restoreInput, "input", "", "seed material (mnemonic, WIF, or hex)")
	walletRestoreCmd.Flags().BoolVar(&restorePassphrase, "passphrase", false, "use a BIP39 passphrase (for mnemonic only)")
	walletRestoreCmd.Flags().BoolVar(&restoreScan, "scan", true, "scan for existing UTXOs after restore")
}
