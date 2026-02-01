package eth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test vectors from EIP-55: https://eips.ethereum.org/EIPS/eip-55
//
//nolint:gochecknoglobals // Test data
var eip55TestVectors = []struct {
	name     string
	address  string
	expected string // Expected checksum address
}{
	{
		name:     "all caps",
		address:  "0x52908400098527886E0F7030069857D2E4169EE7",
		expected: "0x52908400098527886E0F7030069857D2E4169EE7",
	},
	{
		name:     "all lower",
		address:  "0x8617e340b3d01fa5f11f306f4090fd50e238070d",
		expected: "0x8617E340B3D01FA5F11F306F4090FD50E238070D",
	},
	{
		name:     "mixed case 1",
		address:  "0xde709f2102306220921060314715629080e2fb77",
		expected: "0xde709f2102306220921060314715629080e2fb77",
	},
	{
		name:     "mixed case 2",
		address:  "0x27b1fdb04752bbc536007a920d24acb045561c26",
		expected: "0x27b1fdb04752bbc536007a920d24acb045561c26",
	},
	{
		name:     "from EIP-55 spec",
		address:  "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		expected: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
	},
	{
		name:     "from EIP-55 spec 2",
		address:  "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		expected: "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
	},
	{
		name:     "from EIP-55 spec 3",
		address:  "0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
		expected: "0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
	},
	{
		name:     "from EIP-55 spec 4",
		address:  "0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb",
		expected: "0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb",
	},
}

func TestToChecksumAddress(t *testing.T) {
	for _, tc := range eip55TestVectors {
		t.Run(tc.name, func(t *testing.T) {
			result := ToChecksumAddress(tc.address)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestToChecksumAddress_LowerInput(t *testing.T) {
	tests := []struct {
		lower    string
		expected string
	}{
		{
			lower:    "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			expected: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
		},
		{
			lower:    "0xfb6916095ca1df60bb79ce92ce3ea74c37c5d359",
			expected: "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
		},
		{
			lower:    "0xdbf03b407c01e7cd3cbea99509d93f8dddc8c6fb",
			expected: "0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
		},
		{
			lower:    "0xd1220a0cf47c7b9be7a2e6ba89f429762e7b9adb",
			expected: "0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb",
		},
	}

	for _, tc := range tests {
		t.Run(tc.lower, func(t *testing.T) {
			result := ToChecksumAddress(tc.lower)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateChecksumAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		valid   bool
	}{
		// Valid EIP-55 checksummed addresses
		{
			name:    "valid checksum 1",
			address: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
			valid:   true,
		},
		{
			name:    "valid checksum 2",
			address: "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359",
			valid:   true,
		},
		{
			name:    "valid checksum 3",
			address: "0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB",
			valid:   true,
		},
		{
			name:    "valid checksum 4",
			address: "0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb",
			valid:   true,
		},
		// All lowercase is valid (considered non-checksummed)
		{
			name:    "all lowercase",
			address: "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			valid:   true,
		},
		// All uppercase is valid (considered non-checksummed)
		{
			name:    "all uppercase",
			address: "0x5AAEB6053F3E94C9B9A09F33669435E7EF1BEAED",
			valid:   true,
		},
		// Invalid checksum (wrong case on one character)
		{
			name:    "invalid checksum - wrong case",
			address: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAeD", // last char wrong
			valid:   false,
		},
		// Invalid format tests
		{
			name:    "missing 0x prefix",
			address: "5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
			valid:   false,
		},
		{
			name:    "too short",
			address: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeA",
			valid:   false,
		},
		{
			name:    "too long",
			address: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed1",
			valid:   false,
		},
		{
			name:    "invalid character",
			address: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAeG",
			valid:   false,
		},
		{
			name:    "empty string",
			address: "",
			valid:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateChecksumAddress(tc.address)
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestValidateChecksumAddress_Error(t *testing.T) {
	err := ValidateChecksumAddress("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAeD")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum")
}

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		valid   bool
	}{
		{"valid checksummed", "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", true},
		{"valid lowercase", "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed", true},
		{"valid uppercase", "0x5AAEB6053F3E94C9B9A09F33669435E7EF1BEAED", true},
		{"no prefix", "5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", false},
		{"too short", "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeA", false},
		{"too long", "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed00", false},
		{"empty", "", false},
		{"invalid hex char", "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAeZ", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			valid := IsValidAddress(tc.address)
			assert.Equal(t, tc.valid, valid)
		})
	}
}

func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			name:     "already checksummed",
			input:    "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
			expected: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
			hasError: false,
		},
		{
			name:     "lowercase to checksummed",
			input:    "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
			expected: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
			hasError: false,
		},
		{
			name:     "uppercase to checksummed",
			input:    "0x5AAEB6053F3E94C9B9A09F33669435E7EF1BEAED",
			expected: "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed",
			hasError: false,
		},
		{
			name:     "invalid address",
			input:    "invalid",
			expected: "",
			hasError: true,
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
			hasError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := NormalizeAddress(tc.input)
			if tc.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestZeroAddress(t *testing.T) {
	// The zero address should have valid checksum
	zeroAddr := "0x0000000000000000000000000000000000000000"
	checksummed := ToChecksumAddress(zeroAddr)
	assert.Equal(t, zeroAddr, checksummed)

	err := ValidateChecksumAddress(zeroAddr)
	assert.NoError(t, err)
}
