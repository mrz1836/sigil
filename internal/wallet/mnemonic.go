// Package wallet provides wallet management functionality including
// BIP39 mnemonic generation, validation, and seed derivation.
package wallet

import (
	"errors"
	"regexp"
	"strings"

	"github.com/tyler-smith/go-bip39"
)

var (
	// ErrInvalidWordCount indicates the mnemonic must be 12 or 24 words.
	ErrInvalidWordCount = errors.New("word count must be 12 or 24")

	// ErrInvalidMnemonic indicates the mnemonic is not valid.
	ErrInvalidMnemonic = errors.New("invalid mnemonic phrase")

	// whitespaceRegex matches one or more whitespace characters.
	whitespaceRegex = regexp.MustCompile(`\s+`)
)

// GenerateMnemonic creates a new BIP39 mnemonic phrase.
// wordCount must be 12 (128 bits entropy) or 24 (256 bits entropy).
func GenerateMnemonic(wordCount int) (string, error) {
	var bitSize int
	switch wordCount {
	case 12:
		bitSize = 128
	case 24:
		bitSize = 256
	default:
		return "", ErrInvalidWordCount
	}

	entropy, err := bip39.NewEntropy(bitSize)
	if err != nil {
		return "", err
	}

	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", err
	}

	return mnemonic, nil
}

// ValidateMnemonic checks if a mnemonic phrase is valid according to BIP39.
// It verifies word count, word validity, and checksum.
func ValidateMnemonic(mnemonic string) error {
	if mnemonic == "" {
		return ErrInvalidMnemonic
	}

	normalized := NormalizeMnemonicInput(mnemonic)
	if !bip39.IsMnemonicValid(normalized) {
		return ErrInvalidMnemonic
	}

	return nil
}

// NormalizeMnemonicInput cleans and normalizes mnemonic input by:
// - Converting to lowercase
// - Trimming leading and trailing whitespace
// - Collapsing multiple whitespace characters to single spaces
func NormalizeMnemonicInput(input string) string {
	// Convert to lowercase
	input = strings.ToLower(input)

	// Replace all whitespace sequences with a single space
	input = whitespaceRegex.ReplaceAllString(input, " ")

	// Trim leading and trailing whitespace
	input = strings.TrimSpace(input)

	return input
}

// MnemonicToSeed converts a BIP39 mnemonic phrase to a 64-byte seed.
// The passphrase is optional (can be empty string).
// The returned seed should be handled securely and zeroed after use.
func MnemonicToSeed(mnemonic, passphrase string) ([]byte, error) {
	normalized := NormalizeMnemonicInput(mnemonic)

	if !bip39.IsMnemonicValid(normalized) {
		return nil, ErrInvalidMnemonic
	}

	seed := bip39.NewSeed(normalized, passphrase)
	return seed, nil
}

// GetWordList returns the BIP39 English word list.
func GetWordList() []string {
	return bip39.GetWordList()
}

// IsValidWord checks if a word is in the BIP39 word list.
func IsValidWord(word string) bool {
	word = strings.ToLower(word)
	for _, w := range bip39.GetWordList() {
		if w == word {
			return true
		}
	}
	return false
}
