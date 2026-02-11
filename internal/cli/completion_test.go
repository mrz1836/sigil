package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompletion_Bash tests bash completion script generation.
func TestCompletion_Bash(t *testing.T) {
	var buf bytes.Buffer

	// Call the completion generator directly with the root command
	err := rootCmd.GenBashCompletion(&buf)
	require.NoError(t, err)

	// Verify output is not empty
	output := buf.String()
	assert.NotEmpty(t, output, "bash completion should generate output")
	assert.Contains(t, output, "bash", "completion should mention bash")
}

// TestCompletion_Zsh tests zsh completion script generation.
func TestCompletion_Zsh(t *testing.T) {
	var buf bytes.Buffer

	// Call the completion generator directly
	err := rootCmd.GenZshCompletion(&buf)
	require.NoError(t, err)

	// Verify output is not empty
	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Greater(t, len(output), 10, "zsh completion should have content")
}

// TestCompletion_Fish tests fish completion script generation.
func TestCompletion_Fish(t *testing.T) {
	var buf bytes.Buffer

	// Call the completion generator directly
	err := rootCmd.GenFishCompletion(&buf, true)
	require.NoError(t, err)

	// Verify output is not empty
	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "complete") // fish uses 'complete' command
}

// TestCompletion_PowerShell tests powershell completion script generation.
func TestCompletion_PowerShell(t *testing.T) {
	var buf bytes.Buffer

	// Call the completion generator directly
	err := rootCmd.GenPowerShellCompletionWithDesc(&buf)
	require.NoError(t, err)

	// Verify output is not empty
	output := buf.String()
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "Register-ArgumentCompleter") // PowerShell completion marker
}

// TestCompletion_Command tests the completion command handles valid shells.
func TestCompletion_Command(t *testing.T) {
	tests := []struct {
		name  string
		shell string
	}{
		{"bash", "bash"},
		{"zsh", "zsh"},
		{"fish", "fish"},
		{"powershell", "powershell"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			// Redirect stdout temporarily
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run completion command logic directly
			var err error
			switch tt.shell {
			case "bash":
				err = rootCmd.GenBashCompletion(&buf)
			case "zsh":
				err = rootCmd.GenZshCompletion(&buf)
			case "fish":
				err = rootCmd.GenFishCompletion(&buf, true)
			case "powershell":
				err = rootCmd.GenPowerShellCompletionWithDesc(&buf)
			}

			// Restore stdout
			_ = w.Close()
			os.Stdout = oldStdout
			_ = r.Close()

			require.NoError(t, err, "completion generation should succeed for %s", tt.shell)
			assert.NotEmpty(t, buf.String(), "completion output should not be empty for %s", tt.shell)
		})
	}
}
