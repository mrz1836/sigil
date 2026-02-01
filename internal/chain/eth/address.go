package eth

import (
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/sha3"

	sigilerrors "github.com/mrz1836/sigil/pkg/errors"
)

// IsValidAddress checks if the address is a valid Ethereum address format.
// This validates the format (40 hex chars with 0x prefix) but does not validate checksum.
func IsValidAddress(address string) bool {
	if len(address) != 42 {
		return false
	}
	if !strings.HasPrefix(address, "0x") {
		return false
	}
	// Check all characters are valid hex
	for _, c := range address[2:] {
		if !isHexChar(c) {
			return false
		}
	}
	return true
}

// ToChecksumAddress converts an Ethereum address to EIP-55 checksum format.
// If the input is invalid, it returns the original input unchanged.
func ToChecksumAddress(address string) string {
	if !IsValidAddress(address) {
		return address
	}

	// Remove 0x prefix and lowercase
	addr := strings.ToLower(address[2:])

	// Hash the lowercase address
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write([]byte(addr))
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Build checksummed address
	result := make([]byte, 42)
	result[0] = '0'
	result[1] = 'x'

	for i := 0; i < 40; i++ {
		c := addr[i]
		// If the hash nibble is >= 8, uppercase the character
		hashNibble := hash[i]
		if hashNibble >= '8' && c >= 'a' && c <= 'f' {
			//nolint:gosec // Safe: i bounded by loop [0,40), result size is 42
			result[i+2] = c - 32 // Convert to uppercase
		} else {
			//nolint:gosec // Safe: i bounded by loop [0,40), result size is 42
			result[i+2] = c
		}
	}

	return string(result)
}

// ValidateChecksumAddress validates that an Ethereum address has correct EIP-55 checksum.
// All lowercase and all uppercase addresses are considered valid (non-checksummed).
// Mixed-case addresses must have the correct checksum.
func ValidateChecksumAddress(address string) error {
	if !IsValidAddress(address) {
		return sigilerrors.WithDetails(sigilerrors.ErrInvalidAddress, map[string]string{
			"address": address,
		})
	}

	// All lowercase or all uppercase is valid (non-checksummed)
	addrPart := address[2:]
	if addrPart == strings.ToLower(addrPart) || addrPart == strings.ToUpper(addrPart) {
		return nil
	}

	// For mixed case, verify checksum
	expected := ToChecksumAddress(address)
	if address != expected {
		return sigilerrors.WithDetails(sigilerrors.ErrInvalidChecksum, map[string]string{
			"expected": expected,
			"actual":   address,
		})
	}

	return nil
}

// NormalizeAddress validates and converts an address to EIP-55 checksum format.
// Returns an error if the address is invalid.
func NormalizeAddress(address string) (string, error) {
	if !IsValidAddress(address) {
		return "", sigilerrors.WithDetails(sigilerrors.ErrInvalidAddress, map[string]string{
			"address": address,
		})
	}
	return ToChecksumAddress(address), nil
}

// isHexChar returns true if c is a valid hexadecimal character.
func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
