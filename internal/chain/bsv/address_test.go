package bsv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBase58CheckAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		valid   bool
	}{
		// Valid P2PKH mainnet addresses (start with 1)
		{
			name:    "valid mainnet P2PKH 1",
			address: "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
			valid:   true,
		},
		{
			name:    "valid mainnet P2PKH 2",
			address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", // Satoshi's address
			valid:   true,
		},
		// Valid P2SH mainnet addresses (start with 3)
		{
			name:    "valid mainnet P2SH",
			address: "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",
			valid:   true,
		},
		// Invalid addresses
		{
			name:    "empty string",
			address: "",
			valid:   false,
		},
		{
			name:    "too short",
			address: "1BvBMSEY",
			valid:   false,
		},
		{
			name:    "invalid character (0)",
			address: "10vBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
			valid:   false,
		},
		{
			name:    "invalid character (O)",
			address: "1OvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
			valid:   false,
		},
		{
			name:    "invalid character (I)",
			address: "1IvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
			valid:   false,
		},
		{
			name:    "invalid character (l)",
			address: "1lvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
			valid:   false,
		},
		{
			name:    "wrong prefix (2)",
			address: "2BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
			valid:   false,
		},
		{
			name:    "Ethereum address format",
			address: "0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0",
			valid:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateBase58CheckAddress(tc.address)
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		valid   bool
	}{
		{"valid P2PKH", "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", true},
		{"valid P2SH", "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			valid := IsValidAddress(tc.address)
			assert.Equal(t, tc.valid, valid)
		})
	}
}

func TestDecodeBase58Check(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		wantVersion byte
		wantErr     bool
	}{
		{
			name:        "P2PKH mainnet (version 0x00)",
			address:     "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2",
			wantVersion: 0x00,
			wantErr:     false,
		},
		{
			name:        "P2SH mainnet (version 0x05)",
			address:     "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy",
			wantVersion: 0x05,
			wantErr:     false,
		},
		{
			name:    "invalid base58",
			address: "0OIl",
			wantErr: true,
		},
		{
			name:    "empty",
			address: "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			version, payload, err := DecodeBase58Check(tc.address)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantVersion, version)
				assert.Len(t, payload, 20) // RIPEMD-160 hash length
			}
		})
	}
}

func TestValidateChecksum(t *testing.T) {
	// Valid address should pass checksum validation
	err := ValidateBase58CheckAddress("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2")
	require.NoError(t, err)

	// Corrupted address (change last character) should fail
	err = ValidateBase58CheckAddress("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum")
}

func TestAddressVersions(t *testing.T) {
	// Test that we correctly identify address versions
	tests := []struct {
		address      string
		expectedType string
		wantVersion  byte
	}{
		{"1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2", "P2PKH", 0x00},
		{"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy", "P2SH", 0x05},
	}

	for _, tc := range tests {
		t.Run(tc.expectedType, func(t *testing.T) {
			version, _, err := DecodeBase58Check(tc.address)
			require.NoError(t, err)
			assert.Equal(t, tc.wantVersion, version)
		})
	}
}
