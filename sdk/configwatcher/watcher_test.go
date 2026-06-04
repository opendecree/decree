package configwatcher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
)

// --- Mock transport for watcher integration tests ---

type mockTransport struct {
	getConfigFn func(ctx context.Context, req *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error)
	subscribeFn func(ctx context.Context, req *configclient.SubscribeRequest) (configclient.Subscription, error)
}

func (m *mockTransport) GetField(context.Context, *configclient.GetFieldRequest) (*configclient.GetFieldResponse, error) {
	panic("not implemented")
}

func (m *mockTransport) GetConfig(ctx context.Context, req *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
	return m.getConfigFn(ctx, req)
}

func (m *mockTransport) GetFields(context.Context, *configclient.GetFieldsRequest) (*configclient.GetFieldsResponse, error) {
	panic("not implemented")
}

func (m *mockTransport) SetField(context.Context, *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error) {
	panic("not implemented")
}

func (m *mockTransport) SetFields(context.Context, *configclient.SetFieldsRequest) (*configclient.SetFieldsResponse, error) {
	panic("not implemented")
}

func (m *mockTransport) Subscribe(ctx context.Context, req *configclient.SubscribeRequest) (configclient.Subscription, error) {
	return m.subscribeFn(ctx, req)
}

// mockSubscription simulates a subscription stream.
type mockSubscription struct {
	ch  chan *configclient.ConfigChange
	ctx context.Context
}

func newMockSubscription(ctx context.Context) *mockSubscription {
	return &mockSubscription{ch: make(chan *configclient.ConfigChange, 16), ctx: ctx}
}

func (s *mockSubscription) Recv() (*configclient.ConfigChange, error) {
	select {
	case <-s.ctx.Done():
		return nil, io.EOF
	case msg, ok := <-s.ch:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil
	}
}

func (s *mockSubscription) send(change *configclient.ConfigChange) {
	s.ch <- change
}

// --- Value unit tests ---

func TestValue_Get_Default(t *testing.T) {
	v := newValue(42, parseInt)
	if got := v.Get(); got != int64(42) {
		t.Errorf("got %v, want %v", got, int64(42))
	}

	val, ok := v.GetWithNull()
	if got := val; got != int64(42) {
		t.Errorf("got %v, want %v", got, int64(42))
	}
	if ok {
		t.Error("expected false for null flag on default value")
	}
}

func TestValue_Update_Set(t *testing.T) {
	v := newValue(0.0, parseFloat)
	v.update("3.14", true)

	if got := v.Get(); got != 3.14 {
		t.Errorf("got %v, want %v", got, 3.14)
	}
	val, ok := v.GetWithNull()
	if got := val; got != 3.14 {
		t.Errorf("got %v, want %v", got, 3.14)
	}
	if !ok {
		t.Error("expected true for non-null value")
	}
}

func TestValue_Update_Null(t *testing.T) {
	v := newValue("default", parseString)
	v.update("hello", true)
	if got := v.Get(); got != "hello" {
		t.Errorf("got %v, want %v", got, "hello")
	}

	v.update("", false) // null
	if got := v.Get(); got != "default" {
		t.Errorf("got %v, want %v", got, "default")
	}
	_, ok := v.GetWithNull()
	if ok {
		t.Error("expected false for null value")
	}
}

func TestValue_Update_ParseError(t *testing.T) {
	v := newValue(int64(99), parseInt)
	v.update("not-a-number", true)

	// Falls back to default on parse error.
	if got := v.Get(); got != int64(99) {
		t.Errorf("got %v, want %v", got, int64(99))
	}
	_, ok := v.GetWithNull()
	if ok {
		t.Error("expected false after parse error fallback")
	}
}

func TestValue_Changes_Channel(t *testing.T) {
	v := newValue(false, parseBool)
	v.update("true", true)

	select {
	case ch := <-v.Changes():
		if !ch.WasNull {
			t.Error("expected WasNull to be true")
		}
		if ch.IsNull {
			t.Error("expected IsNull to be false")
		}
		if ch.Old {
			t.Error("expected Old to be false")
		}
		if !ch.New {
			t.Error("expected New to be true")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected change on channel")
	}
}

func TestValue_Duration(t *testing.T) {
	v := newValue(time.Second, parseDuration)
	v.update("24h", true)
	if got := v.Get(); got != 24*time.Hour {
		t.Errorf("got %v, want %v", got, 24*time.Hour)
	}
}

// --- Watcher integration tests ---

func TestWatcher_SnapshotAndStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sub := newMockSubscription(ctx)

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{
				TenantID: "t1",
				Version:  1,
				Values: []configclient.ConfigValue{
					{FieldPath: "payments.fee", Value: configclient.FloatVal(0.025)},
					{FieldPath: "payments.enabled", Value: configclient.BoolVal(true)},
				},
			}, nil
		},
		subscribeFn: func(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return sub, nil
		},
	}

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts:      options{minBackoff: 10 * time.Millisecond, maxBackoff: 50 * time.Millisecond},
		fields:    make(map[string]*fieldEntry),
		done:      make(chan struct{}),
	}

	fee, err := w.Float("payments.fee", 0.01)
	if err != nil {
		t.Fatalf("Float: %v", err)
	}
	enabled, err := w.Bool("payments.enabled", false)
	if err != nil {
		t.Fatalf("Bool: %v", err)
	}

	err = w.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify initial snapshot values.
	if got := fee.Get(); got != 0.025 {
		t.Errorf("got %v, want %v", got, 0.025)
	}
	if !enabled.Get() {
		t.Error("expected enabled to be true after snapshot")
	}

	// loadSnapshot emits a Change event per field (default → snapshot value) via
	// notifyLocked. Drain fee's buffered event so the gate below observes the
	// stream change, not the buffered snapshot event. Other tests in this file
	// use the same idiom (see ~L993, ~L1022, ~L1045).
	select {
	case <-fee.Changes():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected initial snapshot change event on fee.Changes()")
	}

	// Simulate a stream change.
	sub.send(&configclient.ConfigChange{
		TenantID:  "t1",
		FieldPath: "payments.fee",
		OldValue:  configclient.FloatVal(0.025),
		NewValue:  configclient.FloatVal(0.05),
	})

	// Gate on the stream Changes() event — the value is applied before the event is sent.
	select {
	case <-fee.Changes():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected stream change event on fee.Changes()")
	}

	if got := fee.Get(); got != 0.05 {
		t.Errorf("got %v, want %v", got, 0.05)
	}

	cancel()
	_ = w.Close()
}

func TestWatcher_SnapshotError(t *testing.T) {
	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return nil, fmt.Errorf("connection refused")
		},
		subscribeFn: func(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			panic("should not be called")
		},
	}

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts:      options{minBackoff: 10 * time.Millisecond, maxBackoff: 50 * time.Millisecond},
		fields:    make(map[string]*fieldEntry),
		done:      make(chan struct{}),
	}

	_, _ = w.String("app.name", "default")

	err := w.Start(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestWatcher_RegisterAfterStart verifies that calling a Register* method after
// Start returns ErrStarted and that a field registered before Start is still live.
func TestWatcher_RegisterAfterStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := newMockSubscription(ctx)
	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{TenantID: "t1", Version: 1}, nil
		},
		subscribeFn: func(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return sub, nil
		},
	}

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts:      options{minBackoff: 10 * time.Millisecond, maxBackoff: 50 * time.Millisecond},
		fields:    make(map[string]*fieldEntry),
		done:      make(chan struct{}),
	}

	// Register a field before Start — must succeed.
	name, err := w.String("app.name", "default")
	if err != nil {
		t.Fatalf("String before Start: %v", err)
	}

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Registering after Start must return ErrStarted.
	_, errAfter := w.String("app.other", "x")
	if errAfter == nil {
		t.Fatal("expected ErrStarted, got nil")
	}
	if errAfter != ErrStarted {
		t.Errorf("got %v, want ErrStarted", errAfter)
	}

	// Verify all typed methods return ErrStarted after Start.
	if _, e := w.Int("x", 0); e != ErrStarted {
		t.Errorf("Int: got %v, want ErrStarted", e)
	}
	if _, e := w.Float("x", 0); e != ErrStarted {
		t.Errorf("Float: got %v, want ErrStarted", e)
	}
	if _, e := w.Bool("x", false); e != ErrStarted {
		t.Errorf("Bool: got %v, want ErrStarted", e)
	}
	if _, e := w.Duration("x", 0); e != ErrStarted {
		t.Errorf("Duration: got %v, want ErrStarted", e)
	}
	if _, e := w.Time("x", time.Time{}); e != ErrStarted {
		t.Errorf("Time: got %v, want ErrStarted", e)
	}
	if _, e := w.Raw("x", ""); e != ErrStarted {
		t.Errorf("Raw: got %v, want ErrStarted", e)
	}

	// The pre-registered field must still be accessible.
	if got := name.Get(); got != "default" {
		t.Errorf("Get: got %q, want %q", got, "default")
	}

	cancel()
	_ = w.Close()
}

func TestWatcher_NullField(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sub := newMockSubscription(ctx)

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{
				TenantID: "t1",
				Version:  1,
				Values:   []configclient.ConfigValue{}, // no values — all fields are "missing"
			}, nil
		},
		subscribeFn: func(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return sub, nil
		},
	}

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts:      options{minBackoff: 10 * time.Millisecond, maxBackoff: 50 * time.Millisecond},
		fields:    make(map[string]*fieldEntry),
		done:      make(chan struct{}),
	}

	name, err := w.String("app.name", "fallback")
	if err != nil {
		t.Fatalf("String: %v", err)
	}

	err = w.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Field is missing from snapshot, should return default.
	if got := name.Get(); got != "fallback" {
		t.Errorf("got %v, want %v", got, "fallback")
	}
	_, ok := name.GetWithNull()
	if ok {
		t.Error("expected false for missing field")
	}

	// Now simulate the field being set via stream.
	sub.send(&configclient.ConfigChange{
		TenantID:  "t1",
		FieldPath: "app.name",
		NewValue:  configclient.StringVal("hello"),
	})

	time.Sleep(20 * time.Millisecond)
	if got := name.Get(); got != "hello" {
		t.Errorf("got %v, want %v", got, "hello")
	}

	// Then simulate the field being set to null via stream.
	sub.send(&configclient.ConfigChange{
		TenantID:  "t1",
		FieldPath: "app.name",
		OldValue:  configclient.StringVal("hello"),
		NewValue:  nil,
	})

	time.Sleep(20 * time.Millisecond)
	if got := name.Get(); got != "fallback" {
		t.Errorf("got %v, want %v after null update", got, "fallback")
	}

	cancel()
	_ = w.Close()
}

func TestWatcher_TypeFlipMidStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sub := newMockSubscription(ctx)

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{
				TenantID: "t1",
				Version:  1,
				Values: []configclient.ConfigValue{
					{FieldPath: "payments.fee", Value: configclient.FloatVal(0.025)},
				},
			}, nil
		},
		subscribeFn: func(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return sub, nil
		},
	}

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts:      options{minBackoff: 10 * time.Millisecond, maxBackoff: 50 * time.Millisecond},
		fields:    make(map[string]*fieldEntry),
		done:      make(chan struct{}),
	}

	fee, err := w.Float("payments.fee", 0.01)
	if err != nil {
		t.Fatalf("Float: %v", err)
	}

	if err := w.Start(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := fee.Get(); got != 0.025 {
		t.Errorf("initial: got %v, want %v", got, 0.025)
	}

	// Simulate type flip: server sends a string value for a float field.
	sub.send(&configclient.ConfigChange{
		TenantID:  "t1",
		FieldPath: "payments.fee",
		OldValue:  configclient.FloatVal(0.025),
		NewValue:  configclient.StringVal("not-a-number"),
	})

	time.Sleep(20 * time.Millisecond)

	// Watcher must use the default value, not crash.
	if got := fee.Get(); got != 0.01 {
		t.Errorf("after type flip: got %v, want default %v", got, 0.01)
	}
	_, ok := fee.GetWithNull()
	if ok {
		t.Error("expected field to be marked as not-set after type flip")
	}

	// Stream must still be alive: subsequent valid update must apply.
	sub.send(&configclient.ConfigChange{
		TenantID:  "t1",
		FieldPath: "payments.fee",
		OldValue:  configclient.StringVal("not-a-number"),
		NewValue:  configclient.FloatVal(0.1),
	})

	time.Sleep(20 * time.Millisecond)
	if got := fee.Get(); got != 0.1 {
		t.Errorf("after recovery: got %v, want %v", got, 0.1)
	}

	cancel()
	_ = w.Close()
}

func TestWatcher_ReconnectSnapshotTimeout(t *testing.T) {
	// Verify that a slow loadSnapshot during reconnect times out and does not
	// stall the reconnect loop indefinitely.
	var callCount atomic.Int32
	blocked := make(chan struct{})
	unblocked := make(chan struct{})

	tr := &mockTransport{
		getConfigFn: func(ctx context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			n := int(callCount.Add(1))
			if n == 1 {
				return &configclient.GetConfigResponse{TenantID: "t1"}, nil
			}
			if n == 2 {
				close(blocked)
				<-ctx.Done() // blocks until snapshotTimeout fires
				close(unblocked)
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("stream error")
		},
		subscribeFn: func(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return nil, fmt.Errorf("stream error")
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts: options{
			minBackoff:      5 * time.Millisecond,
			maxBackoff:      10 * time.Millisecond,
			snapshotTimeout: 30 * time.Millisecond,
			logger:          slog.Default(),
		},
		fields: make(map[string]*fieldEntry),
		done:   make(chan struct{}),
	}

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	select {
	case <-blocked:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("reconnect snapshot was not called")
	}

	select {
	case <-unblocked:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("snapshot did not time out within snapshotTimeout + margin")
	}

	cancel()
	_ = w.Close()
}

// TestWatcher_ReconnectVersionCursor verifies that on reconnect the watcher reloads
// the snapshot and passes StartVersion = snapshotVersion+1 to Subscribe, so that
// changes written between the disconnect and the new snapshot are not lost.
func TestWatcher_ReconnectVersionCursor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// snapshotCall tracks how many times GetConfig has been called.
	var snapshotCall atomic.Int32

	// subscribeCall tracks how many times Subscribe has been called.
	var subscribeCall atomic.Int32

	// recordedStartVersions records the StartVersion passed to each Subscribe call.
	var startVersionsMu sync.Mutex
	var recordedStartVersions []int32

	// firstSub is the initial subscription; closing its channel simulates a disconnect.
	firstSubCh := make(chan *configclient.ConfigChange, 16)
	firstSubCtx, firstSubCancel := context.WithCancel(context.Background())

	// secondSub is the reconnect subscription.
	secondSubCh := make(chan *configclient.ConfigChange, 16)

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			call := int(snapshotCall.Add(1))
			switch call {
			case 1:
				// Initial snapshot: version 3.
				return &configclient.GetConfigResponse{
					TenantID: "t1",
					Version:  3,
					Values: []configclient.ConfigValue{
						{FieldPath: "x", Value: configclient.StringVal("v3")},
					},
				}, nil
			default:
				// Reconnect snapshot: version 5 (two changes happened while disconnected).
				return &configclient.GetConfigResponse{
					TenantID: "t1",
					Version:  5,
					Values: []configclient.ConfigValue{
						{FieldPath: "x", Value: configclient.StringVal("v5")},
					},
				}, nil
			}
		},
		subscribeFn: func(subCtx context.Context, req *configclient.SubscribeRequest) (configclient.Subscription, error) {
			call := int(subscribeCall.Add(1))
			var sv int32
			if req.StartVersion != nil {
				sv = *req.StartVersion
			}
			startVersionsMu.Lock()
			recordedStartVersions = append(recordedStartVersions, sv)
			startVersionsMu.Unlock()

			switch call {
			case 1:
				return &mockSubscription{ch: firstSubCh, ctx: firstSubCtx}, nil
			default:
				// Use the watcher's own context so Recv exits when watcher closes.
				return &mockSubscription{ch: secondSubCh, ctx: subCtx}, nil
			}
		},
	}

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts: options{
			minBackoff:      5 * time.Millisecond,
			maxBackoff:      20 * time.Millisecond,
			snapshotTimeout: 100 * time.Millisecond,
			logger:          slog.Default(),
		},
		fields: make(map[string]*fieldEntry),
		done:   make(chan struct{}),
	}

	x, err := w.String("x", "default")
	if err != nil {
		t.Fatalf("String: %v", err)
	}

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Initial snapshot applied.
	if got := x.Get(); got != "v3" {
		t.Errorf("after initial snapshot: got %q, want %q", got, "v3")
	}

	// Wait for the first Subscribe call so we can inspect StartVersion.
	deadline := time.Now().Add(500 * time.Millisecond)
	for int(subscribeCall.Load()) < 1 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for first Subscribe call")
		}
		time.Sleep(2 * time.Millisecond)
	}

	// First Subscribe should have StartVersion=4 (snapshot was v3).
	startVersionsMu.Lock()
	if len(recordedStartVersions) < 1 || recordedStartVersions[0] != 4 {
		t.Errorf("first Subscribe StartVersion: got %v, want [4]", recordedStartVersions)
	}
	startVersionsMu.Unlock()

	// Simulate disconnect by closing the first subscription channel.
	firstSubCancel()
	close(firstSubCh)

	// Wait for reconnect: subscribe call 2 must happen.
	reconnectDeadline := time.Now().Add(500 * time.Millisecond)
	for int(subscribeCall.Load()) < 2 {
		if time.Now().After(reconnectDeadline) {
			t.Fatal("timed out waiting for reconnect")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Reconnect snapshot applied (version 5).
	time.Sleep(10 * time.Millisecond)
	if got := x.Get(); got != "v5" {
		t.Errorf("after reconnect snapshot: got %q, want %q", got, "v5")
	}

	// Second Subscribe must carry StartVersion=6 (reconnect snapshot was v5).
	startVersionsMu.Lock()
	if len(recordedStartVersions) < 2 || recordedStartVersions[1] != 6 {
		t.Errorf("second Subscribe StartVersion: got %v, want [4 6]", recordedStartVersions)
	}
	startVersionsMu.Unlock()

	cancel()
	_ = w.Close()
}

// TestWatcher_CleanStreamEndReconnects verifies that when the server closes the
// stream with io.EOF (OK status, no error), the watcher reconnects with backoff
// rather than treating it as a permanent shutdown.
func TestWatcher_CleanStreamEndReconnects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var subscribeCount atomic.Int32

	// firstSub closes immediately with io.EOF (no context cancel — pure channel close).
	firstSubCh := make(chan *configclient.ConfigChange)
	close(firstSubCh) // closed at construction: Recv returns io.EOF immediately

	// secondSub stays open until the watcher is shut down.
	secondSubCh := make(chan *configclient.ConfigChange)

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{TenantID: "t1", Version: 1}, nil
		},
		subscribeFn: func(subCtx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			call := int(subscribeCount.Add(1))
			if call == 1 {
				// Return the pre-closed channel — Recv will return io.EOF immediately.
				return &mockSubscription{ch: firstSubCh, ctx: subCtx}, nil
			}
			return &mockSubscription{ch: secondSubCh, ctx: subCtx}, nil
		},
	}

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts: options{
			minBackoff:      5 * time.Millisecond,
			maxBackoff:      20 * time.Millisecond,
			snapshotTimeout: 100 * time.Millisecond,
			logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
		fields: make(map[string]*fieldEntry),
		done:   make(chan struct{}),
	}

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the second Subscribe call — proves reconnect happened.
	deadline := time.Now().Add(500 * time.Millisecond)
	for int(subscribeCount.Load()) < 2 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for reconnect after clean stream-end")
		}
		time.Sleep(2 * time.Millisecond)
	}

	cancel()
	_ = w.Close()
}

func TestWatcher_TimeField(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{
				TenantID: "t1",
				Version:  1,
				Values: []configclient.ConfigValue{
					{FieldPath: "deploy.scheduled_at", Value: configclient.TimeVal(ts)},
				},
			}, nil
		},
		subscribeFn: func(ctx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return &mockSubscription{ch: make(chan *configclient.ConfigChange), ctx: ctx}, nil
		},
	}

	w := New(tr, "t1")
	scheduled, err := w.Time("deploy.scheduled_at", time.Time{})
	if err != nil {
		t.Fatalf("Time: %v", err)
	}

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := scheduled.Get(); !got.Equal(ts) {
		t.Errorf("got %v, want %v", got, ts)
	}

	cancel()
	_ = w.Close()
}

// --- DroppedCount tests ---

// TestValue_DroppedCount_IncrementOnFull verifies that DroppedCount increments
// each time an update is dropped because the Changes channel is full.
func TestValue_DroppedCount_IncrementOnFull(t *testing.T) {
	v := newValue(int64(0), parseInt)

	// Pre-fill the channel to capacity (16).
	for i := range 16 {
		v.update(fmt.Sprintf("%d", i), true)
	}
	if got := v.DroppedCount(); got != 0 {
		t.Fatalf("expected 0 drops before overflow, got %d", got)
	}

	// The next 5 updates should each count as a drop (channel remains full
	// because we never drain it; drop-oldest then re-send means the channel
	// stays at capacity, so every subsequent send triggers the drop path).
	const extraUpdates = 5
	for i := range extraUpdates {
		v.update(fmt.Sprintf("%d", 100+i), true)
	}

	if got := v.DroppedCount(); got != extraUpdates {
		t.Errorf("DroppedCount: got %d, want %d", got, extraUpdates)
	}

	// The value itself should reflect the latest update.
	if got := v.Get(); got != int64(100+extraUpdates-1) {
		t.Errorf("Get after drops: got %d, want %d", got, int64(100+extraUpdates-1))
	}
}

// TestValue_DroppedCount_ZeroUnderNormal verifies that no drops are counted
// when the channel is drained normally.
func TestValue_DroppedCount_ZeroUnderNormal(t *testing.T) {
	v := newValue(int64(0), parseInt)

	for i := range 10 {
		v.update(fmt.Sprintf("%d", i), true)
		// Drain each notification immediately.
		select {
		case <-v.Changes():
		case <-time.After(50 * time.Millisecond):
			t.Fatal("expected change on channel")
		}
	}

	if got := v.DroppedCount(); got != 0 {
		t.Errorf("expected 0 drops under normal operation, got %d", got)
	}
}

// TestValue_DroppedCount_WarnLog verifies that a WARN log entry is emitted
// (with the field name) each time a change is dropped.
func TestValue_DroppedCount_WarnLog(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	v := newValue(int64(0), parseInt)
	v.logger = logger
	v.fieldPath = "payments.fee"

	// Fill the channel to capacity.
	for i := range 16 {
		v.update(fmt.Sprintf("%d", i), true)
	}
	// Trigger a drop.
	v.update("999", true)

	if got := v.DroppedCount(); got != 1 {
		t.Fatalf("expected 1 drop, got %d", got)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "configwatcher: change dropped") {
		t.Errorf("expected WARN log containing 'configwatcher: change dropped', got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "payments.fee") {
		t.Errorf("expected WARN log to include field name 'payments.fee', got: %s", logOutput)
	}
}

// safeWriter is a goroutine-safe io.Writer backed by a bytes.Buffer.
type safeWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sw *safeWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.buf.Write(p)
}

func (sw *safeWriter) String() string {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.buf.String()
}

// TestValue_DroppedCount_ViaWatcher verifies that DroppedCount is accessible
// on values obtained through the Watcher and that the Watcher's logger is used.
func TestValue_DroppedCount_ViaWatcher(t *testing.T) {
	sw := &safeWriter{}
	logger := slog.New(slog.NewTextHandler(sw, &slog.HandlerOptions{Level: slog.LevelWarn}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := newMockSubscription(ctx)

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{TenantID: "t1", Version: 1}, nil
		},
		subscribeFn: func(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return sub, nil
		},
	}

	w := New(tr, "t1", WithLogger(logger))
	fee, err := w.Int("payments.fee", 0)
	if err != nil {
		t.Fatalf("Int: %v", err)
	}

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Drain the initial snapshot notification if any.
	select {
	case <-fee.Changes():
	default:
	}

	// Fill channel to capacity by sending stream updates without draining.
	for i := range 16 {
		sub.send(&configclient.ConfigChange{
			TenantID:  "t1",
			FieldPath: "payments.fee",
			NewValue:  configclient.IntVal(int64(i + 1)),
		})
	}
	// Give stream goroutine time to process all sends.
	time.Sleep(50 * time.Millisecond)

	// Send one more to trigger a drop.
	sub.send(&configclient.ConfigChange{
		TenantID:  "t1",
		FieldPath: "payments.fee",
		NewValue:  configclient.IntVal(999),
	})
	time.Sleep(50 * time.Millisecond)

	if got := fee.DroppedCount(); got < 1 {
		t.Errorf("expected at least 1 drop, got %d", got)
	}

	if logOutput := sw.String(); !strings.Contains(logOutput, "payments.fee") {
		t.Errorf("expected WARN log with field name, got: %s", logOutput)
	}

	cancel()
	_ = w.Close()
}

// --- Dedup / unchanged-value tests ---

// TestValue_NoChangeOnSameValue verifies that no Change is sent when update is
// called with a value identical to the current value (unchanged reconnect case).
func TestValue_NoChangeOnSameValue(t *testing.T) {
	v := newValue(int64(0), parseInt)

	// First update: sets the value to 42 — should emit a Change.
	v.update("42", true)
	select {
	case <-v.Changes():
		// Expected: first update always emits.
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected Change for initial set, got none")
	}

	// Second update with the same value — should NOT emit a Change.
	v.update("42", true)
	select {
	case ch := <-v.Changes():
		t.Errorf("unexpected Change for unchanged value: old=%v new=%v", ch.Old, ch.New)
	case <-time.After(50 * time.Millisecond):
		// Correct: no Change emitted.
	}
}

// TestValue_ChangeEmittedOnDifferentValue verifies that a Change IS still sent
// when the value actually changes.
func TestValue_ChangeEmittedOnDifferentValue(t *testing.T) {
	v := newValue(int64(0), parseInt)

	v.update("1", true)
	<-v.Changes() // drain first change

	v.update("2", true)
	select {
	case ch := <-v.Changes():
		if ch.Old != int64(1) || ch.New != int64(2) {
			t.Errorf("got old=%v new=%v, want old=1 new=2", ch.Old, ch.New)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected Change for different value, got none")
	}
}

// TestValue_NoChangeOnSameNull verifies that repeated null updates do not emit.
func TestValue_NoChangeOnSameNull(t *testing.T) {
	v := newValue("default", parseString)

	// First null update from the initial null state — old and new are both null,
	// values are both "default": no change expected.
	v.update("", false)
	select {
	case ch := <-v.Changes():
		t.Errorf("unexpected Change for null→null: %+v", ch)
	case <-time.After(50 * time.Millisecond):
		// Correct: both old and new are null+default, nothing to emit.
	}

	// Set a real value, then go back to null twice: second null must not re-emit.
	v.update("hello", true)
	<-v.Changes() // drain the set

	v.update("", false) // → null (Change emitted)
	<-v.Changes()       // drain the null change

	v.update("", false) // still null (no Change)
	select {
	case ch := <-v.Changes():
		t.Errorf("unexpected Change for repeated null: %+v", ch)
	case <-time.After(50 * time.Millisecond):
		// Correct.
	}
}

// TestValue_NoChangeOnSameTime verifies that time.Time values are compared via
// .Equal() so that monotonic-clock differences do not trigger spurious Changes.
func TestValue_NoChangeOnSameTime(t *testing.T) {
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	v := newValue(time.Time{}, parseTime)

	raw := epoch.Format(time.RFC3339Nano)

	v.update(raw, true)
	<-v.Changes() // drain initial change

	// Send the same RFC3339Nano string again — parsed time equals the stored one.
	v.update(raw, true)
	select {
	case ch := <-v.Changes():
		t.Errorf("unexpected Change for identical time.Time: old=%v new=%v", ch.Old, ch.New)
	case <-time.After(50 * time.Millisecond):
		// Correct.
	}
}

// TestWatcher_ReconnectNoSpuriousChanges verifies that on reconnect, when the
// server returns the same values that were already in effect, no Changes are
// emitted to consumers.
func TestWatcher_ReconnectNoSpuriousChanges(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var snapshotCall atomic.Int32

	firstSubCh := make(chan *configclient.ConfigChange, 4)
	firstSubCtx, firstSubCancel := context.WithCancel(context.Background())
	defer firstSubCancel()

	secondSubCh := make(chan *configclient.ConfigChange, 4)
	var subscribeCall atomic.Int32

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			// Both initial and reconnect snapshots return the same value.
			snapshotCall.Add(1)
			return &configclient.GetConfigResponse{
				TenantID: "t1",
				Version:  1,
				Values: []configclient.ConfigValue{
					{FieldPath: "app.name", Value: configclient.StringVal("hello")},
				},
			}, nil
		},
		subscribeFn: func(subCtx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			call := int(subscribeCall.Add(1))
			if call == 1 {
				return &mockSubscription{ch: firstSubCh, ctx: firstSubCtx}, nil
			}
			return &mockSubscription{ch: secondSubCh, ctx: subCtx}, nil
		},
	}

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts: options{
			minBackoff:      5 * time.Millisecond,
			maxBackoff:      20 * time.Millisecond,
			snapshotTimeout: 100 * time.Millisecond,
			logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
		fields: make(map[string]*fieldEntry),
		done:   make(chan struct{}),
	}

	name, err := w.String("app.name", "default")
	if err != nil {
		t.Fatalf("String: %v", err)
	}

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Drain the initial snapshot Change.
	select {
	case <-name.Changes():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected initial Change from snapshot")
	}

	// Wait for first Subscribe call.
	deadline := time.Now().Add(500 * time.Millisecond)
	for int(subscribeCall.Load()) < 1 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for first Subscribe call")
		}
		time.Sleep(2 * time.Millisecond)
	}

	// Simulate disconnect.
	firstSubCancel()
	close(firstSubCh)

	// Wait for reconnect (second Subscribe call).
	reconnectDeadline := time.Now().Add(500 * time.Millisecond)
	for int(subscribeCall.Load()) < 2 {
		if time.Now().After(reconnectDeadline) {
			t.Fatal("timed out waiting for reconnect")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Give the watcher time to apply the reconnect snapshot.
	time.Sleep(20 * time.Millisecond)

	// No spurious Change should have been emitted: the reconnect snapshot
	// returned the same value "hello" that was already in effect.
	select {
	case ch := <-name.Changes():
		t.Errorf("spurious Change on reconnect with unchanged value: old=%q new=%q", ch.Old, ch.New)
	case <-time.After(50 * time.Millisecond):
		// Correct: no Change emitted.
	}

	// Value must still be accessible.
	if got := name.Get(); got != "hello" {
		t.Errorf("Get after reconnect: got %q, want %q", got, "hello")
	}

	cancel()
	_ = w.Close()
}

// --- Direct typed-value delivery tests ---

// TestTypedUpdater_DirectInt verifies that an IntVal is delivered to an int field
// without going through string parsing.
func TestTypedUpdater_DirectInt(t *testing.T) {
	v := newValue(int64(0), parseInt)
	u := typedUpdater(v)

	u(configclient.IntVal(42))

	if got := v.Get(); got != int64(42) {
		t.Errorf("got %v, want 42", got)
	}
	_, ok := v.GetWithNull()
	if !ok {
		t.Error("expected isSet=true after direct int update")
	}
}

// TestTypedUpdater_DirectFloat verifies that a FloatVal is delivered to a float field
// without going through string parsing.
func TestTypedUpdater_DirectFloat(t *testing.T) {
	v := newValue(0.0, parseFloat)
	u := typedUpdater(v)

	u(configclient.FloatVal(3.14))

	if got := v.Get(); got != 3.14 {
		t.Errorf("got %v, want 3.14", got)
	}
}

// TestTypedUpdater_DirectBool verifies that a BoolVal is delivered directly.
func TestTypedUpdater_DirectBool(t *testing.T) {
	v := newValue(false, parseBool)
	u := typedUpdater(v)

	u(configclient.BoolVal(true))

	if got := v.Get(); !got {
		t.Error("got false, want true")
	}
}

// TestTypedUpdater_DirectDuration verifies that a DurationVal is delivered directly.
func TestTypedUpdater_DirectDuration(t *testing.T) {
	v := newValue(time.Duration(0), parseDuration)
	u := typedUpdater(v)

	u(configclient.DurationVal(5 * time.Minute))

	if got := v.Get(); got != 5*time.Minute {
		t.Errorf("got %v, want 5m", got)
	}
}

// TestTypedUpdater_DirectTime verifies that a TimeVal is delivered directly.
func TestTypedUpdater_DirectTime(t *testing.T) {
	ts := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	v := newValue(time.Time{}, parseTime)
	u := typedUpdater(v)

	u(configclient.TimeVal(ts))

	if got := v.Get(); !got.Equal(ts) {
		t.Errorf("got %v, want %v", got, ts)
	}
}

// TestTypedUpdater_DirectString verifies that a StringVal is delivered directly.
func TestTypedUpdater_DirectString(t *testing.T) {
	v := newValue("", parseString)
	u := typedUpdater(v)

	u(configclient.StringVal("hello"))

	if got := v.Get(); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

// TestTypedUpdater_KindMismatchFallback verifies that when the server sends a
// StringVal for an int field (kind mismatch), the watcher falls back to string
// parsing. A parseable string succeeds; a non-parseable string falls back to default.
func TestTypedUpdater_KindMismatchFallback(t *testing.T) {
	v := newValue(int64(99), parseInt)
	u := typedUpdater(v)

	// StringVal "7" can be parsed as int64 via string fallback.
	u(configclient.StringVal("7"))
	if got := v.Get(); got != int64(7) {
		t.Errorf("parseable fallback: got %v, want 7", got)
	}

	// StringVal "bad" cannot be parsed — falls back to default.
	u(configclient.StringVal("bad"))
	if got := v.Get(); got != int64(99) {
		t.Errorf("unparseable fallback: got %v, want default 99", got)
	}
}

// TestTypedUpdater_URLAndJSON verifies that URL and JSON typed values are delivered
// as strings to a string field directly.
func TestTypedUpdater_URLAndJSON(t *testing.T) {
	v := newValue("", parseString)
	u := typedUpdater(v)

	u(configclient.URLVal("https://example.com"))
	if got := v.Get(); got != "https://example.com" {
		t.Errorf("URL: got %q, want %q", got, "https://example.com")
	}

	u(configclient.JSONVal(`{"key":"val"}`))
	if got := v.Get(); got != `{"key":"val"}` {
		t.Errorf("JSON: got %q, want %q", got, `{"key":"val"}`)
	}
}

// TestValue_UpdateDirect verifies that updateDirect sets the value and emits a Change.
func TestValue_UpdateDirect(t *testing.T) {
	v := newValue(int64(0), parseInt)

	v.updateDirect(int64(55))

	if got := v.Get(); got != int64(55) {
		t.Errorf("got %v, want 55", got)
	}
	_, ok := v.GetWithNull()
	if !ok {
		t.Error("expected isSet=true after updateDirect")
	}

	select {
	case ch := <-v.Changes():
		if ch.WasNull != true {
			t.Error("expected WasNull=true on first direct update")
		}
		if ch.New != int64(55) {
			t.Errorf("Change.New: got %v, want 55", ch.New)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected Change on channel after updateDirect")
	}
}

// TestWatcher_SnapshotDirectDelivery verifies end-to-end that snapshot values are
// delivered via the direct path for int and duration kinds.
func TestWatcher_SnapshotDirectDelivery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{
				TenantID: "t1",
				Version:  1,
				Values: []configclient.ConfigValue{
					{FieldPath: "a.int", Value: configclient.IntVal(42)},
					{FieldPath: "b.dur", Value: configclient.DurationVal(30 * time.Second)},
				},
			}, nil
		},
		subscribeFn: func(ctx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return &mockSubscription{ch: make(chan *configclient.ConfigChange), ctx: ctx}, nil
		},
	}

	w := New(tr, "t1")
	n, err := w.Int("a.int", 0)
	if err != nil {
		t.Fatalf("Int: %v", err)
	}
	d, err := w.Duration("b.dur", 0)
	if err != nil {
		t.Fatalf("Duration: %v", err)
	}

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := n.Get(); got != int64(42) {
		t.Errorf("int field: got %v, want 42", got)
	}
	if got := d.Get(); got != 30*time.Second {
		t.Errorf("duration field: got %v, want 30s", got)
	}
}

// TestWatcher_StartAfterClose verifies that calling Start on an already-closed
// watcher returns ErrClosed without spawning a subscription goroutine.
func TestWatcher_StartAfterClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var subscribeCalls int
	sub := newMockSubscription(ctx)
	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{TenantID: "t1", Version: 1}, nil
		},
		subscribeFn: func(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			subscribeCalls++
			return sub, nil
		},
	}

	w := &Watcher{
		transport:   tr,
		tenantID:    "t1",
		opts:        options{minBackoff: 10 * time.Millisecond, maxBackoff: 50 * time.Millisecond},
		fields:      make(map[string]*fieldEntry),
		cancelReady: make(chan struct{}),
		done:        make(chan struct{}),
	}

	_, _ = w.String("app.name", "default")

	// First Start must succeed.
	if err := w.Start(ctx); err != nil {
		t.Fatalf("first Start: %v", err)
	}

	// Allow the subscription loop time to call Subscribe.
	time.Sleep(20 * time.Millisecond)

	goroutinesBefore := runtime.NumGoroutine()

	// Close the watcher.
	cancel()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Goroutine count should drop back after Close (subscriptionLoop exited).
	time.Sleep(20 * time.Millisecond)
	goroutinesAfterClose := runtime.NumGoroutine()

	// Second Start on a closed watcher must return ErrClosed.
	err := w.Start(context.Background())
	if err == nil {
		t.Fatal("expected ErrClosed from Start after Close, got nil")
	}
	if err != ErrClosed {
		t.Errorf("got %v, want ErrClosed", err)
	}

	// No additional goroutines must have been spawned.
	time.Sleep(20 * time.Millisecond)
	goroutinesAfterSecondStart := runtime.NumGoroutine()

	// subscribeCalls must be exactly 1 — the second Start must not reach Subscribe.
	if subscribeCalls != 1 {
		t.Errorf("Subscribe called %d times, want exactly 1", subscribeCalls)
	}

	// Goroutine count must not have increased after the rejected Start.
	if goroutinesAfterSecondStart > goroutinesAfterClose+1 {
		t.Errorf("goroutine leak: count before second Start=%d, after=%d (goroutinesBefore=%d)",
			goroutinesAfterClose, goroutinesAfterSecondStart, goroutinesBefore)
	}
}

// TestWatcher_StreamDirectDelivery verifies that stream updates are delivered
// via the direct path for int fields.
func TestWatcher_StreamDirectDelivery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := newMockSubscription(ctx)

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{TenantID: "t1", Version: 1}, nil
		},
		subscribeFn: func(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return sub, nil
		},
	}

	w := New(tr, "t1")
	n, err := w.Int("x.count", 0)
	if err != nil {
		t.Fatalf("Int: %v", err)
	}

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	sub.send(&configclient.ConfigChange{
		TenantID:  "t1",
		FieldPath: "x.count",
		NewValue:  configclient.IntVal(100),
	})

	select {
	case <-n.Changes():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("no Change received")
	}

	if got := n.Get(); got != int64(100) {
		t.Errorf("got %v, want 100", got)
	}
}
