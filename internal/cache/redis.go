package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements ConfigCache using Redis.
type RedisCache struct {
	client *redis.Client
	prefix string
}

// NewRedisCache creates a new Redis-backed config cache.
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{
		client: client,
		prefix: "config:",
	}
}

// key returns the Redis key for a tenant's config at a specific version.
// The {tenantID} hashtag pins all per-tenant keys to the same cluster slot,
// which is required for the pipeline DEL in Invalidate to work on Redis Cluster.
func (c *RedisCache) key(tenantID string, version int32) string {
	return fmt.Sprintf("%s{%s}:v%d", c.prefix, tenantID, version)
}

// indexKey returns the Redis SET that tracks every version key for a tenant.
// Shares the same {tenantID} hashtag as key() so both land on the same slot.
func (c *RedisCache) indexKey(tenantID string) string {
	return fmt.Sprintf("config-idx:{%s}", tenantID)
}

func (c *RedisCache) Get(ctx context.Context, tenantID string, version int32) (map[string]string, error) {
	result, err := c.client.HGetAll(ctx, c.key(tenantID, version)).Result()
	if err != nil {
		return nil, fmt.Errorf("cache get: %w", err)
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func (c *RedisCache) Set(ctx context.Context, tenantID string, version int32, values map[string]string, ttl time.Duration) error {
	// Redis HSET rejects zero field/value pairs; skip the call. There is nothing
	// to cache, and a subsequent Get will correctly report a miss.
	if len(values) == 0 {
		return nil
	}
	k := c.key(tenantID, version)
	idx := c.indexKey(tenantID)
	pipe := c.client.Pipeline()
	pipe.HSet(ctx, k, values)
	pipe.Expire(ctx, k, ttl)
	pipe.SAdd(ctx, idx, k)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}

func (c *RedisCache) Invalidate(ctx context.Context, tenantID string) error {
	idx := c.indexKey(tenantID)
	keys, err := c.client.SMembers(ctx, idx).Result()
	if err != nil {
		return fmt.Errorf("cache invalidate smembers: %w", err)
	}
	if len(keys) == 0 {
		return nil
	}
	// DEL all version keys and the index itself in one call.
	keys = append(keys, idx)
	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache invalidate del: %w", err)
	}
	return nil
}

// RedisIdempotencyCache implements IdempotencyCache using Redis SET NX.
// Safe across multiple server replicas.
type RedisIdempotencyCache struct {
	client *redis.Client
	prefix string
}

// NewRedisIdempotencyCache creates a Redis-backed idempotency cache.
func NewRedisIdempotencyCache(client *redis.Client) *RedisIdempotencyCache {
	return &RedisIdempotencyCache{client: client, prefix: "idem:"}
}

func (c *RedisIdempotencyCache) Claim(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := c.client.SetNX(ctx, c.prefix+key, 1, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("idempotency claim: %w", err)
	}
	return ok, nil
}
