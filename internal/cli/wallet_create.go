package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/chain"
	"github.com/mrz1836/sigil/internal/chain/bsv"
	"github.com/mrz1836/sigil/internal/utxostore"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

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
		passphrase, err = promptPassphraseFn()
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
	w, err := wallet.NewWallet(name, []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV})
	if err != nil {
		return nil, err
	}

	err = w.DeriveAddresses(seed, 1)
	if err != nil {
		return nil, err
	}

	password, err := promptNewPasswordFn()
	if err != nil {
		return nil, err
	}
	defer wallet.ZeroBytes(password)

	err = storage.Save(w, seed, password)
	if err != nil {
		return nil, err
	}

	return w, nil
}

// bsvClientAdapter adapts bsv.Client to utxostore.ChainClient interface.
type bsvClientAdapter struct {
	client *bsv.Client
}

// ListUTXOs implements utxostore.ChainClient by converting bsv.UTXO to chain.UTXO.
func (a *bsvClientAdapter) ListUTXOs(ctx context.Context, address string) ([]chain.UTXO, error) {
	bsvUTXOs, err := a.client.ListUTXOs(ctx, address)
	if err != nil {
		return nil, err
	}

	// Convert bsv.UTXO to chain.UTXO
	chainUTXOs := make([]chain.UTXO, len(bsvUTXOs))
	for i, u := range bsvUTXOs {
		chainUTXOs[i] = chain.UTXO{
			TxID:          u.TxID,
			Vout:          u.Vout,
			Amount:        u.Amount,
			ScriptPubKey:  u.ScriptPubKey,
			Address:       u.Address,
			Confirmations: u.Confirmations,
		}
	}
	return chainUTXOs, nil
}

// scanWalletUTXOs scans a wallet for UTXOs and reports results.
func scanWalletUTXOs(w *wallet.Wallet, cmd *cobra.Command) error {
	ctx := GetCmdContext(cmd)
	walletPath := filepath.Join(ctx.Cfg.GetHome(), "wallets", w.Name)
	store := utxostore.New(walletPath)

	// Load existing UTXO data if any
	if err := store.Load(); err != nil {
		return fmt.Errorf("loading UTXO store: %w", err)
	}

	scanCtx, cancel := contextWithTimeout(cmd, 60*time.Second)
	defer cancel()

	outln(cmd.OutOrStdout())
	outln(cmd.OutOrStdout(), "Scanning for UTXOs...")

	// Scan BSV addresses (currently the only UTXO chain supported)
	client := bsv.NewClient(scanCtx, &bsv.ClientOptions{
		APIKey: ctx.Cfg.GetBSVAPIKey(),
	})

	// Wrap client in adapter
	adapter := &bsvClientAdapter{client: client}

	result, err := store.ScanWallet(scanCtx, w, wallet.ChainBSV, adapter)
	if err != nil {
		return fmt.Errorf("scanning wallet: %w", err)
	}

	// Display scan results
	displayScanResults(result, cmd)

	return nil
}

// displayScanResults shows the results of a UTXO scan.
func displayScanResults(result *utxostore.ScanResult, cmd *cobra.Command) {
	w := cmd.OutOrStdout()

	if len(result.Errors) > 0 {
		outln(w, "\nScan completed with errors:")
		for _, err := range result.Errors {
			out(w, "  - %v\n", err)
		}
	}

	outln(w)
	out(w, "Scan Results:\n")
	out(w, "  Addresses scanned: %d\n", result.AddressesScanned)
	out(w, "  UTXOs found: %d\n", result.UTXOsFound)
	out(w, "  Total balance: %d satoshis (%.8f BSV)\n",
		result.TotalBalance, float64(result.TotalBalance)/100000000)
}

func runWalletCreate(cmd *cobra.Command, args []string) error {
	ctx := GetCmdContext(cmd)
	name := args[0]
	storage := wallet.NewFileStorage(filepath.Join(ctx.Cfg.GetHome(), "wallets"))

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
	if createShamir {
		if err := handleShamirCreation(mnemonic, cmd); err != nil {
			return err
		}
	} else {
		displayMnemonic(mnemonic, cmd)
	}

	displayWalletAddresses(w, cmd)

	// Scan for UTXOs if requested
	if createScan {
		if err := scanWalletUTXOs(w, cmd); err != nil {
			// Don't fail wallet creation if scan fails - just warn
			out(cmd.OutOrStderr(), "\nWarning: UTXO scan failed: %v\n", err)
		}
	}

	outln(cmd.OutOrStdout())
	out(cmd.OutOrStdout(), "Wallet '%s' created successfully.\n", name)
	outln(cmd.OutOrStdout(), "Wallet file: "+filepath.Join(ctx.Cfg.GetHome(), "wallets", name+".wallet"))

	return nil
}

// displayMnemonic shows the mnemonic phrase with formatting.
func displayMnemonic(mnemonic string, cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	outln(w)
	outln(w, "===================================================================")
	outln(w, "                    RECOVERY PHRASE")
	outln(w, "===================================================================")
	outln(w)
	outln(w, "Write down these words in order and store them securely.")
	outln(w, "This is the ONLY way to recover your wallet.")
	outln(w)

	words := strings.Fields(mnemonic)
	for i, word := range words {
		out(w, "%2d. %s\n", i+1, word)
	}

	outln(w)
	outln(w, "===================================================================")
	outln(w)
}

// displayShamirShares shows the generated Shamir shares.
func displayShamirShares(shares []string, threshold int, cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	outln(w)
	outln(w, "===================================================================")
	outln(w, "                    SHAMIR SECRET SHARES")
	outln(w, "===================================================================")
	outln(w)
	out(w, "Your wallet seed has been split into %d shares.\n", len(shares))
	out(w, "You need any %d of them to recover your wallet.\n", threshold)
	outln(w)
	outln(w, "Store each share in a separate, secure location.")
	outln(w)

	for i, share := range shares {
		out(w, "Share %d:\n%s\n\n", i+1, share)
	}

	outln(w, "===================================================================")
	outln(w)
}

// displayWalletAddresses shows the derived addresses.
func displayWalletAddresses(wlt *wallet.Wallet, cmd *cobra.Command) {
	w := cmd.OutOrStdout()
	outln(w, "Derived Addresses:")
	for chainID, addresses := range wlt.Addresses {
		if len(addresses) > 0 {
			out(w, "  %s: %s\n", strings.ToUpper(string(chainID)), addresses[0].Address)
		}
	}
}
