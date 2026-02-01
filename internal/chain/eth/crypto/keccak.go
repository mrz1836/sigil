// Package ethcrypto provides Ethereum cryptographic primitives without go-ethereum.
package ethcrypto

import (
	"golang.org/x/crypto/sha3"
)

// Keccak256 computes the Keccak-256 hash of the input data.
// This is the hash function used throughout Ethereum.
func Keccak256(data ...[]byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	for _, b := range data {
		hasher.Write(b)
	}
	return hasher.Sum(nil)
}

// Keccak256Hash computes the Keccak-256 hash and returns it as a 32-byte array.
func Keccak256Hash(data ...[]byte) [32]byte {
	var hash [32]byte
	copy(hash[:], Keccak256(data...))
	return hash
}
