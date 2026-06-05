package configwatcher

import (
	"context"
	"fmt"

	"github.com/opendecree/decree/sdk/configclient"
)

// SetString writes a string value to the server and optimistically updates the
// local cached value for any registered watcher on the same field path.
func (w *Watcher) SetString(ctx context.Context, fieldPath, value string, opts ...WriteOption) error {
	return w.set(ctx, fieldPath, configclient.StringVal(value), opts)
}

// SetBool writes a bool value to the server and optimistically updates the
// local cached value for any registered watcher on the same field path.
func (w *Watcher) SetBool(ctx context.Context, fieldPath string, value bool, opts ...WriteOption) error {
	return w.set(ctx, fieldPath, configclient.BoolVal(value), opts)
}

// SetInt writes an int64 value to the server and optimistically updates the
// local cached value for any registered watcher on the same field path.
func (w *Watcher) SetInt(ctx context.Context, fieldPath string, value int64, opts ...WriteOption) error {
	return w.set(ctx, fieldPath, configclient.IntVal(value), opts)
}

// SetFloat writes a float64 value to the server and optimistically updates the
// local cached value for any registered watcher on the same field path.
func (w *Watcher) SetFloat(ctx context.Context, fieldPath string, value float64, opts ...WriteOption) error {
	return w.set(ctx, fieldPath, configclient.FloatVal(value), opts)
}

// WriteOption configures a write-through operation.
type WriteOption func(*configclient.SetFieldRequest)

// WithDescription sets the change description on the write.
func WithDescription(desc string) WriteOption {
	return func(req *configclient.SetFieldRequest) { req.Description = desc }
}

// WithExpectedChecksum adds an optimistic-concurrency guard to the write.
func WithExpectedChecksum(cs string) WriteOption {
	return func(req *configclient.SetFieldRequest) { req.ExpectedChecksum = &cs }
}

func (w *Watcher) set(ctx context.Context, fieldPath string, tv *configclient.TypedValue, opts []WriteOption) error {
	req := &configclient.SetFieldRequest{
		TenantID:  w.tenantID,
		FieldPath: fieldPath,
		Value:     tv,
	}
	for _, o := range opts {
		o(req)
	}

	_, err := w.transport.SetField(ctx, req)
	if err != nil {
		return fmt.Errorf("configwatcher: set %s: %w", fieldPath, err)
	}

	// Optimistically update the local value if this field is registered.
	w.mu.RLock()
	entry, ok := w.fields[fieldPath]
	w.mu.RUnlock()
	if ok && entry.typedUpdate != nil {
		entry.typedUpdate(tv)
	}
	return nil
}
