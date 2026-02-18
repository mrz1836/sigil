package ethcrypto

import (
	"errors"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

var (
	// ErrInvalidPrivateKey indicates the private key is invalid.
	ErrInvalidPrivateKey = errors.New("invalid private key")

	// ErrInvalidSignature indicates the signature is invalid.
	ErrInvalidSignature = errors.New("invalid signature")

	// ErrInvalidHashLength indicates the hash length is not 32 bytes.
	ErrInvalidHashLength = errors.New("hash must be 32 bytes")

	// ErrInvalidPublicKeyPrefix indicates an invalid public key prefix.
	ErrInvalidPublicKeyPrefix = errors.New("invalid public key prefix")

	// ErrInvalidPublicKeyLength indicates an invalid public key length.
	ErrInvalidPublicKeyLength = errors.New("invalid public key length")
)

// Sign signs the given hash with the private key and returns a 65-byte signature.
// The signature format is [R || S || V] where V is the recovery ID (0 or 1).
// This matches Ethereum's signature format (before EIP-155 chain ID encoding).
func Sign(hash, privateKey []byte) ([]byte, error) {
	if len(hash) != 32 {
		return nil, ErrInvalidHashLength
	}
	if len(privateKey) != 32 {
		return nil, ErrInvalidPrivateKey
	}

	privKey := secp256k1.PrivKeyFromBytes(privateKey)
	if privKey == nil {
		return nil, ErrInvalidPrivateKey
	}

	// Sign with recovery to get the recovery ID
	sig := ecdsa.SignCompact(privKey, hash, false)

	// SignCompact returns [V || R || S] (65 bytes) where V is recovery ID + 27
	// We need to rearrange to [R || S || V] format for Ethereum
	if len(sig) != 65 {
		return nil, ErrInvalidSignature
	}

	// Extract components
	v := sig[0] - 27 // Convert from Bitcoin format (27/28) to Ethereum format (0/1)
	r := sig[1:33]
	s := sig[33:65]

	// Reformat to [R || S || V]
	result := make([]byte, 65)
	copy(result[0:32], r)
	copy(result[32:64], s)
	result[len(result)-1] = v

	return result, nil
}

// PrivateKeyToPublicKey derives the public key from a private key.
// Returns the uncompressed public key (65 bytes: 0x04 || X || Y).
func PrivateKeyToPublicKey(privateKey []byte) ([]byte, error) {
	if len(privateKey) != 32 {
		return nil, ErrInvalidPrivateKey
	}

	privKey := secp256k1.PrivKeyFromBytes(privateKey)
	if privKey == nil {
		return nil, ErrInvalidPrivateKey
	}

	pubKey := privKey.PubKey()
	return pubKey.SerializeUncompressed(), nil
}

// PublicKeyToAddress derives an Ethereum address from an uncompressed public key.
// The public key should be 65 bytes (0x04 prefix + 64 bytes X,Y coordinates)
// or 64 bytes (just the X,Y coordinates without prefix).
func PublicKeyToAddress(publicKey []byte) ([]byte, error) {
	var pubKeyBytes []byte

	switch len(publicKey) {
	case 65:
		// Uncompressed with 0x04 prefix - skip the prefix
		if publicKey[0] != 0x04 {
			return nil, ErrInvalidPublicKeyPrefix
		}
		pubKeyBytes = publicKey[1:]
	case 64:
		// Just the coordinates
		pubKeyBytes = publicKey
	default:
		return nil, ErrInvalidPublicKeyLength
	}

	// Hash the public key bytes (without the 0x04 prefix)
	hash := Keccak256(pubKeyBytes)

	// Take the last 20 bytes as the address
	return hash[12:], nil
}

// DeriveAddress derives an Ethereum address from a private key.
func DeriveAddress(privateKey []byte) ([]byte, error) {
	pubKey, err := PrivateKeyToPublicKey(privateKey)
	if err != nil {
		return nil, err
	}
	return PublicKeyToAddress(pubKey)
}
