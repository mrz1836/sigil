package errors_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

var (
	errInner     = errors.New("inner")
	errRootCause = errors.New("root cause")
	errPlain     = errors.New("plain error")
	errPlainCode = errors.New("plain")
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

func TestSigilError_Error(t *testing.T) {
	t.Parallel()

	t.Run("message only", func(t *testing.T) {
		t.Parallel()
		err := &sigilerr.SigilError{Code: "TEST", Message: "something failed"}
		assert.Equal(t, "something failed", err.Error())
	})

	t.Run("with details sorted", func(t *testing.T) {
		t.Parallel()
		err := &sigilerr.SigilError{
			Code:    "TEST",
			Message: "failed",
			Details: map[string]string{"beta": "2", "alpha": "1"},
		}
		assert.Equal(t, "failed (alpha: 1) (beta: 2)", err.Error())
	})

	t.Run("with cause", func(t *testing.T) {
		t.Parallel()
		err := &sigilerr.SigilError{
			Code:    "TEST",
			Message: "outer",
			Cause:   errInner,
		}
		assert.Equal(t, "outer: inner", err.Error())
	})

	t.Run("with details and cause", func(t *testing.T) {
		t.Parallel()
		err := &sigilerr.SigilError{
			Code:    "TEST",
			Message: "outer",
			Details: map[string]string{"key": "val"},
			Cause:   errInner,
		}
		assert.Equal(t, "outer (key: val): inner", err.Error())
	})
}

func TestSigilError_Error_deterministic(t *testing.T) {
	t.Parallel()
	err := &sigilerr.SigilError{
		Code:    "TEST",
		Message: "msg",
		Details: map[string]string{
			"charlie": "3",
			"alpha":   "1",
			"bravo":   "2",
			"delta":   "4",
		},
	}
	first := err.Error()
	for i := 0; i < 100; i++ {
		assert.Equal(t, first, err.Error(), "Error() output must be deterministic (iteration %d)", i)
	}
}

func TestSigilError_Unwrap(t *testing.T) {
	t.Parallel()

	t.Run("with cause", func(t *testing.T) {
		t.Parallel()
		err := &sigilerr.SigilError{Code: "TEST", Message: "wrapper", Cause: errRootCause}
		assert.Equal(t, errRootCause, err.Unwrap())
	})

	t.Run("nil cause", func(t *testing.T) {
		t.Parallel()
		err := &sigilerr.SigilError{Code: "TEST", Message: "no cause"}
		assert.NoError(t, err.Unwrap())
	})
}

func TestSigilError_Is(t *testing.T) {
	t.Parallel()

	t.Run("matching code", func(t *testing.T) {
		t.Parallel()
		a := &sigilerr.SigilError{Code: "SAME_CODE", Message: "a"}
		b := &sigilerr.SigilError{Code: "SAME_CODE", Message: "b"}
		assert.True(t, a.Is(b))
	})

	t.Run("different code", func(t *testing.T) {
		t.Parallel()
		a := &sigilerr.SigilError{Code: "CODE_A", Message: "a"}
		b := &sigilerr.SigilError{Code: "CODE_B", Message: "b"}
		assert.False(t, a.Is(b))
	})

	t.Run("non-SigilError target", func(t *testing.T) {
		t.Parallel()
		a := &sigilerr.SigilError{Code: "TEST", Message: "a"}
		assert.False(t, a.Is(errPlain))
	})
}

func TestAs(t *testing.T) {
	t.Parallel()

	t.Run("SigilError target", func(t *testing.T) {
		t.Parallel()
		err := sigilerr.Wrap(sigilerr.ErrNotFound, "wrapped")
		var se *sigilerr.SigilError
		assert.True(t, sigilerr.As(err, &se))
		assert.Equal(t, "NOT_FOUND", se.Code)
	})

	t.Run("non-SigilError", func(t *testing.T) {
		t.Parallel()
		var se *sigilerr.SigilError
		assert.False(t, sigilerr.As(errPlain, &se))
	})
}

func TestIs(t *testing.T) {
	t.Parallel()

	t.Run("matching sentinel", func(t *testing.T) {
		t.Parallel()
		wrapped := sigilerr.Wrap(sigilerr.ErrNotFound, "context")
		assert.True(t, sigilerr.Is(wrapped, sigilerr.ErrNotFound))
	})

	t.Run("non-matching", func(t *testing.T) {
		t.Parallel()
		wrapped := sigilerr.Wrap(sigilerr.ErrNotFound, "context")
		assert.False(t, sigilerr.Is(wrapped, sigilerr.ErrPermission))
	})

	t.Run("nil error", func(t *testing.T) {
		t.Parallel()
		assert.False(t, sigilerr.Is(nil, sigilerr.ErrGeneral))
	})
}

func TestCode_edgeCases(t *testing.T) {
	t.Parallel()

	t.Run("SigilError", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "NOT_FOUND", sigilerr.Code(sigilerr.ErrNotFound))
	})

	t.Run("non-SigilError", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "GENERAL_ERROR", sigilerr.Code(errPlainCode))
	})

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "GENERAL_ERROR", sigilerr.Code(nil))
	})
}

func TestWrap_edgeCases(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, sigilerr.Wrap(nil, "context"))
	})

	t.Run("non-SigilError", func(t *testing.T) {
		t.Parallel()
		wrapped := sigilerr.Wrap(errPlain, "context")
		var se *sigilerr.SigilError
		require.ErrorAs(t, wrapped, &se)
		assert.Equal(t, "GENERAL_ERROR", se.Code)
		assert.Equal(t, "context", se.Message)
		assert.Equal(t, errPlain, se.Cause)
	})

	t.Run("format args", func(t *testing.T) {
		t.Parallel()
		wrapped := sigilerr.Wrap(sigilerr.ErrNotFound, "wallet %s index %d", "main", 0)
		assert.Contains(t, wrapped.Error(), "wallet main index 0")
	})

	t.Run("field preservation", func(t *testing.T) {
		t.Parallel()
		original := sigilerr.WithDetails(sigilerr.ErrNotFound, map[string]string{"key": "val"})
		original = sigilerr.WithSuggestion(original, "try this")
		wrapped := sigilerr.Wrap(original, "context")

		var se *sigilerr.SigilError
		require.ErrorAs(t, wrapped, &se)
		assert.Equal(t, "NOT_FOUND", se.Code)
		assert.Equal(t, map[string]string{"key": "val"}, se.Details)
		assert.Equal(t, "try this", se.Suggestion)
		assert.Equal(t, sigilerr.ExitNotFound, se.ExitCode)
	})
}

func TestWithDetails_edgeCases(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, sigilerr.WithDetails(nil, map[string]string{"k": "v"}))
	})

	t.Run("non-SigilError input", func(t *testing.T) {
		t.Parallel()
		result := sigilerr.WithDetails(errPlain, map[string]string{"k": "v"})
		var se *sigilerr.SigilError
		require.ErrorAs(t, result, &se)
		assert.Equal(t, "GENERAL_ERROR", se.Code)
		assert.Equal(t, "plain error", se.Message)
		assert.Equal(t, map[string]string{"k": "v"}, se.Details)
		assert.Equal(t, errPlain, se.Cause)
	})
}

func TestWithSuggestion_edgeCases(t *testing.T) {
	t.Parallel()

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, sigilerr.WithSuggestion(nil, "suggestion"))
	})

	t.Run("non-SigilError input", func(t *testing.T) {
		t.Parallel()
		result := sigilerr.WithSuggestion(errPlain, "try this")
		var se *sigilerr.SigilError
		require.ErrorAs(t, result, &se)
		assert.Equal(t, "GENERAL_ERROR", se.Code)
		assert.Equal(t, "plain error", se.Message)
		assert.Equal(t, "try this", se.Suggestion)
		assert.Equal(t, errPlain, se.Cause)
	})
}

func TestExitCode_nonSigilError(t *testing.T) {
	t.Parallel()
	assert.Equal(t, sigilerr.ExitGeneral, sigilerr.ExitCode(errPlain))
}
