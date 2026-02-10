package chain

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	sigilerr "github.com/mrz1836/sigil/pkg/errors"
)

// Sentinel errors for retry logic.
var (
	ErrRetryable = &sigilerr.SigilError{
		Code:     "RETRYABLE_ERROR",
		Message:  "retryable error",
		ExitCode: sigilerr.ExitGeneral,
	}

	ErrTimeout = &sigilerr.SigilError{
		Code:     "TIMEOUT",
		Message:  "operation timed out",
		ExitCode: sigilerr.ExitGeneral,
	}

	ErrRateLimited = &sigilerr.SigilError{
		Code:     "RATE_LIMITED",
		Message:  "rate limited",
		ExitCode: sigilerr.ExitGeneral,
	}
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxAttempts int           // Maximum number of attempts (including initial)
	BaseDelay   time.Duration // Initial delay between retries
	MaxDelay    time.Duration // Maximum delay between retries
}

// DefaultRetryConfig returns the default retry configuration.
// 4 attempts total (1 initial + 3 retries) with delays: 1s, 2s, 4s.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 4,
		BaseDelay:   time.Second,
		MaxDelay:    4 * time.Second,
	}
}

// Retry executes the operation with exponential backoff retry.
// Uses default configuration: 4 attempts with delays 1s, 2s, 4s.
func Retry[T any](ctx context.Context, operation func() (T, error)) (T, error) {
	return RetryWithConfig(ctx, DefaultRetryConfig(), operation)
}

// RetryWithConfig executes the operation with the specified retry configuration.
func RetryWithConfig[T any](ctx context.Context, cfg RetryConfig, operation func() (T, error)) (T, error) {
	var result T
	var err error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		result, err = operation()
		if err == nil {
			return result, nil
		}

		// Check if error is retryable
		if !IsRetryable(err) {
			return result, err
		}

		// Don't delay after the last attempt
		if attempt < cfg.MaxAttempts-1 {
			delay := calculateDelay(attempt, cfg.BaseDelay, cfg.MaxDelay)

			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return result, ctx.Err()
			case <-timer.C:
			}
		}
	}

	return result, fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, err)
}

// calculateDelay calculates the delay for the given attempt using exponential backoff with jitter.
// Jitter prevents thundering herd when multiple goroutines retry simultaneously.
func calculateDelay(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	delay := baseDelay * (1 << attempt) // 2^attempt * baseDelay
	if delay > maxDelay {
		delay = maxDelay
	}
	// Add jitter: random duration in [delay/2, delay).
	// Cryptographic randomness is not needed for retry jitter.
	half := delay / 2
	return half + rand.N(half) //nolint:gosec // G404: Jitter does not require cryptographic randomness
}

// IsRetryable returns true if the error should trigger a retry.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for known retryable errors
	if errors.Is(err, ErrRetryable) ||
		errors.Is(err, ErrTimeout) ||
		errors.Is(err, ErrRateLimited) ||
		errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}

// ParseRetryAfter parses the Retry-After header value.
// Returns the duration to wait, or 0 if parsing fails.
func ParseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}

	seconds, err := strconv.Atoi(header)
	if err != nil {
		return 0
	}

	return time.Duration(seconds) * time.Second
}

// WrapRetryable wraps an error to mark it as retryable.
func WrapRetryable(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrRetryable, err)
}
