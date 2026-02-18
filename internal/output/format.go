// Package output provides output formatting for Sigil CLI.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Format represents the output format.
type Format string

// Output format constants.
const (
	FormatText Format = "text"
	FormatJSON Format = "json"
	FormatAuto Format = "auto"
)

// Formatter handles output formatting.
type Formatter struct {
	format Format
	writer io.Writer
}

// NewFormatter creates a new formatter with the specified format.
func NewFormatter(format Format, w io.Writer) *Formatter {
	return &Formatter{
		format: format,
		writer: w,
	}
}

// Format returns the current output format.
func (f *Formatter) Format() Format {
	return f.format
}

// Writer returns the output writer.
func (f *Formatter) Writer() io.Writer {
	return f.writer
}

// IsJSON returns true if the formatter outputs JSON.
func (f *Formatter) IsJSON() bool {
	return f.format == FormatJSON
}

// Print writes formatted output.
func (f *Formatter) Print(v any) error {
	if f.format == FormatJSON {
		return f.printJSON(v)
	}
	return f.printText(v)
}

// Printf writes formatted text output.
func (f *Formatter) Printf(format string, args ...any) error {
	_, err := fmt.Fprintf(f.writer, format, args...)
	return err
}

// Println writes a line of text output.
func (f *Formatter) Println(args ...any) error {
	_, err := fmt.Fprintln(f.writer, args...)
	return err
}

// printJSON outputs JSON format.
func (f *Formatter) printJSON(v any) error {
	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// printText outputs text format.
func (f *Formatter) printText(v any) error {
	switch val := v.(type) {
	case string:
		_, err := fmt.Fprintln(f.writer, val)
		return err
	case fmt.Stringer:
		_, err := fmt.Fprintln(f.writer, val.String())
		return err
	default:
		_, err := fmt.Fprintf(f.writer, "%v\n", val)
		return err
	}
}

// DetectFormat determines the appropriate format based on context.
// Returns JSON for non-TTY output, text for TTY, unless explicitly overridden.
func DetectFormat(w io.Writer, explicit Format) Format {
	if explicit != FormatAuto {
		return explicit
	}

	// Check if output is a TTY
	if f, ok := w.(*os.File); ok {
		if term.IsTerminal(int(f.Fd())) { //nolint:gosec // G115: Fd() returns uintptr, safe conversion for term.IsTerminal
			return FormatText
		}
	}

	return FormatJSON
}

// ParseFormat parses a format string.
func ParseFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return FormatJSON
	case "text":
		return FormatText
	default:
		return FormatAuto
	}
}
