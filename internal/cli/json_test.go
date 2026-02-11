package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteJSON_SimpleStruct tests JSON marshaling of a simple struct.
func TestWriteJSON_SimpleStruct(t *testing.T) {
	t.Parallel()

	type testStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	var buf bytes.Buffer
	err := writeJSON(&buf, testStruct{Name: "test", Value: 42})
	require.NoError(t, err)

	// Verify indentation (should have newlines/spaces)
	output := buf.String()
	assert.Contains(t, output, "\n")
	assert.Contains(t, output, "  ") // Two-space indentation

	// Verify valid JSON
	var result testStruct
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, 42, result.Value)
}

// TestWriteJSON_NilValue tests handling of nil values.
func TestWriteJSON_NilValue(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := writeJSON(&buf, nil)
	require.NoError(t, err)

	// nil should be encoded as "null"
	assert.Equal(t, "null\n", buf.String())
}

// TestWriteJSON_ComplexNesting tests nested objects and arrays.
func TestWriteJSON_ComplexNesting(t *testing.T) {
	t.Parallel()

	type nested struct {
		Items []string          `json:"items"`
		Meta  map[string]string `json:"meta"`
	}

	data := nested{
		Items: []string{"one", "two", "three"},
		Meta: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	var buf bytes.Buffer
	err := writeJSON(&buf, data)
	require.NoError(t, err)

	// Verify indentation is present
	output := buf.String()
	assert.Contains(t, output, "\n")
	assert.Contains(t, output, "  ")

	// Verify valid JSON structure
	var result nested
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, []string{"one", "two", "three"}, result.Items)
	assert.Equal(t, "value1", result.Meta["key1"])
	assert.Equal(t, "value2", result.Meta["key2"])
}

// TestWriteJSON_WriterError tests error handling when writer fails.
func TestWriteJSON_WriterError(t *testing.T) {
	t.Parallel()

	// Create a writer that always fails
	errWriter := &errorWriter{err: errors.New("write failed")} //nolint:err113 // test error

	// This should fail during encoding
	err := writeJSON(errWriter, map[string]string{"key": "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write failed")
}

// TestWriteJSON_Map tests encoding of map types.
func TestWriteJSON_Map(t *testing.T) {
	t.Parallel()

	data := map[string]interface{}{
		"string":  "value",
		"number":  123,
		"boolean": true,
		"null":    nil,
		"array":   []int{1, 2, 3},
	}

	var buf bytes.Buffer
	err := writeJSON(&buf, data)
	require.NoError(t, err)

	// Verify valid JSON
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, "value", result["string"])
	assert.InDelta(t, float64(123), result["number"], 0.0) // JSON numbers are float64
	assert.Equal(t, true, result["boolean"])
	assert.Nil(t, result["null"])
}

// TestWriteJSON_EmptyStruct tests empty struct encoding.
func TestWriteJSON_EmptyStruct(t *testing.T) {
	t.Parallel()

	type empty struct{}

	var buf bytes.Buffer
	err := writeJSON(&buf, empty{})
	require.NoError(t, err)

	// Should encode as empty object
	output := buf.String()
	assert.Contains(t, output, "{")
	assert.Contains(t, output, "}")
}

// TestWriteJSON_Array tests array encoding.
func TestWriteJSON_Array(t *testing.T) {
	t.Parallel()

	data := []string{"apple", "banana", "cherry"}

	var buf bytes.Buffer
	err := writeJSON(&buf, data)
	require.NoError(t, err)

	// Verify valid JSON array
	var result []string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, data, result)
}

// errorWriter is a writer that always returns an error.
type errorWriter struct {
	err error
}

func (w *errorWriter) Write(_ []byte) (n int, err error) {
	return 0, w.err
}

var _ io.Writer = (*errorWriter)(nil)
