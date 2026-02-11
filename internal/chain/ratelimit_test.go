package chain_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mrz1836/sigil/internal/chain"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := chain.NewRateLimiter(10, 10) // 10/sec with burst of 10

	// Should allow initial burst
	for i := 0; i < 10; i++ {
		allowed := rl.Allow("test")
		assert.True(t, allowed, "should allow request %d in burst", i)
	}

	// 11th request should be denied (burst exhausted)
	allowed := rl.Allow("test")
	assert.False(t, allowed, "should deny request after burst exhausted")
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := chain.NewRateLimiter(100, 1) // 100/sec with burst of 1

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// First request should succeed immediately
	err := rl.Wait(ctx, "test")
	require.NoError(t, err)

	// Second request should wait briefly
	start := time.Now()
	err = rl.Wait(ctx, "test")
	require.NoError(t, err)
	elapsed := time.Since(start)

	// Should have waited approximately 10ms (1/100 second)
	assert.GreaterOrEqual(t, elapsed, 5*time.Millisecond)
}

func TestRateLimiter_SeparateEndpoints(t *testing.T) {
	rl := chain.NewRateLimiter(10, 2)

	// Each endpoint has its own limiter
	assert.True(t, rl.Allow("endpoint1"))
	assert.True(t, rl.Allow("endpoint1"))
	assert.False(t, rl.Allow("endpoint1")) // exhausted

	// endpoint2 is independent
	assert.True(t, rl.Allow("endpoint2"))
	assert.True(t, rl.Allow("endpoint2"))
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	rl := chain.NewRateLimiter(1, 1) // 1/sec

	// Exhaust the burst
	err := rl.Wait(context.Background(), "test")
	require.NoError(t, err)

	// Cancel the context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Wait should fail with context error
	err = rl.Wait(ctx, "test")
	assert.Error(t, err)
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := chain.NewRateLimiter(100, 100)

	var wg sync.WaitGroup
	successes := make(chan bool, 200)

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			successes <- rl.Allow("test")
		}()
	}

	wg.Wait()
	close(successes)

	// Count successes
	count := 0
	for s := range successes {
		if s {
			count++
		}
	}

	// Should have allowed approximately 100 (the burst size)
	assert.GreaterOrEqual(t, count, 90)
	assert.LessOrEqual(t, count, 110)
}

func TestDefaultRateLimiter(t *testing.T) {
	rl := chain.DefaultRateLimiter()

	require.NotNil(t, rl)

	// Default allows burst of 10
	for i := 0; i < 10; i++ {
		allowed := rl.Allow("test")
		assert.True(t, allowed, "should allow request %d in default burst", i)
	}

	// 11th request should be denied
	allowed := rl.Allow("test")
	assert.False(t, allowed, "should deny request after default burst exhausted")
}

func TestRateLimiter_Reserve(t *testing.T) {
	t.Run("returns valid reservation", func(t *testing.T) {
		rl := chain.NewRateLimiter(10, 5)
		reservation := rl.Reserve("test")

		require.NotNil(t, reservation)
		assert.True(t, reservation.OK())
	})

	t.Run("multiple reservations work", func(t *testing.T) {
		rl := chain.NewRateLimiter(10, 3)

		// Reserve 3 tokens
		r1 := rl.Reserve("test")
		r2 := rl.Reserve("test")
		r3 := rl.Reserve("test")

		assert.True(t, r1.OK())
		assert.True(t, r2.OK())
		assert.True(t, r3.OK())

		// 4th reservation should require delay
		r4 := rl.Reserve("test")
		assert.True(t, r4.OK())
		assert.Greater(t, r4.Delay(), time.Duration(0))
	})

	t.Run("different endpoints independent", func(t *testing.T) {
		rl := chain.NewRateLimiter(10, 2)

		// Exhaust endpoint1
		rl.Reserve("endpoint1")
		rl.Reserve("endpoint1")
		r1 := rl.Reserve("endpoint1")
		assert.Greater(t, r1.Delay(), time.Duration(0))

		// endpoint2 should still have tokens
		r2 := rl.Reserve("endpoint2")
		assert.Equal(t, time.Duration(0), r2.Delay())
	})
}
