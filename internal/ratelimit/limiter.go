package ratelimit

import (
	"container/list"
	"sync"
	"time"

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
//
// Eviction footgun: a key that falls off the LRU would normally re-enter with a full
// burst on its next request, letting an attacker cycle keys to bypass the rate limit.
// To close this, evicted keys are tracked in a quarantine log (same capacity as the
// main cache). A key that re-enters from quarantine starts with zero tokens; it must
// wait for the normal refill interval before making progress.
type InProcessLimiter struct {
	limit      rate.Limit
	burst      int
	maxBuckets int
	mu         sync.Mutex
	ll         *list.List
	keys       map[string]*list.Element
	// qll/qmap: quarantine log for recently-evicted keys.
	qll  *list.List
	qmap map[string]*list.Element
}

type lruEntry struct {
	key string
	rl  *rate.Limiter
}

// InProcessOption configures an InProcessLimiter.
type InProcessOption func(*InProcessLimiter)

// WithMaxBuckets sets the maximum number of key buckets held in memory.
// When the cap is reached the least-recently-used bucket is evicted; the next
// request for that key starts a fresh full bucket unless the key is in the
// quarantine log, in which case it starts with zero tokens. Defaults to 100_000.
func WithMaxBuckets(n int) InProcessOption {
	return func(l *InProcessLimiter) { l.maxBuckets = n }
}

// NewInProcess returns an InProcessLimiter with the given rate (events/sec) and burst size.
func NewInProcess(limit rate.Limit, burst int, opts ...InProcessOption) *InProcessLimiter {
	l := &InProcessLimiter{
		limit:      limit,
		burst:      burst,
		maxBuckets: defaultMaxBuckets,
	}
	for _, o := range opts {
		o(l)
	}
	l.ll = list.New()
	l.keys = make(map[string]*list.Element)
	l.qll = list.New()
	l.qmap = make(map[string]*list.Element)
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
	if _, quarantined := l.qmap[key]; quarantined {
		// Drain initial tokens so the re-entering key cannot exploit the free burst.
		rl.AllowN(time.Now(), l.burst)
	}
	el := l.ll.PushFront(&lruEntry{key: key, rl: rl})
	l.keys[key] = el
	if l.ll.Len() > l.maxBuckets {
		back := l.ll.Back()
		l.ll.Remove(back)
		evicted := back.Value.(*lruEntry).key
		delete(l.keys, evicted)
		l.quarantine(evicted)
	}
	l.mu.Unlock()
	return rl.Allow()
}

// quarantine adds key to the eviction log, evicting the oldest entry if full.
// Caller must hold l.mu.
func (l *InProcessLimiter) quarantine(key string) {
	if _, ok := l.qmap[key]; ok {
		return // already present; no-op
	}
	qel := l.qll.PushFront(key)
	l.qmap[key] = qel
	if l.qll.Len() > l.maxBuckets {
		qback := l.qll.Back()
		l.qll.Remove(qback)
		delete(l.qmap, qback.Value.(string))
	}
}
