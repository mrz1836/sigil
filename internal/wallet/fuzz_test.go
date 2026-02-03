package wallet

import (
	"testing"
	"unicode/utf8"
)

// FuzzNormalizeMnemonicInput tests that normalization never panics and always
// returns valid UTF-8 output.
//
//nolint:gocognit // Fuzz tests need comprehensive validation
func FuzzNormalizeMnemonicInput(f *testing.F) {
	// Seed with various interesting inputs
	f.Add("")
	f.Add("abandon")
	f.Add("  abandon  abandon  ")
	f.Add("ABANDON ABILITY")
	f.Add("\t\n\r abandon \t ability \n")
	f.Add("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	f.Add(string([]byte{0xFF, 0xFE})) // Invalid UTF-8

	f.Fuzz(func(t *testing.T, input string) {
		result := NormalizeMnemonicInput(input)

		// Result should be valid UTF-8
		if !utf8.ValidString(result) {
			t.Errorf("NormalizeMnemonicInput returned invalid UTF-8 for input %q", input)
		}

		// Result should not have leading/trailing whitespace
		if len(result) > 0 && (result[0] == ' ' || result[len(result)-1] == ' ') {
			t.Errorf("NormalizeMnemonicInput returned string with leading/trailing whitespace for input %q", input)
		}

		// Result should be lowercase
		hasUpper := false
		for _, r := range result {
			if r >= 'A' && r <= 'Z' {
				hasUpper = true
				break
			}
		}
		if hasUpper {
			t.Errorf("NormalizeMnemonicInput returned uppercase character for input %q", input)
		}
	})
}

// FuzzValidateMnemonic tests that mnemonic validation never panics
// and only returns nil for valid BIP39 mnemonics.
func FuzzValidateMnemonic(f *testing.F) {
	// Valid 12-word mnemonic
	f.Add("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	// Invalid inputs
	f.Add("")
	f.Add("abandon")
	f.Add("invalid mnemonic phrase with many words that should fail validation")
	f.Add("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon") // wrong checksum
	f.Add("   ")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		err := ValidateMnemonic(input)

		// If validation passes, verify it's actually a valid mnemonic
		if err == nil {
			// The only valid responses are 12 or 24 word mnemonics
			normalized := NormalizeMnemonicInput(input)
			words := len(splitWords(normalized))
			if words != 12 && words != 24 {
				t.Errorf("ValidateMnemonic returned nil for non-12/24 word input: %q (words: %d)", input, words)
			}
		}
	})
}

// FuzzSuggestWord tests that word suggestion never panics
// and returns reasonable suggestions for near-matches.
func FuzzSuggestWord(f *testing.F) {
	// Valid words
	f.Add("abandon")
	f.Add("ability")
	f.Add("zoo")
	// Near-typos (intentional misspellings for testing)
	f.Add("abondon")  //nolint:misspell // intentional typo
	f.Add("abaility") // intentional typo
	f.Add("zooo")     // intentional typo
	// Random strings
	f.Add("")
	f.Add("xyz")
	f.Add("verylongwordthatdoesnotexistinthewordlist")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		suggestion := SuggestWord(input)

		// If we got a suggestion, verify it's a valid BIP39 word
		if suggestion != "" && !IsValidWord(suggestion) {
			t.Errorf("SuggestWord returned invalid word %q for input %q", suggestion, input)
		}
	})
}

// FuzzDetectTypos tests that typo detection never panics
// and returns reasonable results.
//
//nolint:gocognit // Fuzz tests need comprehensive validation
func FuzzDetectTypos(f *testing.F) {
	f.Add("")
	f.Add("abandon ability")
	f.Add("abondon abaility") //nolint:misspell // intentional typos
	f.Add("abandon abaility") // intentional typo
	f.Add("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		typos := DetectTypos(input)

		// Verify each typo result
		for _, typo := range typos {
			if typo.Index < 0 {
				t.Errorf("DetectTypos returned negative index for input %q", input)
			}
			if typo.Word == "" {
				t.Errorf("DetectTypos returned empty word for input %q", input)
			}
			if typo.Suggestion != "" && !IsValidWord(typo.Suggestion) {
				t.Errorf("DetectTypos returned invalid suggestion %q for input %q", typo.Suggestion, input)
			}
		}
	})
}

// FuzzDetectInputFormat tests that input format detection never panics.
func FuzzDetectInputFormat(f *testing.F) {
	// Mnemonic
	f.Add("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	// WIF (uncompressed)
	f.Add("5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ")
	// WIF (compressed)
	f.Add("KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617")
	// Hex key
	f.Add("0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d")
	f.Add("0x0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d")
	// Invalid
	f.Add("")
	f.Add("invalid")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		format := DetectInputFormat(input)

		// Result should be a valid format
		switch format {
		case FormatUnknown, FormatMnemonic, FormatWIF, FormatHex:
			// Valid
		default:
			t.Errorf("DetectInputFormat returned invalid format %d for input %q", format, input)
		}
	})
}

// FuzzParseWIF tests that WIF parsing never panics.
func FuzzParseWIF(f *testing.F) {
	// Valid WIF (uncompressed mainnet)
	f.Add("5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTJ")
	// Valid WIF (compressed mainnet)
	f.Add("KwdMAjGmerYanjeui5SHS7JkmpZvVipYvB2LJGU1ZxJwYvP98617")
	f.Add("L5oLkpV3aqBjhki6LmvChTCV6odsp4SXM6FfU2Gppt5kFLaHLuZ9")
	// Invalid
	f.Add("")
	f.Add("5")
	f.Add("invalid")
	f.Add("5HueCGU8rMjxEXxiPuD5BDku4MkFqeZyd4dZ1jvhTVqvbTLvyTK") // Wrong checksum
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		key, err := ParseWIF(input)

		if err == nil && len(key) != 32 {
			t.Errorf("ParseWIF returned key with wrong length for input %q: got %d, want 32", input, len(key))
		}
	})
}

// FuzzParseHexKey tests that hex key parsing never panics.
func FuzzParseHexKey(f *testing.F) {
	// Valid hex key
	f.Add("0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d")
	f.Add("0x0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d")
	// Invalid
	f.Add("")
	f.Add("0x")
	f.Add("0c28fca386c7a227600b2fe50b7cae11")                                   // Too short
	f.Add("0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aa1d00") // Too long
	f.Add("0c28fca386c7a227600b2fe50b7cae11ec86d3bf1fbe471be89827e19d72aagg")   // Invalid hex
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		key, err := ParseHexKey(input)

		if err == nil && len(key) != 32 {
			t.Errorf("ParseHexKey returned key with wrong length for input %q: got %d, want 32", input, len(key))
		}
	})
}

// splitWords splits a string into words on whitespace.
func splitWords(s string) []string {
	if s == "" {
		return nil
	}
	var words []string
	word := ""
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if word != "" {
				words = append(words, word)
				word = ""
			}
		} else {
			word += string(r)
		}
	}
	if word != "" {
		words = append(words, word)
	}
	return words
}
