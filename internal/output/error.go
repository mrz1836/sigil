package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// ErrorOutput represents a structured error for JSON output.
type ErrorOutput struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details.
type ErrorDetail struct {
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	Details    map[string]string `json:"details,omitempty"`
	Suggestion string            `json:"suggestion,omitempty"`
	ExitCode   int               `json:"exit_code"`
}

// FormatError formats an error for display.
func FormatError(w io.Writer, err error, format Format) error {
	if err == nil {
		return nil
	}

	if format == FormatJSON {
		return formatErrorJSON(w, err)
	}
	return formatErrorText(w, err)
}

// formatErrorJSON outputs error in JSON format.
func formatErrorJSON(w io.Writer, err error) error {
	var se *sigilerr.SigilError
	if errors.As(err, &se) {
		output := ErrorOutput{
			Error: ErrorDetail{
				Code:       se.Code,
				Message:    se.Message,
				Details:    se.Details,
				Suggestion: se.Suggestion,
				ExitCode:   se.ExitCode,
			},
		}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}

	// Generic error
	output := ErrorOutput{
		Error: ErrorDetail{
			Code:     "GENERAL_ERROR",
			Message:  err.Error(),
			ExitCode: sigilerr.ExitGeneral,
		},
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// formatErrorText outputs error in text format.
func formatErrorText(w io.Writer, err error) error {
	var sb strings.Builder

	var se *sigilerr.SigilError
	if errors.As(err, &se) {
		sb.WriteString(fmt.Sprintf("Error: %s\n", se.Message))

		if len(se.Details) > 0 {
			sb.WriteString("\nDetails:\n")
			for k, v := range se.Details {
				sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
			}
		}

		if se.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("\nSuggestion: %s\n", se.Suggestion))
		}
	} else {
		sb.WriteString(fmt.Sprintf("Error: %s\n", err.Error()))
	}

	_, writeErr := w.Write([]byte(sb.String()))
	return writeErr
}

// FormatSuccess formats a success message.
func FormatSuccess(w io.Writer, message string, format Format) error {
	if format == FormatJSON {
		output := map[string]string{"status": "success", "message": message}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	}
	_, err := fmt.Fprintln(w, message)
	return err
}
