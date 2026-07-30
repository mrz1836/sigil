package output_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
)

// stringerValue exercises the fmt.Stringer branch of printText.
type stringerValue struct{ label string }

func (s stringerValue) String() string { return "STRINGER:" + s.label }

func TestFormatter_Print_TextBranches(t *testing.T) {
	t.Parallel()

	t.Run("stringer value uses String()", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		f := output.NewFormatter(output.FormatText, &buf)
		require.NoError(t, f.Print(stringerValue{label: "hi"}))
		assert.Equal(t, "STRINGER:hi\n", buf.String())
	})

	t.Run("non-string non-stringer uses %v", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		f := output.NewFormatter(output.FormatText, &buf)
		require.NoError(t, f.Print(42))
		assert.Equal(t, "42\n", buf.String())
	})

	t.Run("slice default formatting", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		f := output.NewFormatter(output.FormatText, &buf)
		require.NoError(t, f.Print([]int{1, 2, 3}))
		assert.Equal(t, "[1 2 3]\n", buf.String())
	})
}
