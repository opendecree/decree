package configwatcher

import (
	"context"
	"fmt"
	"io"
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

	fee := w.Float("payments.fee", 0.01)
	enabled := w.Bool("payments.enabled", false)

	err := w.Start(ctx)
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

	// Simulate a stream change.
	sub.send(&configclient.ConfigChange{
		TenantID:  "t1",
		FieldPath: "payments.fee",
		OldValue:  configclient.FloatVal(0.025),
		NewValue:  configclient.FloatVal(0.05),
	})

	// Wait for change to propagate.
	select {
	case ch := <-fee.Changes():
		_ = ch
	case <-time.After(100 * time.Millisecond):
	}

	// Read updated value.
	time.Sleep(10 * time.Millisecond) // let stream update propagate
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

	_ = w.String("app.name", "default")

	err := w.Start(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
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

	name := w.String("app.name", "fallback")

	err := w.Start(ctx)
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
	scheduled := w.Time("deploy.scheduled_at", time.Time{})

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := scheduled.Get(); !got.Equal(ts) {
		t.Errorf("got %v, want %v", got, ts)
	}

	cancel()
	_ = w.Close()
}
