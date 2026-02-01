// Package bitcoin provides Bitcoin protocol-specific cryptographic operations.
// This package isolates legacy cryptographic primitives required by the
// Bitcoin protocol that cannot be replaced without breaking compatibility.
package bitcoin

import (
	"crypto/sha256"

	// RIPEMD160 is deprecated but REQUIRED by Bitcoin protocol (BIP-13, BIP-16).
	// Bitcoin P2PKH addresses use Hash160 = RIPEMD160(SHA256(pubkey)).
	// This is a protocol requirement and cannot be changed.
	//nolint:gosec,staticcheck // G507,SA1019: RIPEMD160 required by Bitcoin protocol
	"golang.org/x/crypto/ripemd160"
)

// Hash160 computes RIPEMD160(SHA256(data)) as required by Bitcoin protocol.
// This is the standard Bitcoin address hashing function (P2PKH).
//
// Security Note: RIPEMD160 is deprecated for NEW applications, but this
// implementation is for Bitcoin protocol compatibility ONLY.
//
//nolint:gosec // G406: RIPEMD160 usage required by Bitcoin spec
func Hash160(data []byte) []byte {
	sha256Hash := sha256.Sum256(data)
	ripemd := ripemd160.New()
	ripemd.Write(sha256Hash[:])
	return ripemd.Sum(nil)
}
