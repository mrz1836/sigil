package bitcoin

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHash160(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string // hex encoded input
		expected string // hex encoded output
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "b472a266d0bd89c13706a4132ccfb16f7c3b9fcb",
		},
		{
			name:     "Bitcoin public key example",
			input:    "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
			expected: "751e76e8199196d454941c45d1b3a323f1433bd6",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input, err := hex.DecodeString(tc.input)
			if err != nil {
				t.Fatal(err)
			}

			result := Hash160(input)
			resultHex := hex.EncodeToString(result)

			assert.Equal(t, tc.expected, resultHex)
			assert.Len(t, result, 20) // RIPEMD160 produces 20 bytes
		})
	}
}

func TestHash160_Consistency(t *testing.T) {
	t.Parallel()

	// Verify that Hash160 always produces the same output for the same input
	input := []byte("test data")

	result1 := Hash160(input)
	result2 := Hash160(input)

	assert.Equal(t, result1, result2)
}

func TestDoubleSHA256(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string // hex encoded input
		expected string // hex encoded output
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "5df6e0e2761359d30a8275058e299fcc0381534545f55cf43e41983f5d4c9456",
		},
		{
			name: "Bitcoin block header (block 0 hash)",
			// This is the genesis block header
			input:    "0100000000000000000000000000000000000000000000000000000000000000000000003ba3edfd7a7b12b27ac72c3e67768f617fc81bc3888a51323a9fb8aa4b1e5e4a29ab5f49ffff001d1dac2b7c",
			expected: "6fe28c0ab6f1b372c1a6a246ae63f74f931e8365e15a089c68d6190000000000",
		},
		{
			name:     "simple string",
			input:    "68656c6c6f", // "hello" in hex
			expected: "9595c9df90075148eb06860365df33584b75bff782a510c6cd4883a419833d50",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input, err := hex.DecodeString(tc.input)
			if err != nil {
				t.Fatal(err)
			}

			result := DoubleSHA256(input)
			resultHex := hex.EncodeToString(result)

			assert.Equal(t, tc.expected, resultHex)
			assert.Len(t, result, 32) // SHA256 produces 32 bytes
		})
	}
}

func TestDoubleSHA256_Consistency(t *testing.T) {
	t.Parallel()

	input := []byte("test data for consistency")

	result1 := DoubleSHA256(input)
	result2 := DoubleSHA256(input)

	assert.Equal(t, result1, result2)
}

func TestBase58Encode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string // hex encoded input
		expected string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "single zero byte",
			input:    "00",
			expected: "1",
		},
		{
			name:     "multiple leading zeros",
			input:    "000000",
			expected: "111",
		},
		{
			name:     "simple value",
			input:    "61", // 'a' = 97
			expected: "2g",
		},
		{
			name:     "Hello World",
			input:    "48656c6c6f20576f726c6421",
			expected: "2NEpo7TZRRrLZSi2U",
		},
		{
			name:     "leading zeros with trailing data",
			input:    "000000287fb4cd",
			expected: "111233QC4",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input, err := hex.DecodeString(tc.input)
			if err != nil {
				t.Fatal(err)
			}

			result := Base58Encode(input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBase58Decode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    string // hex encoded expected output
		expectError bool
	}{
		{
			name:        "empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid character 0",
			input:       "0abc",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid character O",
			input:       "abcO",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid character I",
			input:       "abIc",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid character l",
			input:       "ablc",
			expected:    "",
			expectError: true,
		},
		{
			name:        "single 1 (leading zero)",
			input:       "1",
			expected:    "00",
			expectError: false,
		},
		{
			name:        "multiple leading 1s",
			input:       "111",
			expected:    "000000",
			expectError: false,
		},
		{
			name:        "simple value",
			input:       "2g",
			expected:    "61",
			expectError: false,
		},
		{
			name:        "Hello World",
			input:       "2NEpo7TZRRrLZSi2U",
			expected:    "48656c6c6f20576f726c6421",
			expectError: false,
		},
		{
			name:        "leading zeros with trailing data",
			input:       "111233QC4",
			expected:    "000000287fb4cd",
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := Base58Decode(tc.input)

			if tc.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidBase58)
			} else {
				require.NoError(t, err)
				resultHex := hex.EncodeToString(result)
				assert.Equal(t, tc.expected, resultHex)
			}
		})
	}
}

func TestBase58Roundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string // hex encoded
	}{
		{
			name: "single byte",
			data: "ff",
		},
		{
			name: "leading zeros",
			data: "000001020304",
		},
		{
			name: "random data",
			data: "deadbeef",
		},
		{
			name: "Bitcoin pubkey hash",
			data: "00751e76e8199196d454941c45d1b3a323f1433bd6",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			original, err := hex.DecodeString(tc.data)
			if err != nil {
				t.Fatal(err)
			}

			encoded := Base58Encode(original)
			decoded, err := Base58Decode(encoded)

			require.NoError(t, err)
			assert.Equal(t, original, decoded)
		})
	}
}

func TestBase58CheckEncode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		version  byte
		payload  string // hex encoded
		expected string
	}{
		{
			name:     "mainnet P2PKH address",
			version:  0x00,
			payload:  "751e76e8199196d454941c45d1b3a323f1433bd6",
			expected: "1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAMH",
		},
		{
			name:     "testnet P2PKH address",
			version:  0x6f,
			payload:  "751e76e8199196d454941c45d1b3a323f1433bd6",
			expected: "mrCDrCybB6J1vRfbwM5hemdJz73FwDBC8r",
		},
		{
			name:     "empty payload",
			version:  0x00,
			payload:  "",
			expected: "1Wh4bh",
		},
		{
			name:     "WIF private key mainnet",
			version:  0x80,
			payload:  "0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d",
			expected: "5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payload, err := hex.DecodeString(tc.payload)
			if err != nil {
				t.Fatal(err)
			}

			result := Base58CheckEncode(tc.version, payload)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBase58CheckDecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    string // hex encoded (version + payload)
		expectError bool
	}{
		{
			name:        "valid mainnet P2PKH address",
			input:       "1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAMH",
			expected:    "00751e76e8199196d454941c45d1b3a323f1433bd6",
			expectError: false,
		},
		{
			name:        "empty payload address",
			input:       "1Wh4bh",
			expected:    "00",
			expectError: false,
		},
		{
			name:        "WIF private key",
			input:       "5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ",
			expected:    "800c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d",
			expectError: false,
		},
		{
			name:        "empty string",
			input:       "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "too short (less than 4 bytes)",
			input:       "1",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid checksum",
			input:       "1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAMJ", // changed last char
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid Base58 character",
			input:       "1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAM0", // '0' is invalid
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := Base58CheckDecode(tc.input)

			if tc.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidBase58)
			} else {
				require.NoError(t, err)
				resultHex := hex.EncodeToString(result)
				assert.Equal(t, tc.expected, resultHex)
			}
		})
	}
}

func TestBase58CheckRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version byte
		payload string // hex encoded
	}{
		{
			name:    "mainnet P2PKH",
			version: 0x00,
			payload: "751e76e8199196d454941c45d1b3a323f1433bd6",
		},
		{
			name:    "testnet P2PKH",
			version: 0x6f,
			payload: "751e76e8199196d454941c45d1b3a323f1433bd6",
		},
		{
			name:    "WIF private key",
			version: 0x80,
			payload: "0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d",
		},
		{
			name:    "empty payload",
			version: 0x00,
			payload: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payload, err := hex.DecodeString(tc.payload)
			if err != nil {
				t.Fatal(err)
			}

			encoded := Base58CheckEncode(tc.version, payload)
			decoded, err := Base58CheckDecode(encoded)

			require.NoError(t, err)

			// Decoded result is version + payload
			expected := append([]byte{tc.version}, payload...)
			assert.Equal(t, expected, decoded)
		})
	}
}
