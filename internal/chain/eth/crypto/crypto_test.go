package ethcrypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeccak256(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "empty input",
			input:    []byte{},
			expected: "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
		},
		{
			name:     "hello world",
			input:    []byte("hello"),
			expected: "1c8aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8",
		},
		{
			name:     "transfer function signature",
			input:    []byte("transfer(address,uint256)"),
			expected: "a9059cbb2ab09eb219583f4a59a5d0623ade346d962bcd4e46b11da047c9049b",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := Keccak256(tc.input)
			assert.Equal(t, tc.expected, hex.EncodeToString(result))
		})
	}
}

func TestKeccak256Hash(t *testing.T) {
	t.Parallel()
	hash := Keccak256Hash([]byte("hello"))
	assert.Len(t, hash, 32)
	assert.Equal(t, "1c8aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8", hex.EncodeToString(hash[:]))
}

func TestPrivateKeyToPublicKey(t *testing.T) {
	t.Parallel()

	// Known test vector
	privKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	privKey, err := hex.DecodeString(privKeyHex)
	require.NoError(t, err)

	pubKey, err := PrivateKeyToPublicKey(privKey)
	require.NoError(t, err)
	assert.Len(t, pubKey, 65)
	assert.Equal(t, byte(0x04), pubKey[0]) // Uncompressed prefix
}

func TestPrivateKeyToPublicKey_InvalidKey(t *testing.T) {
	t.Parallel()

	_, err := PrivateKeyToPublicKey([]byte{1, 2, 3}) // Too short
	assert.Error(t, err)
}

func TestDeriveAddress(t *testing.T) {
	t.Parallel()

	// Test vector: known private key and its corresponding address
	// Private key (32 bytes)
	privKeyHex := "4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"
	privKey, err := hex.DecodeString(privKeyHex)
	require.NoError(t, err)

	addr, err := DeriveAddress(privKey)
	require.NoError(t, err)
	assert.Len(t, addr, 20)

	// The expected address for this private key is 0x2c7536E3605D9C16a7a3D7b1898e529396a65c23
	expectedAddr := "2c7536e3605d9c16a7a3d7b1898e529396a65c23"
	assert.Equal(t, expectedAddr, hex.EncodeToString(addr))
}

func TestSign(t *testing.T) {
	t.Parallel()

	privKeyHex := "4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"
	privKey, err := hex.DecodeString(privKeyHex)
	require.NoError(t, err)

	// Hash of some data
	hash := Keccak256([]byte("hello"))

	sig, err := Sign(hash, privKey)
	require.NoError(t, err)
	assert.Len(t, sig, 65)

	// V should be 0 or 1
	assert.True(t, sig[64] == 0 || sig[64] == 1)
}

func TestSign_InvalidHash(t *testing.T) {
	t.Parallel()

	privKey := make([]byte, 32)
	_, err := Sign([]byte{1, 2, 3}, privKey) // Wrong hash length
	assert.Error(t, err)
}

func TestSign_InvalidKey(t *testing.T) {
	t.Parallel()

	hash := make([]byte, 32)
	_, err := Sign(hash, []byte{1, 2, 3}) // Wrong key length
	assert.Error(t, err)
}

func TestHexToAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid with prefix", "0x742d35Cc6634C0532925a3b844Bc454e4438f44e", false},
		{"valid without prefix", "742d35Cc6634C0532925a3b844Bc454e4438f44e", false},
		{"too short", "0x742d35Cc", true},
		{"invalid hex", "0xGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := HexToAddress(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAddressHex(t *testing.T) {
	t.Parallel()

	addr, err := HexToAddress("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	require.NoError(t, err)

	// Hex() returns lowercase with 0x prefix
	assert.Equal(t, "0x742d35cc6634c0532925a3b844bc454e4438f44e", addr.Hex())
}

func TestToChecksumAddress(t *testing.T) {
	t.Parallel()

	// EIP-55 test vectors
	tests := []struct {
		input    string
		expected string
	}{
		{"0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed", "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"},
		{"0xfb6916095ca1df60bb79ce92ce3ea74c37c5d359", "0xfB6916095ca1df60bB79Ce92cE3Ea74c37c5d359"},
		{"0xdbf03b407c01e7cd3cbea99509d93f8dddc8c6fb", "0xdbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB"},
		{"0xd1220a0cf47c7b9be7a2e6ba89f429762e7b9adb", "0xD1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			result := ToChecksumAddress(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestLeftPadBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		length   int
		expected []byte
	}{
		{"pad short", []byte{1, 2}, 4, []byte{0, 0, 1, 2}},
		{"no pad needed", []byte{1, 2, 3, 4}, 4, []byte{1, 2, 3, 4}},
		{"longer than target", []byte{1, 2, 3, 4, 5}, 4, []byte{1, 2, 3, 4, 5}},
		{"empty input", []byte{}, 4, []byte{0, 0, 0, 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := LeftPadBytes(tc.input, tc.length)
			assert.Equal(t, tc.expected, result)
		})
	}
}
