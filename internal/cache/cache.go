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
