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
//	fee, _ := w.Float("payments.fee_rate", 0.01)
//	enabled, _ := w.Bool("payments.enabled", false)
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
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
)

// ErrStarted is returned by Register* methods (String, Int, Bool, etc.) when
// the watcher has already been started. All fields must be registered before
// calling [Watcher.Start]; a field registered after Start will never receive
// stream updates.
var ErrStarted = errors.New("configwatcher: cannot register field after Start")

// ErrClosed is returned by [Watcher.Start] when the watcher has already been
// closed. A closed watcher cannot be restarted; create a new one instead.
var ErrClosed = errors.New("configwatcher: watcher is closed")

// Watcher monitors a tenant's configuration via a subscription stream.
// Register typed field accessors before calling [Watcher.Start].
type Watcher struct {
	transport configclient.Transport
	tenantID  string
	opts      options

	mu      sync.RWMutex
	fields  map[string]*fieldEntry // field path → entry
	closed  bool
	started bool
	cancel  context.CancelFunc
	// cancelReady is closed by Start after w.cancel is assigned, allowing
	// a concurrent Close to safely wait until cancel is available.
	cancelReady chan struct{}
	done        chan struct{}
}

type fieldEntry struct {
	rawUpdate   func(value string, isSet bool)
	typedUpdate func(tv *configclient.TypedValue)
	closeFunc   func()
}

type options struct {
	minBackoff      time.Duration
	maxBackoff      time.Duration
	snapshotTimeout time.Duration
	logger          *slog.Logger
}

// New creates a new watcher for the given tenant's configuration.
// Register typed field accessors (String, Int, Bool, etc.) before calling Start.
//
// The transport is used for both snapshot loading and subscription streaming.
// Use grpctransport.NewConfigTransport to create a gRPC-backed transport.
func New(transport configclient.Transport, tenantID string, opts ...Option) *Watcher {
	o := options{
		minBackoff:      500 * time.Millisecond,
		maxBackoff:      30 * time.Second,
		snapshotTimeout: 10 * time.Second,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	for _, opt := range opts {
		opt(&o)
	}

	return &Watcher{
		transport:   transport,
		tenantID:    tenantID,
		opts:        o,
		fields:      make(map[string]*fieldEntry),
		cancelReady: make(chan struct{}),
		done:        make(chan struct{}),
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

// WithLogger sets a custom logger. By default all log output is discarded.
// Pass an explicit logger to opt into watcher diagnostics.
func WithLogger(logger *slog.Logger) Option {
	return func(o *options) { o.logger = logger }
}

// WithSnapshotTimeout sets the deadline applied to each loadSnapshot call
// inside the reconnect loop. Defaults to 10s.
func WithSnapshotTimeout(d time.Duration) Option {
	return func(o *options) { o.snapshotTimeout = d }
}

// --- Field registration ---

// String registers a string field and returns a live [Value] handle.
// The defaultVal is returned when the field is null or missing.
// Must be called before [Watcher.Start]; returns [ErrStarted] if called after.
func (w *Watcher) String(fieldPath string, defaultVal string) (*Value[string], error) {
	return registerField(w, fieldPath, defaultVal, parseString)
}

// Int registers an integer field and returns a live [Value] handle.
// Values are stored as decimal strings (e.g. "42") and parsed to int64.
// The defaultVal is returned when the field is null, missing, or unparseable.
// Must be called before [Watcher.Start]; returns [ErrStarted] if called after.
func (w *Watcher) Int(fieldPath string, defaultVal int64) (*Value[int64], error) {
	return registerField(w, fieldPath, defaultVal, parseInt)
}

// Float registers a floating-point field and returns a live [Value] handle.
// Values are stored as decimal strings (e.g. "3.14") and parsed to float64.
// The defaultVal is returned when the field is null, missing, or unparseable.
// Must be called before [Watcher.Start]; returns [ErrStarted] if called after.
func (w *Watcher) Float(fieldPath string, defaultVal float64) (*Value[float64], error) {
	return registerField(w, fieldPath, defaultVal, parseFloat)
}

// Bool registers a boolean field and returns a live [Value] handle.
// Values are stored as "true" or "false" strings.
// The defaultVal is returned when the field is null, missing, or unparseable.
// Must be called before [Watcher.Start]; returns [ErrStarted] if called after.
func (w *Watcher) Bool(fieldPath string, defaultVal bool) (*Value[bool], error) {
	return registerField(w, fieldPath, defaultVal, parseBool)
}

// Duration registers a duration field and returns a live [Value] handle.
// Values are stored as Go-style duration strings (e.g. "24h", "500ms").
// The defaultVal is returned when the field is null, missing, or unparseable.
// Must be called before [Watcher.Start]; returns [ErrStarted] if called after.
func (w *Watcher) Duration(fieldPath string, defaultVal time.Duration) (*Value[time.Duration], error) {
	return registerField(w, fieldPath, defaultVal, parseDuration)
}

// Time registers a timestamp field and returns a live [Value] handle.
// Values are stored as RFC3339Nano strings and parsed to time.Time.
// The defaultVal is returned when the field is null, missing, or unparseable.
// Must be called before [Watcher.Start]; returns [ErrStarted] if called after.
func (w *Watcher) Time(fieldPath string, defaultVal time.Time) (*Value[time.Time], error) {
	return registerField(w, fieldPath, defaultVal, parseTime)
}

// Raw registers a string field with no type conversion and returns a live [Value] handle.
// This is equivalent to [Watcher.String] — provided for clarity of intent.
// Must be called before [Watcher.Start]; returns [ErrStarted] if called after.
func (w *Watcher) Raw(fieldPath string, defaultVal string) (*Value[string], error) {
	return registerField(w, fieldPath, defaultVal, parseString)
}

func registerField[T any](w *Watcher, fieldPath string, defaultVal T, parse func(string) (T, error)) (*Value[T], error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started {
		return nil, ErrStarted
	}
	v := newValue(defaultVal, parse)
	v.fieldPath = fieldPath
	v.logger = w.opts.logger
	w.fields[fieldPath] = &fieldEntry{
		rawUpdate:   func(value string, isSet bool) { v.update(value, isSet) },
		typedUpdate: typedUpdater(v),
		closeFunc:   func() { v.close() },
	}
	return v, nil
}

// typedUpdater returns a closure that delivers a *configclient.TypedValue directly
// to v when the Kind matches T, and falls back to string parsing otherwise.
func typedUpdater[T any](v *Value[T]) func(tv *configclient.TypedValue) {
	return func(tv *configclient.TypedValue) {
		switch tv.Kind() {
		case configclient.KindInteger:
			if n, ok := tv.IntValue(); ok {
				if direct, ok := any(n).(T); ok {
					v.updateDirect(direct)
					return
				}
			}
		case configclient.KindNumber:
			if f, ok := tv.FloatValue(); ok {
				if direct, ok := any(f).(T); ok {
					v.updateDirect(direct)
					return
				}
			}
		case configclient.KindBool:
			if b, ok := tv.BoolValue(); ok {
				if direct, ok := any(b).(T); ok {
					v.updateDirect(direct)
					return
				}
			}
		case configclient.KindTime:
			if t, ok := tv.TimeValue(); ok {
				if direct, ok := any(t).(T); ok {
					v.updateDirect(direct)
					return
				}
			}
		case configclient.KindDuration:
			if d, ok := tv.DurationValue(); ok {
				if direct, ok := any(d).(T); ok {
					v.updateDirect(direct)
					return
				}
			}
		case configclient.KindString, configclient.KindURL, configclient.KindJSON:
			s := tv.String()
			if direct, ok := any(s).(T); ok {
				v.updateDirect(direct)
				return
			}
		}
		// Kind mismatch or unrecognized kind: fall back to string parsing.
		v.update(tv.String(), true)
	}
}

// --- Lifecycle ---

// Start loads the initial configuration snapshot and begins the subscription
// stream in a background goroutine. The context controls the watcher's lifetime —
// when cancelled, the subscription is terminated and all [Value.Changes] channels
// are closed.
//
// Start is idempotent: a second call returns nil without starting another loop.
// Start returns [ErrClosed] if the watcher has already been closed.
//
// Fields must be registered (via String, Int, Bool, etc.) before calling Start.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return ErrClosed
	}
	if w.started {
		w.mu.Unlock()
		return nil
	}
	w.started = true
	// Lazily initialize cancelReady for callers that construct Watcher directly
	// (e.g. in tests) without going through New.
	if w.cancelReady == nil {
		w.cancelReady = make(chan struct{})
	}
	w.mu.Unlock()

	version, err := w.loadSnapshot(ctx)
	if err != nil {
		// Roll back so the caller can retry.
		w.mu.Lock()
		w.started = false
		// Close the old cancelReady before replacing it so that any concurrent
		// Close that already captured this channel is unblocked. Close checks
		// w.cancel after the wait; it will be nil here, so it skips cancel().
		old := w.cancelReady
		w.cancelReady = make(chan struct{}) // fresh channel for the next Start attempt
		w.mu.Unlock()
		close(old)
		return err
	}

	loopCtx, cancel := context.WithCancel(ctx)

	w.mu.Lock()
	w.cancel = cancel
	w.mu.Unlock()

	// Signal any concurrent Close that cancel is now assigned.
	close(w.cancelReady)

	go w.subscriptionLoop(loopCtx, version)
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
	started := w.started
	w.mu.Unlock()

	if started {
		// Wait until Start has finished assigning w.cancel. This prevents a
		// race where Close is called while Start is executing but hasn't yet
		// stored the cancel function.
		w.mu.RLock()
		ready := w.cancelReady
		w.mu.RUnlock()

		if ready != nil {
			<-ready
		}

		w.mu.RLock()
		cancel := w.cancel
		w.mu.RUnlock()

		if cancel != nil {
			cancel()
			<-w.done // Wait for subscription goroutine to exit.
		}
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

// loadSnapshot fetches the full config snapshot and returns the snapshot version.
func (w *Watcher) loadSnapshot(ctx context.Context) (int32, error) {
	resp, err := w.transport.GetConfig(ctx, &configclient.GetConfigRequest{
		TenantID: w.tenantID,
	})
	if err != nil {
		return 0, err
	}

	// Build a map of field path → TypedValue.
	values := make(map[string]*configclient.TypedValue, len(resp.Values))
	for _, v := range resp.Values {
		values[v.FieldPath] = v.Value
	}

	w.mu.RLock()
	defer w.mu.RUnlock()
	for path, entry := range w.fields {
		if tv, ok := values[path]; ok && tv != nil {
			entry.typedUpdate(tv)
		} else {
			entry.rawUpdate("", false)
		}
	}
	return resp.Version, nil
}

// subscriptionLoop is the single background goroutine that owns the done channel.
// It is started by [Watcher.Start] and runs until ctx is cancelled. [Watcher.Close]
// reaps it by calling cancel() and then blocking on <-w.done, so the goroutine is
// guaranteed to have exited before Close returns.
//
// On each reconnect the loop reloads a fresh snapshot before resubscribing. This
// snapshot-then-subscribe ordering ensures that changes received during the disconnect
// window are never missed: the snapshot captures the current state and the subscribe
// cursor (snapshotVersion+1) replays any changes the server recorded after that point.
func (w *Watcher) subscriptionLoop(ctx context.Context, snapshotVersion int32) {
	defer close(w.done)

	backoff := w.opts.minBackoff
	fieldPaths := w.registeredPaths()

	for {
		err := w.subscribe(ctx, fieldPaths, snapshotVersion)
		if ctx.Err() != nil {
			return // Context cancelled — clean shutdown.
		}

		if err == nil || errors.Is(err, io.EOF) {
			w.opts.logger.InfoContext(ctx, "subscription stream closed by server, reconnecting",
				"backoff", backoff)
		} else {
			w.opts.logger.WarnContext(ctx, "subscription stream error, reconnecting",
				"error", err, "backoff", backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Exponential backoff.
		backoff = min(backoff*2, w.opts.maxBackoff)

		// Reload snapshot on reconnect; use the new version as the cursor so
		// Subscribe replays any changes the server received while disconnected.
		snapCtx, snapCancel := context.WithTimeout(ctx, w.opts.snapshotTimeout)
		newVersion, snapErr := w.loadSnapshot(snapCtx)
		snapCancel()
		if snapErr != nil {
			w.opts.logger.WarnContext(ctx, "failed to reload snapshot on reconnect", "error", snapErr)
		} else {
			snapshotVersion = newVersion
			backoff = w.opts.minBackoff // Reset backoff on successful snapshot.
		}
	}
}

func (w *Watcher) subscribe(ctx context.Context, fieldPaths []string, snapshotVersion int32) error {
	startVersion := snapshotVersion + 1
	sub, err := w.transport.Subscribe(ctx, &configclient.SubscribeRequest{
		TenantID:     w.tenantID,
		FieldPaths:   fieldPaths,
		StartVersion: &startVersion,
	})
	if err != nil {
		return err
	}

	for {
		change, err := sub.Recv()
		if err != nil {
			return err
		}

		w.mu.RLock()
		if entry, ok := w.fields[change.FieldPath]; ok {
			if change.NewValue != nil {
				entry.typedUpdate(change.NewValue)
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
