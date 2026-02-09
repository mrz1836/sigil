package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/backup"
	"github.com/mrz1836/sigil/internal/output"
	"github.com/mrz1836/sigil/internal/wallet"
)

// newBackupListTestCmd creates a cobra.Command with CommandContext for runBackupList testing.
func newBackupListTestCmd(home string, format output.Format) (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	SetCmdContext(cmd, &CommandContext{
		Cfg: &mockConfigProvider{home: home},
		Fmt: &mockFormatProvider{format: format},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	return cmd, &buf
}

// TestRunBackupList_Empty tests listing when no backups exist.
func TestRunBackupList_Empty(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	// Create backups directory
	backupsDir := filepath.Join(tmpDir, "backups")
	require.NoError(t, os.MkdirAll(backupsDir, 0o750))

	tests := []struct {
		name     string
		format   output.Format
		contains []string
	}{
		{
			name:     "text output",
			format:   output.FormatText,
			contains: []string{"No backups found", "sigil backup create"},
		},
		{
			name:     "json output",
			format:   output.FormatJSON,
			contains: []string{"[]"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, buf := newBackupListTestCmd(tmpDir, tc.format)

			err := runBackupList(cmd, nil)
			require.NoError(t, err)

			result := buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, result, s)
			}
		})
	}
}

// TestRunBackupList_Multiple tests listing multiple backups.
func TestRunBackupList_Multiple(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	// Create backups directory with test backup files
	backupsDir := filepath.Join(tmpDir, "backups")
	require.NoError(t, os.MkdirAll(backupsDir, 0o750))

	// Create fake backup files
	backupFiles := []string{
		"wallet1-2024-01-15-120000.sigil",
		"wallet2-2024-01-16-130000.sigil",
		"main-2024-01-17-140000.sigil",
	}
	for _, f := range backupFiles {
		path := filepath.Join(backupsDir, f)
		require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))
	}

	// Also create a non-backup file that should be ignored
	require.NoError(t, os.WriteFile(filepath.Join(backupsDir, "readme.txt"), []byte("ignore"), 0o600))

	tests := []struct {
		name        string
		format      output.Format
		contains    []string
		notContains []string
	}{
		{
			name:        "text output",
			format:      output.FormatText,
			contains:    []string{"Backups:", "wallet1-2024-01-15-120000.sigil", "wallet2-2024-01-16-130000.sigil", "main-2024-01-17-140000.sigil", "Backup directory:"},
			notContains: []string{"readme.txt"},
		},
		{
			name:        "json output",
			format:      output.FormatJSON,
			contains:    []string{"wallet1-2024-01-15-120000.sigil", "wallet2-2024-01-16-130000.sigil", "main-2024-01-17-140000.sigil"},
			notContains: []string{"readme.txt"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, buf := newBackupListTestCmd(tmpDir, tc.format)

			err := runBackupList(cmd, nil)
			require.NoError(t, err)

			result := buf.String()
			for _, s := range tc.contains {
				assert.Contains(t, result, s)
			}
			for _, s := range tc.notContains {
				assert.NotContains(t, result, s)
			}
		})
	}
}

// TestBackupRestore_WalletExists tests that restore fails when wallet already exists.
func TestBackupRestore_WalletExists(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	password := []byte("testpassword123")

	// Create an existing wallet
	w, err := wallet.NewWallet("existing", []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)

	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, password))

	// Create a backup of the wallet
	backupDir := filepath.Join(tmpDir, "backups")
	svc := backup.NewService(backupDir, storage)

	_, backupPath, err := svc.Create("existing", password)
	require.NoError(t, err)

	// Try to restore - should fail because wallet exists
	err = svc.Restore(backupPath, password, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, wallet.ErrWalletExists)
}

// TestBackupVerify_InvalidFile tests verify with non-existent file.
func TestBackupVerify_InvalidFile(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	backupDir := filepath.Join(tmpDir, "backups")
	svc := backup.NewService(backupDir, storage)

	// Try to verify a non-existent file
	_, err := svc.Verify("/nonexistent/path/backup.sigil")
	require.Error(t, err)
	assert.ErrorIs(t, err, backup.ErrBackupNotFound)
}

// TestBackupVerify_CorruptedFile tests verify with corrupted backup.
func TestBackupVerify_CorruptedFile(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	backupDir := filepath.Join(tmpDir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0o750))

	svc := backup.NewService(backupDir, storage)

	// Create a corrupted backup file (invalid JSON)
	corruptedPath := filepath.Join(backupDir, "corrupted.sigil")
	require.NoError(t, os.WriteFile(corruptedPath, []byte("not valid json"), 0o600))

	_, err := svc.Verify(corruptedPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, backup.ErrInvalidFormat)
}

// TestBackupCreateVerifyRestore_E2E is an end-to-end test for the full backup flow.
func TestBackupCreateVerifyRestore_E2E(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	backupDir := filepath.Join(tmpDir, "backups")
	password := []byte("secure_password_123")

	// Step 1: Create a wallet
	w, err := wallet.NewWallet("test_wallet", []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV})
	require.NoError(t, err)

	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, password))

	// Store original addresses for comparison
	origETHAddr := w.Addresses[wallet.ChainETH][0].Address
	origBSVAddr := w.Addresses[wallet.ChainBSV][0].Address

	// Step 2: Create backup
	svc := backup.NewService(backupDir, storage)
	bak, backupPath, err := svc.Create("test_wallet", password)
	require.NoError(t, err)
	assert.NotEmpty(t, backupPath)
	assert.Equal(t, "test_wallet", bak.Manifest.WalletName)
	assert.Contains(t, bak.Manifest.Chains, "eth")
	assert.Contains(t, bak.Manifest.Chains, "bsv")

	// Step 3: Verify backup (structure only)
	manifest, err := svc.Verify(backupPath)
	require.NoError(t, err)
	assert.Equal(t, "test_wallet", manifest.WalletName)

	// Step 4: Verify with decryption
	manifest, err = svc.VerifyWithDecryption(backupPath, password)
	require.NoError(t, err)
	assert.Equal(t, "test_wallet", manifest.WalletName)

	// Step 5: Verify decryption fails with wrong password
	_, err = svc.VerifyWithDecryption(backupPath, []byte("wrong_password"))
	require.ErrorIs(t, err, backup.ErrDecryptionFailed)

	// Step 6: Restore to new wallet
	err = svc.Restore(backupPath, password, "restored_wallet")
	require.NoError(t, err)

	// Step 7: Verify restored wallet exists and matches
	exists, err := storage.Exists("restored_wallet")
	require.NoError(t, err)
	assert.True(t, exists)

	restoredW, restoredSeed, err := storage.Load("restored_wallet", password)
	require.NoError(t, err)
	defer wallet.ZeroBytes(restoredSeed)

	assert.Equal(t, "restored_wallet", restoredW.Name)
	assert.Equal(t, origETHAddr, restoredW.Addresses[wallet.ChainETH][0].Address)
	assert.Equal(t, origBSVAddr, restoredW.Addresses[wallet.ChainBSV][0].Address)

	// Step 8: List backups should show the backup
	backups, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, backups, 1)
	assert.Contains(t, backups[0], "test_wallet")
	assert.Contains(t, backups[0], ".sigil")
}

// TestBackupRestore_WithNewName tests restoring with a custom name.
func TestBackupRestore_WithNewName(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	backupDir := filepath.Join(tmpDir, "backups")
	password := []byte("testpassword")

	// Create original wallet
	w, err := wallet.NewWallet("original", []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)

	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, password))

	origAddr := w.Addresses[wallet.ChainETH][0].Address

	// Create backup
	svc := backup.NewService(backupDir, storage)
	_, backupPath, err := svc.Create("original", password)
	require.NoError(t, err)

	// Restore with new name
	err = svc.Restore(backupPath, password, "renamed_wallet")
	require.NoError(t, err)

	// Verify the renamed wallet
	exists, err := storage.Exists("renamed_wallet")
	require.NoError(t, err)
	assert.True(t, exists)

	renamedW, _, err := storage.Load("renamed_wallet", password)
	require.NoError(t, err)

	assert.Equal(t, "renamed_wallet", renamedW.Name)
	assert.Equal(t, origAddr, renamedW.Addresses[wallet.ChainETH][0].Address)
}

// TestBackupList_CreatesDirIfMissing tests that List creates the backup directory.
func TestBackupList_CreatesDirIfMissing(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	backupDir := filepath.Join(tmpDir, "backups")

	// Ensure backups dir doesn't exist
	_, err := os.Stat(backupDir)
	require.True(t, os.IsNotExist(err))

	svc := backup.NewService(backupDir, storage)

	// List should create the directory
	backups, err := svc.List()
	require.NoError(t, err)
	assert.Empty(t, backups)

	// Verify directory was created
	info, err := os.Stat(backupDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestBackupVerify_InvalidChecksum tests verify with mismatched checksum.
func TestBackupVerify_InvalidChecksum(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	backupDir := filepath.Join(tmpDir, "backups")
	require.NoError(t, os.MkdirAll(backupDir, 0o750))

	svc := backup.NewService(backupDir, storage)

	// Create a backup file with invalid checksum
	invalidBackup := `{
		"version": 1,
		"manifest": {
			"wallet_name": "test",
			"created_at": "2024-01-15T12:00:00Z",
			"chains": ["eth"],
			"address_count": {"eth": 1},
			"encryption_method": "age"
		},
		"encrypted_data": "c29tZWRhdGE=",
		"checksum": "invalid_checksum_that_does_not_match"
	}`

	backupPath := filepath.Join(backupDir, "invalid.sigil")
	require.NoError(t, os.WriteFile(backupPath, []byte(invalidBackup), 0o600))

	_, err := svc.Verify(backupPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, backup.ErrBackupCorrupted)
}

// TestBackupRestore_WrongPassword tests restore with incorrect password.
// --- Tests for runBackupCreate/Verify/Restore (via mock prompts) ---

func TestRunBackupCreate_HappyPath(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()
	withMockPrompts(t, []byte("testpassword123"), true)

	// Create a wallet to back up
	walletsDir := filepath.Join(tmpDir, "wallets")
	storage := wallet.NewFileStorage(walletsDir)
	w, err := wallet.NewWallet("backupme", []wallet.ChainID{wallet.ChainETH, wallet.ChainBSV})
	require.NoError(t, err)
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)
	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, []byte("testpassword123")))

	origBackupWallet := backupWallet
	defer func() { backupWallet = origBackupWallet }()
	backupWallet = "backupme"

	cmd, buf := newBackupListTestCmd(tmpDir, output.FormatText)
	err = runBackupCreate(cmd, nil)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Backup created successfully")
	assert.Contains(t, result, "backupme")
}

func TestRunBackupCreate_WalletNotFound(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	origBackupWallet := backupWallet
	defer func() { backupWallet = origBackupWallet }()
	backupWallet = "nonexistent"

	cmd, _ := newBackupListTestCmd(tmpDir, output.FormatText)
	err := runBackupCreate(cmd, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, wallet.ErrWalletNotFound)
}

func TestRunBackupVerify_StructureOnly(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()
	// Password prompt returns empty (skip decryption test)
	origPW := promptPasswordFn
	t.Cleanup(func() { promptPasswordFn = origPW })
	promptPasswordFn = func(_ string) ([]byte, error) {
		return []byte{}, nil
	}

	// Create wallet and backup first
	walletsDir := filepath.Join(tmpDir, "wallets")
	storage := wallet.NewFileStorage(walletsDir)
	w, err := wallet.NewWallet("verifyme", []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)
	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, []byte("verifypass123")))

	backupDir := filepath.Join(tmpDir, "backups")
	svc := backup.NewService(backupDir, storage)
	_, backupPath, err := svc.Create("verifyme", []byte("verifypass123"))
	require.NoError(t, err)

	origInput := backupInput
	defer func() { backupInput = origInput }()
	backupInput = backupPath

	cmd, buf := newBackupListTestCmd(tmpDir, output.FormatText)
	err = runBackupVerify(cmd, nil)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "verified successfully")
	assert.Contains(t, result, "verifyme")
}

func TestRunBackupVerify_WithDecryption(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()
	withMockPrompts(t, []byte("verifypass123"), true)

	walletsDir := filepath.Join(tmpDir, "wallets")
	storage := wallet.NewFileStorage(walletsDir)
	w, err := wallet.NewWallet("decryptme", []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)
	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, []byte("verifypass123")))

	backupDir := filepath.Join(tmpDir, "backups")
	svc := backup.NewService(backupDir, storage)
	_, backupPath, err := svc.Create("decryptme", []byte("verifypass123"))
	require.NoError(t, err)

	origInput := backupInput
	defer func() { backupInput = origInput }()
	backupInput = backupPath

	cmd, buf := newBackupListTestCmd(tmpDir, output.FormatText)
	err = runBackupVerify(cmd, nil)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Decryption verified successfully")
}

func TestRunBackupRestore_HappyPath(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()
	withMockPrompts(t, []byte("restorepass123"), true)

	walletsDir := filepath.Join(tmpDir, "wallets")
	storage := wallet.NewFileStorage(walletsDir)
	w, err := wallet.NewWallet("original_wallet", []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)
	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, []byte("restorepass123")))

	backupDir := filepath.Join(tmpDir, "backups")
	svc := backup.NewService(backupDir, storage)
	_, backupPath, err := svc.Create("original_wallet", []byte("restorepass123"))
	require.NoError(t, err)

	origInput := backupInput
	origName := restoreName
	defer func() {
		backupInput = origInput
		restoreName = origName
	}()
	backupInput = backupPath
	restoreName = "restored_from_backup"

	cmd, buf := newBackupListTestCmd(tmpDir, output.FormatText)
	err = runBackupRestore(cmd, nil)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Wallet restored successfully")
	assert.Contains(t, result, "restored_from_backup")

	exists, err := storage.Exists("restored_from_backup")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRunBackupRestore_WalletExists(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()
	withMockPrompts(t, []byte("existpass123"), true)

	walletsDir := filepath.Join(tmpDir, "wallets")
	storage := wallet.NewFileStorage(walletsDir)
	w, err := wallet.NewWallet("existing_wallet", []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)
	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)
	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, []byte("existpass123")))

	backupDir := filepath.Join(tmpDir, "backups")
	svc := backup.NewService(backupDir, storage)
	_, backupPath, err := svc.Create("existing_wallet", []byte("existpass123"))
	require.NoError(t, err)

	origInput := backupInput
	origName := restoreName
	defer func() {
		backupInput = origInput
		restoreName = origName
	}()
	backupInput = backupPath
	restoreName = "" // Use original name, which already exists

	cmd, _ := newBackupListTestCmd(tmpDir, output.FormatText)
	err = runBackupRestore(cmd, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, wallet.ErrWalletExists)
}

func TestBackupRestore_WrongPassword(t *testing.T) {
	tmpDir, testCleanup := setupTestEnv(t)
	defer testCleanup()

	storage := wallet.NewFileStorage(filepath.Join(tmpDir, "wallets"))
	backupDir := filepath.Join(tmpDir, "backups")
	password := []byte("correct_password")

	// Create wallet and backup
	w, err := wallet.NewWallet("test", []wallet.ChainID{wallet.ChainETH})
	require.NoError(t, err)

	mnemonic, err := wallet.GenerateMnemonic(12)
	require.NoError(t, err)
	seed, err := wallet.MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)
	defer wallet.ZeroBytes(seed)

	require.NoError(t, w.DeriveAddresses(seed, 1))
	require.NoError(t, storage.Save(w, seed, password))

	svc := backup.NewService(backupDir, storage)
	_, backupPath, err := svc.Create("test", password)
	require.NoError(t, err)

	// Try to restore with wrong password
	err = svc.Restore(backupPath, []byte("wrong_password"), "new_wallet")
	require.Error(t, err)
	assert.ErrorIs(t, err, backup.ErrDecryptionFailed)
}
