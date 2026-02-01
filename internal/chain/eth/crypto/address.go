package ethcrypto

import (
	"encoding/hex"
	"errors"
	"strings"
)

const (
	// AddressLength is the expected length of an Ethereum address.
	AddressLength = 20
)

// ErrInvalidAddressFormat indicates the address format is invalid.
var ErrInvalidAddressFormat = errors.New("invalid address format")

// Address represents a 20-byte Ethereum address.
type Address [AddressLength]byte

// BytesToAddress converts a byte slice to an Address.
// If the slice is shorter than 20 bytes, it is left-padded with zeros.
// If longer, only the last 20 bytes are used.
func BytesToAddress(b []byte) Address {
	var a Address
	if len(b) > AddressLength {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
	return a
}

// HexToAddress converts a hex string to an Address.
// The string may optionally start with "0x".
func HexToAddress(s string) (Address, error) {
	s = strings.TrimPrefix(s, "0x")
	if len(s) != AddressLength*2 {
		return Address{}, ErrInvalidAddressFormat
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return Address{}, ErrInvalidAddressFormat
	}

	return BytesToAddress(b), nil
}

// Bytes returns the address as a byte slice.
func (a Address) Bytes() []byte {
	return a[:]
}

// Hex returns the address as a hex string with 0x prefix.
func (a Address) Hex() string {
	return "0x" + hex.EncodeToString(a[:])
}

// String returns the EIP-55 checksummed hex representation.
func (a Address) String() string {
	return ToChecksumAddress(a.Hex())
}

// ToChecksumAddress converts an Ethereum address to EIP-55 checksum format.
func ToChecksumAddress(address string) string {
	// Remove 0x prefix and lowercase
	addr := strings.ToLower(strings.TrimPrefix(address, "0x"))
	if len(addr) != 40 {
		return address
	}

	// Hash the lowercase address
	hash := hex.EncodeToString(Keccak256([]byte(addr)))

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

// LeftPadBytes pads a byte slice with zeros on the left to the specified length.
func LeftPadBytes(b []byte, length int) []byte {
	if len(b) >= length {
		return b
	}
	result := make([]byte, length)
	copy(result[length-len(b):], b)
	return result
}
