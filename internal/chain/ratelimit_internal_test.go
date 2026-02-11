package chain

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLimiter_DoubleCheckLock(t *testing.T) {
	t.Run("concurrent access creates only one limiter per endpoint", func(t *testing.T) {
		rl := NewRateLimiter(10, 10)

		var wg sync.WaitGroup
		const goroutines = 100
		limiters := make(chan interface{}, goroutines)

		// Launch many goroutines simultaneously
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				limiter := rl.getLimiter("endpoint1")
				limiters <- limiter
			}()
		}

		wg.Wait()
		close(limiters)

		// All should receive the same limiter instance
		var first interface{}
		count := 0
		for limiter := range limiters {
			if first == nil {
				first = limiter
			}
			count++
			assert.Same(t, first, limiter, "all goroutines should get same limiter instance")
		}

		assert.Equal(t, goroutines, count)
	})

	t.Run("different endpoints get different limiters", func(t *testing.T) {
		rl := NewRateLimiter(10, 10)

		limiter1 := rl.getLimiter("endpoint1")
		limiter2 := rl.getLimiter("endpoint2")

		require.NotNil(t, limiter1)
		require.NotNil(t, limiter2)
		assert.NotSame(t, limiter1, limiter2, "different endpoints should have different limiters")
	})

	t.Run("same endpoint gets same limiter", func(t *testing.T) {
		rl := NewRateLimiter(10, 10)

		limiter1 := rl.getLimiter("endpoint1")
		limiter2 := rl.getLimiter("endpoint1")

		require.NotNil(t, limiter1)
		assert.Same(t, limiter1, limiter2, "same endpoint should reuse limiter")
	})
}

func TestGetLimiter_CreateNewLimiter(t *testing.T) {
	t.Run("creates limiter on first access", func(t *testing.T) {
		rl := NewRateLimiter(10, 10)

		// Verify no limiters exist initially
		rl.mu.RLock()
		count := len(rl.limiters)
		rl.mu.RUnlock()
		assert.Equal(t, 0, count)

		// Access endpoint
		limiter := rl.getLimiter("new-endpoint")
		require.NotNil(t, limiter)

		// Verify limiter was created
		rl.mu.RLock()
		count = len(rl.limiters)
		rl.mu.RUnlock()
		assert.Equal(t, 1, count)
	})

	t.Run("limiter has correct rate and burst", func(t *testing.T) {
		ratePerSec := 5.0
		burst := 10
		rl := NewRateLimiter(ratePerSec, burst)

		limiter := rl.getLimiter("test")

		// Test that the limiter respects the burst
		for i := 0; i < burst; i++ {
			allowed := limiter.Allow()
			assert.True(t, allowed, "should allow within burst limit")
		}

		// Next request should be denied (burst exhausted)
		allowed := limiter.Allow()
		assert.False(t, allowed, "should deny after burst exhausted")
	})
}
