package output_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
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

// TestTable_EmptyTable tests rendering of an empty table (no headers, no rows).
func TestTable_EmptyTable(t *testing.T) {
	t.Parallel()
	table := output.NewTable()

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

// TestTable_HeadersOnly tests rendering a table with headers but no rows.
func TestTable_HeadersOnly(t *testing.T) {
	t.Parallel()
	table := output.NewTable("Name", "Value", "Status")

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Name")
	assert.Contains(t, result, "Value")
	assert.Contains(t, result, "Status")
	assert.Contains(t, result, "---") // Separator line
}

// TestTable_RaggedRows tests rows with mismatched column counts.
func TestTable_RaggedRows(t *testing.T) {
	t.Parallel()
	table := output.NewTable("A", "B", "C")
	table.AddRow("1", "2")      // Missing column
	table.AddRow("3", "4", "5") // Complete
	table.AddRow("6")           // Only one column

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	// Should still render without panic
	assert.Contains(t, result, "1")
	assert.Contains(t, result, "3")
	assert.Contains(t, result, "6")
}

// TestTable_EmptyCells tests tables with empty cells.
func TestTable_EmptyCells(t *testing.T) {
	t.Parallel()
	table := output.NewTable("Name", "Value")
	table.AddRow("", "value1")
	table.AddRow("name2", "")
	table.AddRow("", "")

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Name")
	assert.Contains(t, result, "Value")
}

// TestTable_SingleCell tests a minimal 1x1 table.
func TestTable_SingleCell(t *testing.T) {
	t.Parallel()
	table := output.NewTable("Header")
	table.AddRow("Value")

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Header")
	assert.Contains(t, result, "Value")
}

// TestTable_VeryLongContent tests handling of very long content (1000+ chars).
func TestTable_VeryLongContent(t *testing.T) {
	t.Parallel()
	longValue := strings.Repeat("a", 1000)
	table := output.NewTable("Name", "Value")
	table.AddRow("test", longValue)

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, longValue)
}

// TestTable_UnicodeContent tests Unicode characters (Chinese, emoji).
func TestTable_UnicodeContent(t *testing.T) {
	t.Parallel()
	table := output.NewTable("Name", "Description")
	//nolint:gosmopolitan // Intentional unicode test
	table.AddRow("测试", "Test in Chinese")
	table.AddRow("Emoji", "🚀 🎉 ✨")
	//nolint:gosmopolitan // Intentional unicode test
	table.AddRow("Mixed", "English 中文 🔥")

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	//nolint:gosmopolitan // Intentional unicode test
	assert.Contains(t, result, "测试")
	assert.Contains(t, result, "🚀")
	//nolint:gosmopolitan // Intentional unicode test
	assert.Contains(t, result, "中文")
}

// TestTable_MultiCharSeparator tests using multi-character separators.
func TestTable_MultiCharSeparator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		separator string
	}{
		{"pipe with spaces", " | "},
		{"tab", "\t"},
		{"arrow", " -> "},
		{"double space", "  "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			table := output.NewTable("A", "B")
			table.AddRow("1", "2")
			table.SetSeparator(tc.separator)

			var buf bytes.Buffer
			err := table.Render(&buf)
			require.NoError(t, err)
			assert.Contains(t, buf.String(), tc.separator)
		})
	}
}

// TestTable_SpecialCharacters tests special characters in cells.
func TestTable_SpecialCharacters(t *testing.T) {
	t.Parallel()
	table := output.NewTable("Name", "Value")
	table.AddRow("quotes", `"test"`)
	table.AddRow("newline", "line1\nline2")
	table.AddRow("tab", "col1\tcol2")
	table.AddRow("special", "<>&\"'")

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	// Should handle without error
	assert.NotEmpty(t, result)
}

// TestTable_ManyColumns tests tables with many columns.
func TestTable_ManyColumns(t *testing.T) {
	t.Parallel()
	headers := make([]string, 20)
	row := make([]string, 20)
	for i := 0; i < 20; i++ {
		headers[i] = "Col" + string(rune('A'+i))
		row[i] = "val" + string(rune('A'+i))
	}

	table := output.NewTable(headers...)
	table.AddRow(row...)

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "ColA")
	assert.Contains(t, result, "ColT") // 20th column
}

// TestTable_ManyRows tests tables with many rows.
func TestTable_ManyRows(t *testing.T) {
	t.Parallel()
	table := output.NewTable("Index", "Value")
	for i := 0; i < 100; i++ {
		table.AddRow(string(rune('0'+i%10)), "value"+string(rune('0'+i%10)))
	}

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	assert.NotEmpty(t, result)
	// Just verify it doesn't crash with many rows
}

// TestTable_WhitespaceContent tests content with leading/trailing whitespace.
func TestTable_WhitespaceContent(t *testing.T) {
	t.Parallel()
	table := output.NewTable("Name", "Value")
	table.AddRow("  leading", "trailing  ")
	table.AddRow("\tTab", "Space ")
	table.AddRow("", "   ")

	var buf bytes.Buffer
	err := table.Render(&buf)
	require.NoError(t, err)

	result := buf.String()
	// Whitespace should be preserved
	assert.Contains(t, result, "  leading")
	assert.Contains(t, result, "trailing  ")
}
