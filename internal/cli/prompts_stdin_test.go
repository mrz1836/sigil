package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// The mnemonic below is the canonical BIP39 all-"abandon" public test vector.
const testMnemonic12 = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"

// feedStdinPipe replaces os.Stdin with a pipe pre-loaded with input and registers
// restoration via t.Cleanup. These tests mutate the global os.Stdin, so callers
// must NOT run in parallel.
func feedStdinPipe(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, werr := w.WriteString(input)
	require.NoError(t, werr)
	require.NoError(t, w.Close())

	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origStdin
		_ = r.Close()
	})
}

// silenceStderrToDevNull redirects os.Stderr to /dev/null so prompt banners do not
// clutter test output. Restored via t.Cleanup. Not parallel-safe.
func silenceStderrToDevNull(t *testing.T) {
	t.Helper()
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	require.NoError(t, err)
	origStderr := os.Stderr
	os.Stderr = devnull
	t.Cleanup(func() {
		os.Stderr = origStderr
		_ = devnull.Close()
	})
}

// TestPromptConfirmationStdin_Yes drives the real promptConfirmation over a piped
// stdin. Both "y" and "yes" (case-insensitive) confirm. Not parallel: mutates os.Stdin.
func TestPromptConfirmationStdin_Yes(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"lowercase y", "y\n"},
		{"word yes", "yes\n"},
		{"uppercase Y", "Y\n"},
		{"mixed YES", "YES\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			silenceStderrToDevNull(t)
			feedStdinPipe(t, tc.input)
			assert.True(t, promptConfirmation(), "input %q should confirm", tc.input)
		})
	}
}

// TestPromptConfirmationStdin_No covers the rejection branches, including an empty
// (immediate EOF) stream where fmt.Scanln errors and the function returns false.
func TestPromptConfirmationStdin_No(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"lowercase n", "n\n"},
		{"word no", "no\n"},
		{"random", "maybe\n"},
		{"empty EOF", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			silenceStderrToDevNull(t)
			feedStdinPipe(t, tc.input)
			assert.False(t, promptConfirmation(), "input %q should not confirm", tc.input)
		})
	}
}

// TestPromptSeedMaterialStdin_SingleToken exercises the happy path where fmt.Scanln
// reads a single whitespace-delimited token followed by a newline and returns it.
func TestPromptSeedMaterialStdin_SingleToken(t *testing.T) {
	silenceStderrToDevNull(t)
	feedStdinPipe(t, "seedmaterialtoken\n")

	got, err := promptSeedMaterial()
	require.NoError(t, err)
	assert.Equal(t, "seedmaterialtoken", got)
}

// TestPromptMnemonicInteractiveStdin_TwelveWords feeds a valid 12-word mnemonic and
// asserts the assembled phrase is returned once validation succeeds at word 12.
func TestPromptMnemonicInteractiveStdin_TwelveWords(t *testing.T) {
	silenceStderrToDevNull(t)
	feedStdinPipe(t, testMnemonic12+"\n")

	got, err := promptMnemonicInteractive()
	require.NoError(t, err)
	assert.Equal(t, testMnemonic12, got)
}

// TestPromptMnemonicInteractiveStdin_Partial feeds fewer words than a valid mnemonic;
// on EOF the loop breaks and returns whatever words were collected, without error.
func TestPromptMnemonicInteractiveStdin_Partial(t *testing.T) {
	silenceStderrToDevNull(t)
	feedStdinPipe(t, "hello world\n")

	got, err := promptMnemonicInteractive()
	require.NoError(t, err)
	assert.Equal(t, "hello world", got)
}

// TestPromptMnemonicInteractiveStdin_NoInput covers the empty-stream branch where no
// words are read and an ErrInvalidInput ("no input provided") is returned.
func TestPromptMnemonicInteractiveStdin_NoInput(t *testing.T) {
	silenceStderrToDevNull(t)
	feedStdinPipe(t, "")

	got, err := promptMnemonicInteractive()
	require.Error(t, err)
	assert.Empty(t, got)
	assert.ErrorIs(t, err, sigilerr.ErrInvalidInput)
}
