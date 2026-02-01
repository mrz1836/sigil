// Package wallet provides wallet management functionality including
// BIP39 mnemonic generation, validation, and seed derivation.
package wallet

import (
	"errors"
	"math"
	"regexp"
	"strings"

	"github.com/agnivade/levenshtein"
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

// MaxTypoDistance is the maximum Levenshtein distance to consider a suggestion.
// Words with distance > 2 are considered too different to suggest.
const MaxTypoDistance = 2

// TypoInfo contains information about a detected typo and its suggestion.
type TypoInfo struct {
	// Index is the word position in the mnemonic (0-based).
	Index int
	// Word is the original (possibly misspelled) word.
	Word string
	// Suggestion is the closest BIP39 word, or empty if none found.
	Suggestion string
	// Distance is the Levenshtein distance to the suggestion.
	Distance int
}

// SuggestWord finds the closest BIP39 word to the input using Levenshtein distance.
// Returns empty string if no word is close enough (distance > MaxTypoDistance).
func SuggestWord(input string) string {
	input = strings.ToLower(input)
	wordList := bip39.GetWordList()

	minDist := math.MaxInt
	var suggestion string

	for _, word := range wordList {
		dist := levenshtein.ComputeDistance(input, word)
		if dist < minDist {
			minDist = dist
			suggestion = word
		}
		// Early exit for exact match
		if dist == 0 {
			return word
		}
	}

	if minDist <= MaxTypoDistance {
		return suggestion
	}
	return ""
}

// DetectTypos scans a mnemonic phrase and returns information about detected typos.
// It identifies words that are not in the BIP39 word list and suggests corrections.
func DetectTypos(mnemonic string) []TypoInfo {
	if mnemonic == "" {
		return nil
	}

	normalized := NormalizeMnemonicInput(mnemonic)
	words := strings.Fields(normalized)
	var typos []TypoInfo

	for i, word := range words {
		if !IsValidWord(word) {
			suggestion := SuggestWord(word)
			distance := 0
			if suggestion != "" {
				distance = levenshtein.ComputeDistance(word, suggestion)
			}
			typos = append(typos, TypoInfo{
				Index:      i,
				Word:       word,
				Suggestion: suggestion,
				Distance:   distance,
			})
		}
	}

	return typos
}

// FormatTypoSuggestions formats typo information into human-readable suggestions.
func FormatTypoSuggestions(typos []TypoInfo) string {
	if len(typos) == 0 {
		return ""
	}

	var lines []string
	for _, typo := range typos {
		// Word position is 1-indexed for human readability
		wordNum := typo.Index + 1
		var line string
		if typo.Suggestion != "" {
			line = "Word " + itoa(wordNum) + ": '" + typo.Word + "' - did you mean '" + typo.Suggestion + "'?"
		} else {
			line = "Word " + itoa(wordNum) + ": '" + typo.Word + "' is not a valid BIP39 word"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// itoa converts an int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
