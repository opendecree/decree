package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultMaxEntries = 10000

// MemoryCache implements ConfigCache using an in-memory map with TTL and
// bounded size. When the cache is full, expired entries are evicted first,
// then the oldest entry is removed. A background goroutine periodically
// sweeps expired entries.
type MemoryCache struct {
	mu         sync.RWMutex
	entries    map[string]memoryCacheEntry
	order      []string // insertion order for eviction
	maxEntries int
	stopSweep  chan struct{}
}

type memoryCacheEntry struct {
	values    map[string]string
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory config cache.
// maxEntries sets the upper bound on cached entries (0 uses default of 10000).
func NewMemoryCache(maxEntries int) *MemoryCache {
	if maxEntries <= 0 {
		maxEntries = defaultMaxEntries
	}
	c := &MemoryCache{
		entries:    make(map[string]memoryCacheEntry),
		maxEntries: maxEntries,
		stopSweep:  make(chan struct{}),
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
	for k, v := range entry.values {
		result[k] = v
	}
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
	for ck, v := range values {
		copied[ck] = v
	}

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
	c.rebuildOrder()
	return nil
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

// sweepLoop periodically removes expired entries.
func (c *MemoryCache) sweepLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.sweep()
		case <-c.stopSweep:
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
	c.rebuildOrder()
}
