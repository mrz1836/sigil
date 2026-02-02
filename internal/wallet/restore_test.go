package wallet

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectInputFormat tests format auto-detection for restore input.
func TestDetectInputFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected InputFormat
	}{
		// Mnemonic detection
		{
			name:     "12 word mnemonic",
			input:    "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
			expected: FormatMnemonic,
		},
		{
			name:     "24 word mnemonic",
			input:    "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art",
			expected: FormatMnemonic,
		},
		{
			name:     "mnemonic with extra whitespace",
			input:    "  abandon   abandon  abandon abandon abandon abandon abandon abandon abandon abandon abandon about  ",
			expected: FormatMnemonic,
		},
		{
			name:     "mnemonic with tabs/newlines",
			input:    "abandon\tabandon\nabandon abandon abandon abandon abandon abandon abandon abandon abandon about",
			expected: FormatMnemonic,
		},

		// WIF detection (mainnet)
		{
			name:     "WIF compressed mainnet (5)",
			input:    "5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ",
			expected: FormatWIF,
		},
		{
			name:     "WIF compressed mainnet (K)",
			input:    "KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617",
			expected: FormatWIF,
		},
		{
			name:     "WIF compressed mainnet (L)",
			input:    "L2RJNV7bLNnKvGyCHV6p5ZBXwrwFLhq6bDZ27LJevL99rEMYf8tC",
			expected: FormatWIF,
		},

		// Hex private key detection
		{
			name:     "hex 64 chars lowercase",
			input:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: FormatHex,
		},
		{
			name:     "hex 64 chars uppercase",
			input:    "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF",
			expected: FormatHex,
		},
		{
			name:     "hex 64 chars mixed case",
			input:    "0123456789AbCdEf0123456789aBcDeF0123456789ABCDEF0123456789abcdef",
			expected: FormatHex,
		},
		{
			name:     "hex with 0x prefix",
			input:    "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: FormatHex,
		},

		// Unknown format
		{
			name:     "too few words for mnemonic",
			input:    "abandon abandon abandon",
			expected: FormatUnknown,
		},
		{
			name:     "too short for hex",
			input:    "0123456789abcdef",
			expected: FormatUnknown,
		},
		{
			name:     "random string",
			input:    "hello world this is not valid",
			expected: FormatUnknown,
		},
		{
			name:     "empty string",
			input:    "",
			expected: FormatUnknown,
		},
		{
			name:     "hex with invalid chars",
			input:    "012345678gabcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: FormatUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DetectInputFormat(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestParseWIF tests WIF private key parsing.
func TestParseWIF(t *testing.T) {
	// Using known test vectors from Bitcoin wiki
	// https://en.bitcoin.it/wiki/Wallet_import_format
	tests := []struct {
		name    string
		wif     string
		wantErr bool
		keyLen  int // expected key length (32 bytes for valid)
	}{
		{
			// Uncompressed WIF from test vector
			name:    "valid WIF uncompressed (5)",
			wif:     "5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ",
			wantErr: false,
			keyLen:  32,
		},
		{
			// Compressed WIF from test vector
			name:    "valid WIF compressed (K)",
			wif:     "KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617",
			wantErr: false,
			keyLen:  32,
		},
		{
			name:    "invalid WIF checksum",
			wif:     "5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTK",
			wantErr: true,
		},
		{
			name:    "invalid WIF too short",
			wif:     "5HueCGU8rMjxEXx",
			wantErr: true,
		},
		{
			name:    "invalid WIF bad character",
			wif:     "5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTO",
			wantErr: true,
		},
		{
			name:    "empty string",
			wif:     "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, err := ParseWIF(tc.wif)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, key, tc.keyLen)
		})
	}
}

// TestParseHexKey tests hex private key parsing.
func TestParseHexKey(t *testing.T) {
	tests := []struct {
		name    string
		hex     string
		wantErr bool
		keyLen  int
	}{
		{
			name:    "valid 64 char hex",
			hex:     "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantErr: false,
			keyLen:  32,
		},
		{
			name:    "valid hex with 0x prefix",
			hex:     "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantErr: false,
			keyLen:  32,
		},
		{
			name:    "valid hex uppercase",
			hex:     "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789",
			wantErr: false,
			keyLen:  32,
		},
		{
			name:    "invalid hex too short",
			hex:     "0123456789abcdef",
			wantErr: true,
		},
		{
			name:    "invalid hex bad char",
			hex:     "012345678gabcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantErr: true,
		},
		{
			name:    "empty string",
			hex:     "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, err := ParseHexKey(tc.hex)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, key, tc.keyLen)
		})
	}
}

// TestSanitizeBitcoinAddress tests Bitcoin address sanitization.
func TestSanitizeBitcoinAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Clean inputs
		{
			name:     "valid P2PKH address",
			input:    "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "valid P2SH address",
			input:    "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",
			expected: "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",
		},

		// Whitespace handling
		{
			name:     "leading whitespace",
			input:    "  1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "trailing whitespace",
			input:    "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa  ",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "both sides whitespace",
			input:    "   1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa   ",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "with tabs",
			input:    "\t1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa\t",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "with newlines",
			input:    "\n1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa\n",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},

		// Invalid characters removed
		{
			name:     "with zero (invalid in Base58)",
			input:    "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa0",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "with capital O (invalid in Base58)",
			input:    "O1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "with capital I (invalid in Base58)",
			input:    "I1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "with lowercase l (invalid in Base58)",
			input:    "l1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},

		// Special characters
		{
			name:     "with dash",
			input:    "1A1zP1eP5QGefi2D-MPTfTL5SLmv7DivfNa",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "with underscore",
			input:    "1A1zP1eP5QGefi2D_MPTfTL5SLmv7DivfNa",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "with colon prefix (colon removed)",
			input:    "bitcoin:1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "bitcoin1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "with unicode",
			input:    "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa🚀",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},
		{
			name:     "with emoji prefix",
			input:    "💰1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			expected: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \t\n   ",
			expected: "",
		},
		{
			name:     "only invalid chars",
			input:    "0OIl-_@#$%",
			expected: "",
		},
		{
			name:     "spaces between chars (copy-paste error)",
			input:    "1A1z P1eP 5QGe fi2D",
			expected: "1A1zP1eP5QGefi2D",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := SanitizeBitcoinAddress(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestSanitizeWIF tests WIF private key sanitization.
func TestSanitizeWIF(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Clean WIF
		{
			name:     "valid WIF compressed",
			input:    "KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617",
			expected: "KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617",
		},
		{
			name:     "valid WIF uncompressed",
			input:    "5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ",
			expected: "5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ",
		},

		// Whitespace handling
		{
			name:     "with leading space",
			input:    " KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617",
			expected: "KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617",
		},
		{
			name:     "with trailing space",
			input:    "KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617 ",
			expected: "KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617",
		},

		// Invalid characters
		{
			name:     "with invalid 0",
			input:    "0KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617",
			expected: "KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617",
		},
		{
			name:     "with special chars",
			input:    "KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617!@#",
			expected: "KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := SanitizeWIF(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestSanitizeBitcoinAddress_ValidBase58Chars verifies only Base58 chars remain.
func TestSanitizeBitcoinAddress_ValidBase58Chars(t *testing.T) {
	t.Parallel()

	// Base58 alphabet (excludes 0, O, I, l)
	base58Chars := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	// Test that all Base58 chars are preserved
	result := SanitizeBitcoinAddress(base58Chars)
	assert.Equal(t, base58Chars, result, "all Base58 chars should be preserved")

	// Test that excluded chars are removed
	excluded := "0OIl"
	result = SanitizeBitcoinAddress(excluded)
	assert.Empty(t, result, "excluded chars should be removed")

	// Test mixed input - note: order is preserved as chars appear
	mixed := "0O1I2l3A4B5"
	result = SanitizeBitcoinAddress(mixed)
	assert.Equal(t, "123A4B5", result, "only valid Base58 chars should remain in order")
}

// TestSanitizeBitcoinAddress_PreservesValidAddress verifies valid addresses pass through.
func TestSanitizeBitcoinAddress_PreservesValidAddress(t *testing.T) {
	t.Parallel()

	// P2PKH and P2SH addresses use only Base58 characters
	p2pkhAndP2shAddresses := []string{
		"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", // Satoshi's address (P2PKH)
		"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy", // P2SH
	}

	for _, addr := range p2pkhAndP2shAddresses {
		t.Run(addr[:8], func(t *testing.T) {
			t.Parallel()
			result := SanitizeBitcoinAddress(addr)
			// Valid Base58 addresses should pass through unchanged
			assert.Equal(t, addr, result, "valid Base58 address should be preserved")
		})
	}

	// Bech32 addresses contain '0' which is not valid Base58
	// so they will be partially sanitized
	t.Run("bech32_partial", func(t *testing.T) {
		t.Parallel()
		bech32 := "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq"
		result := SanitizeBitcoinAddress(bech32)
		// '0' is removed from bech32 addresses
		assert.NotContains(t, result, "0", "0 should be removed from bech32")
		assert.Less(t, len(result), len(bech32), "result should be shorter")
	})
}
