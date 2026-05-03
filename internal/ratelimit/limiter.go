package ratelimit

import (
	"container/list"
	"sync"

	"golang.org/x/time/rate"
)

const defaultMaxBuckets = 100_000

// Limiter decides whether a request for a given key should proceed.
// Implementations may be in-process or backed by an external store (e.g. Redis).
type Limiter interface {
	Allow(key string) bool
}

// InProcessLimiter is a per-key token-bucket limiter backed by golang.org/x/time/rate.
// Each unique key gets its own bucket, allocated on first use. Buckets are evicted
// via LRU when the cache reaches capacity, preventing unbounded memory growth.
type InProcessLimiter struct {
	limit      rate.Limit
	burst      int
	maxBuckets int
	mu         sync.Mutex
	ll         *list.List
	keys       map[string]*list.Element
}

type lruEntry struct {
	key string
	rl  *rate.Limiter
}

// InProcessOption configures an InProcessLimiter.
type InProcessOption func(*InProcessLimiter)

// WithMaxBuckets sets the maximum number of key buckets held in memory.
// When the cap is reached the least-recently-used bucket is evicted; the next
// request for that key starts a fresh full bucket. Defaults to 100_000.
func WithMaxBuckets(n int) InProcessOption {
	return func(l *InProcessLimiter) { l.maxBuckets = n }
}

// NewInProcess returns an InProcessLimiter with the given rate (events/sec) and burst size.
func NewInProcess(limit rate.Limit, burst int, opts ...InProcessOption) *InProcessLimiter {
	l := &InProcessLimiter{
		limit:      limit,
		burst:      burst,
		maxBuckets: defaultMaxBuckets,
		ll:         list.New(),
		keys:       make(map[string]*list.Element),
	}
	for _, o := range opts {
		o(l)
	}
	return l
}

// Allow reports whether a request for key should be allowed under the rate limit.
func (l *InProcessLimiter) Allow(key string) bool {
	l.mu.Lock()
	if el, ok := l.keys[key]; ok {
		l.ll.MoveToFront(el)
		rl := el.Value.(*lruEntry).rl
		l.mu.Unlock()
		return rl.Allow()
	}
	rl := rate.NewLimiter(l.limit, l.burst)
	el := l.ll.PushFront(&lruEntry{key: key, rl: rl})
	l.keys[key] = el
	if l.ll.Len() > l.maxBuckets {
		back := l.ll.Back()
		l.ll.Remove(back)
		delete(l.keys, back.Value.(*lruEntry).key)
	}
	l.mu.Unlock()
	return rl.Allow()
}
