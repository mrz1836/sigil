package chain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculateDelay(t *testing.T) {
	baseDelay := 100 * time.Millisecond
	maxDelay := 500 * time.Millisecond

	t.Run("delay within max", func(t *testing.T) {
		// Attempt 0: 100ms * 2^0 = 100ms
		delay := calculateDelay(0, baseDelay, maxDelay)
		assert.GreaterOrEqual(t, delay, 50*time.Millisecond) // 100/2 minimum
		assert.Less(t, delay, 100*time.Millisecond)          // < 100 due to jitter
	})

	t.Run("delay within max for attempt 1", func(t *testing.T) {
		// Attempt 1: 100ms * 2^1 = 200ms
		delay := calculateDelay(1, baseDelay, maxDelay)
		assert.GreaterOrEqual(t, delay, 100*time.Millisecond) // 200/2 minimum
		assert.Less(t, delay, 200*time.Millisecond)           // < 200 due to jitter
	})

	t.Run("delay exceeds max (capped)", func(t *testing.T) {
		// Attempt 10: 100ms * 2^10 = 102400ms, but capped at 500ms
		delay := calculateDelay(10, baseDelay, maxDelay)
		assert.GreaterOrEqual(t, delay, 250*time.Millisecond) // 500/2 minimum
		assert.Less(t, delay, 500*time.Millisecond)           // < 500 due to jitter
	})

	t.Run("large attempt numbers capped", func(t *testing.T) {
		// Attempt 5: 100ms * 2^5 = 3200ms, capped at 500ms
		delay := calculateDelay(5, baseDelay, maxDelay)
		assert.GreaterOrEqual(t, delay, 250*time.Millisecond)
		assert.Less(t, delay, 500*time.Millisecond)
	})

	t.Run("different base delays", func(t *testing.T) {
		smallBase := 10 * time.Millisecond
		delay := calculateDelay(0, smallBase, time.Second)
		assert.GreaterOrEqual(t, delay, 5*time.Millisecond)
		assert.Less(t, delay, 10*time.Millisecond)
	})
}
