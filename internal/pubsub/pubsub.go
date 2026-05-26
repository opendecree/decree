// Package pubsub provides config-change event delivery.
//
// # Delivery semantics
//
// Both the Redis and in-memory backends provide at-most-once delivery.
// Events are fire-and-forget: a network disconnect during Redis PUBLISH, or a
// full subscriber channel in the memory backend, silently drops the event with
// no retry or replay.  Subscribers must tolerate gaps and duplicate detection
// is the caller's responsibility.
//
// # Event identity
//
// Every event carries an EventID (UUID v4) set by the publisher.  The EventID
// is stable across re-delivery attempts so consumers can deduplicate.  Seq is
// a monotonically increasing counter scoped to the publisher instance; a
// subscriber can detect a gap by comparing consecutive Seq values, but note
// that the sequence resets when the publisher restarts.
//
// # Payload size
//
// Publishers reject events whose serialised size exceeds MaxPayloadBytes.
// Callers storing large values should keep OldValue/NewValue short and fetch
// the full value by (TenantID, FieldPath, Version) from the config store.
package pubsub

import (
	"context"
	"errors"
	"time"
)

// MaxPayloadBytes is the maximum serialised size of a ConfigChangeEvent that
// publishers will accept. Events larger than this limit are rejected with
// ErrPayloadTooLarge instead of being delivered.
const MaxPayloadBytes = 65536

// ErrPayloadTooLarge is returned by Publish when the serialised event exceeds MaxPayloadBytes.
var ErrPayloadTooLarge = errors.New("pubsub: event payload exceeds maximum size")

// ConfigChangeEvent represents a change to a config value.
type ConfigChangeEvent struct {
	EventID   string    `json:"event_id"` // UUID v4, unique per event
	Seq       int64     `json:"seq"`      // monotonic per-publisher sequence
	TenantID  string    `json:"tenant_id"`
	Version   int32     `json:"version"`
	FieldPath string    `json:"field_path"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	ChangedBy string    `json:"changed_by"`
	ChangedAt time.Time `json:"changed_at"`
}

// Publisher publishes config change events.
type Publisher interface {
	Publish(ctx context.Context, event ConfigChangeEvent) error
	Close() error
}

// Subscriber subscribes to config change events.
type Subscriber interface {
	// Subscribe returns a channel of events for the given tenant.
	// Close the returned cancel function to unsubscribe.
	Subscribe(ctx context.Context, tenantID string) (<-chan ConfigChangeEvent, context.CancelFunc, error)
	Close() error
}
