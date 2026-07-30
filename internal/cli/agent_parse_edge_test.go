package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseDuration_HourErrors covers the "h"-suffix error branch (non-numeric
// and non-positive hours), which the primary duration table does not exercise.
func TestParseDuration_HourErrors(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"0h", "-3h", "xh"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			got, err := parseDuration(input)
			require.Error(t, err)
			assert.Zero(t, got)
		})
	}
}

// TestParseSatAmount_ErrorBranches covers the malformed-number error paths for
// the "sat" suffix and the decimal-BSV whole/fraction parts.
func TestParseSatAmount_ErrorBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "non-numeric sat suffix", input: "abcsat"},
		{name: "invalid whole part", input: "x.5"},
		{name: "invalid fraction part", input: "1.xy"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseSatAmount(tc.input)
			require.Error(t, err)
			assert.Zero(t, got)
		})
	}
}
