package cache

import (
	"context"
	"fmt"
	"maps"
	"math/rand/v2"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxEntries    = 10000
	defaultSweepInterval = time.Minute
)

// MemoryCacheOption configures optional MemoryCache settings.
type MemoryCacheOption func(*memoryCacheConfig)

type memoryCacheConfig struct {
	sweepInterval time.Duration
}

// WithSweepInterval sets the base interval between background sweep passes.
// Each pass adds ±10% jitter to prevent synchronized sweeps across instances.
func WithSweepInterval(d time.Duration) MemoryCacheOption {
	return func(cfg *memoryCacheConfig) {
		cfg.sweepInterval = d
	}
}

// MemoryCache implements ConfigCache using an in-memory map with TTL and
// bounded size. When the cache is full, expired entries are evicted first,
// then the oldest entry is removed. A background goroutine periodically
// sweeps expired entries.
type MemoryCache struct {
	mu            sync.RWMutex
	lru           *LRU[string, memoryCacheEntry]
	negEntries    *LRU[string, time.Time] // negative-cache: key → expiry; LRU-bounded to maxEntries
	maxEntries    int
	sweepInterval time.Duration
	cancel        context.CancelFunc
	stopOnce      sync.Once
}

type memoryCacheEntry struct {
	values    map[string]string
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory config cache.
// maxEntries sets the upper bound on cached entries (0 uses default of 10000).
// The background sweep goroutine stops when ctx is cancelled or Stop is called.
func NewMemoryCache(ctx context.Context, maxEntries int, opts ...MemoryCacheOption) *MemoryCache {
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	cfg := memoryCacheConfig{sweepInterval: defaultSweepInterval}
	for _, o := range opts {
		o(&cfg)
	}
	sweepCtx, cancel := context.WithCancel(ctx)
	c := &MemoryCache{
		lru:           NewLRU[string, memoryCacheEntry](maxEntries),
		negEntries:    NewLRU[string, time.Time](maxEntries),
		maxEntries:    maxEntries,
		sweepInterval: cfg.sweepInterval,
		cancel:        cancel,
	}
	go c.sweepLoop(sweepCtx)
	return c
}

func (c *MemoryCache) key(tenantID string, version int32) string {
	return fmt.Sprintf("%s:v%d", tenantID, version)
}

func (c *MemoryCache) Get(_ context.Context, tenantID string, version int32) (map[string]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.lru.Peek(c.key(tenantID, version))
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, nil
	}

	// Return a copy to prevent mutation.
	result := make(map[string]string, len(entry.values))
	maps.Copy(result, entry.values)
	return result, nil
}

func (c *MemoryCache) Set(_ context.Context, tenantID string, version int32, values map[string]string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	k := c.key(tenantID, version)

	// If key does not exist and we are at capacity, try to evict expired entries
	// first; if still full, LRU.Set will auto-evict the LRU entry.
	if _, exists := c.lru.Peek(k); !exists && c.lru.Len() >= c.maxEntries {
		c.evictExpired()
	}

	// Copy values to prevent external mutation.
	copied := make(map[string]string, len(values))
	maps.Copy(copied, values)

	c.lru.Set(k, memoryCacheEntry{
		values:    copied,
		expiresAt: time.Now().Add(ttl),
	})
	return nil
}

func (c *MemoryCache) Invalidate(_ context.Context, tenantID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := tenantID + ":"
	c.lru.Range(func(k string, _ memoryCacheEntry) {
		if strings.HasPrefix(k, prefix) {
			c.lru.Delete(k)
		}
	})
	c.negEntries.Range(func(k string, _ time.Time) {
		if strings.HasPrefix(k, prefix) {
			c.negEntries.Delete(k)
		}
	})
	return nil
}

func (c *MemoryCache) SetNegative(_ context.Context, tenantID string, version int32, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.negEntries.Set(c.key(tenantID, version), time.Now().Add(ttl))
	return nil
}

func (c *MemoryCache) GetNegative(_ context.Context, tenantID string, version int32) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	exp, ok := c.negEntries.Peek(c.key(tenantID, version))
	if !ok || time.Now().After(exp) {
		return false, nil
	}
	return true, nil
}

// Len returns the number of entries in the cache.
func (c *MemoryCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}

// Stop stops the background sweep goroutine. Safe to call more than once.
func (c *MemoryCache) Stop() {
	c.stopOnce.Do(c.cancel)
}

// evictExpired removes expired entries. Caller must hold mu.
func (c *MemoryCache) evictExpired() {
	now := time.Now()
	c.lru.DeleteWhere(func(_ string, e memoryCacheEntry) bool {
		return now.After(e.expiresAt)
	})
}

// sweepLoop periodically removes expired entries. Each iteration adds ±10%
// jitter to the configured interval to prevent synchronized sweeps when many
// instances share the same configuration. The loop exits when ctx is cancelled.
func (c *MemoryCache) sweepLoop(ctx context.Context) {
	for {
		half := c.sweepInterval / 10
		jitter := time.Duration(rand.Int64N(int64(2*half))) - half
		timer := time.NewTimer(c.sweepInterval + jitter)
		select {
		case <-timer.C:
			c.sweep()
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

func (c *MemoryCache) sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.evictExpired()

	now := time.Now()
	c.negEntries.DeleteWhere(func(_ string, exp time.Time) bool {
		return now.After(exp)
	})
}

// MemoryIdempotencyOption configures optional MemoryIdempotencyCache settings.
type MemoryIdempotencyOption func(*memoryIdempotencyConfig)

type memoryIdempotencyConfig struct {
	sweepInterval time.Duration
}

// WithIdempotencySweepInterval sets the base sweep interval for the idempotency cache.
func WithIdempotencySweepInterval(d time.Duration) MemoryIdempotencyOption {
	return func(cfg *memoryIdempotencyConfig) {
		cfg.sweepInterval = d
	}
}

// MemoryIdempotencyCache implements IdempotencyCache with an in-memory LRU.
// Suitable for single-instance dev/test deployments; does not share state across
// server replicas. Use RedisIdempotencyCache in production.
type MemoryIdempotencyCache struct {
	mu            sync.Mutex
	lru           *LRU[string, time.Time]
	maxEntries    int
	sweepInterval time.Duration
	cancel        context.CancelFunc
	stopOnce      sync.Once
}

// NewMemoryIdempotencyCache creates an in-memory idempotency cache.
// maxEntries bounds the number of live claims (0 uses default of 10000).
// The background sweep goroutine stops when ctx is cancelled or Stop is called.
func NewMemoryIdempotencyCache(ctx context.Context, maxEntries int, opts ...MemoryIdempotencyOption) *MemoryIdempotencyCache {
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	cfg := memoryIdempotencyConfig{sweepInterval: defaultSweepInterval}
	for _, o := range opts {
		o(&cfg)
	}
	sweepCtx, cancel := context.WithCancel(ctx)
	c := &MemoryIdempotencyCache{
		lru:           NewLRU[string, time.Time](maxEntries),
		maxEntries:    maxEntries,
		sweepInterval: cfg.sweepInterval,
		cancel:        cancel,
	}
	go c.sweepLoop(sweepCtx)
	return c
}

func (c *MemoryIdempotencyCache) Claim(_ context.Context, key string, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if exp, ok := c.lru.Peek(key); ok && time.Now().Before(exp) {
		return false, nil
	}
	c.lru.Set(key, time.Now().Add(ttl))
	return true, nil
}

// Len returns the number of live claims in the cache.
func (c *MemoryIdempotencyCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lru.Len()
}

// Stop stops the background sweep goroutine. Safe to call more than once.
func (c *MemoryIdempotencyCache) Stop() {
	c.stopOnce.Do(c.cancel)
}

func (c *MemoryIdempotencyCache) sweepLoop(ctx context.Context) {
	for {
		half := c.sweepInterval / 10
		jitter := time.Duration(rand.Int64N(int64(2*half))) - half
		timer := time.NewTimer(c.sweepInterval + jitter)
		select {
		case <-timer.C:
			c.sweepExpired()
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

func (c *MemoryIdempotencyCache) sweepExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.lru.DeleteWhere(func(_ string, exp time.Time) bool {
		return now.After(exp)
	})
}
