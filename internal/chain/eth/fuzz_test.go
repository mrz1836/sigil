//nolint:gocognit,nestif // Fuzz tests need comprehensive validation with nested conditions
package eth

import (
	"strings"
	"testing"
)

// FuzzIsValidAddress tests that address validation never panics.
func FuzzIsValidAddress(f *testing.F) {
	// Valid address
	f.Add("0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0")

	// Invalid addresses
	f.Add("")                                           // Empty string
	f.Add("0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E")  // Too short
	f.Add("742d35Cc6634C0532925a3b844Bc9e7595f8b2E0")   // Missing prefix
	f.Add("0x742d35Gg6634C0532925a3b844Bc9e7595f8b2E0") // Invalid hex

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := IsValidAddress(input)

		// If valid, verify format
		if result {
			if len(input) != 42 {
				t.Errorf("IsValidAddress returned true for non-42 char input: %q (len %d)", input, len(input))
			}
			if !strings.HasPrefix(input, "0x") {
				t.Errorf("IsValidAddress returned true for input without 0x prefix: %q", input)
			}
		}
	})
}

// FuzzToChecksumAddress tests that checksum conversion never panics.
func FuzzToChecksumAddress(f *testing.F) {
	f.Add("0x742d35cc6634c0532925a3b844bc9e7595f8b2e0")
	f.Add("0x742D35CC6634C0532925A3B844BC9E7595F8B2E0")
	f.Add("0x0000000000000000000000000000000000000000")
	f.Add("")
	f.Add("not an address")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		result := ToChecksumAddress(input)

		if IsValidAddress(input) {
			// For valid input, result should also be valid
			if !IsValidAddress(result) {
				t.Errorf("ToChecksumAddress returned invalid address for valid input %q: %q", input, result)
			}
			if len(result) != 42 {
				t.Errorf("ToChecksumAddress returned wrong length for valid input %q: %q (len %d)", input, result, len(result))
			}
		} else {
			// For invalid input, should return unchanged
			if result != input {
				t.Errorf("ToChecksumAddress modified invalid input %q to %q", input, result)
			}
		}
	})
}

// FuzzValidateChecksumAddress tests that checksum validation never panics.
func FuzzValidateChecksumAddress(f *testing.F) {
	// Valid checksummed
	f.Add("0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0")
	// Valid all lowercase
	f.Add("0x742d35cc6634c0532925a3b844bc9e7595f8b2e0")
	// Valid all uppercase
	f.Add("0x742D35CC6634C0532925A3B844BC9E7595F8B2E0")
	// Invalid checksum (mixed case, wrong checksum)
	f.Add("0x742d35cC6634C0532925a3b844Bc9e7595f8b2E0")
	// Invalid format
	f.Add("")
	f.Add("0x")
	f.Add("invalid")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		err := ValidateChecksumAddress(input)

		// If input is not a valid address format, should return error
		if !IsValidAddress(input) && err == nil {
			t.Errorf("ValidateChecksumAddress returned nil for invalid address format: %q", input)
		}
	})
}

// FuzzNormalizeAddress tests that address normalization never panics.
func FuzzNormalizeAddress(f *testing.F) {
	f.Add("0x742d35cc6634c0532925a3b844bc9e7595f8b2e0") // valid lowercase
	f.Add("0x742D35CC6634C0532925A3B844BC9E7595F8B2E0") // valid uppercase
	f.Add("")                                              // empty string
	f.Add("invalid")                                       // short invalid
	f.Add("0xGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGGG")  // correct length, invalid hex

	f.Fuzz(func(t *testing.T, input string) {
		// Early exit: Fast length check before validation
		// Ethereum addresses must be exactly 42 characters (0x + 40 hex chars)
		if len(input) != 42 {
			return
		}

		// Early exit: Fast prefix check
		// Ethereum addresses must start with 0x
		if !strings.HasPrefix(input, "0x") {
			return
		}

		// Should not panic
		result, err := NormalizeAddress(input)

		if IsValidAddress(input) {
			// For valid input, should succeed
			if err != nil {
				t.Errorf("NormalizeAddress returned error for valid address %q: %v", input, err)
			}
			if !IsValidAddress(result) {
				t.Errorf("NormalizeAddress returned invalid address for valid input %q: %q", input, result)
			}
		} else {
			// For invalid input, should return error
			if err == nil {
				t.Errorf("NormalizeAddress returned nil error for invalid address: %q", input)
			}
		}
	})
}
