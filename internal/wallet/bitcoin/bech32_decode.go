package bitcoin

import (
	"errors"
	"fmt"
	"strings"
)

// Bech32/segwit decoding errors.
var (
	ErrBech32MixedCase   = errors.New("bech32 string has mixed case")
	ErrBech32TooLong     = errors.New("bech32 string exceeds 90 characters")
	ErrBech32NoSeparator = errors.New("bech32 string missing separator '1'")
	ErrBech32BadHRP      = errors.New("bech32 human-readable part is invalid")
	ErrBech32BadChar     = errors.New("bech32 string contains invalid character")
	ErrBech32BadChecksum = errors.New("bech32 checksum mismatch")
	ErrSegwitHRPMismatch = errors.New("segwit address has unexpected human-readable part")
	ErrSegwitVersion     = errors.New("segwit witness version unsupported")
	ErrSegwitProgramLen  = errors.New("segwit witness program has invalid length")
)

// bech32Checksum is the polymod constant for a valid BECH32 string (BIP173).
// The BECH32M variant used by segwit v1+ (taproot) has a different constant
// (0x2bc830a3); by requiring BECH32 here we reject bech32m addresses, which is
// intentional since only witness v0 programs are supported.
const bech32Checksum = 1

// bech32Reverse returns the 5-bit value of a bech32 charset character, or -1 if
// the character is not part of the charset.
func bech32Reverse(c byte) int {
	return strings.IndexByte(bech32Charset, c)
}

// bech32VerifyChecksum reports whether the HRP and 5-bit data (including the
// trailing 6-symbol checksum) form a valid BECH32 string.
func bech32VerifyChecksum(hrp string, data []byte) bool {
	values := append(bech32HRPExpand(hrp), data...)
	return bech32Polymod(values) == bech32Checksum
}

// Bech32Decode decodes a bech32 (BIP173) string into its human-readable part and
// 5-bit data payload with the 6-symbol checksum verified and stripped. It enforces
// the BIP173 constraints: total length <= 90 characters, no mixed case, a separator
// '1', a non-empty HRP with characters in the ASCII [33,126] range, and a valid
// BECH32 checksum. It intentionally rejects the BECH32M variant (segwit v1+/taproot)
// so that only witness v0 programs decode successfully via SegwitDecode.
func Bech32Decode(s string) (hrp string, data []byte, err error) {
	if len(s) > 90 {
		return "", nil, fmt.Errorf("%w: %d characters", ErrBech32TooLong, len(s))
	}

	// Reject mixed case; normalize to lowercase for processing.
	lower := strings.ToLower(s)
	upper := strings.ToUpper(s)
	if s != lower && s != upper {
		return "", nil, ErrBech32MixedCase
	}
	s = lower

	// The separator is the last '1'. Everything before is the HRP; everything
	// after is the data part (which must hold at least the 6-symbol checksum).
	sep := strings.LastIndexByte(s, '1')
	if sep < 1 || sep+7 > len(s) {
		return "", nil, ErrBech32NoSeparator
	}
	hrp = s[:sep]
	if err = validateBech32HRP(hrp); err != nil {
		return "", nil, err
	}

	decoded, err := decodeBech32Data(s[sep+1:])
	if err != nil {
		return "", nil, err
	}

	if !bech32VerifyChecksum(hrp, decoded) {
		return "", nil, ErrBech32BadChecksum
	}

	// Strip the trailing 6-symbol checksum.
	return hrp, decoded[:len(decoded)-6], nil
}

// validateBech32HRP checks the human-readable part is printable ASCII [33,126].
func validateBech32HRP(hrp string) error {
	for i := range len(hrp) {
		if hrp[i] < 33 || hrp[i] > 126 {
			return fmt.Errorf("%w: char 0x%02x", ErrBech32BadHRP, hrp[i])
		}
	}
	return nil
}

// decodeBech32Data converts a bech32 data part (charset characters) into its
// 5-bit values, returning an error on any character outside the charset.
func decodeBech32Data(dataPart string) ([]byte, error) {
	decoded := make([]byte, 0, len(dataPart))
	for i := range len(dataPart) {
		v := bech32Reverse(dataPart[i])
		if v < 0 {
			return nil, fmt.Errorf("%w: '%c'", ErrBech32BadChar, dataPart[i])
		}
		//nolint:gosec // v is a validated bech32 value in [0,31], fits in a byte
		decoded = append(decoded, byte(v))
	}
	return decoded, nil
}

// SegwitDecode decodes a native SegWit (BIP173) address, returning the witness
// version and the decoded 8-bit witness program. Only version 0 (P2WPKH with a
// 20-byte program, P2WSH with a 32-byte program) is supported; version 1+
// (taproot / BECH32M) and any other program length are rejected. expectedHRP
// scopes the address to a network ("bc" mainnet, "tb" testnet) — a mismatch is
// an error, providing cross-network protection.
func SegwitDecode(expectedHRP, addr string) (version byte, program []byte, err error) {
	hrp, data, err := Bech32Decode(addr)
	if err != nil {
		return 0, nil, err
	}
	if hrp != expectedHRP {
		return 0, nil, fmt.Errorf("%w: got %q, want %q", ErrSegwitHRPMismatch, hrp, expectedHRP)
	}
	if len(data) == 0 {
		return 0, nil, fmt.Errorf("%w: empty data", ErrSegwitVersion)
	}

	version = data[0]
	if version != 0 {
		return 0, nil, fmt.Errorf("%w: v%d (only v0 supported)", ErrSegwitVersion, version)
	}

	// Convert the witness program from 5-bit groups back to 8-bit bytes. Padding
	// must be zero and minimal (pad=false enforces BIP173 padding rules).
	program, err = ConvertBits(data[1:], 5, 8, false)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %w", ErrSegwitProgramLen, err)
	}
	if len(program) != 20 && len(program) != 32 {
		return 0, nil, fmt.Errorf("%w: %d bytes (v0 requires 20 or 32)", ErrSegwitProgramLen, len(program))
	}

	return version, program, nil
}
