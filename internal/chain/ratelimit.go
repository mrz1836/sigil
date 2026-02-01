package chain

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// RateLimiter provides per-endpoint rate limiting using token bucket algorithm.
type RateLimiter struct {
	limiters   map[string]*rate.Limiter
	mu         sync.RWMutex
	rateLimit  rate.Limit
	burstLimit int
}

// NewRateLimiter creates a new rate limiter with the specified rate and burst.
// rate is requests per second, burst is the maximum burst size.
func NewRateLimiter(ratePerSecond float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiters:   make(map[string]*rate.Limiter),
		rateLimit:  rate.Limit(ratePerSecond),
		burstLimit: burst,
	}
}

// DefaultRateLimiter returns a rate limiter with default settings.
// Default: 5 requests/second, burst of 10.
func DefaultRateLimiter() *RateLimiter {
	return NewRateLimiter(5, 10)
}

// Allow checks if a request to the endpoint is allowed.
// Returns true if the request should proceed, false if it should be rate limited.
func (r *RateLimiter) Allow(endpoint string) bool {
	return r.getLimiter(endpoint).Allow()
}

// Wait blocks until a request to the endpoint is allowed or the context is canceled.
func (r *RateLimiter) Wait(ctx context.Context, endpoint string) error {
	return r.getLimiter(endpoint).Wait(ctx)
}

// Reserve returns a rate.Reservation for more complex rate limiting scenarios.
func (r *RateLimiter) Reserve(endpoint string) *rate.Reservation {
	return r.getLimiter(endpoint).Reserve()
}

// getLimiter returns the limiter for the given endpoint, creating one if needed.
func (r *RateLimiter) getLimiter(endpoint string) *rate.Limiter {
	r.mu.RLock()
	limiter, exists := r.limiters[endpoint]
	r.mu.RUnlock()

	if exists {
		return limiter
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = r.limiters[endpoint]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(r.rateLimit, r.burstLimit)
	r.limiters[endpoint] = limiter
	return limiter
}
