package wallet

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

const (
	// MaxAddressDerivation prevents resource exhaustion and integer overflow.
	MaxAddressDerivation = 100000
)

var (
	// ErrWalletNotFound indicates the wallet does not exist.
	ErrWalletNotFound = errors.New("wallet not found")

	// ErrWalletExists indicates a wallet with that name already exists.
	ErrWalletExists = errors.New("wallet already exists")

	// ErrInvalidWalletName indicates the wallet name is invalid.
	ErrInvalidWalletName = errors.New("wallet name must be 1-64 alphanumeric characters or underscores")

	// ErrInvalidAddressCount indicates the address count is invalid.
	ErrInvalidAddressCount = errors.New("invalid address count")

	// walletNameRegex validates wallet names: alphanumeric + underscore, 1-64 chars.
	walletNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{1,64}$`)
)

// Wallet represents an HD wallet with multi-chain address derivation.
type Wallet struct {
	// Name is the unique identifier for this wallet.
	Name string `json:"name"`

	// CreatedAt is the wallet creation timestamp.
	CreatedAt time.Time `json:"created_at"`

	// Addresses contains derived addresses per chain.
	Addresses map[Chain][]Address `json:"addresses"`

	// EnabledChains lists which chains are active for this wallet.
	EnabledChains []Chain `json:"enabled_chains"`

	// DerivationConfig holds chain-specific derivation settings.
	DerivationConfig DerivationConfig `json:"derivation_config"`

	// Version is the wallet file format version.
	Version int `json:"version"`
}

// DerivationConfig holds derivation settings for a wallet.
type DerivationConfig struct {
	// DefaultAccount is the default BIP44 account index.
	DefaultAccount uint32 `json:"default_account"`

	// AddressGap is the gap limit for address scanning.
	AddressGap int `json:"address_gap"`

	// Paths contains custom derivation paths per chain.
	Paths map[Chain]string `json:"paths,omitempty"`
}

// Summary is a lightweight wallet representation for listing.
type Summary struct {
	// Name is the unique wallet identifier.
	Name string `json:"name"`

	// CreatedAt is the wallet creation timestamp.
	CreatedAt time.Time `json:"created_at"`

	// EnabledChains lists which chains are active.
	EnabledChains []Chain `json:"enabled_chains"`

	// Addresses maps chain to primary address.
	Addresses map[Chain]string `json:"addresses"`
}

// ValidateWalletName checks if a wallet name is valid.
func ValidateWalletName(name string) error {
	if !walletNameRegex.MatchString(name) {
		return ErrInvalidWalletName
	}
	return nil
}

// NewWallet creates a new wallet with the given name and seed.
func NewWallet(name string, enabledChains []Chain) (*Wallet, error) {
	if err := ValidateWalletName(name); err != nil {
		return nil, err
	}

	if len(enabledChains) == 0 {
		enabledChains = []Chain{ChainETH, ChainBSV}
	}

	return &Wallet{
		Name:          name,
		CreatedAt:     time.Now().UTC(),
		Addresses:     make(map[Chain][]Address),
		EnabledChains: enabledChains,
		DerivationConfig: DerivationConfig{
			DefaultAccount: 0,
			AddressGap:     20,
			Paths:          make(map[Chain]string),
		},
		Version: 1,
	}, nil
}

// DeriveAddresses derives addresses for all enabled chains.
func (w *Wallet) DeriveAddresses(seed []byte, count int) error {
	// Validate bounds before conversion to uint32
	if count < 0 {
		return fmt.Errorf("%w: must be non-negative", ErrInvalidAddressCount)
	}
	if count > MaxAddressDerivation {
		return fmt.Errorf("%w: %d exceeds maximum %d",
			ErrInvalidAddressCount, count, MaxAddressDerivation)
	}

	for _, chain := range w.EnabledChains {
		addresses := make([]Address, 0, count)

		// Safe conversion: count validated to be in [0, MaxAddressDerivation]
		for i := 0; i < count; i++ {
			//nolint:gosec // G115: Safe conversion - i < count <= 100000 < MaxUint32
			idx := uint32(i)

			addr, err := DeriveAddress(seed, chain,
				w.DerivationConfig.DefaultAccount, idx)
			if err != nil {
				return fmt.Errorf("deriving address %d for chain %s: %w",
					idx, chain, err)
			}
			addresses = append(addresses, *addr)
		}
		w.Addresses[chain] = addresses
	}
	return nil
}

// GetPrimaryAddress returns the first address for a chain.
func (w *Wallet) GetPrimaryAddress(chain Chain) (string, bool) {
	addresses, ok := w.Addresses[chain]
	if !ok || len(addresses) == 0 {
		return "", false
	}
	return addresses[0].Address, true
}

// ToSummary creates a summary representation of the wallet.
func (w *Wallet) ToSummary() Summary {
	addresses := make(map[Chain]string)
	for chain := range w.Addresses {
		if addr, ok := w.GetPrimaryAddress(chain); ok {
			addresses[chain] = addr
		}
	}

	return Summary{
		Name:          w.Name,
		CreatedAt:     w.CreatedAt,
		EnabledChains: w.EnabledChains,
		Addresses:     addresses,
	}
}
