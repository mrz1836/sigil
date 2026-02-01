package bsv

import (
	"testing"
)

// FuzzIsValidAddress tests that BSV address validation never panics.
func FuzzIsValidAddress(f *testing.F) {
	// Valid P2PKH addresses (start with 1)
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa") // Genesis block address
	f.Add("1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2")

	// Valid P2SH addresses (start with 3)
	f.Add("3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy")

	// Invalid addresses
	f.Add("")
	f.Add("0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0") // ETH address
	f.Add("1")                                          // Too short
	f.Add("1AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")        // Invalid checksum
	f.Add("1111111111111111111111111111111111")         // All 1s
	f.Add("OIl0")                                       // Invalid base58 chars (O, I, l, 0)
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := IsValidAddress(input)

		// If valid, verify it starts with 1 or 3
		if result && len(input) > 0 {
			first := input[0]
			if first != '1' && first != '3' {
				t.Errorf("IsValidAddress returned true for address not starting with 1 or 3: %q", input)
			}
		}
	})
}

// FuzzValidateBase58CheckAddress tests full address validation.
func FuzzValidateBase58CheckAddress(f *testing.F) {
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
	f.Add("3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy")
	f.Add("")
	f.Add("invalid")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(_ *testing.T, input string) {
		// Should not panic - that's the main test
		_ = ValidateBase58CheckAddress(input)
	})
}

// FuzzDecodeBase58Check tests base58check decoding.
func FuzzDecodeBase58Check(f *testing.F) {
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
	f.Add("3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy")
	f.Add("")
	f.Add("1")
	f.Add("111111111111111111111111111111111")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(_ *testing.T, input string) {
		// Should not panic - that's the main test
		_, _, _ = DecodeBase58Check(input)
	})
}

// FuzzBase58Decode tests raw base58 decoding.
func FuzzBase58Decode(f *testing.F) {
	f.Add("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
	f.Add("")
	f.Add("1")
	f.Add("1111")
	f.Add("abc")
	f.Add("OIl0") // Invalid chars

	f.Fuzz(func(_ *testing.T, input string) {
		// Should not panic - that's the main test
		_, _ = base58Decode(input)
	})
}

// FuzzBase58Encode tests base58 encoding.
func FuzzBase58Encode(f *testing.F) {
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})
	f.Add([]byte{0x00, 0x00, 0x00, 0x01})
	f.Add([]byte{0x01}) // Non-empty minimal input
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})

	f.Fuzz(func(t *testing.T, input []byte) {
		// Skip empty input - base58Encode returns empty string which base58Decode rejects
		if len(input) == 0 {
			return
		}

		// Should not panic
		encoded := base58Encode(input)

		// Skip if encoded is empty (happens for all-zero input with no content)
		if encoded == "" {
			return
		}

		// All characters should be valid base58
		validateBase58Chars(t, encoded, input)

		// Decode should succeed for non-empty encoded values
		_, err := base58Decode(encoded)
		if err != nil {
			t.Errorf("base58Decode failed for encoded value %q (input %v): %v", encoded, input, err)
		}
	})
}

func validateBase58Chars(t *testing.T, encoded string, input []byte) {
	t.Helper()
	for _, c := range encoded {
		if _, ok := base58AlphabetMap[c]; !ok {
			t.Errorf("base58Encode returned invalid character '%c' for input %v", c, input)
		}
	}
}
