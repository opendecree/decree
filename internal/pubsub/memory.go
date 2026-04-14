package pubsub

import (
	"context"
	"sync"
)

// MemoryPubSub implements both Publisher and Subscriber using in-memory channels.
// Safe for concurrent use.
type MemoryPubSub struct {
	mu          sync.RWMutex
	subscribers map[string][]chan ConfigChangeEvent // tenantID → channels
}

// NewMemoryPubSub creates a new in-memory pub/sub.
func NewMemoryPubSub() *MemoryPubSub {
	return &MemoryPubSub{
		subscribers: make(map[string][]chan ConfigChangeEvent),
	}
}

func (ps *MemoryPubSub) Publish(_ context.Context, event ConfigChangeEvent) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	for _, ch := range ps.subscribers[event.TenantID] {
		select {
		case ch <- event:
		default:
			// Drop if subscriber is slow.
		}
	}
	return nil
}

func (ps *MemoryPubSub) Subscribe(_ context.Context, tenantID string) (<-chan ConfigChangeEvent, context.CancelFunc, error) {
	ch := make(chan ConfigChangeEvent, 64)

	ps.mu.Lock()
	ps.subscribers[tenantID] = append(ps.subscribers[tenantID], ch)
	ps.mu.Unlock()

	cancel := func() {
		ps.mu.Lock()
		defer ps.mu.Unlock()

		subs := ps.subscribers[tenantID]
		for i, s := range subs {
			if s == ch {
				ps.subscribers[tenantID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		close(ch)
	}

	return ch, cancel, nil
}

func (ps *MemoryPubSub) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for tenantID, subs := range ps.subscribers {
		for _, ch := range subs {
			close(ch)
		}
		delete(ps.subscribers, tenantID)
	}
	return nil
}
