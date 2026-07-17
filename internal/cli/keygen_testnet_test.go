package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/wallet"
)

// TestGenerateOneKeyTestnetWIF verifies that testnet WIF generation produces the
// correct prefixes and that the keys round-trip through ParseWIF.
func TestGenerateOneKeyTestnetWIF(t *testing.T) {
	t.Parallel()

	const testnetWIFVersion = 0xef

	tests := []struct {
		name       string
		format     string
		wantPrefix byte
		wantWIFLen int // 52 compressed, 51 uncompressed
	}{
		{"testnet compressed", "wif", 'c', 52},
		{"testnet uncompressed", "wif-uncompressed", '9', 51},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key, err := generateOneKey(tc.format, testnetWIFVersion)
			require.NoError(t, err)
			require.NotEmpty(t, key)
			assert.Equal(t, tc.wantPrefix, key[0], "unexpected WIF prefix for %s: %q", tc.format, key)
			assert.Len(t, key, tc.wantWIFLen)

			// The generated WIF must decode to a 32-byte private key.
			priv, parseErr := wallet.ParseWIF(key)
			require.NoError(t, parseErr)
			assert.Len(t, priv, 32)
		})
	}
}

// TestGenerateOneKeyMainnetWIFPrefixes documents the mainnet prefixes for contrast.
func TestGenerateOneKeyMainnetWIFPrefixes(t *testing.T) {
	t.Parallel()

	compressed, err := generateOneKey("wif", 0x80)
	require.NoError(t, err)
	assert.Contains(t, []byte{'K', 'L'}, compressed[0])

	uncompressed, err := generateOneKey("wif-uncompressed", 0x80)
	require.NoError(t, err)
	assert.Equal(t, byte('5'), uncompressed[0])
}
