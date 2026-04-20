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
