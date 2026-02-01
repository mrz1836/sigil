package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mrz1836/sigil/internal/sigilcrypto"
	"github.com/mrz1836/sigil/internal/wallet"
)

const (
	// BackupExtension is the file extension for backups.
	BackupExtension = ".sigil"

	// BackupDirPermissions is the permission mode for the backup directory.
	BackupDirPermissions = 0o750

	// BackupFilePermissions is the permission mode for backup files.
	BackupFilePermissions = 0o600
)

// Service provides backup operations.
type Service struct {
	backupDir string
	storage   wallet.Storage
}

// NewService creates a new backup service.
func NewService(backupDir string, walletStorage wallet.Storage) *Service {
	return &Service{
		backupDir: backupDir,
		storage:   walletStorage,
	}
}

// Create creates a backup of a wallet.
// The password should be zeroed by the caller after this call returns.
func (s *Service) Create(walletName string, password []byte) (*Backup, string, error) {
	// Load the wallet
	wlt, seed, err := s.storage.Load(walletName, password)
	if err != nil {
		return nil, "", fmt.Errorf("loading wallet: %w", err)
	}
	defer wallet.ZeroBytes(seed)

	// Serialize wallet data
	walletJSON, err := json.Marshal(wlt)
	if err != nil {
		return nil, "", fmt.Errorf("serializing wallet: %w", err)
	}

	walletData := WalletData{
		Seed:       seed,
		WalletJSON: walletJSON,
	}

	dataJSON, err := json.Marshal(walletData)
	if err != nil {
		return nil, "", fmt.Errorf("serializing backup data: %w", err)
	}

	// Encrypt the data
	encryptedData, err := sigilcrypto.Encrypt(dataJSON, string(password))
	if err != nil {
		return nil, "", fmt.Errorf("encrypting backup: %w", err)
	}

	// Build address count
	addressCount := make(map[string]int)
	for chain, addrs := range wlt.Addresses {
		addressCount[string(chain)] = len(addrs)
	}

	// Build chains list
	chains := make([]string, 0, len(wlt.EnabledChains))
	for _, chain := range wlt.EnabledChains {
		chains = append(chains, string(chain))
	}

	// Create manifest
	manifest := NewManifest(walletName, chains, addressCount)

	// Create backup
	backup := NewBackup(manifest, encryptedData)

	// Write to file
	backupPath, err := s.writeBackup(backup)
	if err != nil {
		return nil, "", fmt.Errorf("writing backup: %w", err)
	}

	return backup, backupPath, nil
}

// Verify verifies a backup file's integrity without decrypting.
func (s *Service) Verify(backupPath string) (*Manifest, error) {
	backup, err := s.readBackup(backupPath)
	if err != nil {
		return nil, err
	}

	if err := backup.Validate(); err != nil {
		return nil, err
	}

	return &backup.Manifest, nil
}

// VerifyWithDecryption verifies a backup and tests decryption.
// The password should be zeroed by the caller after this call returns.
func (s *Service) VerifyWithDecryption(backupPath string, password []byte) (*Manifest, error) {
	backup, err := s.readBackup(backupPath)
	if err != nil {
		return nil, err
	}

	// Validate structure
	if validationErr := backup.Validate(); validationErr != nil {
		return nil, validationErr
	}

	// Test decryption
	_, err = sigilcrypto.Decrypt(backup.EncryptedData, string(password))
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return &backup.Manifest, nil
}

// Restore restores a wallet from a backup.
// The password should be zeroed by the caller after this call returns.
func (s *Service) Restore(backupPath string, password []byte, newWalletName string) error {
	// Read backup
	backup, err := s.readBackup(backupPath)
	if err != nil {
		return err
	}

	// Validate backup
	if validationErr := backup.Validate(); validationErr != nil {
		return validationErr
	}

	// Decrypt data
	decrypted, err := sigilcrypto.Decrypt(backup.EncryptedData, string(password))
	if err != nil {
		return ErrDecryptionFailed
	}
	defer wallet.ZeroBytes(decrypted)

	// Parse wallet data
	var walletData WalletData
	if err := json.Unmarshal(decrypted, &walletData); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidFormat, err)
	}
	defer wallet.ZeroBytes(walletData.Seed)

	// Parse wallet
	var wlt wallet.Wallet
	if err := json.Unmarshal(walletData.WalletJSON, &wlt); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidFormat, err)
	}

	// Use new name if provided
	if newWalletName != "" {
		wlt.Name = newWalletName
	}

	// Save the restored wallet
	if err := s.storage.Save(&wlt, walletData.Seed, password); err != nil {
		return fmt.Errorf("saving restored wallet: %w", err)
	}

	return nil
}

// List returns all backup files in the backup directory.
func (s *Service) List() ([]string, error) {
	if err := os.MkdirAll(s.backupDir, BackupDirPermissions); err != nil {
		return nil, fmt.Errorf("creating backup directory: %w", err)
	}

	entries, err := os.ReadDir(s.backupDir)
	if err != nil {
		return nil, fmt.Errorf("reading backup directory: %w", err)
	}

	var backups []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == BackupExtension {
			backups = append(backups, entry.Name())
		}
	}

	return backups, nil
}

// writeBackup writes a backup to the backup directory.
//
//nolint:funcorder // Keeping helper methods together
func (s *Service) writeBackup(backup *Backup) (string, error) {
	// Ensure directory exists
	if err := os.MkdirAll(s.backupDir, BackupDirPermissions); err != nil {
		return "", fmt.Errorf("creating backup directory: %w", err)
	}

	// Generate filename
	timestamp := time.Now().Format("2006-01-02-150405")
	filename := fmt.Sprintf("%s-%s%s", backup.Manifest.WalletName, timestamp, BackupExtension)
	backupPath := filepath.Join(s.backupDir, filename)

	// Serialize backup
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serializing backup: %w", err)
	}

	// Write file
	if err := os.WriteFile(backupPath, data, BackupFilePermissions); err != nil {
		return "", fmt.Errorf("writing backup file: %w", err)
	}

	return backupPath, nil
}

// readBackup reads a backup from a file.
//
//nolint:funcorder // Keeping helper methods together
func (s *Service) readBackup(path string) (*Backup, error) {
	// #nosec G304 -- path is from user input
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrBackupNotFound
		}
		return nil, fmt.Errorf("reading backup file: %w", err)
	}

	var backup Backup
	if err := json.Unmarshal(data, &backup); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidFormat, err)
	}

	return &backup, nil
}

// BackupPath returns the path to a backup file.
func (s *Service) BackupPath(filename string) string {
	return filepath.Join(s.backupDir, filename)
}
