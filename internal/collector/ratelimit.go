package unsampleprocessor

import (
	"sync"
	"time"
)

// RateLimiter implements a simple token-bucket rate limiter.
// It allows up to maxPerMinute operations per 60-second window.
//
// This rate limiter is specifically designed for the Collector processor:
// when Allow() returns false, the caller must silently drop the span
// (never retry, never buffer, never return an error).
type RateLimiter struct {
	mu          sync.Mutex
	maxPerMin   int
	count       int
	windowStart time.Time
	nowFunc     func() time.Time // injectable for testing
}

// NewRateLimiter creates a rate limiter that allows max operations per minute.
func NewRateLimiter(maxPerMinute int) *RateLimiter {
	return &RateLimiter{
		maxPerMin:   maxPerMinute,
		windowStart: time.Now(),
		nowFunc:     time.Now,
	}
}

// Allow returns true if the operation is within the rate limit.
// If the window has expired, it resets the counter and starts a new window.
// This method is safe for concurrent use.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.nowFunc()

	// Reset window if it's expired.
	if now.Sub(r.windowStart) >= time.Minute {
		r.count = 0
		r.windowStart = now
	}

	if r.count >= r.maxPerMin {
		return false
	}

	r.count++
	return true
}
