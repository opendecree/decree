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

func TestMemoryCache_Sweep_RemovesExpiredEntries(t *testing.T) {
	c := NewMemoryCache(0)
	defer c.Stop()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Millisecond))
	require.NoError(t, c.Set(ctx, "t2", 1, map[string]string{"a": "2"}, time.Millisecond))
	require.NoError(t, c.Set(ctx, "t3", 1, map[string]string{"a": "3"}, time.Hour))
	assert.Equal(t, 3, c.Len())

	time.Sleep(5 * time.Millisecond)
	c.sweep()

	assert.Equal(t, 1, c.Len(), "only t3 should remain after sweep")

	got, _ := c.Get(ctx, "t3", 1)
	assert.Equal(t, "3", got["a"])
}

func TestMemoryCache_Sweep_NoExpired_NoOp(t *testing.T) {
	c := NewMemoryCache(0)
	defer c.Stop()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Hour))
	require.NoError(t, c.Set(ctx, "t2", 1, map[string]string{"a": "2"}, time.Hour))

	c.sweep()
	assert.Equal(t, 2, c.Len())
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

func TestMemoryCache_WithSweepInterval_SweepsOnSchedule(t *testing.T) {
	interval := 50 * time.Millisecond
	c := NewMemoryCache(0, WithSweepInterval(interval))
	defer c.Stop()
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Millisecond))
	require.NoError(t, c.Set(ctx, "t2", 1, map[string]string{"a": "2"}, time.Hour))

	// Wait long enough for at least one sweep (interval + max jitter = 55ms).
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, 1, c.Len(), "expired entry should be swept by background goroutine")
	got, _ := c.Get(ctx, "t2", 1)
	assert.Equal(t, "2", got["a"], "live entry must remain")
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

// --- Negative cache ---

func TestMemoryCache_NegativeCache_SetAndGet(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	neg, err := c.GetNegative(ctx, "t1", 1)
	require.NoError(t, err)
	assert.False(t, neg, "miss before set")

	require.NoError(t, c.SetNegative(ctx, "t1", 1, time.Minute))

	neg, err = c.GetNegative(ctx, "t1", 1)
	require.NoError(t, err)
	assert.True(t, neg, "hit after set")
}

func TestMemoryCache_NegativeCache_TTLExpiry(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	require.NoError(t, c.SetNegative(ctx, "t1", 1, time.Millisecond))
	time.Sleep(5 * time.Millisecond)

	neg, err := c.GetNegative(ctx, "t1", 1)
	require.NoError(t, err)
	assert.False(t, neg, "expired entry must report miss")
}

func TestMemoryCache_NegativeCache_InvalidateClears(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	require.NoError(t, c.SetNegative(ctx, "t1", 1, time.Minute))
	require.NoError(t, c.SetNegative(ctx, "t1", 2, time.Minute))
	require.NoError(t, c.SetNegative(ctx, "t2", 1, time.Minute))

	require.NoError(t, c.Invalidate(ctx, "t1"))

	neg, _ := c.GetNegative(ctx, "t1", 1)
	assert.False(t, neg, "t1:v1 must be cleared")
	neg, _ = c.GetNegative(ctx, "t1", 2)
	assert.False(t, neg, "t1:v2 must be cleared")

	neg, _ = c.GetNegative(ctx, "t2", 1)
	assert.True(t, neg, "t2 must be unaffected")
}

func TestMemoryCache_NegativeCache_Sweep_RemovesExpired(t *testing.T) {
	c := NewMemoryCache(0)
	defer c.Stop()
	ctx := context.Background()

	require.NoError(t, c.SetNegative(ctx, "t1", 1, time.Millisecond))
	require.NoError(t, c.SetNegative(ctx, "t2", 1, time.Hour))

	time.Sleep(5 * time.Millisecond)
	c.sweep()

	neg, _ := c.GetNegative(ctx, "t1", 1)
	assert.False(t, neg, "expired t1 swept")
	neg, _ = c.GetNegative(ctx, "t2", 1)
	assert.True(t, neg, "live t2 remains")
}

func TestMemoryCache_NegativeCache_IndependentFromPositive(t *testing.T) {
	c := NewMemoryCache(0)
	ctx := context.Background()

	// Set positive entry and negative entry for same key.
	require.NoError(t, c.Set(ctx, "t1", 1, map[string]string{"a": "1"}, time.Minute))
	require.NoError(t, c.SetNegative(ctx, "t1", 2, time.Minute))

	// Positive entry not affected by negative check.
	pos, _ := c.Get(ctx, "t1", 1)
	assert.NotNil(t, pos)

	// Negative entry not affected by positive set.
	neg, _ := c.GetNegative(ctx, "t1", 2)
	assert.True(t, neg)

	// v1 has no negative entry; v2 has no positive entry.
	neg1, _ := c.GetNegative(ctx, "t1", 1)
	assert.False(t, neg1)
	pos2, _ := c.Get(ctx, "t1", 2)
	assert.Nil(t, pos2)
}
