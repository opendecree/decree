package validation

import "sync"

const defaultMaxValidatorEntries = 1000

// ValidatorCache caches field validators per tenant ID.
// Thread-safe via RWMutex. Bounded by maxEntries — when full, the oldest
// tenant's validators are evicted.
type ValidatorCache struct {
	mu         sync.RWMutex
	cache      map[string]map[string]*FieldValidator // tenantID → (fieldPath → validator)
	order      []string                              // insertion order for eviction
	maxEntries int
}

// NewValidatorCache creates an empty validator cache.
// maxEntries sets the upper bound on cached tenants (0 uses default of 1000).
func NewValidatorCache(maxEntries int) *ValidatorCache {
	if maxEntries <= 0 {
		maxEntries = defaultMaxValidatorEntries
	}
	return &ValidatorCache{
		cache:      make(map[string]map[string]*FieldValidator),
		maxEntries: maxEntries,
	}
}

// Get returns cached validators for a tenant, or nil if not cached.
func (c *ValidatorCache) Get(tenantID string) (map[string]*FieldValidator, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.cache[tenantID]
	return v, ok
}

// Set stores validators for a tenant.
func (c *ValidatorCache) Set(tenantID string, validators map[string]*FieldValidator) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.cache[tenantID]; !exists {
		c.evictIfNeeded()
		c.order = append(c.order, tenantID)
	}
	c.cache[tenantID] = validators
}

// Invalidate removes cached validators for a tenant.
func (c *ValidatorCache) Invalidate(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, tenantID)
	c.rebuildOrder()
}

// Len returns the number of cached tenants.
func (c *ValidatorCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// evictIfNeeded removes the oldest tenant when at capacity. Caller must hold mu.
func (c *ValidatorCache) evictIfNeeded() {
	if len(c.cache) < c.maxEntries {
		return
	}
	for _, k := range c.order {
		if _, exists := c.cache[k]; exists {
			delete(c.cache, k)
			break
		}
	}
	c.rebuildOrder()
}

// rebuildOrder removes stale entries from the order slice. Caller must hold mu.
func (c *ValidatorCache) rebuildOrder() {
	cleaned := c.order[:0]
	for _, k := range c.order {
		if _, exists := c.cache[k]; exists {
			cleaned = append(cleaned, k)
		}
	}
	c.order = cleaned
}
