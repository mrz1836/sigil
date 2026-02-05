package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/config"
	"github.com/mrz1836/sigil/internal/output"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// errTestRandom is used for testing non-sigil error handling.
var errTestRandom = sigilerr.New("TEST_ERROR", "some random error")

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		name string
		info BuildInfo
		want string
	}{
		{
			name: "all fields populated",
			info: BuildInfo{
				Version: "v1.2.3",
				Commit:  "abc1234",
				Date:    "2024-01-15",
			},
			want: "v1.2.3 (commit: abc1234, built: 2024-01-15)",
		},
		{
			name: "all fields empty",
			info: BuildInfo{},
			want: "dev (commit: unknown, built: unknown)",
		},
		{
			name: "only version empty",
			info: BuildInfo{
				Version: "",
				Commit:  "def5678",
				Date:    "2024-02-20",
			},
			want: "dev (commit: def5678, built: 2024-02-20)",
		},
		{
			name: "only commit empty",
			info: BuildInfo{
				Version: "v2.0.0",
				Commit:  "",
				Date:    "2024-03-25",
			},
			want: "v2.0.0 (commit: unknown, built: 2024-03-25)",
		},
		{
			name: "only date empty",
			info: BuildInfo{
				Version: "v3.0.0",
				Commit:  "ghi9012",
				Date:    "",
			},
			want: "v3.0.0 (commit: ghi9012, built: unknown)",
		},
		{
			name: "version and commit empty",
			info: BuildInfo{
				Version: "",
				Commit:  "",
				Date:    "2024-04-30",
			},
			want: "dev (commit: unknown, built: 2024-04-30)",
		},
		{
			name: "version and date empty",
			info: BuildInfo{
				Version: "",
				Commit:  "jkl3456",
				Date:    "",
			},
			want: "dev (commit: jkl3456, built: unknown)",
		},
		{
			name: "commit and date empty",
			info: BuildInfo{
				Version: "v4.0.0",
				Commit:  "",
				Date:    "",
			},
			want: "v4.0.0 (commit: unknown, built: unknown)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatVersion(tc.info)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error returns success",
			err:  nil,
			want: sigilerr.ExitSuccess,
		},
		{
			name: "general error",
			err:  sigilerr.ErrGeneral,
			want: sigilerr.ExitGeneral,
		},
		{
			name: "invalid input error",
			err:  sigilerr.ErrInvalidInput,
			want: sigilerr.ExitInput,
		},
		{
			name: "authentication error",
			err:  sigilerr.ErrAuthentication,
			want: sigilerr.ExitAuth,
		},
		{
			name: "not found error",
			err:  sigilerr.ErrNotFound,
			want: sigilerr.ExitNotFound,
		},
		{
			name: "permission error",
			err:  sigilerr.ErrPermission,
			want: sigilerr.ExitPermission,
		},
		{
			name: "insufficient funds error",
			err:  sigilerr.ErrInsufficientFunds,
			want: sigilerr.ExitPermission,
		},
		{
			name: "wallet not found error",
			err:  sigilerr.ErrWalletNotFound,
			want: sigilerr.ExitNotFound,
		},
		{
			name: "wallet exists error",
			err:  sigilerr.ErrWalletExists,
			want: sigilerr.ExitInput,
		},
		{
			name: "invalid mnemonic error",
			err:  sigilerr.ErrInvalidMnemonic,
			want: sigilerr.ExitInput,
		},
		{
			name: "decryption failed error",
			err:  sigilerr.ErrDecryptionFailed,
			want: sigilerr.ExitAuth,
		},
		{
			name: "network error",
			err:  sigilerr.ErrNetworkError,
			want: sigilerr.ExitGeneral,
		},
		{
			name: "config not found error",
			err:  sigilerr.ErrConfigNotFound,
			want: sigilerr.ExitNotFound,
		},
		{
			name: "non-sigil error returns general",
			err:  errTestRandom,
			want: sigilerr.ExitGeneral,
		},
		{
			name: "wrapped sigil error preserves exit code",
			err:  sigilerr.Wrap(sigilerr.ErrAuthentication, "failed to authenticate"),
			want: sigilerr.ExitAuth,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExitCode(tc.err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestGlobalGetters tests Config(), Logger(), Formatter(), Context() getters.
// NOT parallel: mutates package-level globals.
func TestGlobalGetters(t *testing.T) {
	// Save original values
	origCfg := cfg
	origLogger := logger
	origFormatter := formatter
	origCmdCtx := cmdCtx
	defer func() {
		cfg = origCfg
		logger = origLogger
		formatter = origFormatter
		cmdCtx = origCmdCtx
	}()

	testCfg := config.Defaults()
	testLogger := config.NullLogger()
	testFmt := output.NewFormatter(output.FormatText, nil)
	testCtx := &CommandContext{Cfg: testCfg}

	cfg = testCfg
	logger = testLogger
	formatter = testFmt
	cmdCtx = testCtx

	assert.Equal(t, testCfg, Config())
	assert.Equal(t, testLogger, Logger())
	assert.Equal(t, testFmt, Formatter())
	assert.Equal(t, testCtx, Context())
}

// TestCleanup_NilLogger verifies cleanup doesn't panic with nil logger.
func TestCleanup_NilLogger(t *testing.T) {
	origLogger := logger
	defer func() { logger = origLogger }()

	logger = nil
	assert.NotPanics(t, func() { cleanup() })
}

// TestCleanup_WithLogger verifies cleanup doesn't panic with a valid logger.
func TestCleanup_WithLogger(t *testing.T) {
	origLogger := logger
	defer func() { logger = origLogger }()

	logger = config.NullLogger()
	assert.NotPanics(t, func() { cleanup() })
}

// TestFormatErr_NilFormatter verifies formatErr with nil formatter doesn't panic.
func TestFormatErr_NilFormatter(t *testing.T) {
	origFormatter := formatter
	defer func() { formatter = origFormatter }()

	formatter = nil
	assert.NotPanics(t, func() { formatErr(sigilerr.ErrGeneral) })
}

// TestFormatErr_WithFormatter verifies formatErr with a valid formatter doesn't panic.
func TestFormatErr_WithFormatter(t *testing.T) {
	origFormatter := formatter
	defer func() { formatter = origFormatter }()

	formatter = output.NewFormatter(output.FormatText, nil)
	assert.NotPanics(t, func() { formatErr(sigilerr.ErrGeneral) })
}

// TestFormatErr_JSONFormat verifies formatErr with JSON formatter doesn't panic.
func TestFormatErr_JSONFormat(t *testing.T) {
	origFormatter := formatter
	defer func() { formatter = origFormatter }()

	formatter = output.NewFormatter(output.FormatJSON, nil)
	assert.NotPanics(t, func() { formatErr(sigilerr.ErrInvalidInput) })
}

// --- Tests for initGlobals ---

// saveGlobals saves all package-level globals and returns a restore function.
func saveGlobals(t *testing.T) func() {
	t.Helper()
	origCfg := cfg
	origLogger := logger
	origFormatter := formatter
	origCmdCtx := cmdCtx
	origHomeDir := homeDir
	origOutputFormat := outputFormat
	origVerbose := verbose
	return func() {
		cfg = origCfg
		logger = origLogger
		formatter = origFormatter
		cmdCtx = origCmdCtx
		homeDir = origHomeDir
		outputFormat = origOutputFormat
		verbose = origVerbose
	}
}

func TestInitGlobals_DefaultConfig(t *testing.T) {
	restore := saveGlobals(t)
	defer restore()

	tmpDir, err := os.MkdirTemp("", "sigil-initglobals-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Set homeDir to temp dir (no config file there)
	homeDir = tmpDir
	outputFormat = ""
	verbose = false

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err = initGlobals(cmd)
	require.NoError(t, err)

	// Verify globals are initialized
	require.NotNil(t, cfg, "cfg should be set")
	require.NotNil(t, logger, "logger should be set")
	require.NotNil(t, formatter, "formatter should be set")
	require.NotNil(t, cmdCtx, "cmdCtx should be set")

	assert.Equal(t, tmpDir, cfg.Home)
}

func TestInitGlobals_CustomHome(t *testing.T) {
	restore := saveGlobals(t)
	defer restore()

	tmpDir, err := os.MkdirTemp("", "sigil-initglobals-home")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	homeDir = tmpDir
	outputFormat = ""
	verbose = false

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err = initGlobals(cmd)
	require.NoError(t, err)

	assert.Equal(t, tmpDir, cfg.Home)
}

func TestInitGlobals_VerboseFlag(t *testing.T) {
	restore := saveGlobals(t)
	defer restore()

	tmpDir, err := os.MkdirTemp("", "sigil-initglobals-verbose")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	homeDir = tmpDir
	outputFormat = ""
	verbose = true

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err = initGlobals(cmd)
	require.NoError(t, err)

	assert.True(t, cfg.Output.Verbose)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

func TestInitGlobals_OutputFormatFlag(t *testing.T) {
	restore := saveGlobals(t)
	defer restore()

	tmpDir, err := os.MkdirTemp("", "sigil-initglobals-format")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	homeDir = tmpDir
	outputFormat = "json"
	verbose = false

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err = initGlobals(cmd)
	require.NoError(t, err)

	assert.Equal(t, "json", cfg.Output.DefaultFormat)
}

func TestInitGlobals_WithExistingConfig(t *testing.T) {
	restore := saveGlobals(t)
	defer restore()

	tmpDir, err := os.MkdirTemp("", "sigil-initglobals-existing")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a valid config file
	testCfg := config.Defaults()
	testCfg.Home = tmpDir
	testCfg.Logging.Level = "warn"
	configPath := config.Path(tmpDir)
	require.NoError(t, os.MkdirAll(tmpDir, 0o750))
	require.NoError(t, config.Save(testCfg, configPath))

	homeDir = tmpDir
	outputFormat = ""
	verbose = false

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err = initGlobals(cmd)
	require.NoError(t, err)

	assert.Equal(t, "warn", cfg.Logging.Level)
}

func TestInitGlobals_EnvHome(t *testing.T) {
	restore := saveGlobals(t)
	defer restore()

	tmpDir, err := os.MkdirTemp("", "sigil-initglobals-env")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Use env var instead of flag
	homeDir = ""
	outputFormat = ""
	verbose = false
	t.Setenv(config.EnvHome, tmpDir)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err = initGlobals(cmd)
	require.NoError(t, err)

	assert.Equal(t, tmpDir, cfg.Home)
}

// --- Tests for Execute ---

// TestCleanup_LoggerCloseError verifies cleanup doesn't panic when logger.Close() returns an error.
func TestCleanup_LoggerCloseError(t *testing.T) {
	origLogger := logger
	defer func() { logger = origLogger }()

	// Create a real logger with a temp file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	testLogger, err := config.NewLogger(config.ParseLogLevel("debug"), logPath)
	require.NoError(t, err)

	// Close the underlying file to force an error on the next Close()
	require.NoError(t, testLogger.Close())

	// Set the already-closed logger as the global
	logger = testLogger

	// cleanup() should not panic even though Close() will return an error
	assert.NotPanics(t, func() { cleanup() })
}

func TestExecute_VersionFlag(t *testing.T) {
	restore := saveGlobals(t)
	defer restore()

	// Reset rootCmd args for version test
	origArgs := os.Args
	os.Args = []string{"sigil", "version"}
	defer func() { os.Args = origArgs }()

	err := Execute(BuildInfo{Version: "v1.0.0-test", Commit: "abc", Date: "2026-01-01"})
	assert.NoError(t, err)
}
