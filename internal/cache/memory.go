package cache

import (
	"container/list"
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
	entries       map[string]memoryCacheEntry
	negEntries    map[string]time.Time     // negative-cache: key → expiry
	lru           *list.List               // front = oldest, back = newest
	lruIndex      map[string]*list.Element // key → list element for O(1) removal
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
		entries:       make(map[string]memoryCacheEntry),
		negEntries:    make(map[string]time.Time),
		lru:           list.New(),
		lruIndex:      make(map[string]*list.Element),
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

	entry, ok := c.entries[c.key(tenantID, version)]
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

	// If key already exists, just update it (no new LRU entry).
	if _, exists := c.entries[k]; !exists {
		c.evictIfNeeded()
		c.lruIndex[k] = c.lru.PushBack(k)
	}

	// Copy values to prevent external mutation.
	copied := make(map[string]string, len(values))
	maps.Copy(copied, values)

	c.entries[k] = memoryCacheEntry{
		values:    copied,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (c *MemoryCache) Invalidate(_ context.Context, tenantID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := tenantID + ":"
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			c.deleteEntry(k)
		}
	}
	for k := range c.negEntries {
		if strings.HasPrefix(k, prefix) {
			delete(c.negEntries, k)
		}
	}
	return nil
}

func (c *MemoryCache) SetNegative(_ context.Context, tenantID string, version int32, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.negEntries[c.key(tenantID, version)] = time.Now().Add(ttl)
	return nil
}

func (c *MemoryCache) GetNegative(_ context.Context, tenantID string, version int32) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	exp, ok := c.negEntries[c.key(tenantID, version)]
	if !ok || time.Now().After(exp) {
		return false, nil
	}
	return true, nil
}

// Len returns the number of entries in the cache.
func (c *MemoryCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Stop stops the background sweep goroutine. Safe to call more than once.
func (c *MemoryCache) Stop() {
	c.stopOnce.Do(c.cancel)
}

// deleteEntry removes an entry and its LRU tracking. Caller must hold mu.
func (c *MemoryCache) deleteEntry(k string) {
	delete(c.entries, k)
	if elem, ok := c.lruIndex[k]; ok {
		c.lru.Remove(elem)
		delete(c.lruIndex, k)
	}
}

// evictIfNeeded removes entries when at capacity. Caller must hold mu.
func (c *MemoryCache) evictIfNeeded() {
	if len(c.entries) < c.maxEntries {
		return
	}

	// First pass: remove expired entries.
	now := time.Now()
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			c.deleteEntry(k)
		}
	}
	if len(c.entries) < c.maxEntries {
		return
	}

	// Still full: evict oldest (front of LRU list).
	if front := c.lru.Front(); front != nil {
		c.deleteEntry(front.Value.(string))
	}
}

// MemoryIdempotencyCache implements IdempotencyCache with an in-memory map.
// Suitable for single-instance dev/test deployments; does not share state across
// server replicas. Use RedisIdempotencyCache in production.
type MemoryIdempotencyCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

// NewMemoryIdempotencyCache creates an in-memory idempotency cache.
func NewMemoryIdempotencyCache() *MemoryIdempotencyCache {
	return &MemoryIdempotencyCache{entries: make(map[string]time.Time)}
}

func (c *MemoryIdempotencyCache) Claim(_ context.Context, key string, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if exp, ok := c.entries[key]; ok && time.Now().Before(exp) {
		return false, nil
	}
	c.entries[key] = time.Now().Add(ttl)
	return true, nil
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

	now := time.Now()
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			c.deleteEntry(k)
		}
	}
	for k, exp := range c.negEntries {
		if now.After(exp) {
			delete(c.negEntries, k)
		}
	}
}
