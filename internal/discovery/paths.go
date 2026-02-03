// Package discovery provides multi-path wallet scanning and fund recovery.
// It supports discovering funds across different derivation path schemes used
// by various BSV wallets (RelayX, MoneyButton, HandCash, etc.).
package discovery

// PathScheme defines a derivation path scheme used by specific wallets.
// Different wallets use different BIP44 coin types and derivation structures.
type PathScheme struct {
	// Name is the human-readable name of the derivation scheme.
	Name string

	// Wallets lists wallets known to use this derivation scheme.
	Wallets []string

	// CoinType is the BIP44 coin type (e.g., 0=BTC, 145=BCH, 236=BSV).
	CoinType uint32

	// Purpose is the BIP44 purpose (typically 44 for BIP44, 0 for legacy).
	Purpose uint32

	// Accounts is the list of account indices to scan (typically just [0]).
	Accounts []uint32

	// ScanChange indicates whether to scan both external (0) and internal (1) chains.
	ScanChange bool

	// IsLegacy indicates non-standard derivation (e.g., m/0'/index for HandCash).
	IsLegacy bool

	// Priority determines scan order (lower = scanned first).
	Priority int
}

// BIP44 coin type constants.
const (
	// CoinTypeBTC is the Bitcoin coin type (used by MoneyButton, ElectrumSV imports).
	CoinTypeBTC uint32 = 0

	// CoinTypeBCH is the Bitcoin Cash coin type (used by Exodus, Simply.Cash).
	CoinTypeBCH uint32 = 145

	// CoinTypeBSV is the Bitcoin SV coin type (standard BSV wallets).
	CoinTypeBSV uint32 = 236
)

// BIP44 purpose constants.
const (
	// PurposeBIP44 is the standard BIP44 purpose.
	PurposeBIP44 uint32 = 44

	// PurposeLegacy is used for legacy non-BIP44 paths like m/0'.
	PurposeLegacy uint32 = 0
)

// Priority constants for scan ordering.
const (
	// PriorityBSVStandard is the highest priority (most likely to have funds).
	PriorityBSVStandard = 1

	// PriorityBitcoinLegacy is for wallets using BTC coin type.
	PriorityBitcoinLegacy = 2

	// PriorityBitcoinCash is for wallets using BCH coin type.
	PriorityBitcoinCash = 3

	// PriorityHandCashLegacy is for old HandCash 1.x wallets.
	PriorityHandCashLegacy = 4

	// PriorityMultiAccount is for power users with multiple accounts.
	PriorityMultiAccount = 5
)

// DefaultSchemes returns the standard set of path schemes to scan.
// Schemes are ordered by priority (most common/likely first).
func DefaultSchemes() []PathScheme {
	return []PathScheme{
		{
			Name:       "BSV Standard",
			Wallets:    []string{"RelayX", "RockWallet", "Twetch", "Trezor", "Ledger", "KeepKey"},
			CoinType:   CoinTypeBSV,
			Purpose:    PurposeBIP44,
			Accounts:   []uint32{0},
			ScanChange: true,
			Priority:   PriorityBSVStandard,
		},
		{
			Name:       "Bitcoin Legacy",
			Wallets:    []string{"MoneyButton", "ElectrumSV"},
			CoinType:   CoinTypeBTC,
			Purpose:    PurposeBIP44,
			Accounts:   []uint32{0},
			ScanChange: true,
			Priority:   PriorityBitcoinLegacy,
		},
		{
			Name:       "Bitcoin Cash",
			Wallets:    []string{"Exodus", "Simply.Cash", "BCH Fork Splits"},
			CoinType:   CoinTypeBCH,
			Purpose:    PurposeBIP44,
			Accounts:   []uint32{0},
			ScanChange: true,
			Priority:   PriorityBitcoinCash,
		},
		{
			Name:       "HandCash Legacy",
			Wallets:    []string{"HandCash 1.x"},
			CoinType:   CoinTypeBTC,
			Purpose:    PurposeLegacy,
			Accounts:   []uint32{0},
			ScanChange: false,
			IsLegacy:   true,
			Priority:   PriorityHandCashLegacy,
		},
		{
			Name:       "Multi-Account BSV",
			Wallets:    []string{"Power Users"},
			CoinType:   CoinTypeBSV,
			Purpose:    PurposeBIP44,
			Accounts:   []uint32{1, 2, 3, 4},
			ScanChange: true,
			Priority:   PriorityMultiAccount,
		},
	}
}

// SchemeByName returns a path scheme by its name, or nil if not found.
func SchemeByName(name string) *PathScheme {
	for _, scheme := range DefaultSchemes() {
		if scheme.Name == name {
			return &scheme
		}
	}
	return nil
}

// SchemesForWallet returns all path schemes that a specific wallet might use.
func SchemesForWallet(walletName string) []PathScheme {
	var matches []PathScheme
	for _, scheme := range DefaultSchemes() {
		for _, w := range scheme.Wallets {
			if w == walletName {
				matches = append(matches, scheme)
				break
			}
		}
	}
	return matches
}

// SortByPriority returns schemes sorted by priority (ascending).
func SortByPriority(schemes []PathScheme) []PathScheme {
	// Make a copy to avoid mutating the input
	result := make([]PathScheme, len(schemes))
	copy(result, schemes)

	// Simple insertion sort (small slice, stable sort preferred)
	for i := 1; i < len(result); i++ {
		key := result[i]
		j := i - 1
		for j >= 0 && result[j].Priority > key.Priority {
			result[j+1] = result[j]
			j--
		}
		result[j+1] = key
	}

	return result
}
