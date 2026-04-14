package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MemoryCache implements ConfigCache using an in-memory map with TTL.
// Safe for concurrent use.
type MemoryCache struct {
	mu      sync.RWMutex
	entries map[string]memoryCacheEntry
}

type memoryCacheEntry struct {
	values    map[string]string
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory config cache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		entries: make(map[string]memoryCacheEntry),
	}
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

	// Copy values to prevent external mutation.
	copied := make(map[string]string, len(values))
	for k, v := range values {
		copied[k] = v
	}

	c.entries[c.key(tenantID, version)] = memoryCacheEntry{
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
	return nil
}
