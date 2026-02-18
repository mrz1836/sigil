package wallet

import (
	"fmt"
	"regexp"
	"time"

	"github.com/mrz1836/go-sanitize"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// MaxAddressDerivation prevents resource exhaustion and integer overflow.
	MaxAddressDerivation = 100000
)

var (
	// ErrWalletNotFound indicates the wallet does not exist.
	// Uses SigilError for proper exit code (ExitNotFound = 4).
	ErrWalletNotFound = sigilerr.ErrWalletNotFound

	// ErrWalletExists indicates a wallet with that name already exists.
	// Uses SigilError for proper exit code (ExitInput = 2).
	ErrWalletExists = sigilerr.ErrWalletExists

	// ErrInvalidWalletName indicates the wallet name is invalid.
	ErrInvalidWalletName = sigilerr.WithSuggestion(sigilerr.ErrInvalidInput, "wallet name must be 1-64 alphanumeric characters, underscores, or hyphens")

	// ErrInvalidAddressCount indicates the address count is invalid.
	ErrInvalidAddressCount = sigilerr.WithSuggestion(sigilerr.ErrInvalidInput, "invalid address count")

	// walletNameRegex validates wallet names: alphanumeric + underscore + hyphen, 1-64 chars.
	walletNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
)

// Wallet represents an HD wallet with multi-chain address derivation.
type Wallet struct {
	// Name is the unique identifier for this wallet.
	Name string `json:"name"`

	// CreatedAt is the wallet creation timestamp.
	CreatedAt time.Time `json:"created_at"`

	// Addresses contains derived receiving addresses per chain (external chain).
	Addresses map[ChainID][]Address `json:"addresses"`

	// ChangeAddresses contains derived change addresses per chain (internal chain).
	ChangeAddresses map[ChainID][]Address `json:"change_addresses,omitempty"`

	// EnabledChains lists which chains are active for this wallet.
	EnabledChains []ChainID `json:"enabled_chains"`

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
	Paths map[ChainID]string `json:"paths,omitempty"`
}

// Summary is a lightweight wallet representation for listing.
type Summary struct {
	// Name is the unique wallet identifier.
	Name string `json:"name"`

	// CreatedAt is the wallet creation timestamp.
	CreatedAt time.Time `json:"created_at"`

	// EnabledChains lists which chains are active.
	EnabledChains []ChainID `json:"enabled_chains"`

	// Addresses maps chain to primary address.
	Addresses map[ChainID]string `json:"addresses"`
}

// ValidateWalletName checks if a wallet name is valid.
func ValidateWalletName(name string) error {
	if !walletNameRegex.MatchString(name) {
		return ErrInvalidWalletName
	}
	return nil
}

// SuggestWalletName provides a sanitized version of an invalid wallet name.
// It uses sanitize.PathName to clean the input, keeping only ASCII alphanumeric
// characters, hyphens, and underscores. The result is truncated to 64 characters.
// Returns empty string if the input cannot be sanitized to a valid name.
func SuggestWalletName(name string) string {
	suggested := sanitize.PathName(name)
	if suggested == "" {
		return ""
	}
	if len(suggested) > 64 {
		suggested = suggested[:64]
	}
	return suggested
}

// NewWallet creates a new wallet with the given name and seed.
func NewWallet(name string, enabledChains []ChainID) (*Wallet, error) {
	if err := ValidateWalletName(name); err != nil {
		return nil, err
	}

	if len(enabledChains) == 0 {
		enabledChains = []ChainID{ChainETH, ChainBSV}
	}

	return &Wallet{
		Name:            name,
		CreatedAt:       time.Now().UTC(),
		Addresses:       make(map[ChainID][]Address),
		ChangeAddresses: make(map[ChainID][]Address),
		EnabledChains:   enabledChains,
		DerivationConfig: DerivationConfig{
			DefaultAccount: 0,
			AddressGap:     20,
			Paths:          make(map[ChainID]string),
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
func (w *Wallet) GetPrimaryAddress(chain ChainID) (string, bool) {
	addresses, ok := w.Addresses[chain]
	if !ok || len(addresses) == 0 {
		return "", false
	}
	return addresses[0].Address, true
}

// ToSummary creates a summary representation of the wallet.
func (w *Wallet) ToSummary() Summary {
	addresses := make(map[ChainID]string)
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

// DeriveNextReceiveAddress derives the next receiving address for a chain.
// The address is appended to Addresses and returned.
func (w *Wallet) DeriveNextReceiveAddress(seed []byte, chain ChainID) (*Address, error) {
	nextIndex := w.GetReceiveAddressCount(chain)
	if nextIndex >= MaxAddressDerivation {
		return nil, fmt.Errorf("%w: would exceed maximum %d",
			ErrInvalidAddressCount, MaxAddressDerivation)
	}

	//nolint:gosec // G115: Safe - validated against MaxAddressDerivation
	addr, err := DeriveAddressWithChange(seed, chain,
		w.DerivationConfig.DefaultAccount, ExternalChain, uint32(nextIndex))
	if err != nil {
		return nil, fmt.Errorf("deriving receive address %d for chain %s: %w",
			nextIndex, chain, err)
	}

	w.Addresses[chain] = append(w.Addresses[chain], *addr)
	return addr, nil
}

// DeriveNextChangeAddress derives the next change address for a chain.
// The address is appended to ChangeAddresses and returned.
func (w *Wallet) DeriveNextChangeAddress(seed []byte, chain ChainID) (*Address, error) {
	// Initialize ChangeAddresses map if nil (for backward compatibility with old wallets)
	if w.ChangeAddresses == nil {
		w.ChangeAddresses = make(map[ChainID][]Address)
	}

	nextIndex := w.GetChangeAddressCount(chain)
	if nextIndex >= MaxAddressDerivation {
		return nil, fmt.Errorf("%w: would exceed maximum %d",
			ErrInvalidAddressCount, MaxAddressDerivation)
	}

	//nolint:gosec // G115: Safe - validated against MaxAddressDerivation
	addr, err := DeriveAddressWithChange(seed, chain,
		w.DerivationConfig.DefaultAccount, InternalChain, uint32(nextIndex))
	if err != nil {
		return nil, fmt.Errorf("deriving change address %d for chain %s: %w",
			nextIndex, chain, err)
	}

	w.ChangeAddresses[chain] = append(w.ChangeAddresses[chain], *addr)
	return addr, nil
}

// GetReceiveAddressCount returns the number of derived receiving addresses for a chain.
func (w *Wallet) GetReceiveAddressCount(chain ChainID) int {
	return len(w.Addresses[chain])
}

// GetChangeAddressCount returns the number of derived change addresses for a chain.
func (w *Wallet) GetChangeAddressCount(chain ChainID) int {
	if w.ChangeAddresses == nil {
		return 0
	}
	return len(w.ChangeAddresses[chain])
}

// GetReceiveAddress returns a receiving address by index, or nil if not found.
func (w *Wallet) GetReceiveAddress(chain ChainID, index int) *Address {
	addrs := w.Addresses[chain]
	if index < 0 || index >= len(addrs) {
		return nil
	}
	return &addrs[index]
}

// GetChangeAddress returns a change address by index, or nil if not found.
func (w *Wallet) GetChangeAddress(chain ChainID, index int) *Address {
	if w.ChangeAddresses == nil {
		return nil
	}
	addrs := w.ChangeAddresses[chain]
	if index < 0 || index >= len(addrs) {
		return nil
	}
	return &addrs[index]
}

// GetAllAddresses returns all addresses (receive + change) for a chain.
func (w *Wallet) GetAllAddresses(chain ChainID) []Address {
	result := make([]Address, 0, w.GetReceiveAddressCount(chain)+w.GetChangeAddressCount(chain))
	result = append(result, w.Addresses[chain]...)
	if w.ChangeAddresses != nil {
		result = append(result, w.ChangeAddresses[chain]...)
	}
	return result
}

// FindAddressByString searches for an address string in both receive and change addresses.
// Returns the Address and whether it's a change address, or nil if not found.
func (w *Wallet) FindAddressByString(chain ChainID, addressStr string) (*Address, bool) {
	// Search receive addresses
	for i := range w.Addresses[chain] {
		if w.Addresses[chain][i].Address == addressStr {
			return &w.Addresses[chain][i], false
		}
	}
	// Search change addresses
	if w.ChangeAddresses != nil {
		for i := range w.ChangeAddresses[chain] {
			if w.ChangeAddresses[chain][i].Address == addressStr {
				return &w.ChangeAddresses[chain][i], true
			}
		}
	}
	return nil, false
}
