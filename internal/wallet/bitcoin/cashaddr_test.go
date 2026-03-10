package bitcoin

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCashAddrEncode_P2PKH_KnownVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		hash     string // hex-encoded 20-byte hash
		addrType byte
		wantFull string // full cashaddr with prefix
	}{
		{
			name:     "spec vector - 20-byte P2PKH hash160 of generator point",
			hash:     "751e76e8199196d454941c45d1b3a323f1433bd6",
			addrType: CashAddrTypeP2PKH,
			wantFull: "bitcoincash:qp63uahgrxged4z5jswyt5dn5v3lzsem6cy4spdc2h",
		},
		{
			name:     "zero hash P2PKH",
			hash:     "0000000000000000000000000000000000000000",
			addrType: CashAddrTypeP2PKH,
			wantFull: "bitcoincash:qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqfnhks603",
		},
		{
			name:     "all-ones hash P2PKH",
			hash:     "ffffffffffffffffffffffffffffffffffffffff",
			addrType: CashAddrTypeP2PKH,
			wantFull: "bitcoincash:qrlllllllllllllllllllllllllllllllu5y7pl6pz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			hash, err := hex.DecodeString(tt.hash)
			require.NoError(t, err)

			got, err := CashAddrEncode("bitcoincash", tt.addrType, hash)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFull, got)
		})
	}
}

func TestCashAddrEncodeShort_P2PKH(t *testing.T) {
	t.Parallel()

	hash, _ := hex.DecodeString("751e76e8199196d454941c45d1b3a323f1433bd6")
	got, err := CashAddrEncodeShort("bitcoincash", CashAddrTypeP2PKH, hash)
	require.NoError(t, err)

	// Should start with q (P2PKH type byte leads to 'q' prefix)
	assert.Equal(t, byte('q'), got[0], "P2PKH cashaddr should start with 'q'")
	// Should not contain the prefix
	assert.NotContains(t, got, "bitcoincash:")
}

func TestCashAddrEncodeShort_P2SH(t *testing.T) {
	t.Parallel()

	hash, _ := hex.DecodeString("751e76e8199196d454941c45d1b3a323f1433bd6")
	got, err := CashAddrEncodeShort("bitcoincash", CashAddrTypeP2SH, hash)
	require.NoError(t, err)

	// P2SH type byte leads to 'p' prefix
	assert.Equal(t, byte('p'), got[0], "P2SH cashaddr should start with 'p'")
}

func TestCashAddrEncode_InvalidHashLength(t *testing.T) {
	t.Parallel()
	// 15 bytes is not a valid hash length
	_, err := CashAddrEncode("bitcoincash", CashAddrTypeP2PKH, make([]byte, 15))
	assert.Error(t, err)
}

func TestCashAddrEncode_DeterministicOutput(t *testing.T) {
	t.Parallel()

	hash, _ := hex.DecodeString("751e76e8199196d454941c45d1b3a323f1433bd6")

	// Encode twice and ensure same result
	addr1, err := CashAddrEncode("bitcoincash", CashAddrTypeP2PKH, hash)
	require.NoError(t, err)
	addr2, err := CashAddrEncode("bitcoincash", CashAddrTypeP2PKH, hash)
	require.NoError(t, err)
	assert.Equal(t, addr1, addr2)
}

func TestCashAddrSizeBits(t *testing.T) {
	t.Parallel()

	validLens := map[int]byte{20: 0, 24: 1, 28: 2, 32: 3, 40: 4, 48: 5, 56: 6, 64: 7}
	for hashLen, want := range validLens {
		got, err := cashAddrSizeBits(hashLen)
		require.NoError(t, err, "hashLen=%d", hashLen)
		assert.Equal(t, want, got, "hashLen=%d", hashLen)
	}

	// Invalid lengths
	for _, bad := range []int{0, 1, 10, 19, 21, 33, 100} {
		_, err := cashAddrSizeBits(bad)
		assert.Error(t, err, "hashLen=%d should be invalid", bad)
	}
}
