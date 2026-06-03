package validation

import (
	"sync"

	"github.com/opendecree/decree/internal/cache"
)

const defaultMaxValidatorEntries = 1000

// ValidatorCache caches field validators per tenant ID.
// Thread-safe via RWMutex. Bounded by maxEntries — when full, the least-recently
// used tenant's validators are evicted.
type ValidatorCache struct {
	mu         sync.RWMutex
	lru        *cache.LRU[string, map[string]*FieldValidator]
	maxEntries int
}

// NewValidatorCache creates an empty validator cache.
// maxEntries sets the upper bound on cached tenants (0 uses default of 1000).
func NewValidatorCache(maxEntries int) *ValidatorCache {
	if maxEntries <= 0 {
		maxEntries = defaultMaxValidatorEntries
	}
	return &ValidatorCache{
		lru:        cache.NewLRU[string, map[string]*FieldValidator](maxEntries),
		maxEntries: maxEntries,
	}
}

// Get returns cached validators for a tenant, or nil if not cached.
// Uses Peek to preserve insertion-order eviction semantics.
func (c *ValidatorCache) Get(tenantID string) (map[string]*FieldValidator, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Peek(tenantID)
}

// Set stores validators for a tenant.
func (c *ValidatorCache) Set(tenantID string, validators map[string]*FieldValidator) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lru.Set(tenantID, validators)
}

// Invalidate removes cached validators for a tenant.
func (c *ValidatorCache) Invalidate(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lru.Delete(tenantID)
}

// Len returns the number of cached tenants.
func (c *ValidatorCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lru.Len()
}
