package errors_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

func TestExitCodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"success", nil, sigilerr.ExitSuccess},
		{"general error", sigilerr.ErrGeneral, sigilerr.ExitGeneral},
		{"input error", sigilerr.ErrInvalidInput, sigilerr.ExitInput},
		{"auth error", sigilerr.ErrAuthentication, sigilerr.ExitAuth},
		{"not found error", sigilerr.ErrNotFound, sigilerr.ExitNotFound},
		{"permission error", sigilerr.ErrPermission, sigilerr.ExitPermission},
		{"insufficient funds", sigilerr.ErrInsufficientFunds, sigilerr.ExitPermission},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			code := sigilerr.ExitCode(tt.err)
			assert.Equal(t, tt.expected, code)
		})
	}
}

func TestExitCodeWrappedError(t *testing.T) {
	t.Parallel()
	wrapped := sigilerr.Wrap(sigilerr.ErrNotFound, "wallet main")
	code := sigilerr.ExitCode(wrapped)
	assert.Equal(t, sigilerr.ExitNotFound, code)
}

func TestSentinelErrors(t *testing.T) {
	t.Parallel()
	// Verify that wrapping preserves error identity
	wrapped := sigilerr.Wrap(sigilerr.ErrGeneral, "wrapped")
	require.ErrorIs(t, wrapped, sigilerr.ErrGeneral)

	wrapped = sigilerr.Wrap(sigilerr.ErrInvalidInput, "wrapped")
	require.ErrorIs(t, wrapped, sigilerr.ErrInvalidInput)

	wrapped = sigilerr.Wrap(sigilerr.ErrAuthentication, "wrapped")
	require.ErrorIs(t, wrapped, sigilerr.ErrAuthentication)

	wrapped = sigilerr.Wrap(sigilerr.ErrNotFound, "wrapped")
	require.ErrorIs(t, wrapped, sigilerr.ErrNotFound)

	wrapped = sigilerr.Wrap(sigilerr.ErrPermission, "wrapped")
	require.ErrorIs(t, wrapped, sigilerr.ErrPermission)

	wrapped = sigilerr.Wrap(sigilerr.ErrInsufficientFunds, "wrapped")
	require.ErrorIs(t, wrapped, sigilerr.ErrInsufficientFunds)
}

func TestErrorCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err      error
		expected string
	}{
		{sigilerr.ErrGeneral, "GENERAL_ERROR"},
		{sigilerr.ErrInvalidInput, "INVALID_INPUT"},
		{sigilerr.ErrAuthentication, "AUTHENTICATION_FAILED"},
		{sigilerr.ErrNotFound, "NOT_FOUND"},
		{sigilerr.ErrPermission, "PERMISSION_DENIED"},
		{sigilerr.ErrInsufficientFunds, "INSUFFICIENT_FUNDS"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			var se *sigilerr.SigilError
			require.ErrorAs(t, tt.err, &se)
			assert.Equal(t, tt.expected, se.Code)
		})
	}
}

func TestWithDetails(t *testing.T) {
	t.Parallel()
	details := map[string]string{
		"required":  "0.5",
		"available": "0.1",
		"symbol":    "ETH",
	}

	err := sigilerr.WithDetails(sigilerr.ErrInsufficientFunds, details)

	var se *sigilerr.SigilError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, details, se.Details)
}

func TestWithSuggestion(t *testing.T) {
	t.Parallel()
	suggestion := "Check balance with 'sigil balance show --wallet main'"
	err := sigilerr.WithSuggestion(sigilerr.ErrInsufficientFunds, suggestion)

	var se *sigilerr.SigilError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, suggestion, se.Suggestion)
}

func TestWithDetailsAndSuggestion(t *testing.T) {
	t.Parallel()
	details := map[string]string{"key": "value"}
	suggestion := "Try this instead"

	err := sigilerr.WithDetails(sigilerr.ErrGeneral, details)
	err = sigilerr.WithSuggestion(err, suggestion)

	var se *sigilerr.SigilError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, details, se.Details)
	assert.Equal(t, suggestion, se.Suggestion)
}

func TestWrap(t *testing.T) {
	t.Parallel()
	wrapped := sigilerr.Wrap(sigilerr.ErrNotFound, "wallet %s", "main")
	assert.Contains(t, wrapped.Error(), "wallet main")
	assert.ErrorIs(t, wrapped, sigilerr.ErrNotFound)
}

func TestNew(t *testing.T) {
	t.Parallel()
	err := sigilerr.New("CUSTOM_ERROR", "custom error message")
	assert.Equal(t, "custom error message", err.Error())

	var se *sigilerr.SigilError
	require.ErrorAs(t, err, &se)
	assert.Equal(t, "CUSTOM_ERROR", se.Code)
}
