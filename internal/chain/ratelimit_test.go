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
