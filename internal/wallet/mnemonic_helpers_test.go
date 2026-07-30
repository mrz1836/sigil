package wallet

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetWordList(t *testing.T) {
	t.Parallel()

	list := GetWordList()
	// BIP39 English word list is exactly 2048 words, sorted.
	assert.Len(t, list, 2048)
	assert.Equal(t, "abandon", list[0])
	assert.Equal(t, "zoo", list[2047])
}

func TestItoa(t *testing.T) {
	t.Parallel()

	tests := map[int]string{
		0:    "0",
		1:    "1",
		9:    "9",
		10:   "10",
		42:   "42",
		100:  "100",
		2048: "2048",
		-7:   "-7",
		-100: "-100",
	}
	for in, want := range tests {
		assert.Equal(t, want, itoa(in), "itoa(%d)", in)
	}
}

func TestFormatTypoSuggestions(t *testing.T) {
	t.Parallel()

	// No typos → empty string.
	assert.Empty(t, FormatTypoSuggestions(nil))
	assert.Empty(t, FormatTypoSuggestions([]TypoInfo{}))

	typos := []TypoInfo{
		{Index: 0, Word: "abandonn", Suggestion: "abandon", Distance: 1},
		{Index: 4, Word: "zzzzz", Suggestion: "", Distance: 0},
	}
	out := FormatTypoSuggestions(typos)

	// Word positions are rendered 1-indexed for humans.
	assert.Contains(t, out, "Word 1: 'abandonn' - did you mean 'abandon'?")
	// No suggestion → explicit "not a valid BIP39 word" message.
	assert.Contains(t, out, "Word 5: 'zzzzz' is not a valid BIP39 word")
	// Entries are newline-separated (one separator for two entries).
	assert.Equal(t, 1, strings.Count(out, "\n"))
}
