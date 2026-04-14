package pubsub

import (
	"context"
	"time"
)

// ConfigChangeEvent represents a change to a config value.
type ConfigChangeEvent struct {
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
