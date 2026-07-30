package bsv

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"github.com/mrz1836/sigil/internal/wallet/bitcoin"
	sigilerrors "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// Address version bytes for mainnet.
	versionP2PKH = 0x00 // P2PKH addresses start with 1
	versionP2SH  = 0x05 // P2SH addresses start with 3

	// Address version bytes for testnet.
	versionP2PKHTestnet = 0x6f // P2PKH addresses start with m or n
	versionP2SHTestnet  = 0xc4 // P2SH addresses start with 2

	// checksumLen is the length of the checksum in bytes.
	checksumLen = 4

	// payloadLen is the length of the address payload (RIPEMD-160 hash).
	payloadLen = 20
)

var (
	// ErrInvalidBase58 indicates invalid base58 encoding.
	ErrInvalidBase58 = errors.New("invalid base58 encoding")

	// ErrInvalidChecksum indicates checksum validation failed.
	ErrInvalidChecksum = errors.New("invalid checksum")

	// ErrInvalidAddressLength indicates the address has wrong length.
	ErrInvalidAddressLength = errors.New("invalid address length")
)

// IsValidAddress checks if a mainnet BSV address is valid (format only).
func IsValidAddress(address string) bool {
	return ValidateBase58CheckAddress(address) == nil
}

// IsValidAddressForNetwork checks if a BSV address is valid for the given network.
func IsValidAddressForNetwork(address string, network Network) bool {
	return ValidateBase58CheckAddressForNetwork(address, network) == nil
}

// ValidateBase58CheckAddress validates a mainnet BSV address with full checksum verification.
func ValidateBase58CheckAddress(address string) error {
	return validateForVersions(address, base58Regex, versionP2PKH, versionP2SH)
}

// ValidateBase58CheckAddressTestnet validates a testnet BSV address with full checksum verification.
func ValidateBase58CheckAddressTestnet(address string) error {
	return validateForVersions(address, base58RegexTestnet, versionP2PKHTestnet, versionP2SHTestnet)
}

// ValidateBase58CheckAddressForNetwork validates a BSV address against the given network.
// A mainnet address is rejected on testnet and vice versa (cross-network protection).
func ValidateBase58CheckAddressForNetwork(address string, network Network) error {
	if network == NetworkTestnet {
		return ValidateBase58CheckAddressTestnet(address)
	}
	return ValidateBase58CheckAddress(address)
}

// validateForVersions validates an address against a leading-character regex and an
// allow-list of version bytes, verifying the full Base58Check checksum.
func validateForVersions(address string, re *regexp.Regexp, allowed ...byte) error {
	if address == "" {
		return ErrInvalidAddress
	}

	// Quick format check first (leading-character + length)
	if !re.MatchString(address) {
		return ErrInvalidAddress
	}

	// Decode and verify checksum
	version, _, err := DecodeBase58Check(address)
	if err != nil {
		return err
	}

	// Validate version byte against the allow-list
	for _, v := range allowed {
		if version == v {
			return nil
		}
	}

	return sigilerrors.WithDetails(sigilerrors.ErrUnsupportedVersion, map[string]string{
		"version": fmt.Sprintf("0x%02x", version),
	})
}

// DecodeBase58Check decodes a Base58Check encoded address.
// Returns the version byte and the payload (typically a 20-byte public key hash).
func DecodeBase58Check(address string) (version byte, payload []byte, err error) {
	if address == "" {
		return 0, nil, ErrInvalidBase58
	}

	// Decode base58
	decoded, err := base58Decode(address)
	if err != nil {
		return 0, nil, err
	}

	// Minimum length: 1 (version) + 20 (payload) + 4 (checksum)
	minLen := 1 + payloadLen + checksumLen
	if len(decoded) < minLen {
		return 0, nil, ErrInvalidAddressLength
	}

	// Split into data and checksum
	data := decoded[:len(decoded)-checksumLen]
	checksum := decoded[len(decoded)-checksumLen:]

	// Verify checksum (first 4 bytes of double SHA256)
	expectedChecksum := doubleSHA256Checksum(data)
	if !bytes.Equal(checksum, expectedChecksum) {
		return 0, nil, fmt.Errorf("%w: expected %x, got %x", ErrInvalidChecksum, expectedChecksum, checksum)
	}

	// Extract version and payload
	version = data[0]
	payload = data[1:]

	return version, payload, nil
}

// EncodeBase58Check encodes data with version byte and checksum.
func EncodeBase58Check(version byte, payload []byte) string {
	// Combine version and payload
	data := make([]byte, 1+len(payload))
	data[0] = version
	copy(data[1:], payload)

	// Calculate checksum
	checksum := doubleSHA256Checksum(data)

	// Combine data and checksum
	full := make([]byte, len(data)+len(checksum))
	copy(full, data)
	copy(full[len(data):], checksum)

	// Encode as base58
	return base58Encode(full)
}

// base58Decode decodes a base58 string to bytes. It delegates to the single
// audited Base58 implementation in internal/wallet/bitcoin (as the BTC client
// does) so there is one reviewed crypto path; outputs are identical.
func base58Decode(s string) ([]byte, error) {
	decoded, err := bitcoin.Base58Decode(s)
	if err != nil {
		return nil, ErrInvalidBase58
	}
	return decoded, nil
}

// base58Encode encodes bytes to base58 via the canonical implementation.
func base58Encode(input []byte) string {
	return bitcoin.Base58Encode(input)
}

// doubleSHA256Checksum computes the first 4 bytes of double SHA256.
func doubleSHA256Checksum(data []byte) []byte {
	return bitcoin.DoubleSHA256(data)[:checksumLen]
}
