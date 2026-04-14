package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache_SetAndGet(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1", "b": "2"}, time.Minute))

	got, err := c.Get(ctx, "t1", 1)
	require.NoError(t, err)
	assert.Equal(t, "1", got["a"])
	assert.Equal(t, "2", got["b"])
}

func TestMemoryCache_Miss(t *testing.T) {
	c := NewMemoryCache()
	got, err := c.Get(context.Background(), "t1", 1)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryCache_TTLExpiry(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Millisecond))
	time.Sleep(5 * time.Millisecond)

	got, err := c.Get(ctx, "t1", 1)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryCache_Invalidate(t *testing.T) {
	c := NewMemoryCache()
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
	c := NewMemoryCache()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Minute))

	got, _ := c.Get(ctx, "t1", 1)
	got["a"] = "mutated"

	got2, _ := c.Get(ctx, "t1", 1)
	assert.Equal(t, "1", got2["a"], "cache should not be affected by external mutation")
}

func TestMemoryCache_DifferentVersions(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "v1"}, time.Minute))
	require.NoError(t, c.Set(ctx, "t1", 2, map[string]string{"a": "v2"}, time.Minute))

	got1, _ := c.Get(ctx, "t1", 1)
	got2, _ := c.Get(ctx, "t1", 2)
	assert.Equal(t, "v1", got1["a"])
	assert.Equal(t, "v2", got2["a"])
}
