package pubsub

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel/metric"
)

// MemoryPubSub implements both Publisher and Subscriber using in-memory channels.
// Safe for concurrent use.
type MemoryPubSub struct {
	mu          sync.RWMutex
	subscribers map[string][]*memSub // tenantID → subscriptions
	logger      *slog.Logger
	drops       metric.Int64Counter
	dropsOK     bool
	seq         atomic.Int64
}

// memSub pairs a subscriber channel with a sync.Once so the channel is closed
// exactly once regardless of whether cancel() or Close() runs first.
type memSub struct {
	ch        chan ConfigChangeEvent
	closeOnce sync.Once
}

func (s *memSub) closeChannel() { s.closeOnce.Do(func() { close(s.ch) }) }

// MemoryOption configures a MemoryPubSub.
type MemoryOption func(*MemoryPubSub)

// WithLogger sets the logger used for drop warnings.
func WithLogger(l *slog.Logger) MemoryOption {
	return func(ps *MemoryPubSub) { ps.logger = l }
}

// WithDroppedCounter sets the OTel counter incremented on every dropped event.
func WithDroppedCounter(c metric.Int64Counter) MemoryOption {
	return func(ps *MemoryPubSub) {
		ps.drops = c
		ps.dropsOK = true
	}
}

// NewMemoryPubSub creates a new in-memory pub/sub.
func NewMemoryPubSub(opts ...MemoryOption) *MemoryPubSub {
	ps := &MemoryPubSub{
		subscribers: make(map[string][]*memSub),
		logger:      slog.Default(),
	}
	for _, o := range opts {
		o(ps)
	}
	return ps
}

func (ps *MemoryPubSub) Publish(_ context.Context, event ConfigChangeEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if len(data) > MaxPayloadBytes {
		return ErrPayloadTooLarge
	}

	event.EventID = newEventID()
	event.Seq = ps.seq.Add(1)

	ps.mu.RLock()
	defer ps.mu.RUnlock()

	for _, sub := range ps.subscribers[event.TenantID] {
		select {
		case sub.ch <- event:
		default:
			ps.logger.Debug("pubsub: dropped event — subscriber channel full",
				"tenant_id", event.TenantID,
				"change_count", len(event.Changes),
			)
			if ps.dropsOK {
				ps.drops.Add(context.Background(), 1)
			}
		}
	}
	return nil
}

func (ps *MemoryPubSub) Subscribe(_ context.Context, tenantID string) (<-chan ConfigChangeEvent, context.CancelFunc, error) {
	sub := &memSub{ch: make(chan ConfigChangeEvent, 64)}

	ps.mu.Lock()
	ps.subscribers[tenantID] = append(ps.subscribers[tenantID], sub)
	ps.mu.Unlock()

	cancel := func() {
		ps.mu.Lock()
		defer ps.mu.Unlock()

		subs := ps.subscribers[tenantID]
		for i, s := range subs {
			if s == sub {
				ps.subscribers[tenantID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		sub.closeChannel()
	}

	return sub.ch, cancel, nil
}

func (ps *MemoryPubSub) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	for tenantID, subs := range ps.subscribers {
		for _, sub := range subs {
			sub.closeChannel()
		}
		delete(ps.subscribers, tenantID)
	}
	return nil
}
