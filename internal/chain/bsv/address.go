package bsv

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"

	sigilerrors "github.com/mrz1836/sigil/pkg/errors"
)

const (
	// Address version bytes for mainnet.
	versionP2PKH = 0x00 // P2PKH addresses start with 1
	versionP2SH  = 0x05 // P2SH addresses start with 3

	// checksumLen is the length of the checksum in bytes.
	checksumLen = 4

	// payloadLen is the length of the address payload (RIPEMD-160 hash).
	payloadLen = 20

	// Base58 alphabet (excludes 0, O, I, l).
	base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
)

var (
	// ErrInvalidBase58 indicates invalid base58 encoding.
	ErrInvalidBase58 = errors.New("invalid base58 encoding")

	// ErrInvalidChecksum indicates checksum validation failed.
	ErrInvalidChecksum = errors.New("invalid checksum")

	// ErrInvalidAddressLength indicates the address has wrong length.
	ErrInvalidAddressLength = errors.New("invalid address length")

	// base58AlphabetMap maps base58 characters to their values.
	//nolint:gochecknoglobals // Required for base58 encoding/decoding
	base58AlphabetMap = make(map[rune]int)
)

//nolint:gochecknoinits // Required for base58 alphabet map initialization
func init() {
	for i, c := range base58Alphabet {
		base58AlphabetMap[c] = i
	}
}

// IsValidAddress checks if a BSV address is valid (format only).
func IsValidAddress(address string) bool {
	return ValidateBase58CheckAddress(address) == nil
}

// ValidateBase58CheckAddress validates a BSV address with full checksum verification.
func ValidateBase58CheckAddress(address string) error {
	if address == "" {
		return ErrInvalidAddress
	}

	// Quick format check first
	if !base58Regex.MatchString(address) {
		return ErrInvalidAddress
	}

	// Decode and verify checksum
	version, _, err := DecodeBase58Check(address)
	if err != nil {
		return err
	}

	// Validate version byte
	if version != versionP2PKH && version != versionP2SH {
		return sigilerrors.WithDetails(sigilerrors.ErrUnsupportedVersion, map[string]string{
			"version": fmt.Sprintf("0x%02x", version),
		})
	}

	return nil
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

// base58Decode decodes a base58 string to bytes.
func base58Decode(s string) ([]byte, error) {
	if s == "" {
		return nil, ErrInvalidBase58
	}

	// Count leading '1's (which represent leading zero bytes)
	leadingOnes := 0
	for _, c := range s {
		if c == '1' {
			leadingOnes++
		} else {
			break
		}
	}

	// Convert base58 to big integer
	result := big.NewInt(0)
	base := big.NewInt(58)

	for _, c := range s {
		value, ok := base58AlphabetMap[c]
		if !ok {
			return nil, fmt.Errorf("%w: invalid character '%c'", ErrInvalidBase58, c)
		}

		result.Mul(result, base)
		result.Add(result, big.NewInt(int64(value)))
	}

	// Convert to bytes
	decoded := result.Bytes()

	// Add leading zero bytes
	output := make([]byte, leadingOnes+len(decoded))
	copy(output[leadingOnes:], decoded)

	return output, nil
}

// base58Encode encodes bytes to base58.
func base58Encode(input []byte) string {
	// Count leading zeros
	leadingZeros := 0
	for _, b := range input {
		if b == 0 {
			leadingZeros++
		} else {
			break
		}
	}

	// Convert to big integer
	x := new(big.Int).SetBytes(input)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var result []byte
	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		result = append(result, base58Alphabet[mod.Int64()])
	}

	// Add leading '1's for each leading zero byte
	for i := 0; i < leadingZeros; i++ {
		result = append(result, '1')
	}

	// Reverse the result
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

// doubleSHA256Checksum computes the first 4 bytes of double SHA256.
func doubleSHA256Checksum(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:checksumLen]
}
