package chain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sigil/internal/chain"
)

func TestRetry_SuccessFirstAttempt(t *testing.T) {
	attempts := 0
	result, err := chain.Retry(context.Background(), func() (string, error) {
		attempts++
		return "success", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 1, attempts)
}

func TestRetry_SuccessAfterRetry(t *testing.T) {
	attempts := 0
	result, err := chain.Retry(context.Background(), func() (string, error) {
		attempts++
		if attempts < 3 {
			return "", chain.ErrRetryable
		}
		return "success", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, 3, attempts)
}

var errNonRetryable = errors.New("non-retryable error")

func TestRetry_NonRetryableError(t *testing.T) {
	attempts := 0

	_, err := chain.Retry(context.Background(), func() (string, error) {
		attempts++
		return "", errNonRetryable
	})

	require.Error(t, err)
	assert.Equal(t, 1, attempts) // Should not retry
}

func TestRetry_MaxAttempts(t *testing.T) {
	attempts := 0

	_, err := chain.Retry(context.Background(), func() (string, error) {
		attempts++
		return "", chain.ErrRetryable
	})

	require.Error(t, err)
	assert.Equal(t, 4, attempts) // 1 initial + 3 retries
}

func TestRetry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := chain.Retry(ctx, func() (string, error) {
		attempts++
		return "", chain.ErrRetryable
	})

	require.Error(t, err)
	assert.Less(t, attempts, 4) // Should have been canceled before all attempts
}

func TestRetry_CustomConfig(t *testing.T) {
	cfg := chain.RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   time.Millisecond,
		MaxDelay:    time.Millisecond * 10,
	}

	attempts := 0
	_, err := chain.RetryWithConfig(context.Background(), cfg, func() (string, error) {
		attempts++
		return "", chain.ErrRetryable
	})

	require.Error(t, err)
	assert.Equal(t, 2, attempts)
}

var errSomeError = errors.New("some error")

func TestIsRetryable(t *testing.T) {
	assert.True(t, chain.IsRetryable(chain.ErrRetryable))
	assert.True(t, chain.IsRetryable(chain.ErrTimeout))
	assert.True(t, chain.IsRetryable(chain.ErrRateLimited))
	assert.True(t, chain.IsRetryable(context.DeadlineExceeded))

	assert.False(t, chain.IsRetryable(errSomeError))
	assert.False(t, chain.IsRetryable(nil))
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		header   string
		expected time.Duration
	}{
		{"5", 5 * time.Second},
		{"120", 120 * time.Second},
		{"0", 0},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			result := chain.ParseRetryAfter(tt.header)
			assert.Equal(t, tt.expected, result)
		})
	}
}
