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
	subscribers map[string][]chan ConfigChangeEvent // tenantID → channels
	logger      *slog.Logger
	drops       metric.Int64Counter
	dropsOK     bool
	seq         atomic.Int64
}

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
		subscribers: make(map[string][]chan ConfigChangeEvent),
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

	for _, ch := range ps.subscribers[event.TenantID] {
		select {
		case ch <- event:
		default:
			ps.logger.Debug("pubsub: dropped event — subscriber channel full",
				"tenant_id", event.TenantID,
				"field_path", event.FieldPath,
			)
			if ps.dropsOK {
				ps.drops.Add(context.Background(), 1)
			}
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
