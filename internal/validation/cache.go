package validation

import "sync"

// ValidatorCache caches field validators per tenant ID.
// Thread-safe via RWMutex.
type ValidatorCache struct {
	mu    sync.RWMutex
	cache map[string]map[string]*FieldValidator // tenantID → (fieldPath → validator)
}

// NewValidatorCache creates an empty validator cache.
func NewValidatorCache() *ValidatorCache {
	return &ValidatorCache{
		cache: make(map[string]map[string]*FieldValidator),
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
	c.cache[tenantID] = validators
}

// Invalidate removes cached validators for a tenant.
func (c *ValidatorCache) Invalidate(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, tenantID)
}
