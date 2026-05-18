package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRedisCache_SetEmptyValuesIsNoOp verifies that Set returns nil without
// touching the Redis client when values is empty. go-redis' HSET rejects zero
// field/value pairs ("ERR wrong number of arguments for 'hset' command"), so
// we must skip the call. A nil client would panic on any pipeline use; the
// test passes only when we take the early-return path.
func TestRedisCache_SetEmptyValuesIsNoOp(t *testing.T) {
	c := &RedisCache{client: nil, prefix: "config:"}

	err := c.Set(context.Background(), "t1", 1, map[string]string{}, time.Minute)
	require.NoError(t, err)

	err = c.Set(context.Background(), "t1", 1, nil, time.Minute)
	require.NoError(t, err)
}

func TestNewRedisCache(t *testing.T) {
	c := NewRedisCache(nil)
	require.NotNil(t, c)
	require.Equal(t, "config:", c.prefix)
}

func TestRedisCache_Key(t *testing.T) {
	c := &RedisCache{prefix: "config:"}
	require.Equal(t, "config:tenant-1:v7", c.key("tenant-1", 7))
	require.Equal(t, "config:t:v0", c.key("t", 0))
}

func TestRedisCache_TenantPattern(t *testing.T) {
	c := &RedisCache{prefix: "config:"}
	require.Equal(t, "config:tenant-1:*", c.tenantPattern("tenant-1"))
}
