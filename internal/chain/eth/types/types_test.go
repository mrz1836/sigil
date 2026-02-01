package ethtypes

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLegacyTx_SigningHash(t *testing.T) {
	t.Parallel()

	// Test vector from EIP-155
	to, _ := hex.DecodeString("3535353535353535353535353535353535353535")
	value := new(big.Int).SetBytes(hexBytes("0de0b6b3a7640000")) // 1 ETH
	gasPrice := big.NewInt(20000000000)                          // 20 Gwei
	gasLimit := uint64(21000)
	nonce := uint64(9)
	chainID := big.NewInt(1)

	tx := NewLegacyTx(nonce, to, value, gasLimit, gasPrice, nil)
	hash := tx.SigningHash(chainID)

	// Expected hash from EIP-155 test vector
	expected := "daf5a779ae972f972197303d7b574746c7ef83eadac0f2791ad23db92e4c8e53"
	assert.Equal(t, expected, hex.EncodeToString(hash))
}

func TestLegacyTx_Sign(t *testing.T) {
	t.Parallel()

	// Test vector from EIP-155
	to, _ := hex.DecodeString("3535353535353535353535353535353535353535")
	value := new(big.Int).SetBytes(hexBytes("0de0b6b3a7640000"))
	gasPrice := big.NewInt(20000000000)
	gasLimit := uint64(21000)
	nonce := uint64(9)
	chainID := big.NewInt(1)

	tx := NewLegacyTx(nonce, to, value, gasLimit, gasPrice, nil)

	// EIP-155 test private key
	privKey := hexBytes("4646464646464646464646464646464646464646464646464646464646464646")

	err := tx.Sign(privKey, chainID)
	require.NoError(t, err)

	// Verify signature values are set
	assert.NotNil(t, tx.V)
	assert.NotNil(t, tx.R)
	assert.NotNil(t, tx.S)

	// V should be 37 or 38 for chain ID 1 (1 * 2 + 35 + recovery_id)
	assert.True(t, tx.V.Cmp(big.NewInt(37)) == 0 || tx.V.Cmp(big.NewInt(38)) == 0)

	// Verify the signed transaction can be serialized
	raw := tx.RawBytes()
	assert.NotEmpty(t, raw)
}

func TestLegacyTx_Hash(t *testing.T) {
	t.Parallel()

	to, _ := hex.DecodeString("3535353535353535353535353535353535353535")
	value := big.NewInt(1000000000000000000)
	gasPrice := big.NewInt(20000000000)
	gasLimit := uint64(21000)
	nonce := uint64(0)
	chainID := big.NewInt(1)

	tx := NewLegacyTx(nonce, to, value, gasLimit, gasPrice, nil)
	privKey := hexBytes("4646464646464646464646464646464646464646464646464646464646464646")

	err := tx.Sign(privKey, chainID)
	require.NoError(t, err)

	hash := tx.Hash()
	assert.Len(t, hash, 32)

	hashHex := tx.HashHex()
	assert.Len(t, hashHex, 66) // 0x + 64 hex chars
	assert.Equal(t, "0x", hashHex[:2])
}

func TestAddress_HexToAddress(t *testing.T) {
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

func TestAddress_String(t *testing.T) {
	t.Parallel()

	// Test EIP-55 checksum
	addr, err := HexToAddress("0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed")
	require.NoError(t, err)

	// String() should return checksummed address
	assert.Equal(t, "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", addr.String())
}

func TestAddress_IsZero(t *testing.T) {
	t.Parallel()

	var zero Address
	assert.True(t, zero.IsZero())

	nonZero, _ := HexToAddress("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	assert.False(t, nonZero.IsZero())
}

func TestMustHexToAddress(t *testing.T) {
	t.Parallel()

	// Valid address should not panic
	addr := MustHexToAddress("0x742d35Cc6634C0532925a3b844Bc454e4438f44e")
	assert.False(t, addr.IsZero())

	// Invalid address should panic
	assert.Panics(t, func() {
		MustHexToAddress("invalid")
	})
}

func hexBytes(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}
