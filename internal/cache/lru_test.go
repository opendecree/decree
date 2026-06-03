package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLRU_GetSet(t *testing.T) {
	l := NewLRU[string, int](10)
	l.Set("a", 1)
	l.Set("b", 2)

	v, ok := l.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v)

	v, ok = l.Get("b")
	assert.True(t, ok)
	assert.Equal(t, 2, v)

	_, ok = l.Get("missing")
	assert.False(t, ok)
}

func TestLRU_Eviction(t *testing.T) {
	l := NewLRU[string, int](3)
	l.Set("a", 1)
	l.Set("b", 2)
	l.Set("c", 3)
	assert.Equal(t, 3, l.Len())

	// Adding a 4th entry should evict the LRU (a).
	l.Set("d", 4)
	assert.Equal(t, 3, l.Len())

	_, ok := l.Get("a")
	assert.False(t, ok, "a should be evicted")

	v, ok := l.Get("d")
	assert.True(t, ok)
	assert.Equal(t, 4, v)
}

func TestLRU_Delete(t *testing.T) {
	l := NewLRU[string, int](10)
	l.Set("a", 1)
	l.Delete("a")

	_, ok := l.Get("a")
	assert.False(t, ok)
	assert.Equal(t, 0, l.Len())

	// Delete on missing key is a no-op.
	l.Delete("nonexistent")
}

func TestLRU_Promotion(t *testing.T) {
	l := NewLRU[string, int](3)
	l.Set("a", 1)
	l.Set("b", 2)
	l.Set("c", 3)

	// Access "a" to promote it to MRU; "b" becomes oldest.
	_, _ = l.Get("a")

	// Adding a new entry should evict "b" (now oldest), not "a".
	l.Set("d", 4)

	_, ok := l.Get("b")
	assert.False(t, ok, "b should be evicted (oldest after a was promoted)")

	_, ok = l.Get("a")
	assert.True(t, ok, "a should survive (promoted to MRU)")
}

func TestLRU_Peek_NoPromotion(t *testing.T) {
	l := NewLRU[string, int](3)
	l.Set("a", 1)
	l.Set("b", 2)
	l.Set("c", 3)

	// Peek "a" — should not promote it.
	v, ok := l.Peek("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v)

	// Adding a new entry should evict "a" (still oldest).
	l.Set("d", 4)

	_, ok = l.Get("a")
	assert.False(t, ok, "a should be evicted (Peek did not promote it)")
}

func TestLRU_Update_NoEviction(t *testing.T) {
	l := NewLRU[string, int](2)
	l.Set("a", 1)
	l.Set("b", 2)

	// Update existing key — must not grow or evict.
	l.Set("a", 99)
	assert.Equal(t, 2, l.Len())

	v, ok := l.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 99, v)
}

func TestLRU_Range_OldestFirst(t *testing.T) {
	l := NewLRU[string, int](10)
	l.Set("a", 1)
	l.Set("b", 2)
	l.Set("c", 3)

	var keys []string
	l.Range(func(k string, _ int) {
		keys = append(keys, k)
	})
	assert.Equal(t, []string{"a", "b", "c"}, keys)
}

func TestLRU_Range_SafeDelete(t *testing.T) {
	l := NewLRU[string, int](10)
	l.Set("a", 1)
	l.Set("b", 2)
	l.Set("c", 3)

	// Delete inside Range must not panic or skip entries.
	l.Range(func(k string, _ int) {
		l.Delete(k)
	})
	assert.Equal(t, 0, l.Len())
}

func TestLRU_Len(t *testing.T) {
	l := NewLRU[string, int](10)
	assert.Equal(t, 0, l.Len())
	l.Set("a", 1)
	assert.Equal(t, 1, l.Len())
	l.Set("b", 2)
	assert.Equal(t, 2, l.Len())
	l.Delete("a")
	assert.Equal(t, 1, l.Len())
}
