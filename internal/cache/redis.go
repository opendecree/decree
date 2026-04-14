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

func (c *RedisCache) key(tenantID string, version int32) string {
	return fmt.Sprintf("%s%s:v%d", c.prefix, tenantID, version)
}

func (c *RedisCache) tenantPattern(tenantID string) string {
	return fmt.Sprintf("%s%s:*", c.prefix, tenantID)
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
	key := c.key(tenantID, version)
	pipe := c.client.Pipeline()
	pipe.HSet(ctx, key, values)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("cache set: %w", err)
	}
	return nil
}

func (c *RedisCache) Invalidate(ctx context.Context, tenantID string) error {
	iter := c.client.Scan(ctx, 0, c.tenantPattern(tenantID), 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("cache invalidate scan: %w", err)
	}
	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("cache invalidate del: %w", err)
		}
	}
	return nil
}
