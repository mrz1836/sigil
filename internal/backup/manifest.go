// Package backup provides wallet backup and restore functionality.
package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrBackupNotFound indicates the backup file was not found.
	ErrBackupNotFound = errors.New("backup file not found")

	// ErrBackupCorrupted indicates the backup checksum failed.
	ErrBackupCorrupted = errors.New("backup corrupted - checksum mismatch")

	// ErrDecryptionFailed indicates backup decryption failed.
	ErrDecryptionFailed = errors.New("backup decryption failed")

	// ErrInvalidFormat indicates the backup format is invalid.
	ErrInvalidFormat = errors.New("invalid backup format")
)

// BackupVersion is the current backup format version.
const BackupVersion = 1

// Backup represents a complete wallet backup.
type Backup struct {
	// Version is the backup format version.
	Version int `json:"version"`

	// Manifest contains backup metadata.
	Manifest Manifest `json:"manifest"`

	// EncryptedData is the age-encrypted wallet data.
	EncryptedData []byte `json:"encrypted_data"`

	// Checksum is the SHA256 hash of EncryptedData.
	Checksum string `json:"checksum"`
}

// Manifest contains metadata about the backup.
type Manifest struct {
	// WalletName is the name of the backed up wallet.
	WalletName string `json:"wallet_name"`

	// CreatedAt is when the backup was created.
	CreatedAt time.Time `json:"created_at"`

	// Chains lists the chains included in the backup.
	Chains []string `json:"chains"`

	// AddressCount is the number of addresses per chain.
	AddressCount map[string]int `json:"address_count"`

	// EncryptionMethod describes the encryption used.
	EncryptionMethod string `json:"encryption_method"`

	// HostInfo contains optional host information.
	HostInfo string `json:"host_info,omitempty"`
}

// WalletData represents the decrypted wallet data within a backup.
type WalletData struct {
	// Seed is the encrypted wallet seed.
	Seed []byte `json:"seed"`

	// WalletJSON is the JSON representation of the wallet.
	WalletJSON json.RawMessage `json:"wallet"`
}

// NewManifest creates a new backup manifest.
func NewManifest(walletName string, chains []string, addressCount map[string]int) Manifest {
	return Manifest{
		WalletName:       walletName,
		CreatedAt:        time.Now().UTC(),
		Chains:           chains,
		AddressCount:     addressCount,
		EncryptionMethod: "age",
	}
}

// CalculateChecksum computes the SHA256 checksum of data.
func CalculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// VerifyChecksum verifies that data matches the expected checksum.
func VerifyChecksum(data []byte, expected string) error {
	actual := CalculateChecksum(data)
	if actual != expected {
		return fmt.Errorf("%w: expected %s, got %s", ErrBackupCorrupted, expected, actual)
	}
	return nil
}

// NewBackup creates a new backup with the given manifest and encrypted data.
func NewBackup(manifest Manifest, encryptedData []byte) *Backup {
	return &Backup{
		Version:       BackupVersion,
		Manifest:      manifest,
		EncryptedData: encryptedData,
		Checksum:      CalculateChecksum(encryptedData),
	}
}

// Validate checks the backup for consistency.
func (b *Backup) Validate() error {
	if b.Version != BackupVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrInvalidFormat, b.Version)
	}

	if b.Manifest.WalletName == "" {
		return fmt.Errorf("%w: missing wallet name", ErrInvalidFormat)
	}

	if len(b.EncryptedData) == 0 {
		return fmt.Errorf("%w: no encrypted data", ErrInvalidFormat)
	}

	return VerifyChecksum(b.EncryptedData, b.Checksum)
}
