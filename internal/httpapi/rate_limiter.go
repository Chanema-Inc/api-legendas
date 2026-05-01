package httpapi

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]*rateLimitEntry
}

type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:   limit,
		window:  window,
		clients: map[string]*rateLimitEntry{},
	}
}

func (limiter *RateLimiter) Allow(clientID string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now().UTC()
	entry, found := limiter.clients[clientID]
	if !found || now.Sub(entry.windowStart) >= limiter.window {
		limiter.clients[clientID] = &rateLimitEntry{count: 1, windowStart: now}
		return true
	}

	if entry.count >= limiter.limit {
		return false
	}

	entry.count++
	return true
}
