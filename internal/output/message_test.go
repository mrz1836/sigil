package output_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
)

// captureStdout redirects os.Stdout for the duration of fn and returns what was written.
// It is not safe for parallel use because it mutates the process-global os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	require.NoError(t, w.Close())

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}

// captureStderr is captureStdout for os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()
	require.NoError(t, w.Close())

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}

func TestInfo(t *testing.T) {
	out := captureStdout(t, func() { output.Info("hello") })
	assert.Contains(t, out, "hello")
	assert.Contains(t, out, "ℹ️")
	assert.Contains(t, out, "\n")
}

func TestInfof(t *testing.T) {
	out := captureStdout(t, func() { output.Infof("count=%d name=%s", 3, "abc") })
	assert.Contains(t, out, "count=3 name=abc")
	assert.Contains(t, out, "ℹ️")
}

func TestWarn(t *testing.T) {
	// Warnings go to stderr.
	out := captureStderr(t, func() { output.Warn("be careful") })
	assert.Contains(t, out, "be careful")
	assert.Contains(t, out, "⚠️")
}

func TestWarnf(t *testing.T) {
	out := captureStderr(t, func() { output.Warnf("retry %d of %d", 2, 5) })
	assert.Contains(t, out, "retry 2 of 5")
	assert.Contains(t, out, "⚠️")
}

func TestSuccess(t *testing.T) {
	out := captureStdout(t, func() { output.Success("done") })
	assert.Contains(t, out, "done")
	assert.Contains(t, out, "✅")
}

func TestSuccessf(t *testing.T) {
	out := captureStdout(t, func() { output.Successf("saved %s", "wallet.json") })
	assert.Contains(t, out, "saved wallet.json")
	assert.Contains(t, out, "✅")
}
