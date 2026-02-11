package output_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/output"
	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// failingWriter implements io.Writer but always returns an error.
type failingWriter struct{}

func (failingWriter) Write(_ []byte) (n int, err error) {
	//nolint:err113 // Test error, not wrapped
	return 0, errors.New("write failed")
}

// TestFormatError_NilError tests that nil errors produce no output.
func TestFormatError_NilError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format output.Format
	}{
		{"JSON format", output.FormatJSON},
		{"Text format", output.FormatText},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := output.FormatError(&buf, nil, tc.format)
			require.NoError(t, err)
			assert.Empty(t, buf.String())
		})
	}
}

// TestFormatError_GenericError_JSON tests JSON formatting of generic Go errors.
func TestFormatError_GenericError_JSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	//nolint:err113 // Test error, intentionally not wrapped
	err := output.FormatError(&buf, errors.New("something went wrong"), output.FormatJSON)
	require.NoError(t, err)

	var result output.ErrorOutput
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)

	assert.Equal(t, "GENERAL_ERROR", result.Error.Code)
	assert.Equal(t, "something went wrong", result.Error.Message)
	assert.Equal(t, sigilerr.ExitGeneral, result.Error.ExitCode)
	assert.Empty(t, result.Error.Details)
	assert.Empty(t, result.Error.Suggestion)
}

// TestFormatError_GenericError_Text tests text formatting of generic Go errors.
func TestFormatError_GenericError_Text(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	//nolint:err113 // Test error, intentionally not wrapped
	err := output.FormatError(&buf, errors.New("something went wrong"), output.FormatText)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Error: something went wrong")
	assert.NotContains(t, result, "Details:")
	assert.NotContains(t, result, "Suggestion:")
}

// TestFormatError_SigilError_AllFields_JSON tests SigilError with all fields populated in JSON.
func TestFormatError_SigilError_AllFields_JSON(t *testing.T) {
	t.Parallel()

	err := sigilerr.WithDetails(sigilerr.ErrInsufficientFunds, map[string]string{
		"required":  "1.5",
		"available": "0.3",
		"network":   "ETH",
	})
	err = sigilerr.WithSuggestion(err, "Add more funds or reduce amount")

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatJSON)
	require.NoError(t, formatErr)

	var result output.ErrorOutput
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)

	assert.Equal(t, "INSUFFICIENT_FUNDS", result.Error.Code)
	assert.Contains(t, result.Error.Message, "insufficient funds")
	assert.Equal(t, sigilerr.ExitPermission, result.Error.ExitCode)
	assert.Len(t, result.Error.Details, 3)
	assert.Equal(t, "1.5", result.Error.Details["required"])
	assert.Equal(t, "0.3", result.Error.Details["available"])
	assert.Equal(t, "ETH", result.Error.Details["network"])
	assert.Equal(t, "Add more funds or reduce amount", result.Error.Suggestion)
}

// TestFormatError_SigilError_AllFields_Text tests SigilError with all fields populated in text.
func TestFormatError_SigilError_AllFields_Text(t *testing.T) {
	t.Parallel()

	err := sigilerr.WithDetails(sigilerr.ErrInsufficientFunds, map[string]string{
		"required":  "1.5",
		"available": "0.3",
	})
	err = sigilerr.WithSuggestion(err, "Check balance with 'sigil balance show'")

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatText)
	require.NoError(t, formatErr)

	result := buf.String()
	assert.Contains(t, result, "Error: insufficient funds")
	assert.Contains(t, result, "Details:")
	assert.Contains(t, result, "required: 1.5")
	assert.Contains(t, result, "available: 0.3")
	assert.Contains(t, result, "Suggestion: Check balance with 'sigil balance show'")
}

// TestFormatError_EmptyDetails_JSON tests that empty details map is omitted from JSON.
func TestFormatError_EmptyDetails_JSON(t *testing.T) {
	t.Parallel()

	err := sigilerr.ErrInsufficientFunds

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatJSON)
	require.NoError(t, formatErr)

	var result output.ErrorOutput
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)

	// Empty details should be omitted (due to omitempty tag)
	assert.Nil(t, result.Error.Details)

	// Verify the JSON doesn't contain the "details" key
	jsonStr := buf.String()
	assert.NotContains(t, jsonStr, `"details"`)
}

// TestFormatError_EmptyDetails_Text tests that empty details are not rendered in text format.
func TestFormatError_EmptyDetails_Text(t *testing.T) {
	t.Parallel()

	err := sigilerr.ErrInsufficientFunds

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatText)
	require.NoError(t, formatErr)

	result := buf.String()
	assert.NotContains(t, result, "Details:")
}

// TestFormatError_MultipleDetails_JSON tests multiple details are serialized correctly.
func TestFormatError_MultipleDetails_JSON(t *testing.T) {
	t.Parallel()

	details := map[string]string{
		"alpha":   "value1",
		"bravo":   "value2",
		"charlie": "value3",
		"delta":   "value4",
	}
	err := sigilerr.WithDetails(sigilerr.ErrInvalidAddress, details)

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatJSON)
	require.NoError(t, formatErr)

	var result output.ErrorOutput
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)

	assert.Len(t, result.Error.Details, 4)
	for k, v := range details {
		assert.Equal(t, v, result.Error.Details[k])
	}
}

// TestFormatError_SpecialCharactersInDetails_JSON tests special characters in JSON.
func TestFormatError_SpecialCharactersInDetails_JSON(t *testing.T) {
	t.Parallel()

	details := map[string]string{
		"quote":   `value with "quotes"`,
		"newline": "value\nwith\nnewlines",
		//nolint:gosmopolitan // Intentional unicode test
		"unicode": "emoji 🔥 and 中文",
		"tab":     "value\twith\ttabs",
	}
	err := sigilerr.WithDetails(sigilerr.ErrInvalidAddress, details)

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatJSON)
	require.NoError(t, formatErr)

	var result output.ErrorOutput
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)

	// Verify all special characters are preserved
	assert.Equal(t, details["quote"], result.Error.Details["quote"])
	assert.Equal(t, details["newline"], result.Error.Details["newline"])
	assert.Equal(t, details["unicode"], result.Error.Details["unicode"])
	assert.Equal(t, details["tab"], result.Error.Details["tab"])
}

// TestFormatError_SpecialCharactersInDetails_Text tests special characters in text format.
func TestFormatError_SpecialCharactersInDetails_Text(t *testing.T) {
	t.Parallel()

	//nolint:gosmopolitan // Intentional unicode test
	details := map[string]string{
		"unicode": "emoji 🔥 and 中文",
		"special": "chars: <>&\"'",
	}
	err := sigilerr.WithDetails(sigilerr.ErrInvalidAddress, details)

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatText)
	require.NoError(t, formatErr)

	result := buf.String()
	//nolint:gosmopolitan // Intentional unicode test
	assert.Contains(t, result, "emoji 🔥 and 中文")
	assert.Contains(t, result, "chars: <>&\"'")
}

// TestFormatError_JSONIndentation tests that JSON is properly indented with 2 spaces.
func TestFormatError_JSONIndentation(t *testing.T) {
	t.Parallel()

	err := sigilerr.WithDetails(sigilerr.ErrInvalidAddress, map[string]string{
		"address": "0xinvalid",
	})

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatJSON)
	require.NoError(t, formatErr)

	jsonStr := buf.String()

	// Verify indentation (2 spaces)
	assert.Contains(t, jsonStr, "{\n  \"error\":")
	assert.Contains(t, jsonStr, "    \"code\":")
}

// TestFormatError_DetailsDeterminism_Text tests that details are rendered in consistent order.
// CRITICAL: This tests that map iteration is sorted for deterministic output.
func TestFormatError_DetailsDeterminism_Text(t *testing.T) {
	t.Parallel()

	details := map[string]string{
		"zulu":    "last",
		"alpha":   "first",
		"charlie": "middle",
		"bravo":   "second",
	}

	// Run the formatting 5 times and collect outputs
	outputs := make([]string, 5)
	for i := 0; i < 5; i++ {
		err := sigilerr.WithDetails(sigilerr.ErrInvalidAddress, details)
		var buf bytes.Buffer
		formatErr := output.FormatError(&buf, err, output.FormatText)
		require.NoError(t, formatErr)
		outputs[i] = buf.String()
	}

	// All outputs should be identical
	for i := 1; i < len(outputs); i++ {
		assert.Equal(t, outputs[0], outputs[i], "output %d differs from output 0", i)
	}

	// Verify alphabetical order in the output
	result := outputs[0]
	alphaIdx := strings.Index(result, "alpha:")
	bravoIdx := strings.Index(result, "bravo:")
	charlieIdx := strings.Index(result, "charlie:")
	zuluIdx := strings.Index(result, "zulu:")

	assert.Greater(t, bravoIdx, alphaIdx, "bravo should come after alpha")
	assert.Greater(t, charlieIdx, bravoIdx, "charlie should come after bravo")
	assert.Greater(t, zuluIdx, charlieIdx, "zulu should come after charlie")
}

// TestFormatError_LongSuggestion tests that long suggestions are handled correctly.
func TestFormatError_LongSuggestion(t *testing.T) {
	t.Parallel()

	longSuggestion := "This is a very long suggestion that spans multiple conceptual lines. " +
		"It provides detailed instructions on how to fix the error, including specific commands, " +
		"URLs to documentation, and troubleshooting steps that the user should follow carefully."

	err := sigilerr.WithSuggestion(sigilerr.ErrInvalidAddress, longSuggestion)

	tests := []struct {
		name   string
		format output.Format
	}{
		{"JSON format", output.FormatJSON},
		{"Text format", output.FormatText},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			formatErr := output.FormatError(&buf, err, tc.format)
			require.NoError(t, formatErr)

			result := buf.String()
			assert.Contains(t, result, longSuggestion)
		})
	}
}

// TestFormatError_EmptySuggestion tests that empty suggestions are omitted.
func TestFormatError_EmptySuggestion(t *testing.T) {
	t.Parallel()

	err := sigilerr.ErrInvalidAddress // No suggestion

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatJSON)
	require.NoError(t, formatErr)

	jsonStr := buf.String()
	// Empty suggestion should be omitted (due to omitempty tag)
	assert.NotContains(t, jsonStr, `"suggestion"`)
}

// TestFormatError_WriterError tests that write failures are propagated as errors.
func TestFormatError_WriterError(t *testing.T) {
	t.Parallel()

	fw := failingWriter{}
	err := sigilerr.ErrInvalidAddress

	writeErr := output.FormatError(&fw, err, output.FormatJSON)
	require.Error(t, writeErr)
	assert.Contains(t, writeErr.Error(), "write failed")
}

// TestFormatError_VeryLargeDetails tests handling of large details maps.
func TestFormatError_VeryLargeDetails(t *testing.T) {
	t.Parallel()

	// Create a map with 100 entries
	details := make(map[string]string)
	for i := 0; i < 100; i++ {
		key := string(rune('a' + (i % 26)))
		if i >= 26 {
			key = key + string(rune('0'+(i/26)))
		}
		details[key] = "value_" + key
	}

	err := sigilerr.WithDetails(sigilerr.ErrInvalidAddress, details)

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatJSON)
	require.NoError(t, formatErr)

	var result output.ErrorOutput
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)

	assert.Len(t, result.Error.Details, 100)
}

// TestFormatError_LongDetailValues tests very long detail values.
func TestFormatError_LongDetailValues(t *testing.T) {
	t.Parallel()

	longValue := strings.Repeat("a", 1000)
	details := map[string]string{
		"long": longValue,
	}

	err := sigilerr.WithDetails(sigilerr.ErrInvalidAddress, details)

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatJSON)
	require.NoError(t, formatErr)

	var result output.ErrorOutput
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)

	assert.Equal(t, longValue, result.Error.Details["long"])
	assert.Len(t, result.Error.Details["long"], 1000)
}

// TestFormatSuccess_JSON tests FormatSuccess with JSON format.
func TestFormatSuccess_JSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := output.FormatSuccess(&buf, "Operation completed successfully", output.FormatJSON)
	require.NoError(t, err)

	var result map[string]string
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)

	assert.Equal(t, "success", result["status"])
	assert.Equal(t, "Operation completed successfully", result["message"])
}

// TestFormatSuccess_TextFormat tests FormatSuccess with text format.
func TestFormatSuccess_TextFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := output.FormatSuccess(&buf, "Operation completed", output.FormatText)
	require.NoError(t, err)

	result := buf.String()
	assert.Contains(t, result, "Operation completed")
	assert.True(t, strings.HasSuffix(result, "\n"), "should end with newline")
}

// TestFormatSuccess_EmptyMessage tests FormatSuccess with empty message.
func TestFormatSuccess_EmptyMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format output.Format
	}{
		{"JSON format", output.FormatJSON},
		{"Text format", output.FormatText},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := output.FormatSuccess(&buf, "", tc.format)
			require.NoError(t, err)
			assert.NotEmpty(t, buf.String())
		})
	}
}

// TestFormatSuccess_SpecialCharacters tests FormatSuccess with special characters.
func TestFormatSuccess_SpecialCharacters(t *testing.T) {
	t.Parallel()

	//nolint:gosmopolitan // Intentional unicode test
	message := "Success with 🎉 emoji and 中文 characters"

	tests := []struct {
		name   string
		format output.Format
	}{
		{"JSON format", output.FormatJSON},
		{"Text format", output.FormatText},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := output.FormatSuccess(&buf, message, tc.format)
			require.NoError(t, err)

			result := buf.String()
			assert.Contains(t, result, "🎉")
			//nolint:gosmopolitan // Intentional unicode test
			assert.Contains(t, result, "中文")
		})
	}
}

// TestFormatSuccess_WriterError tests that write failures are propagated.
func TestFormatSuccess_WriterError(t *testing.T) {
	t.Parallel()

	fw := failingWriter{}
	err := output.FormatSuccess(&fw, "test", output.FormatText)
	assert.Error(t, err)
}

// TestFormatError_DetailsSorted_Text verifies that details are always sorted in text output.
func TestFormatError_DetailsSorted_Text(t *testing.T) {
	t.Parallel()

	// Create details in random order
	details := map[string]string{
		"3_third":  "c",
		"1_first":  "a",
		"4_fourth": "d",
		"2_second": "b",
	}

	err := sigilerr.WithDetails(sigilerr.ErrInvalidAddress, details)

	var buf bytes.Buffer
	formatErr := output.FormatError(&buf, err, output.FormatText)
	require.NoError(t, formatErr)

	result := buf.String()

	// Find positions of each key in the output
	positions := make(map[string]int)
	for key := range details {
		positions[key] = strings.Index(result, key)
		assert.NotEqual(t, -1, positions[key], "key %s not found", key)
	}

	// Verify they appear in sorted order
	keys := make([]string, 0, len(details))
	for k := range details {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i := 1; i < len(keys); i++ {
		prevKey := keys[i-1]
		currKey := keys[i]
		assert.Less(t, positions[prevKey], positions[currKey],
			"key %s should appear before %s", prevKey, currKey)
	}
}

// TestFormatError_UnicodeInAllFields tests unicode handling in all error fields.
func TestFormatError_UnicodeInAllFields(t *testing.T) {
	t.Parallel()

	// Create a custom error with unicode in message
	//nolint:gosmopolitan // Intentional unicode test
	baseErr := &sigilerr.SigilError{
		Code:     "UNICODE_TEST",
		Message:  "错误消息 with emoji 🔥",
		ExitCode: 1,
		Details: map[string]string{
			"field1": "值1 with 🎉",
			"field2": "value2 🚀",
		},
		Suggestion: "建议: Try something with ✨",
	}

	var buf bytes.Buffer
	err := output.FormatError(&buf, baseErr, output.FormatJSON)
	require.NoError(t, err)

	var result output.ErrorOutput
	jsonErr := json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, jsonErr)

	//nolint:gosmopolitan // Intentional unicode test
	assert.Contains(t, result.Error.Message, "错误消息")
	assert.Contains(t, result.Error.Message, "🔥")
	//nolint:gosmopolitan // Intentional unicode test
	assert.Contains(t, result.Error.Details["field1"], "值1")
	assert.Contains(t, result.Error.Details["field1"], "🎉")
	//nolint:gosmopolitan // Intentional unicode test
	assert.Contains(t, result.Error.Suggestion, "建议")
	assert.Contains(t, result.Error.Suggestion, "✨")
}
