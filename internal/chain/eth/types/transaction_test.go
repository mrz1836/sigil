package ethtypes

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test vectors
var (
	//nolint:gochecknoglobals // Test vectors
	testPrivateKey = mustDecodeHex("4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318")
	//nolint:gochecknoglobals // Test vectors
	testToAddress = mustDecodeHex("742d35Cc6634C0532925a3b844Bc454e4438f44e")
)

func mustDecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func TestNewLegacyTx(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nonce    uint64
		to       []byte
		value    *big.Int
		gasLimit uint64
		gasPrice *big.Int
		data     []byte
	}{
		{
			name:     "simple transfer",
			nonce:    0,
			to:       testToAddress,
			value:    big.NewInt(1000000000000000000), // 1 ETH in wei
			gasLimit: 21000,
			gasPrice: big.NewInt(20000000000), // 20 gwei
			data:     nil,
		},
		{
			name:     "contract creation (nil to)",
			nonce:    5,
			to:       nil,
			value:    big.NewInt(0),
			gasLimit: 100000,
			gasPrice: big.NewInt(30000000000),  // 30 gwei
			data:     []byte{0x60, 0x60, 0x60}, // Some bytecode
		},
		{
			name:     "zero values",
			nonce:    0,
			to:       testToAddress,
			value:    big.NewInt(0),
			gasLimit: 21000,
			gasPrice: big.NewInt(0),
			data:     nil,
		},
		{
			name:     "max uint64 nonce",
			nonce:    ^uint64(0), // Max uint64
			to:       testToAddress,
			value:    big.NewInt(1),
			gasLimit: 21000,
			gasPrice: big.NewInt(1),
			data:     nil,
		},
		{
			name:     "large data",
			nonce:    10,
			to:       testToAddress,
			value:    big.NewInt(0),
			gasLimit: 500000,
			gasPrice: big.NewInt(50000000000), // 50 gwei
			data:     make([]byte, 1024),      // 1KB of data
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tx := NewLegacyTx(tc.nonce, tc.to, tc.value, tc.gasLimit, tc.gasPrice, tc.data)
			require.NotNil(t, tx)

			assert.Equal(t, tc.nonce, tx.Nonce)
			assert.Equal(t, tc.to, tx.To)
			assert.Equal(t, tc.value, tx.Value)
			assert.Equal(t, tc.gasLimit, tx.GasLimit)
			assert.Equal(t, tc.gasPrice, tx.GasPrice)
			assert.Equal(t, tc.data, tx.Data)

			// Signature should be nil for new transaction
			assert.Nil(t, tx.V)
			assert.Nil(t, tx.R)
			assert.Nil(t, tx.S)
			assert.False(t, tx.IsSigned())
		})
	}
}

func TestSigningHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tx      *LegacyTx
		chainID *big.Int
	}{
		{
			name: "mainnet (chain ID 1)",
			tx: NewLegacyTx(
				0,
				testToAddress,
				big.NewInt(1000000000000000000),
				21000,
				big.NewInt(20000000000),
				nil,
			),
			chainID: big.NewInt(1),
		},
		{
			name: "goerli (chain ID 5)",
			tx: NewLegacyTx(
				0,
				testToAddress,
				big.NewInt(1000000000000000000),
				21000,
				big.NewInt(20000000000),
				nil,
			),
			chainID: big.NewInt(5),
		},
		{
			name: "polygon (chain ID 137)",
			tx: NewLegacyTx(
				0,
				testToAddress,
				big.NewInt(1000000000000000000),
				21000,
				big.NewInt(20000000000),
				nil,
			),
			chainID: big.NewInt(137),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hash1 := tc.tx.SigningHash(tc.chainID)
			require.Len(t, hash1, 32, "hash should be 32 bytes")

			// Hash should be deterministic
			hash2 := tc.tx.SigningHash(tc.chainID)
			assert.Equal(t, hash1, hash2, "signing hash should be deterministic")
		})
	}

	t.Run("different chain IDs produce different hashes", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		hash1 := tx.SigningHash(big.NewInt(1))
		hash5 := tx.SigningHash(big.NewInt(5))

		assert.NotEqual(t, hash1, hash5, "different chain IDs should produce different hashes")
	})
}

func TestSign(t *testing.T) {
	t.Parallel()

	t.Run("successful signing mainnet", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(1)
		err := tx.Sign(testPrivateKey, chainID)
		require.NoError(t, err)

		// Verify signature components are set
		require.NotNil(t, tx.V)
		require.NotNil(t, tx.R)
		require.NotNil(t, tx.S)
		assert.True(t, tx.IsSigned())

		// Verify EIP-155 V calculation: v = recovery_id + chainID * 2 + 35
		// For chainID 1: v should be 37 or 38 (35 + 1*2 + recovery_id)
		vInt := tx.V.Int64()
		assert.True(t, vInt == 37 || vInt == 38, "EIP-155 V should be 37 or 38 for mainnet, got %d", vInt)

		// R and S should be non-zero
		assert.Positive(t, tx.R.Sign(), "R should be positive")
		assert.Positive(t, tx.S.Sign(), "S should be positive")
	})

	t.Run("successful signing goerli", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(5)
		err := tx.Sign(testPrivateKey, chainID)
		require.NoError(t, err)

		// For chainID 5: v should be 45 or 46 (35 + 5*2 + recovery_id)
		vInt := tx.V.Int64()
		assert.True(t, vInt == 45 || vInt == 46, "EIP-155 V should be 45 or 46 for goerli, got %d", vInt)
	})

	t.Run("successful signing polygon", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(137)
		err := tx.Sign(testPrivateKey, chainID)
		require.NoError(t, err)

		// For chainID 137: v should be 309 or 310 (35 + 137*2 + recovery_id)
		vInt := tx.V.Int64()
		assert.True(t, vInt == 309 || vInt == 310, "EIP-155 V should be 309 or 310 for polygon, got %d", vInt)
	})

	t.Run("invalid private key length", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		invalidKey := []byte{1, 2, 3} // Too short
		err := tx.Sign(invalidKey, big.NewInt(1))
		require.Error(t, err)

		// Transaction should remain unsigned
		assert.False(t, tx.IsSigned())
	})

	t.Run("deterministic signatures", func(t *testing.T) {
		t.Parallel()

		tx1 := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		tx2 := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(1)
		err1 := tx1.Sign(testPrivateKey, chainID)
		require.NoError(t, err1)

		err2 := tx2.Sign(testPrivateKey, chainID)
		require.NoError(t, err2)

		// Signatures should be identical for identical transactions
		assert.Equal(t, tx1.V, tx2.V)
		assert.Equal(t, tx1.R, tx2.R)
		assert.Equal(t, tx1.S, tx2.S)
	})
}

func TestRawBytes(t *testing.T) {
	t.Parallel()

	t.Run("unsigned transaction", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		raw := tx.RawBytes()
		assert.NotEmpty(t, raw, "raw bytes should not be empty")

		// RLP encoding should start with 0xc0-0xff (list prefix)
		assert.GreaterOrEqual(t, raw[0], byte(0xc0), "should be RLP list")
	})

	t.Run("signed transaction", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(1)
		err := tx.Sign(testPrivateKey, chainID)
		require.NoError(t, err)

		raw := tx.RawBytes()
		assert.NotEmpty(t, raw, "raw bytes should not be empty")

		// Signed transaction should be longer than unsigned
		unsignedTx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)
		unsignedRaw := unsignedTx.RawBytes()
		assert.Greater(t, len(raw), len(unsignedRaw), "signed tx should be longer")
	})

	t.Run("deterministic encoding", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(1)
		err := tx.Sign(testPrivateKey, chainID)
		require.NoError(t, err)

		raw1 := tx.RawBytes()
		raw2 := tx.RawBytes()

		assert.Equal(t, raw1, raw2, "RawBytes should be deterministic")
	})
}

func TestHash(t *testing.T) {
	t.Parallel()

	t.Run("hash is 32 bytes", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(1)
		err := tx.Sign(testPrivateKey, chainID)
		require.NoError(t, err)

		hash := tx.Hash()
		assert.Len(t, hash, 32, "hash should be 32 bytes")
	})

	t.Run("hash is deterministic", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(1)
		err := tx.Sign(testPrivateKey, chainID)
		require.NoError(t, err)

		hash1 := tx.Hash()
		hash2 := tx.Hash()

		assert.Equal(t, hash1, hash2, "hash should be deterministic")
	})

	t.Run("different transactions have different hashes", func(t *testing.T) {
		t.Parallel()

		tx1 := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		tx2 := NewLegacyTx(
			1, // Different nonce
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(1)
		err1 := tx1.Sign(testPrivateKey, chainID)
		require.NoError(t, err1)

		err2 := tx2.Sign(testPrivateKey, chainID)
		require.NoError(t, err2)

		hash1 := tx1.Hash()
		hash2 := tx2.Hash()

		assert.NotEqual(t, hash1, hash2, "different transactions should have different hashes")
	})
}

func TestHashHex(t *testing.T) {
	t.Parallel()

	t.Run("has 0x prefix", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(1)
		err := tx.Sign(testPrivateKey, chainID)
		require.NoError(t, err)

		hashHex := tx.HashHex()
		assert.Greater(t, len(hashHex), 2, "hash hex should not be empty")
		assert.Equal(t, "0x", hashHex[:2], "hash hex should have 0x prefix")
	})

	t.Run("matches Hash()", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(1)
		err := tx.Sign(testPrivateKey, chainID)
		require.NoError(t, err)

		hash := tx.Hash()
		hashHex := tx.HashHex()

		expectedHex := "0x" + hex.EncodeToString(hash)
		assert.Equal(t, expectedHex, hashHex, "HashHex should match Hash()")
	})

	t.Run("is lowercase hex", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(1)
		err := tx.Sign(testPrivateKey, chainID)
		require.NoError(t, err)

		hashHex := tx.HashHex()

		// Should be 66 characters: "0x" + 64 hex chars (32 bytes)
		assert.Len(t, hashHex, 66, "hash hex should be 66 characters")

		// Verify it's valid hex
		_, err = hex.DecodeString(hashHex[2:])
		assert.NoError(t, err, "hash hex should be valid hex")
	})
}

func TestIsSigned(t *testing.T) {
	t.Parallel()

	t.Run("unsigned transaction", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		assert.False(t, tx.IsSigned(), "new transaction should not be signed")
	})

	t.Run("signed transaction", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		chainID := big.NewInt(1)
		err := tx.Sign(testPrivateKey, chainID)
		require.NoError(t, err)

		assert.True(t, tx.IsSigned(), "signed transaction should be signed")
	})

	t.Run("partially signed transaction", func(t *testing.T) {
		t.Parallel()

		tx := NewLegacyTx(
			0,
			testToAddress,
			big.NewInt(1000000000000000000),
			21000,
			big.NewInt(20000000000),
			nil,
		)

		// Manually set only V
		tx.V = big.NewInt(37)
		assert.False(t, tx.IsSigned(), "transaction with only V should not be signed")

		// Set R
		tx.R = big.NewInt(1)
		assert.False(t, tx.IsSigned(), "transaction with only V and R should not be signed")

		// Set S - now fully signed
		tx.S = big.NewInt(1)
		assert.True(t, tx.IsSigned(), "transaction with V, R, and S should be signed")
	})
}
