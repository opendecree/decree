package cache

import (
	"context"
	"time"
)

// ConfigCache provides caching for config values (without descriptions).
type ConfigCache interface {
	// Get retrieves cached config values for a tenant at a version.
	// Returns nil, nil on cache miss.
	Get(ctx context.Context, tenantID string, version int32) (map[string]string, error)

	// Set stores config values for a tenant at a version.
	Set(ctx context.Context, tenantID string, version int32, values map[string]string, ttl time.Duration) error

	// Invalidate removes all cached config for a tenant.
	Invalidate(ctx context.Context, tenantID string) error
}

// IdempotencyCache deduplicates write operations by key.
type IdempotencyCache interface {
	// Claim marks an idempotency key as seen. Returns true on the first call
	// with this key (caller should proceed with the write), false on subsequent
	// calls (caller should return a cached success without re-applying the write).
	// On cache error, callers should degrade gracefully and proceed with the write.
	Claim(ctx context.Context, key string, ttl time.Duration) (bool, error)
}
