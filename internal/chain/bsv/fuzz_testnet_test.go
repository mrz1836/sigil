package bsv

import (
	"testing"
)

// isTestnetAddrPrefix reports whether c is a valid leading char for a testnet address.
func isTestnetAddrPrefix(c byte) bool {
	return c == 'm' || c == 'n' || c == '2'
}

// FuzzValidateAddressTestnet tests that testnet address validation never panics
// and only accepts addresses whose leading character is a valid testnet prefix.
func FuzzValidateAddressTestnet(f *testing.F) {
	// Valid testnet P2PKH (start with m or n).
	f.Add("mrCDrCybB6J1vRfbwM5hemdJz73FwDBC8r")
	f.Add("n2ZNV88uQbede7C5M5jzi6SyG4GVuPpng6")
	// Mainnet address (must be rejected on testnet).
	f.Add("1BgGZ9tcN4rm9KBzDn7KprQz87SZ26SAMH")
	// Garbage / edge cases.
	f.Add("")
	f.Add("2")
	f.Add("0x742d35Cc6634C0532925a3b844Bc9e7595f8b2E0")
	f.Add("OIl0")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 10000 {
			t.Skip("input too large")
		}

		// Must never panic.
		valid := ValidateBase58CheckAddressTestnet(input) == nil
		if !valid {
			return
		}

		// If accepted, the leading character must be a testnet prefix (m/n/2).
		if len(input) > 0 && !isTestnetAddrPrefix(input[0]) {
			t.Errorf("testnet validator accepted address with invalid prefix: %q", input)
		}

		// Differential invariant: a string valid on testnet must not be valid on
		// mainnet (cross-network encodings are disjoint).
		if ValidateBase58CheckAddress(input) == nil {
			t.Errorf("address accepted on both mainnet and testnet: %q", input)
		}
	})
}
