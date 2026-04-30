package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

// Limiter decides whether a request for a given key should proceed.
// Implementations may be in-process or backed by an external store (e.g. Redis).
type Limiter interface {
	Allow(key string) bool
}

// InProcessLimiter is a per-key token-bucket limiter backed by golang.org/x/time/rate.
// Each unique key gets its own bucket; buckets are allocated on first use.
type InProcessLimiter struct {
	limit rate.Limit
	burst int
	mu    sync.Mutex
	keys  map[string]*rate.Limiter
}

// NewInProcess returns an InProcessLimiter with the given rate (events/sec) and burst size.
func NewInProcess(limit rate.Limit, burst int) *InProcessLimiter {
	return &InProcessLimiter{
		limit: limit,
		burst: burst,
		keys:  make(map[string]*rate.Limiter),
	}
}

// Allow reports whether a request for key should be allowed under the rate limit.
func (l *InProcessLimiter) Allow(key string) bool {
	l.mu.Lock()
	rl, ok := l.keys[key]
	if !ok {
		rl = rate.NewLimiter(l.limit, l.burst)
		l.keys[key] = rl
	}
	l.mu.Unlock()
	return rl.Allow()
}
