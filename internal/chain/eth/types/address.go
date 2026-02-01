package ethtypes

import (
	"encoding/hex"
	"errors"
	"strings"

	ethcrypto "github.com/mrz1836/sigil/internal/chain/eth/crypto"
)

const (
	// AddressLength is the expected length of an Ethereum address.
	AddressLength = 20
)

// ErrInvalidAddress indicates the address format is invalid.
var ErrInvalidAddress = errors.New("invalid address format")

// Address represents a 20-byte Ethereum address.
type Address [AddressLength]byte

// BytesToAddress converts a byte slice to an Address.
func BytesToAddress(b []byte) Address {
	var a Address
	if len(b) > AddressLength {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
	return a
}

// HexToAddress converts a hex string to an Address.
func HexToAddress(s string) (Address, error) {
	s = strings.TrimPrefix(s, "0x")
	if len(s) != AddressLength*2 {
		return Address{}, ErrInvalidAddress
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return Address{}, ErrInvalidAddress
	}

	return BytesToAddress(b), nil
}

// MustHexToAddress converts a hex string to an Address, panicking on error.
// Only use in initialization code with known-good addresses.
func MustHexToAddress(s string) Address {
	addr, err := HexToAddress(s)
	if err != nil {
		panic(err)
	}
	return addr
}

// Bytes returns the address as a byte slice.
func (a Address) Bytes() []byte {
	return a[:]
}

// Hex returns the address as a lowercase hex string with 0x prefix.
func (a Address) Hex() string {
	return "0x" + hex.EncodeToString(a[:])
}

// String returns the EIP-55 checksummed hex representation.
func (a Address) String() string {
	return ethcrypto.ToChecksumAddress(a.Hex())
}

// IsZero returns true if the address is all zeros.
func (a Address) IsZero() bool {
	for _, b := range a {
		if b != 0 {
			return false
		}
	}
	return true
}
