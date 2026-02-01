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
