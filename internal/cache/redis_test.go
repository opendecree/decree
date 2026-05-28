package cache

import (
	"context"
	"strings"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
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
	require.Equal(t, "config:{tenant-1}:v7", c.key("tenant-1", 7))
	require.Equal(t, "config:{t}:v0", c.key("t", 0))
}

func TestRedisCache_IndexKey(t *testing.T) {
	c := &RedisCache{prefix: "config:"}
	require.Equal(t, "config-idx:{tenant-1}", c.indexKey("tenant-1"))
}

// --- miniredis integration tests ---

func newTestRedisCache(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr:       mr.Addr(),
		MaxRetries: 0,
	})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisCache(client), mr
}

func TestRedisCache_GetSet_Happy(t *testing.T) {
	c, _ := newTestRedisCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1", "b": "2"}, time.Minute))

	got, err := c.Get(ctx, "t1", 1)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"a": "1", "b": "2"}, got)
}

func TestRedisCache_GetMiss(t *testing.T) {
	c, _ := newTestRedisCache(t)
	got, err := c.Get(context.Background(), "t1", 1)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRedisCache_Invalidate_Happy(t *testing.T) {
	c, _ := newTestRedisCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t1", 2, map[string]string{"b": "2"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t2", 1, map[string]string{"c": "3"}, time.Minute))

	require.NoError(t, c.Invalidate(ctx, "t1"))

	got, err := c.Get(ctx, "t1", 1)
	require.NoError(t, err)
	assert.Nil(t, got)

	got, err = c.Get(ctx, "t1", 2)
	require.NoError(t, err)
	assert.Nil(t, got)

	// t2 must not be affected by t1 invalidation.
	got, err = c.Get(ctx, "t2", 1)
	require.NoError(t, err)
	assert.Equal(t, "3", got["c"])
}

// TestRedisCache_Set_KeyHasTTL guards against regression where EXPIRE is
// skipped or decoupled from HSET, leaving a persistent (TTL-less) key.
func TestRedisCache_Set_KeyHasTTL(t *testing.T) {
	c, _ := newTestRedisCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Minute))

	ttl, err := c.client.TTL(ctx, c.key("t1", 1)).Result()
	require.NoError(t, err)
	assert.Positive(t, ttl, "key must have a TTL set atomically with HSET")
}

func TestRedisCache_TTLBoundary(t *testing.T) {
	c, mr := newTestRedisCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Second))

	got, err := c.Get(ctx, "t1", 1)
	require.NoError(t, err)
	require.NotNil(t, got, "entry should exist before TTL expires")

	mr.FastForward(2 * time.Second)

	got, err = c.Get(ctx, "t1", 1)
	require.NoError(t, err)
	assert.Nil(t, got, "entry should be gone after TTL expires")
}

func TestRedisCache_RedisDownMidGet(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr:       mr.Addr(),
		MaxRetries: 0,
	})
	t.Cleanup(func() { _ = client.Close() })
	c := NewRedisCache(client)
	ctx := context.Background()

	mr.Close()

	_, err := c.Get(ctx, "t1", 1)
	require.Error(t, err)
}

func TestRedisCache_StaleAfterRollback(t *testing.T) {
	c, _ := newTestRedisCache(t)
	ctx := context.Background()

	// v1 active, then v2 deployed.
	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"fee": "0.01"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t1", 2, map[string]string{"fee": "0.02"}, time.Minute))

	// Rollback: invalidate the entire tenant to clear stale v2 data.
	require.NoError(t, c.Invalidate(ctx, "t1"))

	got, err := c.Get(ctx, "t1", 2)
	require.NoError(t, err)
	assert.Nil(t, got, "stale v2 must be evicted after rollback")

	got, err = c.Get(ctx, "t1", 1)
	require.NoError(t, err)
	assert.Nil(t, got, "v1 also evicted; repopulated on next read-through")

	// Re-warm cache with rolled-back version.
	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"fee": "0.01"}, time.Minute))
	got, err = c.Get(ctx, "t1", 1)
	require.NoError(t, err)
	assert.Equal(t, "0.01", got["fee"])
}

func TestRedisCache_KeyCollisionAcrossTenants(t *testing.T) {
	c, _ := newTestRedisCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "tenant-a", 1, map[string]string{"x": "a"}, time.Minute))
	require.NoError(t, c.Set(ctx, "tenant-b", 1, map[string]string{"x": "b"}, time.Minute))

	gotA, err := c.Get(ctx, "tenant-a", 1)
	require.NoError(t, err)
	assert.Equal(t, "a", gotA["x"])

	gotB, err := c.Get(ctx, "tenant-b", 1)
	require.NoError(t, err)
	assert.Equal(t, "b", gotB["x"])

	// Invalidating tenant-a must not touch tenant-b.
	require.NoError(t, c.Invalidate(ctx, "tenant-a"))

	gotA, err = c.Get(ctx, "tenant-a", 1)
	require.NoError(t, err)
	assert.Nil(t, gotA)

	gotB, err = c.Get(ctx, "tenant-b", 1)
	require.NoError(t, err)
	assert.Equal(t, "b", gotB["x"])
}

func TestRedisCache_Invalidate_EmptyTenantIsNoOp(t *testing.T) {
	c, _ := newTestRedisCache(t)
	require.NoError(t, c.Invalidate(context.Background(), "no-such-tenant"))
}

func TestRedisCache_Invalidate_IndexCleanedUp(t *testing.T) {
	c, _ := newTestRedisCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t1", 2, map[string]string{"b": "2"}, time.Minute))

	members, err := c.client.SMembers(ctx, c.indexKey("t1")).Result()
	require.NoError(t, err)
	require.Len(t, members, 2, "index should hold both version keys before invalidation")

	require.NoError(t, c.Invalidate(ctx, "t1"))

	n, err := c.client.Exists(ctx, c.indexKey("t1")).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), n, "index must be deleted after invalidation")
}

func TestRedisCache_Invalidate_StaleIndexEntryIsNoOp(t *testing.T) {
	c, mr := newTestRedisCache(t)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Second))

	mr.FastForward(2 * time.Second) // data key expired; index still holds the reference

	// Invalidate must succeed even when all indexed keys have already expired.
	require.NoError(t, c.Invalidate(ctx, "t1"))

	// Index itself is cleaned up.
	n, err := c.client.Exists(ctx, c.indexKey("t1")).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

func TestRedisCache_Invalidate_SmembersError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = client.Close() })
	c := NewRedisCache(client)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Minute))

	mr.Close()

	err := c.Invalidate(ctx, "t1")
	require.Error(t, err)
	require.ErrorContains(t, err, "cache invalidate smembers")
}

func TestRedisCache_Invalidate_DelError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = client.Close() })
	c := NewRedisCache(client)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Minute))

	// Inject an error only for DEL so that SMEMBERS succeeds first.
	mr.Server().SetPreHook(server.Hook(func(p *server.Peer, cmd string, args ...string) bool {
		if strings.EqualFold(cmd, "del") {
			p.WriteError("READONLY forced")
			return true
		}
		return false
	}))

	err := c.Invalidate(ctx, "t1")
	require.Error(t, err)
	require.ErrorContains(t, err, "cache invalidate del")
}

// --- Negative cache ---

func TestRedisCache_NegKey(t *testing.T) {
	c := &RedisCache{prefix: "config:"}
	require.Equal(t, "config:{tenant-1}:neg:v3", c.negKey("tenant-1", 3))
}

func TestRedisCache_NegativeCache_Miss(t *testing.T) {
	c, _ := newTestRedisCache(t)
	neg, err := c.GetNegative(context.Background(), "t1", 1)
	require.NoError(t, err)
	assert.False(t, neg, "miss before any SetNegative")
}

func TestRedisCache_NegativeCache_SetAndGet(t *testing.T) {
	c, _ := newTestRedisCache(t)
	ctx := context.Background()

	require.NoError(t, c.SetNegative(ctx, "t1", 1, time.Minute))

	neg, err := c.GetNegative(ctx, "t1", 1)
	require.NoError(t, err)
	assert.True(t, neg)
}

func TestRedisCache_NegativeCache_TTLExpiry(t *testing.T) {
	c, mr := newTestRedisCache(t)
	ctx := context.Background()

	require.NoError(t, c.SetNegative(ctx, "t1", 1, time.Second))
	mr.FastForward(2 * time.Second)

	neg, err := c.GetNegative(ctx, "t1", 1)
	require.NoError(t, err)
	assert.False(t, neg, "expired negative entry must report miss")
}

func TestRedisCache_NegativeCache_InvalidateClears(t *testing.T) {
	c, _ := newTestRedisCache(t)
	ctx := context.Background()

	require.NoError(t, c.SetNegative(ctx, "t1", 1, time.Minute))
	require.NoError(t, c.SetNegative(ctx, "t1", 2, time.Minute))
	require.NoError(t, c.SetNegative(ctx, "t2", 1, time.Minute))

	require.NoError(t, c.Invalidate(ctx, "t1"))

	neg, _ := c.GetNegative(ctx, "t1", 1)
	assert.False(t, neg, "t1:v1 neg must be cleared")
	neg, _ = c.GetNegative(ctx, "t1", 2)
	assert.False(t, neg, "t1:v2 neg must be cleared")

	neg, _ = c.GetNegative(ctx, "t2", 1)
	assert.True(t, neg, "t2 must be unaffected")
}

func TestRedisCache_NegativeCache_RedisDownOnSet(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = client.Close() })
	c := NewRedisCache(client)
	mr.Close()

	err := c.SetNegative(context.Background(), "t1", 1, time.Minute)
	require.Error(t, err)
}

func TestRedisCache_NegativeCache_RedisDownOnGet(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: 0})
	t.Cleanup(func() { _ = client.Close() })
	c := NewRedisCache(client)
	mr.Close()

	_, err := c.GetNegative(context.Background(), "t1", 1)
	require.Error(t, err)
}

// --- RedisIdempotencyCache ---

func newTestRedisIdempotencyCache(t *testing.T) (*RedisIdempotencyCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisIdempotencyCache(client), mr
}

func TestRedisIdempotencyCache_FirstClaimReturnsTrue(t *testing.T) {
	c, _ := newTestRedisIdempotencyCache(t)
	first, err := c.Claim(context.Background(), "k1", time.Minute)
	require.NoError(t, err)
	assert.True(t, first)
}

func TestRedisIdempotencyCache_SecondClaimReturnsFalse(t *testing.T) {
	c, _ := newTestRedisIdempotencyCache(t)
	ctx := context.Background()
	first, _ := c.Claim(ctx, "k1", time.Minute)
	require.True(t, first)
	second, err := c.Claim(ctx, "k1", time.Minute)
	require.NoError(t, err)
	assert.False(t, second)
}

func TestRedisIdempotencyCache_ExpiredKeyAllowsReclaim(t *testing.T) {
	c, mr := newTestRedisIdempotencyCache(t)
	ctx := context.Background()
	_, _ = c.Claim(ctx, "k1", time.Second)
	mr.FastForward(2 * time.Second)
	again, err := c.Claim(ctx, "k1", time.Minute)
	require.NoError(t, err)
	assert.True(t, again)
}

func TestRedisIdempotencyCache_DifferentKeysAreIndependent(t *testing.T) {
	c, _ := newTestRedisIdempotencyCache(t)
	ctx := context.Background()
	a, _ := c.Claim(ctx, "a", time.Minute)
	b, _ := c.Claim(ctx, "b", time.Minute)
	assert.True(t, a)
	assert.True(t, b)
}
