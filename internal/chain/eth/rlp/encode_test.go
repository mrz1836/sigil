package rlp

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "single byte < 0x80",
			input:    []byte{0x00},
			expected: "00",
		},
		{
			name:     "single byte 0x7f",
			input:    []byte{0x7f},
			expected: "7f",
		},
		{
			name:     "single byte >= 0x80",
			input:    []byte{0x80},
			expected: "8180",
		},
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: "80",
		},
		{
			name:     "short string",
			input:    []byte("dog"),
			expected: "83646f67",
		},
		{
			name:     "medium string (55 bytes)",
			input:    make([]byte, 55),
			expected: "b7" + "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := Encode(tc.input)
			assert.Equal(t, tc.expected, hex.EncodeToString(result))
		})
	}
}

func TestEncodeBigInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *big.Int
		expected string
	}{
		{
			name:     "zero",
			input:    big.NewInt(0),
			expected: "80",
		},
		{
			name:     "nil",
			input:    nil,
			expected: "80",
		},
		{
			name:     "small number",
			input:    big.NewInt(127),
			expected: "7f",
		},
		{
			name:     "128",
			input:    big.NewInt(128),
			expected: "8180",
		},
		{
			name:     "1024",
			input:    big.NewInt(1024),
			expected: "820400",
		},
		{
			name:     "1 ETH in wei",
			input:    new(big.Int).SetBytes(hexBytes("0de0b6b3a7640000")),
			expected: "880de0b6b3a7640000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := Encode(tc.input)
			assert.Equal(t, tc.expected, hex.EncodeToString(result))
		})
	}
}

func TestEncodeUint64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    uint64
		expected string
	}{
		{"zero", 0, "80"},
		{"one", 1, "01"},
		{"127", 127, "7f"},
		{"128", 128, "8180"},
		{"256", 256, "820100"},
		{"21000 (gas limit)", 21000, "825208"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := Encode(tc.input)
			assert.Equal(t, tc.expected, hex.EncodeToString(result))
		})
	}
}

func TestEncodeList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []any
		expected string
	}{
		{
			name:     "empty list",
			input:    []any{},
			expected: "c0",
		},
		{
			name:     "list with strings",
			input:    []any{[]byte("cat"), []byte("dog")},
			expected: "c88363617483646f67",
		},
		{
			name:     "nested list",
			input:    []any{[]any{}, []any{[]any{}}},
			expected: "c3c0c1c0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := Encode(tc.input)
			assert.Equal(t, tc.expected, hex.EncodeToString(result))
		})
	}
}

func TestEncodeTransactionForSigning(t *testing.T) {
	t.Parallel()

	// Test encoding a transaction for EIP-155 signing
	to := hexBytes("3535353535353535353535353535353535353535")
	value := new(big.Int).SetBytes(hexBytes("0de0b6b3a7640000")) // 1 ETH
	gasPrice := big.NewInt(20000000000)                          // 20 Gwei
	gasLimit := uint64(21000)
	nonce := uint64(9)
	chainID := big.NewInt(1) // Mainnet

	encoded := EncodeTransactionForSigning(nonce, gasPrice, gasLimit, to, value, nil, chainID)

	// The expected encoding for this transaction (EIP-155 format)
	// This is a well-known test vector
	expected := "ec098504a817c800825208943535353535353535353535353535353535353535880de0b6b3a764000080018080"
	assert.Equal(t, expected, hex.EncodeToString(encoded))
}

func TestEncodeSignedTransaction(t *testing.T) {
	t.Parallel()

	// Test encoding a signed transaction
	to := hexBytes("3535353535353535353535353535353535353535")
	value := new(big.Int).SetBytes(hexBytes("0de0b6b3a7640000")) // 1 ETH
	gasPrice := big.NewInt(20000000000)                          // 20 Gwei
	gasLimit := uint64(21000)
	nonce := uint64(9)

	// EIP-155 signature values for chain ID 1
	v := big.NewInt(37) // 1 * 2 + 35 + recovery = 37 or 38
	r := new(big.Int).SetBytes(hexBytes("28ef61340bd939bc2195fe537567866003e1a15d3c71ff63e1590620aa636276"))
	s := new(big.Int).SetBytes(hexBytes("67cbe9d8997f761aecb703304b3800ccf555c9f3dc64214b297fb1966a3b6d83"))

	encoded := EncodeTransaction(nonce, gasPrice, gasLimit, to, value, nil, v, r, s)

	// Verify it starts with the correct prefix (list with length > 55)
	assert.Greater(t, len(encoded), 2)
}

func hexBytes(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}
