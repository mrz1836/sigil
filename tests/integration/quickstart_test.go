//go:build integration

// Package integration provides end-to-end integration tests for Sigil.
// These tests verify the complete user workflow as documented in quickstart.md.
//
// Run with: go test -tags=integration ./tests/integration/...
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testHome is a temporary directory for test data.
//
//nolint:gochecknoglobals // TestMain requires globals for shared test state
var testHome string

// sigilBinary is the path to the sigil binary.
//
//nolint:gochecknoglobals // TestMain requires globals for shared test state
var sigilBinary string

func TestMain(m *testing.M) {
	// Get the project root (two directories up from tests/integration)
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "..", "..")

	// Build the binary with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	//nolint:gosec // G204: Binary path is controlled by test environment
	buildCmd := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(cwd, "sigil-test"), "./cmd/sigil")
	buildCmd.Dir = projectRoot
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		panic("failed to build sigil binary: " + err.Error() + "\nOutput: " + string(output))
	}

	// Get absolute path to binary
	sigilBinary = filepath.Join(cwd, "sigil-test")

	// Create temp home
	testHome, err = os.MkdirTemp("", "sigil-integration-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}

	// Run tests
	code := m.Run()

	// Cleanup
	_ = os.RemoveAll(testHome)
	_ = os.Remove(sigilBinary)

	os.Exit(code)
}

// runSigil executes the sigil CLI with the given arguments.
func runSigil(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	// Always add --home flag
	fullArgs := append([]string{"--home", testHome}, args...)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//nolint:gosec // G204: Binary path is controlled by test environment
	cmd := exec.CommandContext(ctx, sigilBinary, fullArgs...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		exitCode = -1
	}

	return stdout, stderr, exitCode
}

// TestQuickstartWorkflow tests the complete quickstart.md workflow.
//
//nolint:gocognit,gocyclo // Integration tests require comprehensive step-by-step validation
func TestQuickstartWorkflow(t *testing.T) {
	// Step 1: Initialize configuration
	t.Run("config init", func(t *testing.T) {
		stdout, _, exitCode := runSigil(t, "config", "init")
		if exitCode != 0 {
			t.Fatalf("config init failed with exit code %d: %s", exitCode, stdout)
		}
		if !strings.Contains(stdout, "Configuration initialized") {
			t.Errorf("expected 'Configuration initialized' in output, got: %s", stdout)
		}

		// Verify config file exists
		configPath := filepath.Join(testHome, "config.yaml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Error("config.yaml was not created")
		}
	})

	// Step 2: List wallets (empty)
	t.Run("wallet list empty", func(t *testing.T) {
		stdout, _, exitCode := runSigil(t, "wallet", "list")
		if exitCode != 0 {
			t.Fatalf("wallet list failed with exit code %d", exitCode)
		}
		if !strings.Contains(stdout, "No wallets found") && !strings.Contains(stdout, "[]") {
			t.Errorf("expected empty wallet list message, got: %s", stdout)
		}
	})

	// Step 3: Config show
	// In non-TTY (piped stdout), auto-format outputs JSON.
	t.Run("config show", func(t *testing.T) {
		stdout, _, exitCode := runSigil(t, "config", "show")
		if exitCode != 0 {
			t.Fatalf("config show failed with exit code %d", exitCode)
		}
		if !strings.Contains(stdout, `"version"`) {
			t.Errorf("expected config output with version, got: %s", stdout)
		}
	})

	// Step 4: Config get/set
	t.Run("config get and set", func(t *testing.T) {
		// Set a value
		stdout, _, exitCode := runSigil(t, "config", "set", "output.verbose", "true")
		if exitCode != 0 {
			t.Fatalf("config set failed with exit code %d: %s", exitCode, stdout)
		}

		// Get the value
		stdout, _, exitCode = runSigil(t, "config", "get", "output.verbose")
		if exitCode != 0 {
			t.Fatalf("config get failed with exit code %d", exitCode)
		}
		if !strings.Contains(stdout, "true") {
			t.Errorf("expected 'true' in output, got: %s", stdout)
		}
	})

	// Step 5: Version command
	t.Run("version", func(t *testing.T) {
		stdout, stderr, exitCode := runSigil(t, "version")
		// Version output might go to stderr if there's initialization
		combined := stdout + stderr
		if exitCode != 0 {
			t.Fatalf("version failed with exit code %d, stdout: %s, stderr: %s", exitCode, stdout, stderr)
		}
		if !strings.Contains(combined, "version") {
			t.Errorf("expected version in output, got stdout: %s, stderr: %s", stdout, stderr)
		}
	})

	// Step 6: Version JSON output
	t.Run("version json", func(t *testing.T) {
		stdout, stderr, exitCode := runSigil(t, "version", "-o", "json")
		// Version might output to stderr if stdout is not available
		combined := stdout + stderr
		if exitCode != 0 {
			t.Fatalf("version -o json failed with exit code %d, stdout: %s, stderr: %s", exitCode, stdout, stderr)
		}

		// Try parsing combined output
		var v map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(combined)), &v); err != nil {
			t.Errorf("version output is not valid JSON: %s (stdout: %s, stderr: %s)", combined, stdout, stderr)
		} else if _, ok := v["version"]; !ok {
			t.Errorf("JSON output missing 'version' field: %s", combined)
		}
	})

	// Step 7: Help commands
	t.Run("help commands", func(t *testing.T) {
		commands := []string{
			"--help",
			"wallet --help",
			"wallet create --help",
			"balance --help",
			"tx --help",
			"config --help",
			"backup --help",
		}

		for _, cmdArgs := range commands {
			args := strings.Fields(cmdArgs)
			stdout, _, exitCode := runSigil(t, args...)
			if exitCode != 0 {
				t.Errorf("help for '%s' failed with exit code %d", cmdArgs, exitCode)
			}
			if !strings.Contains(stdout, "Usage:") && !strings.Contains(stdout, "Available Commands:") {
				t.Errorf("expected help output for '%s', got: %s", cmdArgs, stdout)
			}
		}
	})

	// Step 8: Completion scripts
	t.Run("completion scripts", func(t *testing.T) {
		shells := []string{"bash", "zsh", "fish"}
		for _, shell := range shells {
			stdout, _, exitCode := runSigil(t, "completion", shell)
			if exitCode != 0 {
				t.Errorf("completion %s failed with exit code %d", shell, exitCode)
			}
			if len(stdout) < 100 {
				t.Errorf("completion %s output too short: %d bytes", shell, len(stdout))
			}
		}
	})

	// Step 9: Error handling - wallet not found
	t.Run("error wallet not found", func(t *testing.T) {
		_, stderr, exitCode := runSigil(t, "wallet", "show", "nonexistent")
		if exitCode != 4 { // ExitNotFound
			t.Errorf("expected exit code 4 for wallet not found, got %d", exitCode)
		}
		if !strings.Contains(stderr, "WALLET_NOT_FOUND") {
			t.Errorf("expected WALLET_NOT_FOUND error, got: %s", stderr)
		}
	})

	// Step 10: Error handling - invalid command
	t.Run("error invalid command", func(t *testing.T) {
		_, _, exitCode := runSigil(t, "invalidcmd")
		if exitCode != 1 { // ExitGeneral
			t.Errorf("expected exit code 1 for invalid command, got %d", exitCode)
		}
	})
}

// TestJSONOutput tests JSON output format across various commands.
func TestJSONOutput(t *testing.T) {
	t.Run("wallet list json", func(t *testing.T) {
		stdout, _, exitCode := runSigil(t, "wallet", "list", "-o", "json")
		if exitCode != 0 {
			t.Fatalf("wallet list json failed with exit code %d", exitCode)
		}

		var list []interface{}
		if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &list); err != nil {
			t.Errorf("wallet list output is not valid JSON array: %s (error: %v)", stdout, err)
		}
	})

	t.Run("config show json", func(t *testing.T) {
		stdout, _, exitCode := runSigil(t, "config", "show")
		if exitCode != 0 {
			t.Fatalf("config show failed with exit code %d", exitCode)
		}
		// In non-TTY (piped stdout), auto-format outputs JSON.
		if !strings.Contains(stdout, `"version"`) || !strings.Contains(stdout, `"networks"`) {
			t.Errorf("config show should contain config fields, got: %s", stdout)
		}
	})
}

// TestExitCodes verifies correct exit codes for various error conditions.
func TestExitCodes(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{
			name:     "success - help",
			args:     []string{"--help"},
			wantCode: 0,
		},
		{
			name:     "success - version",
			args:     []string{"version"},
			wantCode: 0,
		},
		{
			name:     "general error - unknown command",
			args:     []string{"unknowncmd"},
			wantCode: 1,
		},
		{
			name:     "not found - wallet show nonexistent",
			args:     []string{"wallet", "show", "nonexistent"},
			wantCode: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, exitCode := runSigil(t, tc.args...)
			if exitCode != tc.wantCode {
				t.Errorf("expected exit code %d, got %d", tc.wantCode, exitCode)
			}
		})
	}
}
