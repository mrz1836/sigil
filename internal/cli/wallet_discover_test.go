package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		// String shorter than maxLen - no truncation
		{
			name:     "short string unchanged",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "empty string unchanged",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "exact length unchanged",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},

		// String longer than maxLen - truncated with ellipsis
		{
			name:     "long string truncated with ellipsis",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "long string truncated at 10",
			input:    "this is a very long string",
			maxLen:   10,
			expected: "this is...",
		},
		{
			name:     "address truncation typical use",
			input:    "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
			maxLen:   15,
			expected: "0x742d35Cc66...",
		},

		// Edge cases with small maxLen
		{
			name:     "maxLen 3 no ellipsis",
			input:    "hello",
			maxLen:   3,
			expected: "hel",
		},
		{
			name:     "maxLen 2 no ellipsis",
			input:    "hello",
			maxLen:   2,
			expected: "he",
		},
		{
			name:     "maxLen 1 no ellipsis",
			input:    "hello",
			maxLen:   1,
			expected: "h",
		},
		{
			name:     "maxLen 0 empty result",
			input:    "hello",
			maxLen:   0,
			expected: "",
		},

		// Edge case: maxLen 4 means only 1 char + "..."
		{
			name:     "maxLen 4 gives 1 char plus ellipsis",
			input:    "hello",
			maxLen:   4,
			expected: "h...",
		},

		// Unicode strings (bytes, not runes)
		{
			name:     "unicode string truncated by bytes",
			input:    "hello\xc2\xa9world", // hello©world
			maxLen:   8,
			expected: "hello...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := truncateString(tc.input, tc.maxLen)
			assert.Equal(t, tc.expected, result)
		})
	}
}
