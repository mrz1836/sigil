package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mrz1836/sigil/internal/backup"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

//nolint:gochecknoglobals // Cobra CLI pattern requires package-level flag variables
var (
	// backupInput is the path to a backup file for restore/verify.
	backupInput string
	// backupWallet is the wallet name for backup operations.
	backupWallet string
	// restoreName is the name for the restored wallet.
	restoreName string
)

// backupCmd is the parent command for backup operations.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Manage wallet backups",
	Long:  `Create, verify, and restore encrypted wallet backups.`,
}

// backupCreateCmd creates a backup.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var backupCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a wallet backup",
	Long: `Create an encrypted backup of a wallet.

The backup file will be created in ~/.sigil/backups/ with a timestamped name.
The backup includes the wallet seed and all metadata, encrypted with your password.

Example:
  sigil backup create --wallet main`,
	RunE: runBackupCreate,
}

// backupVerifyCmd verifies a backup.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var backupVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify a backup file",
	Long: `Verify the integrity of a backup file.

This checks the backup structure and SHA256 checksum. Optionally tests
decryption by providing your wallet password.

Example:
  sigil backup verify --input ~/.sigil/backups/main-2024-01-15.sigil`,
	RunE: runBackupVerify,
}

// backupRestoreCmd restores a wallet from backup.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var backupRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore a wallet from backup",
	Long: `Restore a wallet from an encrypted backup file.

You will need the password used when creating the backup.
Optionally specify a new name for the restored wallet.

Example:
  sigil backup restore --input ~/.sigil/backups/main-2024-01-15.sigil
  sigil backup restore --input backup.sigil --name restored_wallet`,
	RunE: runBackupRestore,
}

// backupListCmd lists available backups.
//
//nolint:gochecknoglobals // Cobra CLI pattern requires package-level command variables
var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available backups",
	Long: `List all backup files in the backups directory.

Example:
  sigil backup list`,
	Aliases: []string{"ls"},
	RunE:    runBackupList,
}

//nolint:gochecknoinits // Cobra CLI pattern requires init for command registration
func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.AddCommand(backupCreateCmd)
	backupCmd.AddCommand(backupVerifyCmd)
	backupCmd.AddCommand(backupRestoreCmd)
	backupCmd.AddCommand(backupListCmd)

	backupCreateCmd.Flags().StringVar(&backupWallet, "wallet", "", "wallet name (required)")
	_ = backupCreateCmd.MarkFlagRequired("wallet")

	backupVerifyCmd.Flags().StringVar(&backupInput, "input", "", "path to backup file (required)")
	_ = backupVerifyCmd.MarkFlagRequired("input")

	backupRestoreCmd.Flags().StringVar(&backupInput, "input", "", "path to backup file (required)")
	backupRestoreCmd.Flags().StringVar(&restoreName, "name", "", "new name for restored wallet (optional)")
	_ = backupRestoreCmd.MarkFlagRequired("input")
}

func runBackupCreate(cmd *cobra.Command, _ []string) error {
	// Get backup service
	walletStorage := wallet.NewFileStorage(filepath.Join(cfg.Home, "wallets"))
	backupDir := filepath.Join(cfg.Home, "backups")
	svc := backup.NewService(backupDir, walletStorage)

	// Check wallet exists
	exists, err := walletStorage.Exists(backupWallet)
	if err != nil {
		return err
	}
	if !exists {
		return sigilerr.WithSuggestion(
			wallet.ErrWalletNotFound,
			fmt.Sprintf("wallet '%s' not found. List wallets with: sigil wallet list", backupWallet),
		)
	}

	// Prompt for password
	password, err := promptPassword("Enter wallet password: ")
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(password)

	// Create backup
	bak, backupPath, err := svc.Create(backupWallet, password)
	if err != nil {
		return fmt.Errorf("creating backup: %w", err)
	}

	// Display result
	w := cmd.OutOrStdout()
	outln(w, "Backup created successfully!")
	outln(w)
	out(w, "  File:     %s\n", backupPath)
	out(w, "  Wallet:   %s\n", bak.Manifest.WalletName)
	out(w, "  Chains:   %v\n", bak.Manifest.Chains)
	out(w, "  Checksum: %s\n", bak.Checksum[:16]+"...")
	outln(w)
	outln(w, "Store this backup file securely. You will need your wallet password to restore it.")

	return nil
}

func runBackupVerify(cmd *cobra.Command, _ []string) error {
	// Get backup service
	walletStorage := wallet.NewFileStorage(filepath.Join(cfg.Home, "wallets"))
	backupDir := filepath.Join(cfg.Home, "backups")
	svc := backup.NewService(backupDir, walletStorage)

	// First verify without decryption
	manifest, err := svc.Verify(backupInput)
	if err != nil {
		return fmt.Errorf("verifying backup: %w", err)
	}

	// Ask if user wants to test decryption
	w := cmd.OutOrStdout()
	outln(w, "Backup structure verified successfully!")
	outln(w)
	out(w, "  Wallet:  %s\n", manifest.WalletName)
	out(w, "  Created: %s\n", manifest.CreatedAt.Format("2006-01-02 15:04:05"))
	out(w, "  Chains:  %v\n", manifest.Chains)
	outln(w)

	outln(w, "To test decryption, enter your password (or press Enter to skip):")
	password, err := promptPassword("Password: ")
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(password)

	if len(password) > 0 {
		_, err = svc.VerifyWithDecryption(backupInput, password)
		if err != nil {
			return sigilerr.WithSuggestion(
				sigilerr.ErrAuthentication,
				"decryption test failed - wrong password or corrupted backup",
			)
		}
		outln(w)
		outln(w, "Decryption verified successfully!")
	}

	return nil
}

func runBackupRestore(cmd *cobra.Command, _ []string) error {
	// Get backup service
	walletStorage := wallet.NewFileStorage(filepath.Join(cfg.Home, "wallets"))
	backupDir := filepath.Join(cfg.Home, "backups")
	svc := backup.NewService(backupDir, walletStorage)

	// First verify the backup
	manifest, err := svc.Verify(backupInput)
	if err != nil {
		return fmt.Errorf("verifying backup: %w", err)
	}

	// Determine wallet name
	walletName := manifest.WalletName
	if restoreName != "" {
		walletName = restoreName
	}

	// Check if wallet already exists
	exists, err := walletStorage.Exists(walletName)
	if err != nil {
		return err
	}
	if exists {
		return sigilerr.WithSuggestion(
			wallet.ErrWalletExists,
			fmt.Sprintf("wallet '%s' already exists. Use --name to specify a different name.", walletName),
		)
	}

	// Prompt for password
	password, err := promptPassword("Enter backup password: ")
	if err != nil {
		return err
	}
	defer wallet.ZeroBytes(password)

	// Restore the wallet
	if err := svc.Restore(backupInput, password, restoreName); err != nil {
		return fmt.Errorf("restoring backup: %w", err)
	}

	// Display result
	w := cmd.OutOrStdout()
	outln(w, "Wallet restored successfully!")
	outln(w)
	out(w, "  Name: %s\n", walletName)
	out(w, "  Path: %s\n", filepath.Join(cfg.Home, "wallets", walletName+".wallet"))
	outln(w)
	outln(w, "Verify your addresses with: sigil wallet show "+walletName)

	return nil
}

//nolint:gocognit // Display logic for backup list is complex
func runBackupList(cmd *cobra.Command, _ []string) error {
	// Get backup service
	walletStorage := wallet.NewFileStorage(filepath.Join(cfg.Home, "wallets"))
	backupDir := filepath.Join(cfg.Home, "backups")
	svc := backup.NewService(backupDir, walletStorage)

	// List backups
	backups, err := svc.List()
	if err != nil {
		return fmt.Errorf("listing backups: %w", err)
	}

	w := cmd.OutOrStdout()
	format := formatter.Format()

	if len(backups) == 0 {
		if format == output.FormatJSON {
			outln(w, "[]")
		} else {
			outln(w, "No backups found.")
			outln(w, "Create one with: sigil backup create --wallet <name>")
		}
		return nil
	}

	if format == output.FormatJSON {
		out(w, "[")
		for i, b := range backups {
			if i > 0 {
				out(w, ",")
			}
			out(w, `"%s"`, b)
		}
		outln(w, "]")
	} else {
		outln(w, "Backups:")
		for _, b := range backups {
			out(w, "  %s\n", b)
		}
		outln(w)
		out(w, "Backup directory: %s\n", backupDir)
	}

	return nil
}
