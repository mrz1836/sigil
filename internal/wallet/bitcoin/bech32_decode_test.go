package bitcoin

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type segwitVector struct {
	name    string
	hrp     string
	program string // hex
	addr    string
}

// knownSegwitVectors are (hrp, program) pairs with their canonical BIP173 v0
// addresses. Reused from bech32_test.go's encode vectors so decode round-trips
// the exact same data.
func knownSegwitVectors() []segwitVector {
	return []segwitVector{
		{
			name:    "BTC P2WPKH mainnet",
			hrp:     "bc",
			program: "751e76e8199196d454941c45d1b3a323f1433bd6",
			addr:    "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		},
		{
			name:    "BTC P2WSH mainnet",
			hrp:     "bc",
			program: "1863143c14c5166804bd19203356da136c985678cd4d27a1b8c6329604903262",
			addr:    "bc1qrp33g0q5c5txsp9arysrx4k6zdkfs4nce4xj0gdcccefvpysxf3qccfmv3",
		},
		{
			name:    "BTC P2WPKH testnet",
			hrp:     "tb",
			program: "751e76e8199196d454941c45d1b3a323f1433bd6",
			addr:    "tb1qw508d6qejxtdg4y5r3zarvary0c5xw7kxpjzsx",
		},
	}
}

func TestSegwitDecode_KnownVectors(t *testing.T) {
	t.Parallel()

	for _, tt := range knownSegwitVectors() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			version, program, err := SegwitDecode(tt.hrp, tt.addr)
			require.NoError(t, err)
			assert.Equal(t, byte(0), version, "witness version must be 0")
			assert.Equal(t, tt.program, hex.EncodeToString(program))
		})
	}
}

func TestSegwitDecode_UppercaseRoundTrips(t *testing.T) {
	t.Parallel()

	// BIP173: an all-uppercase address is valid and equivalent to its lowercase form.
	version, program, err := SegwitDecode("bc", strings.ToUpper(knownSegwitVectors()[0].addr))
	require.NoError(t, err)
	assert.Equal(t, byte(0), version)
	assert.Equal(t, knownSegwitVectors()[0].program, hex.EncodeToString(program))
}

func TestSegwitDecode_RoundTrip(t *testing.T) {
	t.Parallel()

	for _, tt := range knownSegwitVectors() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			program, err := hex.DecodeString(tt.program)
			require.NoError(t, err)

			encoded, err := SegwitEncode(tt.hrp, 0, program)
			require.NoError(t, err)
			assert.Equal(t, tt.addr, encoded)

			gotVer, gotProg, err := SegwitDecode(tt.hrp, encoded)
			require.NoError(t, err)
			assert.Equal(t, byte(0), gotVer)
			assert.Equal(t, program, gotProg)
		})
	}
}

func TestBech32Decode_Rejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "mixed case",
			input:   "bc1Qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
			wantErr: ErrBech32MixedCase,
		},
		{
			name:    "too long",
			input:   "bc1" + strings.Repeat("q", 90),
			wantErr: ErrBech32TooLong,
		},
		{
			name:    "no separator",
			input:   "bcqqqqqqqqqqqqqq",
			wantErr: ErrBech32NoSeparator,
		},
		{
			name:    "empty hrp",
			input:   "1qqqqqqqqq",
			wantErr: ErrBech32NoSeparator,
		},
		{
			name:    "invalid data char (b not in charset)",
			input:   "bc1bw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
			wantErr: ErrBech32BadChar,
		},
		{
			name:    "corrupted checksum",
			input:   "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t5",
			wantErr: ErrBech32BadChecksum,
		},
		{
			// BIP350 canonical bech32m string — must fail the BECH32 checksum,
			// proving bech32m (taproot/v1+) is rejected.
			name:    "bech32m constant string",
			input:   "A1LQFN3A",
			wantErr: ErrBech32BadChecksum,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := Bech32Decode(tt.input)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestSegwitDecode_Rejects(t *testing.T) {
	t.Parallel()

	// A witness v1 program encoded with a (v0-style) BECH32 checksum: decodes as
	// bech32 but must be rejected by the version check.
	v1Program, err := hex.DecodeString("751e76e8199196d454941c45d1b3a323f1433bd6")
	require.NoError(t, err)
	v1Addr, err := SegwitEncode("bc", 1, v1Program)
	require.NoError(t, err)

	tests := []struct {
		name    string
		hrp     string
		addr    string
		wantErr error
	}{
		{
			name:    "wrong hrp (mainnet addr on testnet)",
			hrp:     "tb",
			addr:    knownSegwitVectors()[0].addr,
			wantErr: ErrSegwitHRPMismatch,
		},
		{
			name:    "witness version 1 rejected",
			hrp:     "bc",
			addr:    v1Addr,
			wantErr: ErrSegwitVersion,
		},
		{
			name:    "bech32m taproot string rejected at checksum",
			hrp:     "bc",
			addr:    "A1LQFN3A",
			wantErr: ErrBech32BadChecksum,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, decErr := SegwitDecode(tt.hrp, tt.addr)
			require.Error(t, decErr)
			assert.ErrorIs(t, decErr, tt.wantErr)
		})
	}
}

// TestSegwitDecode_ProgramLengthAndVersionEdges crafts otherwise-valid BECH32
// strings that violate the v0 witness-program rules, exercising the branches
// SegwitEncode's own length checks would normally prevent.
func TestSegwitDecode_ProgramLengthAndVersionEdges(t *testing.T) {
	t.Parallel()

	// Helper: encode a raw (version || program-as-5bit) payload as BECH32.
	encodeRaw := func(t *testing.T, version byte, program []byte) string {
		t.Helper()
		conv, err := ConvertBits(program, 8, 5, true)
		require.NoError(t, err)
		data := append([]byte{version}, conv...)
		addr, err := Bech32Encode("bc", data)
		require.NoError(t, err)
		return addr
	}

	t.Run("v0 with 16-byte program rejected", func(t *testing.T) {
		t.Parallel()
		addr := encodeRaw(t, 0, make([]byte, 16))
		_, _, err := SegwitDecode("bc", addr)
		assert.ErrorIs(t, err, ErrSegwitProgramLen)
	})

	t.Run("v0 with empty program rejected", func(t *testing.T) {
		t.Parallel()
		addr := encodeRaw(t, 0, nil)
		_, _, err := SegwitDecode("bc", addr)
		assert.ErrorIs(t, err, ErrSegwitProgramLen)
	})

	t.Run("bech32 with no data (checksum only) rejected", func(t *testing.T) {
		t.Parallel()
		addr, err := Bech32Encode("bc", nil) // "bc1" + 6-symbol checksum, empty data
		require.NoError(t, err)
		_, _, decErr := SegwitDecode("bc", addr)
		assert.ErrorIs(t, decErr, ErrSegwitVersion)
	})

	t.Run("v0 32-byte program accepted", func(t *testing.T) {
		t.Parallel()
		addr := encodeRaw(t, 0, make([]byte, 32))
		version, program, err := SegwitDecode("bc", addr)
		require.NoError(t, err)
		assert.Equal(t, byte(0), version)
		assert.Len(t, program, 32)
	})
}

func FuzzSegwitDecode(f *testing.F) {
	for _, tt := range knownSegwitVectors() {
		f.Add(tt.hrp, tt.addr)
	}
	f.Add("bc", "A1LQFN3A")
	f.Add("bc", "not-an-address")
	f.Add("tb", "")

	f.Fuzz(func(t *testing.T, hrp, addr string) {
		// Must never panic regardless of input.
		version, program, err := SegwitDecode(hrp, addr)
		if err != nil {
			return
		}

		// On success the decode must be self-consistent: v0, canonical program
		// length, and re-encoding reproduces the (lowercased) input.
		require.Equal(t, byte(0), version)
		require.True(t, len(program) == 20 || len(program) == 32)

		reencoded, encErr := SegwitEncode(hrp, version, program)
		require.NoError(t, encErr)
		assert.Equal(t, strings.ToLower(addr), reencoded)
	})
}
