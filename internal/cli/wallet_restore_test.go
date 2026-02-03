package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestDisplayDetectedTypos_NoTypos(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	// Valid 12-word mnemonic
	displayDetectedTypos("abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", cmd)

	assert.Empty(t, buf.String(), "expected no output for valid mnemonic")
}

func TestDisplayDetectedTypos_EmptyMnemonic(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	displayDetectedTypos("", cmd)

	assert.Empty(t, buf.String(), "expected no output for empty mnemonic")
}

func TestDisplayDetectedTypos_WithTypo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	// "abandn" is a typo of "abandon"
	displayDetectedTypos("abandn abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", cmd)

	output := buf.String()
	assert.NotEmpty(t, output, "expected output for typo")
	assert.Contains(t, output, "Possible typos detected")
	assert.Contains(t, output, "Word 1:")
	assert.Contains(t, output, "abandn")
}

func TestDisplayDetectedTypos_InvalidWord(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	// "zzzzzzz" is not close to any BIP39 word
	displayDetectedTypos("zzzzzzz abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", cmd)

	output := buf.String()
	assert.NotEmpty(t, output, "expected output for invalid word")
	assert.Contains(t, output, "Possible typos detected")
	assert.Contains(t, output, "zzzzzzz")
	assert.Contains(t, output, "not a valid BIP39 word")
}

func TestDisplayDetectedTypos_ValidZooMnemonic(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	// All valid BIP39 words
	displayDetectedTypos("zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong", cmd)

	assert.Empty(t, buf.String(), "expected no output for valid mnemonic")
}
