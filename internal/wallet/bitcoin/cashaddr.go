package bitcoin

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidCashAddrHash indicates an unsupported hash length for cashaddr.
var ErrInvalidCashAddrHash = errors.New("invalid cashaddr hash length")

// CashAddr type constants.
const (
	CashAddrTypeP2PKH byte = 0
	CashAddrTypeP2SH  byte = 1
)

// cashAddrCharset is the character set for cashaddr encoding (same as bech32).
const cashAddrCharset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

// cashAddrPolymod computes the BCH checksum for cashaddr.
func cashAddrPolymod(values []uint64) uint64 {
	gen := [5]uint64{
		0x98f2bc8e61,
		0x79b76d99e2,
		0xf33e5fb3c4,
		0xae2eabe2a8,
		0x1e4f43e470,
	}
	c := uint64(1)
	for _, v := range values {
		b := c >> 35
		c = ((c & 0x07ffffffff) << 5) ^ v
		for i := 0; i < 5; i++ {
			if (b>>uint(i))&1 == 1 {
				c ^= gen[i]
			}
		}
	}
	return c ^ 1
}

// cashAddrPrefixExpand expands the prefix for checksum computation.
func cashAddrPrefixExpand(prefix string) []uint64 {
	ret := make([]uint64, 0, len(prefix)+1)
	for _, c := range prefix {
		ret = append(ret, uint64(c&0x1f))
	}
	ret = append(ret, 0)
	return ret
}

// cashAddrSizeBits returns the size bits for a given hash length.
func cashAddrSizeBits(hashLen int) (byte, error) {
	switch hashLen {
	case 20:
		return 0, nil
	case 24:
		return 1, nil
	case 28:
		return 2, nil
	case 32:
		return 3, nil
	case 40:
		return 4, nil
	case 48:
		return 5, nil
	case 56:
		return 6, nil
	case 64:
		return 7, nil
	default:
		return 0, fmt.Errorf("%w: %d", ErrInvalidCashAddrHash, hashLen)
	}
}

// CashAddrEncode encodes a cashaddr address with the given prefix, address type,
// and hash. For standard P2PKH with 20-byte hash, addrType should be CashAddrTypeP2PKH.
// The result includes the prefix (e.g., "bitcoincash:qp...").
func CashAddrEncode(prefix string, addrType byte, hash []byte) (string, error) {
	sizeBits, err := cashAddrSizeBits(len(hash))
	if err != nil {
		return "", err
	}

	// Version byte: (type << 3) | sizeBits
	versionByte := (addrType << 3) | sizeBits

	// Payload: version byte + hash
	payload := append([]byte{versionByte}, hash...)

	// Convert to 5-bit groups
	data, err := ConvertBits(payload, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("convert bits: %w", err)
	}

	// Compute checksum
	prefixExpanded := cashAddrPrefixExpand(prefix)
	checksumInput := make([]uint64, 0, len(prefixExpanded)+len(data)+8)
	checksumInput = append(checksumInput, prefixExpanded...)
	for _, d := range data {
		checksumInput = append(checksumInput, uint64(d))
	}
	// Add 8 zero bytes for checksum computation
	for range 8 {
		checksumInput = append(checksumInput, 0)
	}
	checksum := cashAddrPolymod(checksumInput)

	// Encode data + checksum
	var sb strings.Builder
	sb.Grow(len(prefix) + 1 + len(data) + 8)
	sb.WriteString(prefix)
	sb.WriteByte(':')
	for _, d := range data {
		sb.WriteByte(cashAddrCharset[d])
	}
	for i := 7; i >= 0; i-- {
		sb.WriteByte(cashAddrCharset[(checksum>>(uint(i)*5))&0x1f])
	}

	return sb.String(), nil
}

// CashAddrEncodeShort encodes a cashaddr address and returns it without
// the prefix (e.g., "qp..." instead of "bitcoincash:qp...").
func CashAddrEncodeShort(prefix string, addrType byte, hash []byte) (string, error) {
	full, err := CashAddrEncode(prefix, addrType, hash)
	if err != nil {
		return "", err
	}
	idx := strings.Index(full, ":")
	if idx < 0 {
		return full, nil
	}
	return full[idx+1:], nil
}
