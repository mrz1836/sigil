package cli

import (
	"bufio"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/shamir"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// ErrMinSharesRequired is returned when < 2 shares are provided.
var ErrMinSharesRequired = errors.New("at least 2 shares are required")

// runWalletRestore handles the wallet restore command.
func runWalletRestore(cmd *cobra.Command, args []string) error {
	ctx := GetCmdContext(cmd)
	name := args[0]
	storage := wallet.NewFileStorage(filepath.Join(ctx.Cfg.GetHome(), "wallets"))

	// Validate and check for existing wallet
	if err := validateRestoreTarget(name, storage); err != nil {
		return err
	}

	// Get and process seed material
	seed, err := getSeedForRestore(cmd)
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(seed)

	// Create wallet with derived addresses
	w, err := createWalletWithAddresses(name, seed)
	if err != nil {
		return err
	}

	// Get user confirmation and save
	if err := confirmAndSaveWallet(w, seed, storage, cmd); err != nil {
		return err
	}

	// Scan for UTXOs if requested (default: true for restore)
	if restoreScan {
		if err := scanWalletUTXOs(w, cmd); err != nil {
			// Don't fail wallet restore if scan fails - just warn
			out(cmd.OutOrStderr(), "\nWarning: UTXO scan failed: %v\n", err)
		}
	}

	return nil
}

// validateRestoreTarget validates wallet name and checks it doesn't exist.
func validateRestoreTarget(name string, storage *wallet.FileStorage) error {
	if err := wallet.ValidateWalletName(name); err != nil {
		return sigilerr.WithSuggestion(err, "wallet name must be 1-64 alphanumeric characters or underscores")
	}

	exists, err := storage.Exists(name)
	if err != nil {
		return err
	}
	if exists {
		return sigilerr.WithSuggestion(
			wallet.ErrWalletExists,
			fmt.Sprintf("wallet '%s' already exists. Choose a different name.", name),
		)
	}
	return nil
}

// getSeedForRestore gets seed material from flag or interactive prompt.
func getSeedForRestore(cmd *cobra.Command) ([]byte, error) {
	if restoreShamir {
		return processShamirRestore(cmd)
	}

	input := restoreInput
	if input == "" {
		var err error
		input, err = promptSeedFn()
		if err != nil {
			return nil, err
		}
	}
	return processSeedInput(input, restorePassphrase, cmd)
}

// processShamirRestore handles the interactive collection and combination of Shamir shares.
func processShamirRestore(cmd *cobra.Command) ([]byte, error) {
	outln(cmd.OutOrStdout(), "Enter your Shamir shares one by one.")
	outln(cmd.OutOrStdout(), "Press Enter on an empty line when finished.")
	outln(cmd.OutOrStdout())

	var shares []string
	scanner := bufio.NewScanner(cmd.InOrStdin())

	for i := 1; ; i++ {
		out(cmd.OutOrStdout(), "Share %d: ", i)
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			break
		}
		shares = append(shares, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	if len(shares) < 2 {
		return nil, ErrMinSharesRequired
	}

	mnemonicBytes, err := shamir.Combine(shares)
	if err != nil {
		return nil, fmt.Errorf("failed to combine shares: %w", err)
	}

	// Treat the combined secret as the mnemonic
	return processMnemonicInput(string(mnemonicBytes), restorePassphrase, cmd)
}

// createWalletWithAddresses creates a new wallet and derives addresses.
func createWalletWithAddresses(name string, seed []byte) (*wallet.Wallet, error) {
	w, err := wallet.NewWallet(name, []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV})
	if err != nil {
		return nil, err
	}

	if err := w.DeriveAddresses(seed, 1); err != nil {
		return nil, err
	}

	return w, nil
}

// confirmAndSaveWallet displays addresses, confirms with user, and saves wallet.
func confirmAndSaveWallet(w *wallet.Wallet, seed []byte, storage *wallet.FileStorage, cmd *cobra.Command) error {
	ctx := GetCmdContext(cmd)
	displayAddressVerification(w, cmd)

	if !promptConfirmFn() {
		outln(cmd.OutOrStdout(), "Wallet restoration canceled.")
		return nil
	}

	password, promptErr := promptNewPasswordFn()
	if promptErr != nil {
		return promptErr
	}
	defer wallet.ZeroBytes(password)

	if err := storage.Save(w, seed, password); err != nil {
		return err
	}

	outln(cmd.OutOrStdout())
	out(cmd.OutOrStdout(), "Wallet '%s' restored successfully.\n", w.Name)
	outln(cmd.OutOrStdout(), "Wallet file: "+filepath.Join(ctx.Cfg.GetHome(), "wallets", w.Name+".wallet"))

	return nil
}

// processSeedInput processes seed input based on detected format.
func processSeedInput(input string, usePassphrase bool, cmd *cobra.Command) ([]byte, error) {
	format := wallet.DetectInputFormat(input)

	switch format {
	case wallet.FormatUnknown:
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"unrecognized input format. Expected mnemonic (12/24 words), WIF (51-52 chars starting with 5/K/L), or hex (64 chars)",
		)
	case wallet.FormatMnemonic:
		return processMnemonicInput(input, usePassphrase, cmd)
	case wallet.FormatWIF:
		return wallet.ParseWIF(input)
	case wallet.FormatHex:
		return wallet.ParseHexKey(input)
	default:
		return nil, sigilerr.WithSuggestion(
			sigilerr.ErrInvalidInput,
			"unrecognized input format. Expected mnemonic (12/24 words), WIF (51-52 chars starting with 5/K/L), or hex (64 chars)",
		)
	}
}

// processMnemonicInput validates and converts a mnemonic to seed.
func processMnemonicInput(mnemonic string, usePassphrase bool, cmd *cobra.Command) ([]byte, error) {
	// Check for and display typos
	displayDetectedTypos(mnemonic, cmd)

	// Validate mnemonic
	if err := wallet.ValidateMnemonic(mnemonic); err != nil {
		return nil, sigilerr.WithSuggestion(
			err,
			"the mnemonic phrase is not valid. Check for typos or missing words.",
		)
	}

	// Get passphrase if requested
	passphrase, err := getPassphraseIfNeeded(usePassphrase)
	if err != nil {
		return nil, err
	}

	// Convert to seed
	return wallet.MnemonicToSeed(mnemonic, passphrase)
}

// displayDetectedTypos shows any typos found in the mnemonic.
func displayDetectedTypos(mnemonic string, cmd *cobra.Command) {
	typos := wallet.DetectTypos(mnemonic)
	if len(typos) == 0 {
		return
	}

	w := cmd.OutOrStdout()
	outln(w, "\nPossible typos detected:")
	for _, typo := range typos {
		if typo.Suggestion != "" {
			out(w, "  Word %d: '%s' - did you mean '%s'?\n", typo.Index+1, typo.Word, typo.Suggestion)
		} else {
			out(w, "  Word %d: '%s' is not a valid BIP39 word\n", typo.Index+1, typo.Word)
		}
	}
	outln(w)
}

// getPassphraseIfNeeded prompts for passphrase if requested.
func getPassphraseIfNeeded(usePassphrase bool) (string, error) {
	if !usePassphrase {
		return "", nil
	}
	return promptPassphraseFn()
}

// displayAddressVerification shows derived addresses for user verification.
func displayAddressVerification(wlt *wallet.Wallet, cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	outln(w)
	outln(w, "===================================================================")
	outln(w, "                 VERIFY YOUR ADDRESSES")
	outln(w, "===================================================================")
	outln(w)
	outln(w, "Please verify these addresses match what you expect:")
	outln(w)

	for chainID, addresses := range wlt.Addresses {
		if len(addresses) > 0 {
			out(w, "  %s: %s\n", strings.ToUpper(string(chainID)), addresses[0].Address)
		}
	}

	outln(w)
	outln(w, "===================================================================")
}
