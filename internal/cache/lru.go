package cache

import "container/list"

// LRU is a bounded cache with LRU eviction. NOT thread-safe; callers must synchronize.
// front of order = LRU (oldest), back = MRU (newest).
type LRU[K comparable, V any] struct {
	cap   int
	items map[K]*list.Element
	order *list.List
}

type lruEntry[K comparable, V any] struct {
	key K
	val V
}

// NewLRU creates a new LRU cache with the given capacity.
func NewLRU[K comparable, V any](cap int) *LRU[K, V] {
	return &LRU[K, V]{
		cap:   cap,
		items: make(map[K]*list.Element, cap),
		order: list.New(),
	}
}

// Get returns the value for k and promotes the entry to MRU position.
// Returns the zero value and false if not found.
func (l *LRU[K, V]) Get(k K) (V, bool) {
	elem, ok := l.items[k]
	if !ok {
		var zero V
		return zero, false
	}
	l.order.MoveToBack(elem)
	return elem.Value.(*lruEntry[K, V]).val, true
}

// Peek returns the value for k without changing the LRU order.
// Returns the zero value and false if not found.
func (l *LRU[K, V]) Peek(k K) (V, bool) {
	elem, ok := l.items[k]
	if !ok {
		var zero V
		return zero, false
	}
	return elem.Value.(*lruEntry[K, V]).val, true
}

// Set adds or updates a value. On update, promotes the entry to MRU.
// If at capacity and the key is new, the LRU (oldest) entry is evicted.
func (l *LRU[K, V]) Set(k K, v V) {
	if elem, ok := l.items[k]; ok {
		// Update existing entry and promote to MRU.
		elem.Value.(*lruEntry[K, V]).val = v
		l.order.MoveToBack(elem)
		return
	}
	// New entry: evict LRU if at capacity.
	if l.order.Len() >= l.cap {
		front := l.order.Front()
		if front != nil {
			evicted := front.Value.(*lruEntry[K, V])
			delete(l.items, evicted.key)
			l.order.Remove(front)
		}
	}
	entry := &lruEntry[K, V]{key: k, val: v}
	elem := l.order.PushBack(entry)
	l.items[k] = elem
}

// Delete removes the entry for k. No-op if not found.
func (l *LRU[K, V]) Delete(k K) {
	elem, ok := l.items[k]
	if !ok {
		return
	}
	delete(l.items, k)
	l.order.Remove(elem)
}

// Len returns the number of entries in the cache.
func (l *LRU[K, V]) Len() int {
	return len(l.items)
}

// Range calls fn for every entry in LRU order (oldest first).
// Entries are snapshotted before iteration, so it is safe to call Delete inside fn.
func (l *LRU[K, V]) Range(fn func(k K, v V)) {
	type kv struct {
		k K
		v V
	}
	// Collect into a slice first so Delete-during-iteration is safe.
	entries := make([]kv, 0, len(l.items))
	for e := l.order.Front(); e != nil; e = e.Next() {
		en := e.Value.(*lruEntry[K, V])
		entries = append(entries, kv{en.key, en.val})
	}
	for _, en := range entries {
		fn(en.k, en.v)
	}
}

// DeleteWhere removes all entries for which pred returns true.
// It iterates the internal map (no ordering guarantee) and collects keys to
// delete only when pred fires, avoiding a full snapshot allocation.
func (l *LRU[K, V]) DeleteWhere(pred func(k K, v V) bool) {
	var toDelete []K
	for k, elem := range l.items {
		if pred(k, elem.Value.(*lruEntry[K, V]).val) {
			toDelete = append(toDelete, k)
		}
	}
	for _, k := range toDelete {
		l.Delete(k)
	}
}
