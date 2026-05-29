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
	entries       map[string]memoryCacheEntry
	negEntries    map[string]time.Time // negative-cache: key → expiry
	order         []string             // insertion order for eviction
	maxEntries    int
	sweepInterval time.Duration
	stopSweep     chan struct{}
}

type memoryCacheEntry struct {
	values    map[string]string
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory config cache.
// maxEntries sets the upper bound on cached entries (0 uses default of 10000).
func NewMemoryCache(maxEntries int, opts ...MemoryCacheOption) *MemoryCache {
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	cfg := memoryCacheConfig{sweepInterval: defaultSweepInterval}
	for _, o := range opts {
		o(&cfg)
	}
	c := &MemoryCache{
		entries:       make(map[string]memoryCacheEntry),
		negEntries:    make(map[string]time.Time),
		maxEntries:    maxEntries,
		sweepInterval: cfg.sweepInterval,
		stopSweep:     make(chan struct{}),
	}
	go c.sweepLoop()
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

	// If key already exists, just update it (no new order entry).
	if _, exists := c.entries[k]; !exists {
		c.evictIfNeeded()
		c.order = append(c.order, k)
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
			delete(c.entries, k)
		}
	}
	for k := range c.negEntries {
		if strings.HasPrefix(k, prefix) {
			delete(c.negEntries, k)
		}
	}
	c.rebuildOrder()
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

// Stop stops the background sweep goroutine.
func (c *MemoryCache) Stop() {
	close(c.stopSweep)
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
			delete(c.entries, k)
		}
	}
	if len(c.entries) < c.maxEntries {
		c.rebuildOrder()
		return
	}

	// Still full: evict oldest.
	for _, k := range c.order {
		if _, exists := c.entries[k]; exists {
			delete(c.entries, k)
			break
		}
	}
	c.rebuildOrder()
}

// rebuildOrder rebuilds the order slice from existing entries. Caller must hold mu.
func (c *MemoryCache) rebuildOrder() {
	cleaned := c.order[:0]
	for _, k := range c.order {
		if _, exists := c.entries[k]; exists {
			cleaned = append(cleaned, k)
		}
	}
	c.order = cleaned
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
// instances share the same configuration.
func (c *MemoryCache) sweepLoop() {
	for {
		half := c.sweepInterval / 10
		jitter := time.Duration(rand.Int64N(int64(2*half))) - half
		timer := time.NewTimer(c.sweepInterval + jitter)
		select {
		case <-timer.C:
			c.sweep()
		case <-c.stopSweep:
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
			delete(c.entries, k)
		}
	}
	for k, exp := range c.negEntries {
		if now.After(exp) {
			delete(c.negEntries, k)
		}
	}
	c.rebuildOrder()
}
