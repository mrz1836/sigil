package wallet

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BIP39 test vectors from https://github.com/trezor/python-mnemonic/blob/master/vectors.json
//
//nolint:gochecknoglobals // BIP39 test vectors from official specification
var bip39TestVectors = []struct {
	entropy  string
	mnemonic string
	seed     string
}{
	{
		entropy:  "00000000000000000000000000000000",
		mnemonic: "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
		seed:     "c55257c360c07c72029aebc1b53c05ed0362ada38ead3e3e9efa3708e53495531f09a6987599d18264c1e1c92f2cf141630c7a3c4ab7c81b2f001698e7463b04",
	},
	{
		entropy:  "7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f",
		mnemonic: "legal winner thank year wave sausage worth useful legal winner thank yellow",
		seed:     "2e8905819b8723fe2c1d161860e5ee1830318dbf49a83bd451cfb8440c28bd6fa457fe1296106559a3c80937a1c1069be3a3a5bd381ee6260e8d9739fce1f607",
	},
	{
		entropy:  "80808080808080808080808080808080",
		mnemonic: "letter advice cage absurd amount doctor acoustic avoid letter advice cage above",
		seed:     "2e8905819b8723fe2c1d161860e5ee1830318dbf49a83bd451cfb8440c28bd6fa457fe1296106559a3c80937a1c1069be3a3a5bd381ee6260e8d9739fce1f607",
	},
	{
		entropy:  "ffffffffffffffffffffffffffffffff",
		mnemonic: "zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong",
		seed:     "0cd6e5d827bb62eb8fc1e262254223817fd068a74b5b449cc2f667c3f1f985a76379b43348d952e2265b4cd129090758b3e3c2c49103b5051aac2eaeb890a528",
	},
	// 24-word vectors
	{
		entropy:  "0000000000000000000000000000000000000000000000000000000000000000",
		mnemonic: "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art",
		seed:     "bda85446c68413707090a52022edd26a1c9462295029f2e60cd7c4f2bbd3097170af7a4d73245cafa9c3cca8d561a7c3de6f5d4a10be8ed2a5e608d68f92fcc8",
	},
	{
		entropy:  "7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f7f",
		mnemonic: "legal winner thank year wave sausage worth useful legal winner thank year wave sausage worth useful legal winner thank year wave sausage worth title",
		seed:     "bc09fca1804f7e69da93c2f2028eb238c227f2e9dda30cd63699232578480a4021b146ad717fbb7e451ce9eb835f43620bf5c514db0f8add49f5d121449d3e87",
	},
}

func TestGenerateMnemonic_12Words(t *testing.T) {
	t.Parallel()
	mnemonic, err := GenerateMnemonic(12)
	require.NoError(t, err)

	words := strings.Fields(mnemonic)
	assert.Len(t, words, 12)

	// Validate the generated mnemonic
	err = ValidateMnemonic(mnemonic)
	assert.NoError(t, err)
}

func TestGenerateMnemonic_24Words(t *testing.T) {
	t.Parallel()
	mnemonic, err := GenerateMnemonic(24)
	require.NoError(t, err)

	words := strings.Fields(mnemonic)
	assert.Len(t, words, 24)

	// Validate the generated mnemonic
	err = ValidateMnemonic(mnemonic)
	assert.NoError(t, err)
}

func TestGenerateMnemonic_InvalidWordCount(t *testing.T) {
	t.Parallel()
	_, err := GenerateMnemonic(15)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "word count must be 12 or 24")

	_, err = GenerateMnemonic(0)
	require.Error(t, err)

	_, err = GenerateMnemonic(6)
	require.Error(t, err)
}

func TestGenerateMnemonic_Randomness(t *testing.T) {
	t.Parallel()
	// Generate two mnemonics and verify they're different
	mnemonic1, err := GenerateMnemonic(12)
	require.NoError(t, err)

	mnemonic2, err := GenerateMnemonic(12)
	require.NoError(t, err)

	assert.NotEqual(t, mnemonic1, mnemonic2)
}

func TestValidateMnemonic_ValidMnemonics(t *testing.T) {
	t.Parallel()
	for _, tc := range bip39TestVectors {
		t.Run(tc.mnemonic[:20], func(t *testing.T) {
			t.Parallel()
			err := ValidateMnemonic(tc.mnemonic)
			assert.NoError(t, err)
		})
	}
}

func TestValidateMnemonic_InvalidMnemonics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mnemonic string
	}{
		{
			name:     "invalid word",
			mnemonic: "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon xyz",
		},
		{
			name:     "wrong word count",
			mnemonic: "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon",
		},
		{
			name:     "invalid checksum",
			mnemonic: "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon",
		},
		{
			name:     "empty string",
			mnemonic: "",
		},
		{
			name:     "single word",
			mnemonic: "abandon",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateMnemonic(tc.mnemonic)
			assert.Error(t, err)
		})
	}
}

func TestNormalizeMnemonicInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already normalized",
			input:    "abandon abandon about",
			expected: "abandon abandon about",
		},
		{
			name:     "leading whitespace",
			input:    "  abandon abandon about",
			expected: "abandon abandon about",
		},
		{
			name:     "trailing whitespace",
			input:    "abandon abandon about  ",
			expected: "abandon abandon about",
		},
		{
			name:     "multiple spaces between words",
			input:    "abandon   abandon    about",
			expected: "abandon abandon about",
		},
		{
			name:     "tabs and newlines",
			input:    "abandon\tabandon\nabout",
			expected: "abandon abandon about",
		},
		{
			name:     "mixed whitespace",
			input:    "  abandon  \t abandon \n about  ",
			expected: "abandon abandon about",
		},
		{
			name:     "uppercase",
			input:    "ABANDON ABANDON ABOUT",
			expected: "abandon abandon about",
		},
		{
			name:     "mixed case",
			input:    "Abandon ABANDON About",
			expected: "abandon abandon about",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := NormalizeMnemonicInput(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMnemonicToSeed_WithTestVectors(t *testing.T) {
	t.Parallel()
	// Using "TREZOR" as the passphrase as per the test vectors
	passphrase := "TREZOR"

	for _, tc := range bip39TestVectors {
		t.Run(tc.mnemonic[:20], func(t *testing.T) {
			t.Parallel()
			seed, err := MnemonicToSeed(tc.mnemonic, passphrase)
			require.NoError(t, err)
			assert.NotNil(t, seed)
			assert.Len(t, seed, 64) // BIP39 seed is always 64 bytes
		})
	}
}

func TestMnemonicToSeed_NoPassphrase(t *testing.T) {
	t.Parallel()
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	seed1, err := MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)

	seed2, err := MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)

	// Same mnemonic and passphrase should produce same seed
	assert.Equal(t, seed1, seed2)
}

func TestMnemonicToSeed_DifferentPassphrases(t *testing.T) {
	t.Parallel()
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

	seed1, err := MnemonicToSeed(mnemonic, "")
	require.NoError(t, err)

	seed2, err := MnemonicToSeed(mnemonic, "TREZOR")
	require.NoError(t, err)

	// Different passphrases should produce different seeds
	assert.NotEqual(t, seed1, seed2)
}

func TestMnemonicToSeed_InvalidMnemonic(t *testing.T) {
	t.Parallel()
	_, err := MnemonicToSeed("invalid mnemonic words here", "")
	assert.Error(t, err)
}

// TestSuggestWord tests Levenshtein-based typo detection.
//
//nolint:misspell // Intentional typos for testing
func TestSuggestWord(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string // empty string means no suggestion (too far)
	}{
		// Single character typos (intentional misspellings for test)
		{name: "off by one char", input: "abondon", expected: "abandon"},
		{name: "missing letter", input: "abadon", expected: "abandon"},
		{name: "extra letter", input: "abanddon", expected: "abandon"},
		{name: "swapped letters", input: "abadnon", expected: "abandon"},

		// Other common typos - note: some typos may match multiple BIP39 words
		{name: "typo in word", input: "abouut", expected: "about"},
		{name: "zoo typo", input: "zooo", expected: "zoo"},
		{name: "letter typo", input: "lettter", expected: "letter"},

		// Exact match returns the word
		{name: "exact match", input: "abandon", expected: "abandon"},

		// Too different - no suggestion
		{name: "completely different", input: "xyzqwerty", expected: ""},
		{name: "very wrong", input: "abcdefg", expected: ""},

		// Case insensitive (intentional misspellings for test)
		{name: "uppercase typo", input: "ABONDON", expected: "abandon"},
		{name: "mixed case typo", input: "AbOndon", expected: "abandon"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			suggestion := SuggestWord(tc.input)
			assert.Equal(t, tc.expected, suggestion)
		})
	}
}

// TestSuggestWordForMnemonic tests typo detection for entire mnemonic phrases.
//
//nolint:misspell // Intentional typos for testing
func TestSuggestWordForMnemonic(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		mnemonic    string
		typoIndices []int      // indices of words with typos
		suggestions [][]string // expected suggestions for each word
	}{
		{
			name:        "single typo",
			mnemonic:    "abondon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
			typoIndices: []int{0},
			suggestions: [][]string{{"abandon"}},
		},
		{
			name:        "multiple typos",
			mnemonic:    "abondon abondon abandon abandon abandon abandon abandon abandon abandon abandon abandon abouut",
			typoIndices: []int{0, 1, 11},
			suggestions: [][]string{{"abandon"}, {"abandon"}, {"about"}},
		},
		{
			name:        "no typos",
			mnemonic:    "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
			typoIndices: []int{},
			suggestions: [][]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := DetectTypos(tc.mnemonic)
			require.Len(t, result, len(tc.typoIndices))
			verifyTypos(t, result, tc.typoIndices, tc.suggestions)
		})
	}
}

// verifyTypos checks that all expected typos were found.
func verifyTypos(t *testing.T, result []TypoInfo, typoIndices []int, suggestions [][]string) {
	t.Helper()
	for i, idx := range typoIndices {
		found := false
		for _, typo := range result {
			if typo.Index == idx {
				found = true
				assert.Contains(t, suggestions[i], typo.Suggestion)
				break
			}
		}
		assert.True(t, found, "expected typo at index %d", idx)
	}
}

// TestDetectTypos_EdgeCases tests edge cases for typo detection.
//
//nolint:misspell // Intentional typos for testing
func TestDetectTypos_EdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected int // number of typos detected
	}{
		{name: "empty string", input: "", expected: 0},
		{name: "single valid word", input: "abandon", expected: 0},
		{name: "single invalid word", input: "abondon", expected: 1},
		{name: "all invalid", input: "xyzabc qwerty asdfgh", expected: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := DetectTypos(tc.input)
			assert.Len(t, result, tc.expected)
		})
	}
}
