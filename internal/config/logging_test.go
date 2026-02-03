package config_test

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/config"
)

func TestParseLogLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected config.LogLevel
	}{
		{"off lowercase", "off", config.LogLevelOff},
		{"off uppercase", "OFF", config.LogLevelOff},
		{"off mixed case", "Off", config.LogLevelOff},
		{"none", "none", config.LogLevelOff},
		{"error lowercase", "error", config.LogLevelError},
		{"error uppercase", "ERROR", config.LogLevelError},
		{"debug lowercase", "debug", config.LogLevelDebug},
		{"debug uppercase", "DEBUG", config.LogLevelDebug},
		{"with whitespace", "  debug  ", config.LogLevelDebug},
		{"invalid returns error", "invalid", config.LogLevelError},
		{"empty returns error", "", config.LogLevelError},
		{"unknown value", "warn", config.LogLevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := config.ParseLogLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogLevel_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		level    config.LogLevel
		expected string
	}{
		{"off", config.LogLevelOff, "off"},
		{"error", config.LogLevelError, "error"},
		{"debug", config.LogLevelDebug, "debug"},
		{"unknown defaults to error", config.LogLevel(99), "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.level.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewLogger_LevelOff(t *testing.T) {
	t.Parallel()
	logger, err := config.NewLogger(config.LogLevelOff, "")
	require.NoError(t, err)
	require.NotNil(t, logger)
	defer func() { _ = logger.Close() }()

	assert.Equal(t, config.LogLevelOff, logger.Level())
}

func TestNewLogger_EmptyPath(t *testing.T) {
	t.Parallel()
	logger, err := config.NewLogger(config.LogLevelDebug, "")
	require.NoError(t, err)
	require.NotNil(t, logger)
	defer func() { _ = logger.Close() }()

	// Should not panic when logging with no file
	logger.Debug("test message")
	logger.Error("test error")
}

func TestNewLogger_ValidPath(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelDebug, logPath)
	require.NoError(t, err)
	require.NotNil(t, logger)
	defer func() { _ = logger.Close() }()

	logger.Debug("debug message")
	logger.Error("error message")

	// Verify file was created and has content
	content := readLogFile(t, logPath)
	assert.Contains(t, string(content), "debug message")
	assert.Contains(t, string(content), "error message")
}

func TestNewLogger_TildeExpansion(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	// Create a subdirectory to simulate home
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(homeDir, 0o750))

	// We can't easily test tilde expansion in isolation since it uses os.UserHomeDir()
	// Instead, test that tilde paths work when valid
	logger, err := config.NewLogger(config.LogLevelOff, "~/test.log")
	require.NoError(t, err)
	require.NotNil(t, logger)
	require.NoError(t, logger.Close())
}

func TestNewLogger_CreatesDirectory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "subdir", "deep", "test.log")

	logger, err := config.NewLogger(config.LogLevelDebug, logPath)
	require.NoError(t, err)
	require.NotNil(t, logger)
	defer func() { _ = logger.Close() }()

	// Verify directory was created
	info, err := os.Stat(filepath.Dir(logPath))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNewLogger_InvalidPath(t *testing.T) {
	t.Parallel()
	// Use a path that cannot be created
	_, err := config.NewLogger(config.LogLevelDebug, "/proc/nonexistent/test.log")
	assert.Error(t, err)
}

func TestNullLogger(t *testing.T) {
	t.Parallel()
	logger := config.NullLogger()
	require.NotNil(t, logger)

	assert.Equal(t, config.LogLevelOff, logger.Level())

	// Should not panic when logging
	logger.Debug("test debug")
	logger.Error("test error")

	// Close should not panic
	assert.NoError(t, logger.Close())
}

func TestLogger_Close_NilFile(t *testing.T) {
	t.Parallel()
	logger := config.NullLogger()
	err := logger.Close()
	assert.NoError(t, err)
}

func TestLogger_Close_ValidFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelDebug, logPath)
	require.NoError(t, err)

	err = logger.Close()
	assert.NoError(t, err)
}

func TestLogger_SetLevel_Level(t *testing.T) {
	t.Parallel()
	logger := config.NullLogger()

	logger.SetLevel(config.LogLevelDebug)
	assert.Equal(t, config.LogLevelDebug, logger.Level())

	logger.SetLevel(config.LogLevelError)
	assert.Equal(t, config.LogLevelError, logger.Level())

	logger.SetLevel(config.LogLevelOff)
	assert.Equal(t, config.LogLevelOff, logger.Level())
}

func TestLogger_Debug_LevelFiltering(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelError, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	// Debug should not be logged at error level
	logger.Debug("debug message")

	content := readLogFile(t, logPath)
	assert.NotContains(t, string(content), "debug message")
}

func TestLogger_Error_LevelFiltering(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelError, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	// Error should be logged at error level
	logger.Error("error message")

	content := readLogFile(t, logPath)
	assert.Contains(t, string(content), "error message")
}

func TestLogger_Debug_WithArgs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelDebug, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.Debug("value: %d, string: %s", 42, "hello")

	content := readLogFile(t, logPath)
	assert.Contains(t, string(content), "value: 42, string: hello")
}

func TestLogger_Error_WithArgs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelError, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.Error("error code: %d", 500)

	content := readLogFile(t, logPath)
	assert.Contains(t, string(content), "error code: 500")
}

func TestLogger_Writer(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelDebug, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	writer := logger.Writer(config.LogLevelDebug)
	require.NotNil(t, writer)

	n, err := writer.Write([]byte("written via io.Writer"))
	require.NoError(t, err)
	assert.Equal(t, len("written via io.Writer"), n)

	content := readLogFile(t, logPath)
	assert.Contains(t, string(content), "written via io.Writer")
}

func TestLogger_Writer_LevelFiltering(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelError, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	writer := logger.Writer(config.LogLevelDebug)
	_, err = writer.Write([]byte("debug via writer"))
	require.NoError(t, err)

	content := readLogFile(t, logPath)
	assert.NotContains(t, string(content), "debug via writer")
}

func TestLogger_SetJSONOutput_Structured(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelDebug, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.SetJSONOutput(true)

	slogger := logger.Structured()
	require.NotNil(t, slogger)

	slogger.Info("structured message", "key", "value")

	content := readLogFile(t, logPath)
	assert.Contains(t, string(content), "structured message")
	assert.Contains(t, string(content), "key")
	assert.Contains(t, string(content), "value")
}

func TestLogger_Structured_NilWhenDisabled(t *testing.T) {
	t.Parallel()
	logger := config.NullLogger()

	// Structured should return nil when logger has no file
	slogger := logger.Structured()
	assert.Nil(t, slogger)
}

func TestLogger_DebugAttrs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelDebug, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.DebugAttrs("debug with attrs",
		slog.String("key1", "value1"),
		slog.Int("key2", 42),
	)

	content := readLogFile(t, logPath)
	assert.Contains(t, string(content), "debug with attrs")
	assert.Contains(t, string(content), "key1")
	assert.Contains(t, string(content), "value1")
	assert.Contains(t, string(content), "key2")
	assert.Contains(t, string(content), "42")
}

func TestLogger_DebugAttrs_LevelFiltering(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelError, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	// Debug attrs should not be logged at error level
	logger.DebugAttrs("debug attrs", slog.String("key", "value"))

	content := readLogFile(t, logPath)
	assert.NotContains(t, string(content), "debug attrs")
}

func TestLogger_DebugAttrs_NilLogger(t *testing.T) {
	t.Parallel()
	logger := config.NullLogger()

	// Should not panic
	logger.DebugAttrs("test", slog.String("key", "value"))
}

func TestLogger_ErrorAttrs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelError, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.ErrorAttrs("error with attrs",
		slog.String("error_code", "E001"),
		slog.Bool("fatal", true),
	)

	content := readLogFile(t, logPath)
	assert.Contains(t, string(content), "error with attrs")
	assert.Contains(t, string(content), "error_code")
	assert.Contains(t, string(content), "E001")
	assert.Contains(t, string(content), "fatal")
	assert.Contains(t, string(content), "true")
}

func TestLogger_ErrorAttrs_LevelOff(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelOff, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	// Should not panic or log anything
	logger.ErrorAttrs("error attrs", slog.String("key", "value"))

	// File shouldn't exist since level is off
	_, err = os.Stat(logPath)
	assert.True(t, os.IsNotExist(err))
}

func TestLogger_ErrorAttrs_NilLogger(t *testing.T) {
	t.Parallel()
	logger := config.NullLogger()

	// Should not panic
	logger.ErrorAttrs("test", slog.String("key", "value"))
}

func TestNewStructuredLogger(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewStructuredLogger(config.LogLevelDebug, logPath)
	require.NoError(t, err)
	require.NotNil(t, logger)
	defer func() { _ = logger.Close() }()

	slogger := logger.Structured()
	require.NotNil(t, slogger)

	slogger.Info("json structured message", "key", "value")

	content := readLogFile(t, logPath)
	// JSON output should contain the message
	contentStr := string(content)
	assert.Contains(t, contentStr, "json structured message")
	// JSON format should have curly braces
	assert.Contains(t, contentStr, "{")
}

func TestNewStructuredLogger_InvalidPath(t *testing.T) {
	t.Parallel()
	_, err := config.NewStructuredLogger(config.LogLevelDebug, "/proc/nonexistent/test.log")
	assert.Error(t, err)
}

func TestLogger_LogFormat(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelDebug, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.Debug("test message")

	content := readLogFile(t, logPath)

	// Verify log format: timestamp [LEVEL] message
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Len(t, lines, 1)

	// Should contain [DEBUG] and the message
	assert.Contains(t, lines[0], "[DEBUG]")
	assert.Contains(t, lines[0], "test message")
}

func TestLogger_LevelOff_NoLogging(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create with level off
	logger, err := config.NewLogger(config.LogLevelOff, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	logger.Debug("debug")
	logger.Error("error")

	// File should not be created when level is off
	_, err = os.Stat(logPath)
	assert.True(t, os.IsNotExist(err))
}

func TestLogger_Concurrent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelDebug, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	// Test concurrent access
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			logger.Debug("message %d", n)
			logger.Error("error %d", n)
			_ = logger.Level()
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	content := readLogFile(t, logPath)
	// Should have at least some messages logged
	assert.NotEmpty(t, content)
}

func TestLogger_Writer_Interface(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := config.NewLogger(config.LogLevelDebug, logPath)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	writer := logger.Writer(config.LogLevelDebug)

	// Verify it implements io.Writer by using it
	require.Implements(t, (*io.Writer)(nil), writer)

	// Test with io.Copy
	src := bytes.NewBufferString("copied via io")
	_, err = io.Copy(writer, src)
	require.NoError(t, err)

	content := readLogFile(t, logPath)
	assert.Contains(t, string(content), "copied via io")
}

// readLogFile is a test helper that reads a log file.
// #nosec G304 -- test helper with controlled paths from t.TempDir()
func readLogFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}
