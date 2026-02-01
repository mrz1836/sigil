// Package contracts defines the interface contracts for Sigil MVP.
// These are design artifacts - not compiled code.
// Actual implementations go in internal/wallet/
package contracts

import (
	"context"
	"time"
)

// WalletService defines the interface for wallet operations.
type WalletService interface {
	// Create generates a new wallet with a BIP39 mnemonic.
	// Returns the mnemonic (display once to user) and created wallet.
	Create(ctx context.Context, req CreateWalletRequest) (*CreateWalletResult, error)

	// Restore recreates a wallet from an existing seed phrase or key.
	Restore(ctx context.Context, req RestoreWalletRequest) (*Wallet, error)

	// Get retrieves a wallet by name.
	Get(ctx context.Context, name string) (*Wallet, error)

	// List returns all wallets.
	List(ctx context.Context) ([]WalletSummary, error)

	// Delete removes a wallet (requires confirmation).
	Delete(ctx context.Context, name string) error

	// Unlock decrypts the wallet and returns the seed for signing.
	// The returned seed must be zeroed by the caller after use.
	Unlock(ctx context.Context, name, password string) (*UnlockedWallet, error)
}

// CreateWalletRequest contains parameters for wallet creation.
type CreateWalletRequest struct {
	// Name is the unique wallet identifier.
	Name string

	// Words is the mnemonic length (12 or 24).
	Words int

	// Passphrase is the optional BIP39 passphrase.
	Passphrase string

	// Password is the encryption password for the wallet file.
	Password string

	// EnabledChains lists which chains to enable (default: all).
	EnabledChains []string
}

// CreateWalletResult contains the created wallet and mnemonic.
type CreateWalletResult struct {
	// Wallet is the created wallet.
	Wallet *Wallet

	// Mnemonic is the BIP39 mnemonic phrase (display once, then discard).
	Mnemonic string
}

// RestoreWalletRequest contains parameters for wallet restoration.
type RestoreWalletRequest struct {
	// Name is the unique wallet identifier.
	Name string

	// Input is the seed material (mnemonic, WIF, hex, or file path).
	Input string

	// Format is the input format (auto-detected if empty).
	// Values: "mnemonic", "wif", "hex", "file"
	Format string

	// Passphrase is the optional BIP39 passphrase.
	Passphrase string

	// Password is the encryption password for the wallet file.
	Password string

	// EnabledChains lists which chains to enable (default: all).
	EnabledChains []string
}

// Wallet represents a complete wallet with addresses.
type Wallet struct {
	Name          string
	CreatedAt     time.Time
	Addresses     map[string][]Address // chain -> addresses
	EnabledChains []string
	Version       int
}

// Address represents a derived blockchain address.
type Address struct {
	Path      string
	Index     uint32
	Address   string
	PublicKey string
}

// WalletSummary is a lightweight wallet representation for listing.
type WalletSummary struct {
	Name          string
	CreatedAt     time.Time
	EnabledChains []string
	Addresses     map[string]string // chain -> primary address
}

// UnlockedWallet contains the decrypted seed for signing operations.
type UnlockedWallet struct {
	// Wallet is the wallet metadata.
	Wallet *Wallet

	// Seed is the decrypted BIP39 seed bytes.
	// MUST be zeroed after use.
	Seed []byte

	// Destroy zeros the seed and marks the wallet as locked.
	Destroy func()
}

// Storage defines the interface for wallet persistence.
type Storage interface {
	// Save encrypts and writes a wallet to storage.
	Save(ctx context.Context, wallet *Wallet, encryptedSeed []byte) error

	// Load reads and returns wallet metadata (without decrypting seed).
	Load(ctx context.Context, name string) (*Wallet, []byte, error)

	// Exists checks if a wallet exists.
	Exists(ctx context.Context, name string) (bool, error)

	// List returns all wallet names.
	List(ctx context.Context) ([]string, error)

	// Delete removes a wallet file.
	Delete(ctx context.Context, name string) error
}

// MnemonicService defines mnemonic-related operations.
type MnemonicService interface {
	// Generate creates a new BIP39 mnemonic.
	Generate(words int) (string, error)

	// Validate checks if a mnemonic is valid.
	Validate(mnemonic string) error

	// ValidateWord checks a single word and returns suggestions for typos.
	ValidateWord(word string) (valid bool, suggestions []string)

	// ToSeed converts a mnemonic to seed bytes.
	ToSeed(mnemonic, passphrase string) ([]byte, error)

	// NormalizeInput cleans whitespace from mnemonic input.
	NormalizeInput(input string) string
}

// DerivationService defines key derivation operations.
type DerivationService interface {
	// DeriveAddress derives an address for the given chain and index.
	DeriveAddress(seed []byte, chain string, account, index uint32) (*Address, error)

	// DerivePrivateKey derives a private key for signing.
	// The returned key must be zeroed after use.
	DerivePrivateKey(seed []byte, chain string, account, index uint32) ([]byte, error)

	// GetDerivationPath returns the BIP44 path for a chain.
	GetDerivationPath(chain string, account, index uint32) string
}

// Wallet-related errors.
var (
	ErrWalletNotFound    = Error{Code: "WALLET_NOT_FOUND", Message: "wallet not found"}
	ErrWalletExists      = Error{Code: "WALLET_EXISTS", Message: "wallet already exists"}
	ErrWalletLocked      = Error{Code: "WALLET_LOCKED", Message: "wallet is locked"}
	ErrInvalidMnemonic   = Error{Code: "INVALID_MNEMONIC", Message: "invalid mnemonic phrase"}
	ErrDecryptionFailed  = Error{Code: "DECRYPTION_FAILED", Message: "decryption failed - wrong password or corrupted file"}
	ErrInvalidWalletName = Error{Code: "INVALID_WALLET_NAME", Message: "wallet name must be alphanumeric with underscores"}
	ErrInvalidWordCount  = Error{Code: "INVALID_WORD_COUNT", Message: "mnemonic must be 12 or 24 words"}
)
