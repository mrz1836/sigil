package wallet

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/mrz1836/go-sanitize"

	"github.com/mrz1836/sigil/internal/wallet/bitcoin"
)

// InputFormat represents the detected format of restore input.
type InputFormat int

const (
	// FormatUnknown indicates the input format could not be determined.
	FormatUnknown InputFormat = iota
	// FormatMnemonic indicates a BIP39 mnemonic phrase.
	FormatMnemonic
	// FormatWIF indicates a Wallet Import Format private key.
	FormatWIF
	// FormatHex indicates a hex-encoded private key.
	FormatHex
)

// String returns the string representation of the input format.
func (f InputFormat) String() string {
	switch f {
	case FormatUnknown:
		return "unknown"
	case FormatMnemonic:
		return "mnemonic"
	case FormatWIF:
		return "wif"
	case FormatHex:
		return "hex"
	default:
		return "unknown"
	}
}

var (
	// ErrInvalidWIF indicates the WIF is not valid.
	ErrInvalidWIF = errors.New("invalid WIF format")
	// ErrInvalidHexKey indicates the hex key is not valid.
	ErrInvalidHexKey = errors.New("invalid hex private key")
)

// DetectInputFormat auto-detects the format of wallet restore input.
// It checks for mnemonic phrases, WIF keys, and hex keys.
func DetectInputFormat(input string) InputFormat {
	input = strings.TrimSpace(input)
	if input == "" {
		return FormatUnknown
	}

	if isMnemonicFormat(input) {
		return FormatMnemonic
	}

	if isWIFFormat(input) {
		return FormatWIF
	}

	if isHexKeyFormat(input) {
		return FormatHex
	}

	return FormatUnknown
}

// isMnemonicFormat checks if input looks like a BIP39 mnemonic.
func isMnemonicFormat(input string) bool {
	normalized := NormalizeMnemonicInput(input)
	words := strings.Fields(normalized)

	if len(words) != 12 && len(words) != 24 {
		return false
	}

	// Count valid BIP39 words
	validCount := 0
	for _, word := range words {
		if IsValidWord(word) {
			validCount++
		}
	}

	// If most words are valid BIP39 words, treat as mnemonic
	return validCount >= len(words)/2
}

// isWIFFormat checks if input looks like a WIF private key.
func isWIFFormat(input string) bool {
	if len(input) < 51 || len(input) > 52 {
		return false
	}

	first := input[0]
	if first != '5' && first != 'K' && first != 'L' {
		return false
	}

	return isBase58String(input)
}

// isHexKeyFormat checks if input looks like a hex private key.
func isHexKeyFormat(input string) bool {
	hexInput := input
	if strings.HasPrefix(input, "0x") || strings.HasPrefix(input, "0X") {
		hexInput = input[2:]
	}
	return len(hexInput) == 64 && isHexString(hexInput)
}

// ParseWIF parses a WIF-encoded private key and returns the raw bytes.
func ParseWIF(wif string) ([]byte, error) {
	if wif == "" {
		return nil, ErrInvalidWIF
	}

	wif = strings.TrimSpace(wif)

	// Decode Base58Check
	decoded, err := bitcoin.Base58CheckDecode(wif)
	if err != nil {
		return nil, ErrInvalidWIF
	}

	// WIF format: version byte + 32 byte key + optional compression flag
	// Uncompressed: 1 + 32 = 33 bytes
	// Compressed: 1 + 32 + 1 = 34 bytes
	if len(decoded) != 33 && len(decoded) != 34 {
		return nil, ErrInvalidWIF
	}

	// Version byte should be 0x80 for mainnet
	if decoded[0] != 0x80 {
		return nil, ErrInvalidWIF
	}

	// Extract the 32-byte private key
	key := decoded[1:33]
	return key, nil
}

// ParseHexKey parses a hex-encoded private key and returns the raw bytes.
func ParseHexKey(hexKey string) ([]byte, error) {
	if hexKey == "" {
		return nil, ErrInvalidHexKey
	}

	hexKey = strings.TrimSpace(hexKey)

	// Strip 0x prefix if present
	if strings.HasPrefix(hexKey, "0x") || strings.HasPrefix(hexKey, "0X") {
		hexKey = hexKey[2:]
	}

	// Must be exactly 64 hex characters (32 bytes)
	if len(hexKey) != 64 {
		return nil, ErrInvalidHexKey
	}

	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, ErrInvalidHexKey
	}

	return key, nil
}

// isBase58String checks if a string contains only Base58 characters.
func isBase58String(s string) bool {
	const base58Chars = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, c := range s {
		if !strings.ContainsRune(base58Chars, c) {
			return false
		}
	}
	return true
}

// isHexString checks if a string contains only hex characters.
func isHexString(s string) bool {
	for _, c := range s {
		isDigit := c >= '0' && c <= '9'
		isLowerHex := c >= 'a' && c <= 'f'
		isUpperHex := c >= 'A' && c <= 'F'
		if !isDigit && !isLowerHex && !isUpperHex {
			return false
		}
	}
	return true
}

// SanitizeBitcoinAddress cleans user input for Bitcoin/BSV address validation.
// It removes any characters that are not valid Base58Check characters (Bitcoin alphabet).
// This is useful for cleaning user input that may contain spaces, invalid characters,
// or copy-paste artifacts before validation.
func SanitizeBitcoinAddress(input string) string {
	return sanitize.BitcoinAddress(strings.TrimSpace(input))
}

// SanitizeWIF cleans user input for WIF private key validation.
// It uses the same Base58Check character set as Bitcoin addresses.
func SanitizeWIF(input string) string {
	return sanitize.BitcoinAddress(strings.TrimSpace(input))
}
