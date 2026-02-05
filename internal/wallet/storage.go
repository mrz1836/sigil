package wallet

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrz1836/sigil/internal/sigilcrypto"
)

const (
	// walletFileExtension is the extension for wallet files.
	walletFileExtension = ".wallet"

	// walletFilePermissions is the permission mode for wallet files.
	walletFilePermissions = 0o600

	// walletDirPermissions is the permission mode for the wallets directory.
	walletDirPermissions = 0o750
)

// ErrDecryptionFailed indicates decryption failed (wrong password or corrupted file).
var ErrDecryptionFailed = errors.New("decryption failed - wrong password or corrupted file")

// Storage defines the interface for wallet persistence.
type Storage interface {
	// Save encrypts and writes a wallet to storage.
	// The password should be zeroed by the caller after this call returns.
	Save(wallet *Wallet, seed, password []byte) error

	// Load reads and decrypts a wallet from storage.
	// The password should be zeroed by the caller after this call returns.
	Load(name string, password []byte) (*Wallet, []byte, error)

	// Exists checks if a wallet exists.
	Exists(name string) (bool, error)

	// List returns all wallet names.
	List() ([]string, error)

	// Delete removes a wallet file.
	Delete(name string) error
}

// walletFile represents the encrypted wallet file structure.
type walletFile struct {
	// Wallet contains the wallet metadata (not the seed).
	Wallet *Wallet `json:"wallet"`

	// EncryptedSeed is the age-encrypted seed bytes.
	EncryptedSeed []byte `json:"encrypted_seed"`
}

// FileStorage implements Storage using the filesystem.
type FileStorage struct {
	basePath string
}

// NewFileStorage creates a new file-based storage.
func NewFileStorage(basePath string) *FileStorage {
	return &FileStorage{basePath: basePath}
}

// Save encrypts and writes a wallet to storage.
// The password should be zeroed by the caller after this call returns.
func (s *FileStorage) Save(wallet *Wallet, seed, password []byte) error {
	// Validate wallet name
	if err := ValidateWalletName(wallet.Name); err != nil {
		return err
	}

	// Check if wallet already exists
	exists, err := s.Exists(wallet.Name)
	if err != nil {
		return fmt.Errorf("checking wallet existence: %w", err)
	}
	if exists {
		return ErrWalletExists
	}

	// Ensure directory exists
	err = os.MkdirAll(s.basePath, walletDirPermissions)
	if err != nil {
		return fmt.Errorf("creating wallet directory: %w", err)
	}

	// Encrypt the seed
	encryptedSeed, err := sigilcrypto.Encrypt(seed, string(password))
	if err != nil {
		return fmt.Errorf("encrypting seed: %w", err)
	}

	// Create wallet file structure
	wf := walletFile{
		Wallet:        wallet,
		EncryptedSeed: encryptedSeed,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(wf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling wallet: %w", err)
	}

	// Write to file with secure permissions
	walletPath := s.walletPath(wallet.Name)
	if err := os.WriteFile(walletPath, data, walletFilePermissions); err != nil {
		return fmt.Errorf("writing wallet file: %w", err)
	}

	return nil
}

// Load reads and decrypts a wallet from storage.
// The password should be zeroed by the caller after this call returns.
func (s *FileStorage) Load(name string, password []byte) (*Wallet, []byte, error) {
	// Validate wallet name
	if err := ValidateWalletName(name); err != nil {
		return nil, nil, err
	}

	// Check if wallet exists
	walletPath := s.walletPath(name)
	if _, err := os.Stat(walletPath); os.IsNotExist(err) {
		return nil, nil, ErrWalletNotFound
	}

	// Read wallet file
	// SECURITY: Path is safe because:
	// 1. ValidateWalletName restricts name to [a-zA-Z0-9_-]{1,64}
	// 2. walletPath uses filepath.Join with fixed extension
	// 3. walletPath includes defensive traversal check
	//nolint:gosec // G304: Path validated by ValidateWalletName + walletPath
	data, err := os.ReadFile(walletPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading wallet file: %w", err)
	}

	// Unmarshal JSON
	var wf walletFile
	err = json.Unmarshal(data, &wf)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing wallet file: %w", err)
	}

	// Decrypt the seed
	seed, err := sigilcrypto.Decrypt(wf.EncryptedSeed, string(password))
	if err != nil {
		return nil, nil, ErrDecryptionFailed
	}

	return wf.Wallet, seed, nil
}

// Exists checks if a wallet exists.
func (s *FileStorage) Exists(name string) (bool, error) {
	if err := ValidateWalletName(name); err != nil {
		return false, err
	}

	walletPath := s.walletPath(name)
	_, err := os.Stat(walletPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// List returns all wallet names.
func (s *FileStorage) List() ([]string, error) {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(s.basePath, walletDirPermissions); err != nil {
		return nil, fmt.Errorf("creating wallet directory: %w", err)
	}

	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("reading wallet directory: %w", err)
	}

	// Pre-allocate with estimated capacity
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, walletFileExtension) {
			walletName := strings.TrimSuffix(name, walletFileExtension)
			names = append(names, walletName)
		}
	}

	return names, nil
}

// Delete removes a wallet file.
func (s *FileStorage) Delete(name string) error {
	if err := ValidateWalletName(name); err != nil {
		return err
	}

	walletPath := s.walletPath(name)
	if _, err := os.Stat(walletPath); os.IsNotExist(err) {
		return ErrWalletNotFound
	}

	if err := os.Remove(walletPath); err != nil {
		return fmt.Errorf("removing wallet file: %w", err)
	}

	return nil
}

// LoadMetadata reads wallet metadata without decrypting the seed.
// This is useful for displaying wallet info without requiring the password.
func (s *FileStorage) LoadMetadata(name string) (*Wallet, error) {
	// Validate wallet name
	if err := ValidateWalletName(name); err != nil {
		return nil, err
	}

	// Check if wallet exists
	walletPath := s.walletPath(name)
	if _, err := os.Stat(walletPath); os.IsNotExist(err) {
		return nil, ErrWalletNotFound
	}

	// Read wallet file
	//nolint:gosec // G304: Path validated by ValidateWalletName + walletPath
	data, err := os.ReadFile(walletPath)
	if err != nil {
		return nil, fmt.Errorf("reading wallet file: %w", err)
	}

	// Unmarshal JSON
	var wf walletFile
	err = json.Unmarshal(data, &wf)
	if err != nil {
		return nil, fmt.Errorf("parsing wallet file: %w", err)
	}

	return wf.Wallet, nil
}

// walletPath returns the full path for a wallet file.
// The wallet name has already been validated by ValidateWalletName to match
// [a-zA-Z0-9_-]{1,64}, which prevents path traversal attacks.
func (s *FileStorage) walletPath(name string) string {
	path := filepath.Join(s.basePath, name+walletFileExtension)

	// Defensive check: ensure no directory traversal
	cleanPath := filepath.Clean(path)
	expectedSuffix := string(filepath.Separator) + name + walletFileExtension

	if !strings.HasSuffix(cleanPath, expectedSuffix) {
		// Return empty string - caller will fail with file-not-found error
		// This is defense-in-depth; ValidateWalletName should prevent this
		return ""
	}

	return cleanPath
}
