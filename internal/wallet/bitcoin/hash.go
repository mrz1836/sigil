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

// DoubleSHA256 computes SHA256(SHA256(data)) as used by Bitcoin protocol.
func DoubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

// Base58 alphabet used by Bitcoin.
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// ErrInvalidBase58 indicates invalid Base58 encoding.
var ErrInvalidBase58 = &base58Error{}

type base58Error struct{}

func (*base58Error) Error() string { return "invalid Base58 encoding" }

// Base58Decode decodes a Base58-encoded string.
func Base58Decode(s string) ([]byte, error) {
	if len(s) == 0 {
		return nil, ErrInvalidBase58
	}

	zeros := countLeadingZeros(s)
	b256, err := decodeBase58Payload(s, zeros)
	if err != nil {
		return nil, err
	}

	return buildBase58Result(b256, zeros), nil
}

// countLeadingZeros counts leading '1's (zero bytes in Base58).
func countLeadingZeros(s string) int {
	zeros := 0
	for zeros < len(s) && s[zeros] == '1' {
		zeros++
	}
	return zeros
}

// decodeBase58Payload decodes the non-zero portion of a Base58 string.
func decodeBase58Payload(s string, zeros int) ([]byte, error) {
	size := len(s)*733/1000 + 1 // log(58) / log(256), rounded up
	b256 := make([]byte, size)

	for i := zeros; i < len(s); i++ {
		carry, ok := base58CharValue(s[i])
		if !ok {
			return nil, ErrInvalidBase58
		}
		for j := len(b256) - 1; j >= 0; j-- {
			carry += int(b256[j]) * 58
			b256[j] = byte(carry % 256)
			carry /= 256
		}
	}
	return b256, nil
}

// base58CharValue returns the value of a Base58 character.
func base58CharValue(c byte) (int, bool) {
	for i := 0; i < len(base58Alphabet); i++ {
		if base58Alphabet[i] == c {
			return i, true
		}
	}
	return 0, false
}

// buildBase58Result builds the final decoded result.
func buildBase58Result(b256 []byte, zeros int) []byte {
	j := 0
	for j < len(b256) && b256[j] == 0 {
		j++
	}
	result := make([]byte, zeros+len(b256)-j)
	copy(result[zeros:], b256[j:])
	return result
}

// Base58CheckDecode decodes a Base58Check-encoded string and verifies the checksum.
func Base58CheckDecode(s string) ([]byte, error) {
	decoded, err := Base58Decode(s)
	if err != nil {
		return nil, err
	}

	if len(decoded) < 4 {
		return nil, ErrInvalidBase58
	}

	// Verify checksum (last 4 bytes)
	payload := decoded[:len(decoded)-4]
	checksum := decoded[len(decoded)-4:]
	expectedChecksum := DoubleSHA256(payload)[:4]

	for i := 0; i < 4; i++ {
		if checksum[i] != expectedChecksum[i] {
			return nil, ErrInvalidBase58
		}
	}

	return payload, nil
}

// Base58Encode encodes data to Base58.
func Base58Encode(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Count leading zeros
	var zeros int
	for zeros < len(data) && data[zeros] == 0 {
		zeros++
	}

	// Allocate enough space
	size := (len(data)-zeros)*138/100 + 1 // log(256) / log(58), rounded up
	buf := make([]byte, size)

	// Process the data
	for _, b := range data[zeros:] {
		carry := int(b)
		for j := len(buf) - 1; j >= 0; j-- {
			carry += int(buf[j]) << 8
			buf[j] = byte(carry % 58)
			carry /= 58
		}
	}

	// Skip leading zeros in buffer
	j := 0
	for j < len(buf) && buf[j] == 0 {
		j++
	}

	// Build result string
	result := make([]byte, zeros+len(buf)-j)
	for i := 0; i < zeros; i++ {
		result[i] = '1'
	}
	for i, b := range buf[j:] {
		result[zeros+i] = base58Alphabet[b]
	}

	return string(result)
}

// Base58CheckEncode encodes data with a version byte and checksum.
func Base58CheckEncode(version byte, payload []byte) string {
	// Preallocate for version + payload + 4 byte checksum
	data := make([]byte, 0, 1+len(payload)+4)
	data = append(data, version)
	data = append(data, payload...)

	// Append checksum
	checksum := DoubleSHA256(data)[:4]
	data = append(data, checksum...)

	return Base58Encode(data)
}
