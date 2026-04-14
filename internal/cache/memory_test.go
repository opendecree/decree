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

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1", "b": "2"}, time.Minute))

	got, err := c.Get(ctx, "t1", 1)
	require.NoError(t, err)
	assert.Equal(t, "1", got["a"])
	assert.Equal(t, "2", got["b"])
}

func TestMemoryCache_Miss(t *testing.T) {
	c := NewMemoryCache(0)
	got, err := c.Get(context.Background(), "t1", 1)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryCache_TTLExpiry(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Millisecond))
	time.Sleep(5 * time.Millisecond)

	got, err := c.Get(ctx, "t1", 1)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryCache_Invalidate(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t1", 2, map[string]string{"b": "2"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t2", 1, map[string]string{"c": "3"}, time.Minute))

	require.NoError(t, c.Invalidate(ctx, "t1"))

	got, _ := c.Get(ctx, "t1", 1)
	assert.Nil(t, got)
	got, _ = c.Get(ctx, "t1", 2)
	assert.Nil(t, got)

	// t2 should be unaffected.
	got, _ = c.Get(ctx, "t2", 1)
	assert.Equal(t, "3", got["c"])
}

func TestMemoryCache_ReturnsCopy(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Minute))

	got, _ := c.Get(ctx, "t1", 1)
	got["a"] = "mutated"

	got2, _ := c.Get(ctx, "t1", 1)
	assert.Equal(t, "1", got2["a"], "cache should not be affected by external mutation")
}

func TestMemoryCache_DifferentVersions(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "v1"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t1", 2, map[string]string{"a": "v2"}, time.Minute))

	got1, _ := c.Get(ctx, "t1", 1)
	got2, _ := c.Get(ctx, "t1", 2)
	assert.Equal(t, "v1", got1["a"])
	assert.Equal(t, "v2", got2["a"])
}

func TestMemoryCache_EvictsOldestWhenFull(t *testing.T) {
	c := NewMemoryCache(3)
	defer c.Stop()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t2", 1, map[string]string{"a": "2"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t3", 1, map[string]string{"a": "3"}, time.Minute))
	assert.Equal(t, 3, c.Len())

	// Adding a 4th should evict the oldest (t1).
	require.NoError(t, c.Set(ctx, "t4", 1, map[string]string{"a": "4"}, time.Minute))
	assert.Equal(t, 3, c.Len())

	got, _ := c.Get(ctx, "t1", 1)
	assert.Nil(t, got, "oldest entry should be evicted")

	got, _ = c.Get(ctx, "t4", 1)
	assert.Equal(t, "4", got["a"], "newest entry should exist")
}

func TestMemoryCache_EvictsExpiredBeforeOldest(t *testing.T) {
	c := NewMemoryCache(3)
	defer c.Stop()
	ctx := context.Background()

	// t1 expires immediately, t2 and t3 are long-lived.
	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Millisecond))
	require.NoError(t, c.Set(ctx, "t2", 1, map[string]string{"a": "2"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t3", 1, map[string]string{"a": "3"}, time.Minute))
	time.Sleep(5 * time.Millisecond) // let t1 expire

	// Adding t4 should evict expired t1, not oldest live t2.
	require.NoError(t, c.Set(ctx, "t4", 1, map[string]string{"a": "4"}, time.Minute))
	assert.Equal(t, 3, c.Len())

	got, _ := c.Get(ctx, "t2", 1)
	assert.Equal(t, "2", got["a"], "t2 should survive — expired t1 evicted first")
}

func TestMemoryCache_UpdateExistingDoesNotGrow(t *testing.T) {
	c := NewMemoryCache(2)
	defer c.Stop()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t2", 1, map[string]string{"a": "2"}, time.Minute))
	assert.Equal(t, 2, c.Len())

	// Updating t1 should not trigger eviction.
	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "updated"}, time.Minute))
	assert.Equal(t, 2, c.Len())

	got, _ := c.Get(ctx, "t1", 1)
	assert.Equal(t, "updated", got["a"])
	got, _ = c.Get(ctx, "t2", 1)
	assert.Equal(t, "2", got["a"])
}
