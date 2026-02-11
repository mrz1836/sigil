package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSuggestWalletName tests the wallet name sanitization function.
func TestSuggestWalletName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Clean inputs
		{
			name:     "valid alphanumeric name",
			input:    "myWallet123",
			expected: "myWallet123",
		},
		{
			name:     "valid with underscores",
			input:    "my_wallet_name",
			expected: "my_wallet_name",
		},
		{
			name:     "valid with hyphens",
			input:    "my-wallet-name",
			expected: "my-wallet-name",
		},

		// Whitespace handling
		{
			name:     "leading whitespace",
			input:    "  mywallet",
			expected: "mywallet",
		},
		{
			name:     "trailing whitespace",
			input:    "mywallet  ",
			expected: "mywallet",
		},
		{
			name:     "spaces in name",
			input:    "my wallet name",
			expected: "mywalletname",
		},
		{
			name:     "tabs in name",
			input:    "my\twallet",
			expected: "mywallet",
		},
		{
			name:     "newlines in name",
			input:    "my\nwallet",
			expected: "mywallet",
		},

		// Special characters
		{
			name:     "with @ symbol",
			input:    "my@wallet",
			expected: "mywallet",
		},
		{
			name:     "with # symbol",
			input:    "my#wallet",
			expected: "mywallet",
		},
		{
			name:     "with $ symbol",
			input:    "my$wallet",
			expected: "mywallet",
		},
		{
			name:     "with exclamation",
			input:    "mywallet!",
			expected: "mywallet",
		},
		{
			name:     "with parentheses",
			input:    "my(wallet)",
			expected: "mywallet",
		},
		{
			name:     "with brackets",
			input:    "my[wallet]",
			expected: "mywallet",
		},
		{
			name:     "with quotes",
			input:    `my"wallet"`,
			expected: "mywallet",
		},
		{
			name:     "with single quotes",
			input:    "my'wallet'",
			expected: "mywallet",
		},
		{
			name:     "with periods",
			input:    "my.wallet.name",
			expected: "mywalletname",
		},
		{
			name:     "with slash",
			input:    "my/wallet",
			expected: "mywallet",
		},
		{
			name:     "with backslash",
			input:    "my\\wallet",
			expected: "mywallet",
		},
		{
			name:     "with colon",
			input:    "my:wallet",
			expected: "mywallet",
		},

		// Unicode and international characters
		{
			name:     "with emoji",
			input:    "my\U0001F525wallet", // fire emoji
			expected: "mywallet",
		},
		{
			name:     "with accented characters",
			input:    "mywallet\u00e9\u00e0\u00fc", // éàü
			expected: "mywallet",
		},
		{
			name:     "with CJK characters",
			input:    "my\u94b1\u5305wallet", // Chinese "钱包"
			expected: "mywallet",
		},
		{
			name:     "with Japanese katakana",
			input:    "my\u30a6\u30a9\u30ec\u30c3\u30c8", // ウォレット
			expected: "my",
		},
		{
			name:     "with Arabic characters",
			input:    "\u0645\u062d\u0641\u0638\u0629wallet", // محفظة
			expected: "wallet",
		},
		{
			name:     "with Cyrillic characters",
			input:    "my\u043a\u043e\u0448\u0435\u043b\u0435\u043a", // кошелек
			expected: "my",
		},

		// Length truncation
		{
			name:     "exactly 64 characters",
			input:    "a123456789b123456789c123456789d123456789e123456789f123456789wxyz",
			expected: "a123456789b123456789c123456789d123456789e123456789f123456789wxyz",
		},
		{
			name:     "over 64 characters truncated",
			input:    "a123456789b123456789c123456789d123456789e123456789f123456789wxyz_extra",
			expected: "a123456789b123456789c123456789d123456789e123456789f123456789wxyz",
		},
		{
			name:     "very long input",
			input:    "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz",
			expected: "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz01",
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \t\n  ",
			expected: "",
		},
		{
			name:     "only special characters",
			input:    "!@#$%^&*()",
			expected: "",
		},
		{
			name:     "only unicode",
			input:    "\u65e5\u672c\u8a9e\u4e2d\u6587\ud55c\uad6d\uc5b4", // 日本語中文한국어
			expected: "",
		},
		{
			name:     "mixed special and valid",
			input:    "!!!wallet!!!",
			expected: "wallet",
		},
		{
			name:     "numbers only",
			input:    "12345",
			expected: "12345",
		},
		{
			name:     "single character",
			input:    "a",
			expected: "a",
		},
		{
			name:     "single hyphen",
			input:    "-",
			expected: "-",
		},
		{
			name:     "single underscore",
			input:    "_",
			expected: "_",
		},

		// URL-like inputs (common copy-paste errors)
		{
			name:     "with http prefix",
			input:    "http://mywallet",
			expected: "httpmywallet",
		},

		// Control characters
		{
			name:     "with null byte",
			input:    "my\x00wallet",
			expected: "mywallet",
		},
		{
			name:     "with bell character",
			input:    "my\awallet",
			expected: "mywallet",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := SuggestWalletName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestSuggestWalletName_ValidatesAfterSanitization verifies that suggested names are valid.
func TestSuggestWalletName_ValidatesAfterSanitization(t *testing.T) {
	t.Parallel()

	// Valid inputs should produce valid wallet names
	validInputs := []string{
		"mywallet",
		"my_wallet",
		"my-wallet",
		"MyWallet123",
		"  spaced  ",
		"special@chars#removed",
	}

	for _, input := range validInputs {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			suggested := SuggestWalletName(input)
			if suggested != "" {
				assert.NoError(t, ValidateWalletName(suggested))
			}
		})
	}
}

// TestWallet_NilChangeAddresses tests backward compatibility when ChangeAddresses is nil.
func TestWallet_NilChangeAddresses(t *testing.T) {
	t.Parallel()
	seed, err := MnemonicToSeed("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create wallet with nil ChangeAddresses (simulates old wallet format)
	w := &Wallet{
		Name:            "test",
		EnabledChains:   []ChainID{ChainBSV},
		Addresses:       make(map[ChainID][]Address),
		ChangeAddresses: nil, // Explicitly nil to simulate old format
	}

	// GetChangeAddressCount should return 0 for nil map
	count := w.GetChangeAddressCount(ChainBSV)
	assert.Equal(t, 0, count)

	// GetChangeAddress should return nil for nil map
	addr := w.GetChangeAddress(ChainBSV, 0)
	assert.Nil(t, addr)

	// DeriveNextChangeAddress should initialize the map and work
	changeAddr, err := w.DeriveNextChangeAddress(seed, ChainBSV)
	require.NoError(t, err)
	assert.NotNil(t, changeAddr)
	assert.NotNil(t, w.ChangeAddresses, "ChangeAddresses should be initialized")

	// After initialization, GetChangeAddressCount should work
	count = w.GetChangeAddressCount(ChainBSV)
	assert.Equal(t, 1, count)

	// GetChangeAddress should now return the address
	retrievedAddr := w.GetChangeAddress(ChainBSV, 0)
	assert.NotNil(t, retrievedAddr)
	assert.Equal(t, changeAddr.Address, retrievedAddr.Address)
}

// TestWallet_DeriveNextAddress_MaxIndex tests the MaxAddressDerivation limit.
func TestWallet_DeriveNextAddress_MaxIndex(t *testing.T) {
	t.Parallel()
	seed, err := MnemonicToSeed("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", "")
	if err != nil {
		t.Fatal(err)
	}

	w := &Wallet{
		Name:            "test",
		EnabledChains:   []ChainID{ChainBSV},
		Addresses:       make(map[ChainID][]Address),
		ChangeAddresses: make(map[ChainID][]Address),
	}

	// Pre-fill with MaxAddressDerivation addresses (this would be slow in practice,
	// so we'll simulate by setting the count to near the limit)
	// Create addresses up to MaxAddressDerivation - 1
	addresses := make([]Address, MaxAddressDerivation-1)
	for i := range addresses {
		addresses[i] = Address{
			Address:   "dummy",
			Index:     uint32(i), //nolint:gosec // i is bounded by MaxAddressDerivation (100000), safe for uint32
			Path:      "m/44'/236'/0'/0/0",
			PublicKey: "dummy",
		}
	}
	w.Addresses[ChainBSV] = addresses

	// Deriving one more should work (index MaxAddressDerivation - 1)
	addr, err := w.DeriveNextReceiveAddress(seed, ChainBSV)
	require.NoError(t, err)
	assert.NotNil(t, addr)
	assert.Len(t, w.Addresses[ChainBSV], MaxAddressDerivation)

	// Attempting to derive at MaxAddressDerivation should fail
	_, err = w.DeriveNextReceiveAddress(seed, ChainBSV)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "100000") // Should mention the limit

	// Same test for change addresses
	w.ChangeAddresses[ChainBSV] = make([]Address, MaxAddressDerivation-1)
	for i := range w.ChangeAddresses[ChainBSV] {
		w.ChangeAddresses[ChainBSV][i] = Address{
			Address:   "dummy",
			Index:     uint32(i), //nolint:gosec // i is bounded by MaxAddressDerivation (100000), safe for uint32
			Path:      "m/44'/236'/0'/1/0",
			PublicKey: "dummy",
		}
	}

	// Deriving one more change address should work
	changeAddr, err := w.DeriveNextChangeAddress(seed, ChainBSV)
	require.NoError(t, err)
	assert.NotNil(t, changeAddr)

	// Attempting to derive beyond limit should fail
	_, err = w.DeriveNextChangeAddress(seed, ChainBSV)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "100000")
}
