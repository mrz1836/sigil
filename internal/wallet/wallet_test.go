package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
