package output

import (
	"fmt"
	"os"
)

// Info prints an informational message to stdout with an info prefix.
func Info(msg string) {
	_, _ = fmt.Fprintln(os.Stdout, "ℹ️  "+msg)
}

// Infof prints a formatted informational message to stdout.
func Infof(format string, args ...any) {
	Info(fmt.Sprintf(format, args...))
}

// Warn prints a warning message to stderr with a warning prefix.
func Warn(msg string) {
	_, _ = fmt.Fprintln(os.Stderr, "⚠️  "+msg)
}

// Warnf prints a formatted warning message to stderr.
func Warnf(format string, args ...any) {
	Warn(fmt.Sprintf(format, args...))
}

// Success prints a success message to stdout with a success prefix.
func Success(msg string) {
	_, _ = fmt.Fprintln(os.Stdout, "✅ "+msg)
}

// Successf prints a formatted success message to stdout.
func Successf(format string, args ...any) {
	Success(fmt.Sprintf(format, args...))
}
