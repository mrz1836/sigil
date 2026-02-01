// Package contracts defines the interface contracts for Sigil MVP.
// These are design artifacts - not compiled code.
// Actual implementations go in internal/backup/

package contracts

import (
	"context"
	"time"
)

// BackupService defines the interface for backup operations.
type BackupService interface {
	// Create generates an encrypted backup of a wallet.
	Create(ctx context.Context, req CreateBackupRequest) (*BackupResult, error)

	// Verify checks backup integrity without restoring.
	Verify(ctx context.Context, path string) (*BackupManifest, error)

	// Restore recreates a wallet from a backup file.
	Restore(ctx context.Context, req RestoreBackupRequest) (*Wallet, error)

	// List returns all backup files for a wallet.
	List(ctx context.Context, walletName string) ([]BackupInfo, error)
}

// CreateBackupRequest contains parameters for backup creation.
type CreateBackupRequest struct {
	// WalletName is the name of the wallet to backup.
	WalletName string

	// Password is the encryption password for the backup.
	// If empty, uses the wallet's existing encryption password.
	Password string

	// OutputPath is optional custom output location.
	// Default: ~/.sigil/backups/<name>-<date>.sigil
	OutputPath string
}

// BackupResult contains the created backup information.
type BackupResult struct {
	// Path is the backup file location.
	Path string

	// Checksum is the SHA256 checksum of the encrypted data.
	Checksum string

	// Manifest is the backup metadata.
	Manifest *BackupManifest
}

// BackupManifest contains backup metadata (not encrypted).
type BackupManifest struct {
	// Version is the backup format version.
	Version int `json:"version"`

	// WalletName is the original wallet name.
	WalletName string `json:"wallet_name"`

	// CreatedAt is the backup creation timestamp.
	CreatedAt time.Time `json:"created_at"`

	// Chains lists the chains included in the backup.
	Chains []string `json:"chains"`

	// AddressCount is the number of addresses per chain.
	AddressCount map[string]int `json:"address_count"`

	// SigilVersion is the Sigil version that created this backup.
	SigilVersion string `json:"sigil_version"`
}

// RestoreBackupRequest contains parameters for backup restoration.
type RestoreBackupRequest struct {
	// Path is the backup file location.
	Path string

	// Password is the decryption password for the backup.
	Password string

	// WalletName is the name for the restored wallet.
	// If empty, uses the original wallet name from manifest.
	WalletName string
}

// BackupInfo contains summary information about a backup file.
type BackupInfo struct {
	// Path is the backup file location.
	Path string

	// Manifest is the backup metadata.
	Manifest *BackupManifest

	// Size is the file size in bytes.
	Size int64

	// ModTime is the file modification time.
	ModTime time.Time
}

// Backup represents the complete backup file structure.
type Backup struct {
	// Manifest is the unencrypted metadata.
	Manifest *BackupManifest `json:"manifest"`

	// EncryptedData is the age-encrypted wallet data.
	EncryptedData []byte `json:"encrypted_data"`

	// Checksum is SHA256 of EncryptedData.
	Checksum string `json:"checksum"`
}

// Backup-related errors.
var (
	ErrBackupNotFound       = Error{Code: "BACKUP_NOT_FOUND", Message: "backup file not found"}
	ErrBackupCorrupted      = Error{Code: "BACKUP_CORRUPTED", Message: "backup file is corrupted - checksum mismatch"}
	ErrBackupVersionUnsupported = Error{Code: "BACKUP_VERSION_UNSUPPORTED", Message: "backup version not supported"}
	ErrBackupDecryptFailed  = Error{Code: "BACKUP_DECRYPT_FAILED", Message: "backup decryption failed - wrong password"}
)
