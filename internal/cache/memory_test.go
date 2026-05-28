package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache_SetAndGet(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"a": "1", "b": "2"}, time.Minute))

	got, err := c.Get(ctx, "t1", 1, 0)
	require.NoError(t, err)
	assert.Equal(t, "1", got["a"])
	assert.Equal(t, "2", got["b"])
}

func TestMemoryCache_Miss(t *testing.T) {
	c := NewMemoryCache(0)
	got, err := c.Get(context.Background(), "t1", 1, 0)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryCache_TTLExpiry(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"a": "1"}, time.Millisecond))
	time.Sleep(5 * time.Millisecond)

	got, err := c.Get(ctx, "t1", 1, 0)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryCache_Invalidate(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"a": "1"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t1", 2, 0, map[string]string{"b": "2"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t2", 1, 0, map[string]string{"c": "3"}, time.Minute))

	require.NoError(t, c.Invalidate(ctx, "t1"))

	got, _ := c.Get(ctx, "t1", 1, 0)
	assert.Nil(t, got)
	got, _ = c.Get(ctx, "t1", 2, 0)
	assert.Nil(t, got)

	// t2 should be unaffected.
	got, _ = c.Get(ctx, "t2", 1, 0)
	assert.Equal(t, "3", got["c"])
}

func TestMemoryCache_ReturnsCopy(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"a": "1"}, time.Minute))

	got, _ := c.Get(ctx, "t1", 1, 0)
	got["a"] = "mutated"

	got2, _ := c.Get(ctx, "t1", 1, 0)
	assert.Equal(t, "1", got2["a"], "cache should not be affected by external mutation")
}

func TestMemoryCache_DifferentVersions(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"a": "v1"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t1", 2, 0, map[string]string{"a": "v2"}, time.Minute))

	got1, _ := c.Get(ctx, "t1", 1, 0)
	got2, _ := c.Get(ctx, "t1", 2, 0)
	assert.Equal(t, "v1", got1["a"])
	assert.Equal(t, "v2", got2["a"])
}

func TestMemoryCache_SchemaVersionIsolation(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	// Same configVersion, different schemaVersions → separate cache entries.
	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"x": "sv0"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t1", 1, 1, map[string]string{"x": "sv1"}, time.Minute))

	got0, _ := c.Get(ctx, "t1", 1, 0)
	got1, _ := c.Get(ctx, "t1", 1, 1)
	assert.Equal(t, "sv0", got0["x"])
	assert.Equal(t, "sv1", got1["x"])

	// Invalidate clears both schema versions.
	require.NoError(t, c.Invalidate(ctx, "t1"))
	got0, _ = c.Get(ctx, "t1", 1, 0)
	got1, _ = c.Get(ctx, "t1", 1, 1)
	assert.Nil(t, got0)
	assert.Nil(t, got1)
}

func TestMemoryCache_EvictsOldestWhenFull(t *testing.T) {
	c := NewMemoryCache(3)
	defer c.Stop()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"a": "1"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t2", 1, 0, map[string]string{"a": "2"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t3", 1, 0, map[string]string{"a": "3"}, time.Minute))
	assert.Equal(t, 3, c.Len())

	// Adding a 4th should evict the oldest (t1).
	require.NoError(t, c.Set(ctx, "t4", 1, 0, map[string]string{"a": "4"}, time.Minute))
	assert.Equal(t, 3, c.Len())

	got, _ := c.Get(ctx, "t1", 1, 0)
	assert.Nil(t, got, "oldest entry should be evicted")

	got, _ = c.Get(ctx, "t4", 1, 0)
	assert.Equal(t, "4", got["a"], "newest entry should exist")
}

func TestMemoryCache_EvictsExpiredBeforeOldest(t *testing.T) {
	c := NewMemoryCache(3)
	defer c.Stop()
	ctx := context.Background()

	// t1 expires immediately, t2 and t3 are long-lived.
	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"a": "1"}, time.Millisecond))
	require.NoError(t, c.Set(ctx, "t2", 1, 0, map[string]string{"a": "2"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t3", 1, 0, map[string]string{"a": "3"}, time.Minute))
	time.Sleep(5 * time.Millisecond) // let t1 expire

	// Adding t4 should evict expired t1, not oldest live t2.
	require.NoError(t, c.Set(ctx, "t4", 1, 0, map[string]string{"a": "4"}, time.Minute))
	assert.Equal(t, 3, c.Len())

	got, _ := c.Get(ctx, "t2", 1, 0)
	assert.Equal(t, "2", got["a"], "t2 should survive — expired t1 evicted first")
}

func TestMemoryCache_Sweep_RemovesExpiredEntries(t *testing.T) {
	c := NewMemoryCache(0)
	defer c.Stop()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"a": "1"}, time.Millisecond))
	require.NoError(t, c.Set(ctx, "t2", 1, 0, map[string]string{"a": "2"}, time.Millisecond))
	require.NoError(t, c.Set(ctx, "t3", 1, 0, map[string]string{"a": "3"}, time.Hour))
	assert.Equal(t, 3, c.Len())

	time.Sleep(5 * time.Millisecond)
	c.sweep()

	assert.Equal(t, 1, c.Len(), "only t3 should remain after sweep")

	got, _ := c.Get(ctx, "t3", 1, 0)
	assert.Equal(t, "3", got["a"])
}

func TestMemoryCache_Sweep_NoExpired_NoOp(t *testing.T) {
	c := NewMemoryCache(0)
	defer c.Stop()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"a": "1"}, time.Hour))
	require.NoError(t, c.Set(ctx, "t2", 1, 0, map[string]string{"a": "2"}, time.Hour))

	c.sweep()
	assert.Equal(t, 2, c.Len())
}

func TestMemoryCache_UpdateExistingDoesNotGrow(t *testing.T) {
	c := NewMemoryCache(2)
	defer c.Stop()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"a": "1"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t2", 1, 0, map[string]string{"a": "2"}, time.Minute))
	assert.Equal(t, 2, c.Len())

	// Updating t1 should not trigger eviction.
	require.NoError(t, c.Set(ctx, "t1", 1, 0, map[string]string{"a": "updated"}, time.Minute))
	assert.Equal(t, 2, c.Len())

	got, _ := c.Get(ctx, "t1", 1, 0)
	assert.Equal(t, "updated", got["a"])
	got, _ = c.Get(ctx, "t2", 1, 0)
	assert.Equal(t, "2", got["a"])
}

// --- MemoryIdempotencyCache ---

func TestMemoryIdempotencyCache_FirstClaimReturnsTrue(t *testing.T) {
	c := NewMemoryIdempotencyCache()
	first, err := c.Claim(context.Background(), "k1", time.Minute)
	require.NoError(t, err)
	assert.True(t, first)
}

func TestMemoryIdempotencyCache_SecondClaimReturnsFalse(t *testing.T) {
	c := NewMemoryIdempotencyCache()
	ctx := context.Background()
	first, _ := c.Claim(ctx, "k1", time.Minute)
	require.True(t, first)
	second, err := c.Claim(ctx, "k1", time.Minute)
	require.NoError(t, err)
	assert.False(t, second)
}

func TestMemoryIdempotencyCache_ExpiredKeyAllowsReclaim(t *testing.T) {
	c := NewMemoryIdempotencyCache()
	ctx := context.Background()
	_, _ = c.Claim(ctx, "k1", time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	again, err := c.Claim(ctx, "k1", time.Minute)
	require.NoError(t, err)
	assert.True(t, again)
}

func TestMemoryIdempotencyCache_DifferentKeysAreIndependent(t *testing.T) {
	c := NewMemoryIdempotencyCache()
	ctx := context.Background()
	a, _ := c.Claim(ctx, "a", time.Minute)
	b, _ := c.Claim(ctx, "b", time.Minute)
	assert.True(t, a)
	assert.True(t, b)
}
