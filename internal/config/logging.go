package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LogLevel represents logging verbosity levels.
type LogLevel int

// Log level constants.
const (
	LogLevelOff LogLevel = iota
	LogLevelError
	LogLevelDebug
)

// ParseLogLevel parses a log level string.
func ParseLogLevel(s string) LogLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "off", "none":
		return LogLevelOff
	case "error":
		return LogLevelError
	case "debug":
		return LogLevelDebug
	default:
		return LogLevelError
	}
}

// String returns the string representation of a log level.
func (l LogLevel) String() string {
	switch l {
	case LogLevelOff:
		return "off"
	case LogLevelError:
		return "error"
	case LogLevelDebug:
		return "debug"
	default:
		return "error"
	}
}

// Logger handles logging to a file.
type Logger struct {
	mu       sync.Mutex
	level    LogLevel
	file     *os.File
	filePath string
}

// NewLogger creates a new logger.
func NewLogger(level LogLevel, filePath string) (*Logger, error) {
	logger := &Logger{
		level:    level,
		filePath: filePath,
	}

	if level == LogLevelOff || filePath == "" {
		return logger, nil
	}

	// Expand home directory
	if strings.HasPrefix(filePath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		filePath = filepath.Join(home, filePath[2:])
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}

	// Open log file
	// #nosec G304 -- log file path is from validated config
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}

	logger.file = f
	logger.filePath = filePath

	return logger, nil
}

// Close closes the log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// SetLevel changes the log level.
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Level returns the current log level.
func (l *Logger) Level() LogLevel {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

// Debug logs a debug message.
func (l *Logger) Debug(format string, args ...any) {
	l.log(LogLevelDebug, format, args...)
}

// Error logs an error message.
func (l *Logger) Error(format string, args ...any) {
	l.log(LogLevelError, format, args...)
}

// Writer returns an io.Writer that writes to the logger at the specified level.
func (l *Logger) Writer(level LogLevel) io.Writer {
	return &logWriter{logger: l, level: level}
}

// log writes a log message if the level is appropriate.
func (l *Logger) log(level LogLevel, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.level == LogLevelOff || level > l.level || l.file == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	levelStr := strings.ToUpper(level.String())
	msg := fmt.Sprintf(format, args...)

	_, _ = fmt.Fprintf(l.file, "%s [%s] %s\n", timestamp, levelStr, msg)
}

// logWriter implements io.Writer for the logger.
type logWriter struct {
	logger *Logger
	level  LogLevel
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.logger.log(w.level, "%s", strings.TrimSpace(string(p)))
	return len(p), nil
}

// NullLogger returns a logger that discards all output.
func NullLogger() *Logger {
	return &Logger{level: LogLevelOff}
}
