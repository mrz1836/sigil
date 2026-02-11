package output_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

func TestFormatter_JSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	f := output.NewFormatter(output.FormatJSON, &buf)

	data := map[string]string{"key": "value"}
	err := f.Print(data)
	require.NoError(t, err)

	var result map[string]string
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
}

func TestFormatter_Text(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	f := output.NewFormatter(output.FormatText, &buf)

	err := f.Print("hello world")
	require.NoError(t, err)
	assert.Equal(t, "hello world\n", buf.String())
}

func TestFormatter_Printf(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	f := output.NewFormatter(output.FormatText, &buf)

	err := f.Printf("hello %s\n", "world")
	require.NoError(t, err)
	assert.Equal(t, "hello world\n", buf.String())
}

func TestFormatter_IsJSON(t *testing.T) {
	t.Parallel()
	jsonFmt := output.NewFormatter(output.FormatJSON, nil)
	textFmt := output.NewFormatter(output.FormatText, nil)

	assert.True(t, jsonFmt.IsJSON())
	assert.False(t, textFmt.IsJSON())
}

func TestParseFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected output.Format
	}{
		{"json", output.FormatJSON},
		{"JSON", output.FormatJSON},
		{"text", output.FormatText},
		{"TEXT", output.FormatText},
		{"auto", output.FormatAuto},
		{"", output.FormatAuto},
		{"invalid", output.FormatAuto},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := output.ParseFormat(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectFormat_Explicit(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	assert.Equal(t, output.FormatJSON, output.DetectFormat(&buf, output.FormatJSON))
	assert.Equal(t, output.FormatText, output.DetectFormat(&buf, output.FormatText))
}

func TestDetectFormat_NonTTY(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	// Non-TTY should default to JSON
	result := output.DetectFormat(&buf, output.FormatAuto)
	assert.Equal(t, output.FormatJSON, result)
}

func TestDetectFormat_TTY(t *testing.T) {
	// Skip if not running in a terminal
	if os.Getenv("TEST_TTY") == "" {
		t.Skip("Skipping TTY test - set TEST_TTY=1 to run")
	}

	result := output.DetectFormat(os.Stdout, output.FormatAuto)
	assert.Equal(t, output.FormatText, result)
}

func TestTable_Basic(t *testing.T) {
	t.Parallel()
	table := output.NewTable("Name", "Value")
	table.AddRow("foo", "bar")
	table.AddRow("baz", "qux")

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Name")
	assert.Contains(t, result, "Value")
	assert.Contains(t, result, "foo")
	assert.Contains(t, result, "bar")
	assert.Contains(t, result, "baz")
	assert.Contains(t, result, "qux")
}

func TestTable_NoHeader(t *testing.T) {
	t.Parallel()
	table := output.NewTable("Name", "Value")
	table.SetNoHeader(true)
	table.AddRow("foo", "bar")

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	assert.NotContains(t, result, "Name")
	assert.NotContains(t, result, "---")
	assert.Contains(t, result, "foo")
}

func TestTable_ColumnAlignment(t *testing.T) {
	t.Parallel()
	table := output.NewTable("Short", "LongerHeader")
	table.AddRow("a", "b")
	table.AddRow("longer", "x")

	result := table.String()
	// Columns should be aligned
	assert.Contains(t, result, "Short ")
	assert.Contains(t, result, "LongerHeader")
}

func TestTable_Empty(t *testing.T) {
	t.Parallel()
	table := output.NewTable()

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestFormatError_Text(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	err := sigilerr.WithDetails(sigilerr.ErrInsufficientFunds, map[string]string{
		"required":  "0.5",
		"available": "0.1",
	})
	err = sigilerr.WithSuggestion(err, "Check balance with 'sigil balance show'")

	formatErr := output.FormatError(&buf, err, output.FormatText)
	require.NoError(t, formatErr)

	result := buf.String()
	assert.Contains(t, result, "insufficient funds")
	assert.Contains(t, result, "required: 0.5")
	assert.Contains(t, result, "available: 0.1")
	assert.Contains(t, result, "sigil balance show")
}

func TestFormatError_JSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	err := sigilerr.WithDetails(sigilerr.ErrInsufficientFunds, map[string]string{
		"required": "0.5",
	})

	formatErr := output.FormatError(&buf, err, output.FormatJSON)
	require.NoError(t, formatErr)

	var result output.ErrorOutput
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)

	assert.Equal(t, "INSUFFICIENT_FUNDS", result.Error.Code)
	assert.Equal(t, "0.5", result.Error.Details["required"])
}

func TestFormatError_GenericError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	err := assert.AnError

	formatErr := output.FormatError(&buf, err, output.FormatJSON)
	require.NoError(t, formatErr)

	var result output.ErrorOutput
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)

	assert.Equal(t, "GENERAL_ERROR", result.Error.Code)
}

func TestFormatSuccess(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer

	err := output.FormatSuccess(&buf, "Operation completed", output.FormatJSON)
	require.NoError(t, err)

	var result map[string]string
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)
	assert.Equal(t, "success", result["status"])
	assert.Equal(t, "Operation completed", result["message"])
}

func TestFormatter_Format(t *testing.T) {
	t.Parallel()
	f := output.NewFormatter(output.FormatJSON, nil)
	assert.Equal(t, output.FormatJSON, f.Format())

	f2 := output.NewFormatter(output.FormatText, nil)
	assert.Equal(t, output.FormatText, f2.Format())
}

func TestFormatter_Writer(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	f := output.NewFormatter(output.FormatJSON, &buf)
	assert.Equal(t, &buf, f.Writer())
}

func TestFormatter_Println(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	f := output.NewFormatter(output.FormatText, &buf)

	err := f.Println("hello", "world")
	require.NoError(t, err)
	assert.Equal(t, "hello world\n", buf.String())
}

func TestTable_SetSeparator(t *testing.T) {
	t.Parallel()
	table := output.NewTable("A", "B")
	table.AddRow("1", "2")
	table.SetSeparator(" | ")

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), " | ")
}

func TestFormatSuccess_Text(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := output.FormatSuccess(&buf, "Operation completed", output.FormatText)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Operation completed")
}
