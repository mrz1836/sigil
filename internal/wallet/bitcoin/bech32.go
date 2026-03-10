package bitcoin

import (
	"errors"
	"fmt"
	"strings"
)

// Bech32/segwit encoding errors.
var (
	ErrInvalidBech32Data     = errors.New("invalid bech32 data value")
	ErrInvalidBitConversion  = errors.New("invalid bit conversion")
	ErrIllegalZeroPadding    = errors.New("illegal zero padding")
	ErrNonZeroPadding        = errors.New("non-zero padding")
	ErrInvalidWitnessVersion = errors.New("invalid witness version")
	ErrInvalidWitnessProgram = errors.New("invalid witness program length")
)

// bech32 charset for encoding.
const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

// bech32Polymod computes the BCH checksum polynomial.
func bech32Polymod(values []byte) uint32 {
	gen := [5]uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	chk := uint32(1)
	for _, v := range values {
		b := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ uint32(v)
		for i := 0; i < 5; i++ {
			if (b>>uint(i))&1 == 1 {
				chk ^= gen[i]
			}
		}
	}
	return chk
}

// bech32HRPExpand expands the HRP for checksum computation.
//
//nolint:gosec // G115: HRP characters are ASCII (7-bit), safe to convert rune -> byte
func bech32HRPExpand(hrp string) []byte {
	ret := make([]byte, 0, len(hrp)*2+1)
	for _, c := range hrp {
		ret = append(ret, byte(c>>5))
	}
	ret = append(ret, 0)
	for _, c := range hrp {
		ret = append(ret, byte(c&31))
	}
	return ret
}

// bech32CreateChecksum creates a bech32 checksum for the given HRP and data.
func bech32CreateChecksum(hrp string, data []byte) []byte {
	values := append(bech32HRPExpand(hrp), data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	polymod := bech32Polymod(values) ^ 1
	ret := make([]byte, 6)
	for i := 0; i < 6; i++ {
		ret[i] = byte((polymod >> uint(5*(5-i))) & 31)
	}
	return ret
}

// Bech32Encode encodes a bech32 string from HRP and 5-bit data values.
func Bech32Encode(hrp string, data []byte) (string, error) {
	checksum := bech32CreateChecksum(hrp, data)
	combined := append(data, checksum...)

	var sb strings.Builder
	sb.Grow(len(hrp) + 1 + len(combined))
	sb.WriteString(hrp)
	sb.WriteByte('1')
	for _, d := range combined {
		if int(d) >= len(bech32Charset) {
			return "", fmt.Errorf("%w: %d", ErrInvalidBech32Data, d)
		}
		sb.WriteByte(bech32Charset[d])
	}
	return sb.String(), nil
}

// ConvertBits converts a byte slice from one bit grouping to another.
// It groups input bits of size fromBits into output groups of size toBits.
func ConvertBits(data []byte, fromBits, toBits uint8, pad bool) ([]byte, error) {
	acc := uint32(0)
	bits := uint8(0)
	maxv := uint32((1 << toBits) - 1)

	var ret []byte
	for _, value := range data {
		if uint32(value)>>fromBits != 0 {
			return nil, fmt.Errorf("%w: %d (max %d)", ErrInvalidBitConversion, value, (1<<fromBits)-1)
		}
		acc = (acc << fromBits) | uint32(value)
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			//nolint:gosec // G115: result is masked to maxv which fits in byte
			ret = append(ret, byte((acc>>bits)&maxv))
		}
	}

	if pad {
		if bits > 0 {
			//nolint:gosec // G115: result is masked to maxv which fits in byte
			ret = append(ret, byte((acc<<(toBits-bits))&maxv))
		}
	} else if bits >= fromBits {
		return nil, ErrIllegalZeroPadding
	} else if (acc<<(toBits-bits))&maxv != 0 {
		return nil, ErrNonZeroPadding
	}

	return ret, nil
}

// SegwitEncode encodes a segwit address with the given HRP, witness version,
// and witness program (e.g., 20-byte pubkey hash for P2WPKH).
func SegwitEncode(hrp string, version byte, program []byte) (string, error) {
	if version > 16 {
		return "", fmt.Errorf("%w: %d", ErrInvalidWitnessVersion, version)
	}
	if len(program) < 2 || len(program) > 40 {
		return "", fmt.Errorf("%w: %d", ErrInvalidWitnessProgram, len(program))
	}
	if version == 0 && len(program) != 20 && len(program) != 32 {
		return "", fmt.Errorf("%w: v0 requires 20 or 32 bytes, got %d", ErrInvalidWitnessProgram, len(program))
	}

	conv, err := ConvertBits(program, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("convert bits: %w", err)
	}

	data := append([]byte{version}, conv...)
	return Bech32Encode(hrp, data)
}
