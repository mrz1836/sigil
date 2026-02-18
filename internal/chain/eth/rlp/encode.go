// Package rlp provides minimal RLP (Recursive Length Prefix) encoding for Ethereum transactions.
// This implements only the encoding needed for transaction serialization.
// See: https://ethereum.org/en/developers/docs/data-structures-and-encoding/rlp/
package rlp

import (
	"math/big"
)

// Encode encodes a value to RLP format.
// Supported types: []byte, *big.Int, uint64, []any (for lists).
func Encode(val any) []byte {
	switch v := val.(type) {
	case []byte:
		return encodeBytes(v)
	case *big.Int:
		return encodeBigInt(v)
	case uint64:
		return encodeUint64(v)
	case []any:
		return encodeList(v)
	default:
		return nil
	}
}

// encodeBytes encodes a byte slice.
// - For a single byte in [0x00, 0x7f], the byte is its own RLP encoding.
// - For 0-55 bytes, prefix with (0x80 + length).
// - For >55 bytes, prefix with (0xb7 + length of length) followed by length.
func encodeBytes(b []byte) []byte {
	if len(b) == 1 && b[0] < 0x80 {
		return b
	}
	return concat(encodeLength(len(b), 0x80), b)
}

// encodeBigInt encodes a big.Int as RLP bytes.
// Zero is encoded as empty bytes (0x80).
// Negative values are not supported.
func encodeBigInt(i *big.Int) []byte {
	if i == nil || i.Sign() == 0 {
		return []byte{0x80}
	}
	return encodeBytes(i.Bytes())
}

// encodeUint64 encodes a uint64 as RLP bytes.
func encodeUint64(i uint64) []byte {
	if i == 0 {
		return []byte{0x80}
	}
	return encodeBytes(bigEndianBytes(i))
}

// encodeList encodes a list of items.
// - For 0-55 total bytes, prefix with (0xc0 + length).
// - For >55 bytes, prefix with (0xf7 + length of length) followed by length.
func encodeList(items []any) []byte {
	// Encode all items first to know total size
	encodedItems := make([][]byte, len(items))
	totalLen := 0
	for i, item := range items {
		encodedItems[i] = Encode(item)
		totalLen += len(encodedItems[i])
	}

	// Preallocate content with known size
	content := make([]byte, 0, totalLen)
	for _, encoded := range encodedItems {
		content = append(content, encoded...)
	}
	return concat(encodeLength(len(content), 0xc0), content)
}

// encodeLength encodes the length prefix for strings (offset=0x80) or lists (offset=0xc0).
func encodeLength(length int, offset byte) []byte {
	if length < 56 {
		return []byte{offset + byte(length)} //nolint:gosec // G115: length < 56, safe conversion
	}

	// For lengths >= 56, encode the length as big-endian bytes
	lenBytes := bigEndianBytes(uint64(length))
	return append([]byte{offset + 55 + byte(len(lenBytes))}, lenBytes...) //nolint:gosec // G115: len(lenBytes) <= 8 for any uint64
}

// bigEndianBytes converts a uint64 to minimal big-endian bytes (no leading zeros).
func bigEndianBytes(i uint64) []byte {
	if i == 0 {
		return nil
	}

	// Find the number of significant bytes
	n := 0
	for v := i; v > 0; v >>= 8 {
		n++
	}

	result := make([]byte, n)
	for j := n - 1; j >= 0; j-- {
		result[j] = byte(i)
		i >>= 8
	}
	return result
}

// concat concatenates byte slices.
func concat(slices ...[]byte) []byte {
	// Calculate total length
	totalLen := 0
	for _, s := range slices {
		totalLen += len(s)
	}

	// Preallocate result
	result := make([]byte, 0, totalLen)
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}

// EncodeTransaction encodes an Ethereum legacy transaction for signing or broadcasting.
// Fields: nonce, gasPrice, gasLimit, to, value, data.
// For signing (unsigned tx), pass empty v, r, s.
// For broadcast (signed tx), pass the signature components.
func EncodeTransaction(
	nonce uint64,
	gasPrice *big.Int,
	gasLimit uint64,
	to []byte, // 20 bytes or nil for contract creation
	value *big.Int,
	data []byte,
	v, r, s *big.Int, // Signature components (nil for unsigned)
) []byte {
	items := []any{
		nonce,
		gasPrice,
		gasLimit,
		to,
		value,
		data,
	}

	// Add signature components if provided
	if v != nil {
		items = append(items, v, r, s)
	}

	return Encode(items)
}

// EncodeTransactionForSigning encodes a transaction for EIP-155 signing.
// This includes the chain ID in the transaction data for replay protection.
func EncodeTransactionForSigning(
	nonce uint64,
	gasPrice *big.Int,
	gasLimit uint64,
	to []byte,
	value *big.Int,
	data []byte,
	chainID *big.Int,
) []byte {
	items := []any{
		nonce,
		gasPrice,
		gasLimit,
		to,
		value,
		data,
		chainID,
		uint64(0), // Empty r
		uint64(0), // Empty s
	}

	return Encode(items)
}
