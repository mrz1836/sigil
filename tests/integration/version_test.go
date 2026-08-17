//go:build integration

// Package integration: version_test covers cmd/sigil/main.go, which otherwise
// has no tests. It exercises the ldflags version injection (main.version /
// main.commit / main.buildDate) and the cli.Execute / cli.ExitCode wiring by
// building and running a real binary — the only faithful way to test an
// os.Exit-based entry point.
package integration

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildSigilWithLDFlags builds a sigil binary into a temp dir with the given
// -ldflags string and returns its path. It drives the same main.version/commit/
// buildDate injection path the release build uses.
func buildSigilWithLDFlags(t *testing.T, ldflags string) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	binPath := filepath.Join(t.TempDir(), "sigil-version-test")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	//nolint:gosec // G204: build args are test-controlled constants
	cmd := exec.CommandContext(ctx, "go", "build", "-ldflags", ldflags, "-o", binPath, "./cmd/sigil")
	cmd.Dir = projectRoot
	if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
		t.Fatalf("building sigil with ldflags %q failed: %v\n%s", ldflags, buildErr, out)
	}
	return binPath
}

// runBinary runs an already-built binary with args, returning combined output
// and the process exit code.
func runBinary(t *testing.T, bin string, args ...string) (combined string, exitCode int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//nolint:gosec // G204: binary path is test-controlled
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	if err != nil {
		t.Fatalf("running %s %v: %v", bin, args, err)
	}
	return string(out), 0
}

// TestVersionLDFlagInjection verifies that values injected via -ldflags into
// main.version/commit/buildDate propagate through cli.Execute to --version.
func TestVersionLDFlagInjection(t *testing.T) {
	const (
		wantVersion = "v9.9.9-testinjection"
		wantCommit  = "deadbeefcafe"
		wantDate    = "2026-08-17T12:00:00Z"
	)
	ldflags := strings.Join([]string{
		"-X main.version=" + wantVersion,
		"-X main.commit=" + wantCommit,
		"-X main.buildDate=" + wantDate,
	}, " ")

	bin := buildSigilWithLDFlags(t, ldflags)

	out, code := runBinary(t, bin, "--version")
	if code != 0 {
		t.Fatalf("--version exit code = %d, want 0; output:\n%s", code, out)
	}
	for _, want := range []string{wantVersion, wantCommit, wantDate} {
		if !strings.Contains(out, want) {
			t.Errorf("--version output missing %q; got:\n%s", want, out)
		}
	}
}

// TestExitCodeWiring verifies cli.Execute + cli.ExitCode map command outcomes to
// the expected process exit codes through main().
func TestExitCodeWiring(t *testing.T) {
	t.Run("unknown command exits general error (1)", func(t *testing.T) {
		_, _, code := runSigil(t, "this-command-does-not-exist")
		if code != 1 {
			t.Errorf("unknown command exit code = %d, want 1", code)
		}
	})

	t.Run("help exits success (0)", func(t *testing.T) {
		_, _, code := runSigil(t, "--help")
		if code != 0 {
			t.Errorf("--help exit code = %d, want 0", code)
		}
	})

	t.Run("version exits success with defaults", func(t *testing.T) {
		stdout, _, code := runSigil(t, "--version")
		if code != 0 {
			t.Errorf("--version exit code = %d, want 0", code)
		}
		// The shared binary is built without ldflags, so defaults apply.
		if !strings.Contains(stdout, "dev") {
			t.Errorf("--version output missing default version 'dev'; got: %s", stdout)
		}
		if !strings.Contains(stdout, "unknown") {
			t.Errorf("--version output missing default commit/date 'unknown'; got: %s", stdout)
		}
	})
}
