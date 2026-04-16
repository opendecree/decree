// Package configwatcher provides a high-level Go SDK for live, typed
// configuration values with automatic subscription and reconnect.
//
// Values are always fresh — the watcher loads an initial snapshot, then
// subscribes to a stream for live updates. Typed accessors return
// native Go types (string, int64, float64, bool, time.Duration) with
// null/missing support.
//
// Example:
//
//	transport := grpctransport.NewConfigTransport(conn, grpctransport.WithSubject("myapp"))
//	w := configwatcher.New(transport, "tenant-uuid")
//
//	fee := w.Float("payments.fee_rate", 0.01)
//	enabled := w.Bool("payments.enabled", false)
//
//	w.Start(ctx)
//	defer w.Close()
//
//	fmt.Println(fee.Get())       // 0.025
//	fmt.Println(enabled.Get())   // true
//
//	for change := range fee.Changes() {
//	    log.Printf("fee changed: %v → %v", change.Old, change.New)
//	}
package configwatcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
)

// Watcher monitors a tenant's configuration via a subscription stream.
// Register typed field accessors before calling [Watcher.Start].
type Watcher struct {
	transport configclient.Transport
	tenantID  string
	opts      options

	mu     sync.RWMutex
	fields map[string]*fieldEntry // field path → entry
	closed bool
	cancel context.CancelFunc
	done   chan struct{}
}

type fieldEntry struct {
	rawUpdate func(value string, isSet bool)
	closeFunc func()
}

type options struct {
	minBackoff time.Duration
	maxBackoff time.Duration
	logger     *slog.Logger
}

// New creates a new watcher for the given tenant's configuration.
// Register typed field accessors (String, Int, Bool, etc.) before calling Start.
//
// The transport is used for both snapshot loading and subscription streaming.
// Use grpctransport.NewConfigTransport to create a gRPC-backed transport.
func New(transport configclient.Transport, tenantID string, opts ...Option) *Watcher {
	o := options{
		minBackoff: 500 * time.Millisecond,
		maxBackoff: 30 * time.Second,
		logger:     slog.Default(),
	}
	for _, opt := range opts {
		opt(&o)
	}

	return &Watcher{
		transport: transport,
		tenantID:  tenantID,
		opts:      o,
		fields:    make(map[string]*fieldEntry),
		done:      make(chan struct{}),
	}
}

// Option configures the watcher.
type Option func(*options)

// WithReconnectBackoff configures the exponential backoff for stream reconnection.
// Defaults to 500ms min, 30s max.
func WithReconnectBackoff(min, max time.Duration) Option {
	return func(o *options) {
		o.minBackoff = min
		o.maxBackoff = max
	}
}

// WithLogger sets a custom logger. Defaults to slog.Default().
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) { o.logger = logger }
}

// --- Field registration ---

// String registers a string field and returns a live [Value] handle.
// The defaultVal is returned when the field is null or missing.
// Must be called before [Watcher.Start].
func (w *Watcher) String(fieldPath string, defaultVal string) *Value[string] {
	return registerField(w, fieldPath, defaultVal, parseString)
}

// Int registers an integer field and returns a live [Value] handle.
// Values are stored as decimal strings (e.g. "42") and parsed to int64.
// The defaultVal is returned when the field is null, missing, or unparseable.
// Must be called before [Watcher.Start].
func (w *Watcher) Int(fieldPath string, defaultVal int64) *Value[int64] {
	return registerField(w, fieldPath, defaultVal, parseInt)
}

// Float registers a floating-point field and returns a live [Value] handle.
// Values are stored as decimal strings (e.g. "3.14") and parsed to float64.
// The defaultVal is returned when the field is null, missing, or unparseable.
// Must be called before [Watcher.Start].
func (w *Watcher) Float(fieldPath string, defaultVal float64) *Value[float64] {
	return registerField(w, fieldPath, defaultVal, parseFloat)
}

// Bool registers a boolean field and returns a live [Value] handle.
// Values are stored as "true" or "false" strings.
// The defaultVal is returned when the field is null, missing, or unparseable.
// Must be called before [Watcher.Start].
func (w *Watcher) Bool(fieldPath string, defaultVal bool) *Value[bool] {
	return registerField(w, fieldPath, defaultVal, parseBool)
}

// Duration registers a duration field and returns a live [Value] handle.
// Values are stored as Go-style duration strings (e.g. "24h", "500ms").
// The defaultVal is returned when the field is null, missing, or unparseable.
// Must be called before [Watcher.Start].
func (w *Watcher) Duration(fieldPath string, defaultVal time.Duration) *Value[time.Duration] {
	return registerField(w, fieldPath, defaultVal, parseDuration)
}

// Time registers a timestamp field and returns a live [Value] handle.
// Values are stored as RFC3339Nano strings and parsed to time.Time.
// The defaultVal is returned when the field is null, missing, or unparseable.
// Must be called before [Watcher.Start].
func (w *Watcher) Time(fieldPath string, defaultVal time.Time) *Value[time.Time] {
	return registerField(w, fieldPath, defaultVal, parseTime)
}

// Raw registers a string field with no type conversion and returns a live [Value] handle.
// This is equivalent to [Watcher.String] — provided for clarity of intent.
// Must be called before [Watcher.Start].
func (w *Watcher) Raw(fieldPath string, defaultVal string) *Value[string] {
	return registerField(w, fieldPath, defaultVal, parseString)
}

func registerField[T any](w *Watcher, fieldPath string, defaultVal T, parse func(string) (T, error)) *Value[T] {
	v := newValue(defaultVal, parse)
	w.mu.Lock()
	defer w.mu.Unlock()
	w.fields[fieldPath] = &fieldEntry{
		rawUpdate: func(value string, isSet bool) { v.update(value, isSet) },
		closeFunc: func() { v.close() },
	}
	return v
}

// --- Lifecycle ---

// Start loads the initial configuration snapshot and begins the subscription
// stream in a background goroutine. The context controls the watcher's lifetime —
// when cancelled, the subscription is terminated and all [Value.Changes] channels
// are closed.
//
// Fields must be registered (via String, Int, Bool, etc.) before calling Start.
func (w *Watcher) Start(ctx context.Context) error {
	if err := w.loadSnapshot(ctx); err != nil {
		return err
	}

	ctx, w.cancel = context.WithCancel(ctx)
	go w.subscriptionLoop(ctx)
	return nil
}

// Close stops the subscription stream and closes all [Value.Changes] channels.
// It is safe to call Close multiple times.
func (w *Watcher) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
		<-w.done // Wait for subscription goroutine to exit.
	}

	// Close all value channels.
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, f := range w.fields {
		f.closeFunc()
	}
	return nil
}

// --- Internal ---

func (w *Watcher) loadSnapshot(ctx context.Context) error {
	resp, err := w.transport.GetConfig(ctx, &configclient.GetConfigRequest{
		TenantID: w.tenantID,
	})
	if err != nil {
		return err
	}

	// Build a map of field path → string value.
	values := make(map[string]string, len(resp.Values))
	for _, v := range resp.Values {
		values[v.FieldPath] = v.Value.String()
	}

	w.mu.RLock()
	defer w.mu.RUnlock()
	for path, entry := range w.fields {
		if val, ok := values[path]; ok {
			entry.rawUpdate(val, true)
		} else {
			entry.rawUpdate("", false)
		}
	}
	return nil
}

func (w *Watcher) subscriptionLoop(ctx context.Context) {
	defer close(w.done)

	backoff := w.opts.minBackoff
	fieldPaths := w.registeredPaths()

	for {
		err := w.subscribe(ctx, fieldPaths)
		if ctx.Err() != nil {
			return // Context cancelled — clean shutdown.
		}

		w.opts.logger.WarnContext(ctx, "subscription stream ended, reconnecting",
			"error", err, "backoff", backoff)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Exponential backoff.
		backoff = min(backoff*2, w.opts.maxBackoff)

		// Re-load snapshot on reconnect to catch changes missed during disconnect.
		if err := w.loadSnapshot(ctx); err != nil {
			w.opts.logger.WarnContext(ctx, "failed to reload snapshot on reconnect", "error", err)
		} else {
			backoff = w.opts.minBackoff // Reset backoff on successful snapshot.
		}
	}
}

func (w *Watcher) subscribe(ctx context.Context, fieldPaths []string) error {
	sub, err := w.transport.Subscribe(ctx, &configclient.SubscribeRequest{
		TenantID:   w.tenantID,
		FieldPaths: fieldPaths,
	})
	if err != nil {
		return err
	}

	for {
		change, err := sub.Recv()
		if err != nil {
			return err
		}
		if change == nil {
			continue
		}

		w.mu.RLock()
		if entry, ok := w.fields[change.FieldPath]; ok {
			if change.NewValue != nil {
				entry.rawUpdate(change.NewValue.String(), true)
			} else {
				entry.rawUpdate("", false)
			}
		}
		w.mu.RUnlock()
	}
}

func (w *Watcher) registeredPaths() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	paths := make([]string, 0, len(w.fields))
	for p := range w.fields {
		paths = append(paths, p)
	}
	return paths
}
