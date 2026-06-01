package configwatcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
)

func TestValue_ConcurrentGetUpdate(t *testing.T) {
	v := newValue(int64(0), parseInt)

	const goroutines = 20
	const iterations = 500

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func() {
			defer wg.Done()
			for i := range iterations {
				if g%2 == 0 {
					v.Get()
					v.GetWithNull()
				} else {
					v.update(fmt.Sprintf("%d", i), true)
				}
			}
		}()
	}

	wg.Wait()
}

func TestValue_ConcurrentUpdateAndChanges(t *testing.T) {
	v := newValue("default", parseString)

	var wg sync.WaitGroup
	wg.Add(2)

	// Writer goroutine.
	go func() {
		defer wg.Done()
		for i := range 200 {
			v.update(fmt.Sprintf("val-%d", i), true)
		}
		v.close()
	}()

	// Reader goroutine drains the changes channel.
	go func() {
		defer wg.Done()
		for range v.Changes() {
		}
	}()

	wg.Wait()
}

func TestWatcher_ConcurrentRegisterAndPaths(t *testing.T) {
	w := &Watcher{
		fields: make(map[string]*fieldEntry),
		done:   make(chan struct{}),
		opts: options{
			minBackoff: 10 * time.Millisecond,
			maxBackoff: 50 * time.Millisecond,
			logger:     slog.Default(),
		},
	}

	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := range goroutines {
		go func() {
			defer wg.Done()
			if g%2 == 0 {
				registerField(w, fmt.Sprintf("field-%d", g), int64(0), parseInt)
			} else {
				w.registeredPaths()
			}
		}()
	}

	wg.Wait()
}

func TestWatcher_ConcurrentStartAndClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := &mockSubscription{
		ch:  make(chan *configclient.ConfigChange, 16),
		ctx: ctx,
	}

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{
				TenantID: "t1",
				Version:  1,
				Values:   []configclient.ConfigValue{},
			}, nil
		},
		subscribeFn: func(ctx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return sub, nil
		},
	}

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts:      options{minBackoff: 10 * time.Millisecond, maxBackoff: 50 * time.Millisecond, logger: slog.Default()},
		fields:    make(map[string]*fieldEntry),
		done:      make(chan struct{}),
	}

	_ = w.String("app.name", "default")

	if err := w.Start(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Concurrent reads and close.
	var wg sync.WaitGroup
	wg.Add(10)
	for range 10 {
		go func() {
			defer wg.Done()
			w.registeredPaths()
		}()
	}
	wg.Wait()

	cancel()
	_ = w.Close()
}

// TestWatcher_DoubleStart verifies that calling Start twice is a no-op: the
// second call returns nil without launching a second subscription loop.
// Run with: go test -race ./sdk/configwatcher/
func TestWatcher_DoubleStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var subscribeCalls int
	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{TenantID: "t1", Version: 1}, nil
		},
		subscribeFn: func(sCtx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			subscribeCalls++
			return &mockSubscription{ch: make(chan *configclient.ConfigChange), ctx: sCtx}, nil
		},
	}

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts:      options{minBackoff: 10 * time.Millisecond, maxBackoff: 50 * time.Millisecond, logger: slog.Default()},
		fields:    make(map[string]*fieldEntry),
		done:      make(chan struct{}),
	}

	_ = w.String("app.name", "default")

	if err := w.Start(ctx); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := w.Start(ctx); err != nil {
		t.Fatalf("second Start: %v", err)
	}

	// Give the loop a moment to call Subscribe (it should only do so once).
	time.Sleep(20 * time.Millisecond)

	cancel()
	_ = w.Close()

	if subscribeCalls != 1 {
		t.Errorf("expected exactly 1 Subscribe call, got %d", subscribeCalls)
	}
}

// TestWatcher_StartCloseConcurrent fires Start and Close from separate goroutines
// simultaneously and verifies no race, panic, or goroutine leak occurs.
// Run with: go test -race ./sdk/configwatcher/
func TestWatcher_StartCloseConcurrent(t *testing.T) {
	const iterations = 200

	for range iterations {
		ctx, cancel := context.WithCancel(context.Background())

		tr := &mockTransport{
			getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
				return &configclient.GetConfigResponse{TenantID: "t1", Version: 1}, nil
			},
			subscribeFn: func(sCtx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
				return &mockSubscription{ch: make(chan *configclient.ConfigChange), ctx: sCtx}, nil
			},
		}

		w := &Watcher{
			transport: tr,
			tenantID:  "t1",
			opts:      options{minBackoff: 5 * time.Millisecond, maxBackoff: 10 * time.Millisecond, logger: slog.Default()},
			fields:    make(map[string]*fieldEntry),
			done:      make(chan struct{}),
		}

		_ = w.String("app.name", "default")

		// ready gates Start and Close to begin at the same instant.
		ready := make(chan struct{})

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			<-ready
			_ = w.Start(ctx)
		}()

		go func() {
			defer wg.Done()
			<-ready
			cancel()
			_ = w.Close()
		}()

		close(ready)
		wg.Wait()

		// If a subscription goroutine was started it must exit because ctx is
		// cancelled. Allow a short window for it to drain and close w.done.
		select {
		case <-w.done:
			// Loop exited cleanly (or was never started — done is pre-closed
			// only if subscriptionLoop ran and returned).
		case <-time.After(200 * time.Millisecond):
			// done is still open: the goroutine was never launched (Close won
			// before Start set started=true). This is not a leak.
			w.mu.RLock()
			started := w.started
			w.mu.RUnlock()
			if started {
				t.Error("goroutine launched but w.done never closed")
			}
		}
	}
}

// TestValue_CloseRacesWithUpdate exercises the race between Value.close() and
// Value.update() that previously caused a "send on closed channel" panic.
// Run with: go test -race ./sdk/configwatcher/
func TestValue_CloseRacesWithUpdate(t *testing.T) {
	const iterations = 1000

	for range iterations {
		v := newValue(int64(0), parseInt)

		// ready gates both goroutines to start at the same instant.
		ready := make(chan struct{})

		var wg sync.WaitGroup
		wg.Add(2)

		// Goroutine 1: hammer update.
		go func() {
			defer wg.Done()
			<-ready
			for i := range 50 {
				v.update(fmt.Sprintf("%d", i), true)
			}
		}()

		// Goroutine 2: close immediately after the gate opens.
		go func() {
			defer wg.Done()
			<-ready
			v.close()
		}()

		close(ready)
		wg.Wait()
	}
}

// TestWatcher_CloseWhileStreamSends verifies that Close does not panic when
// the subscription stream delivers updates concurrently with shutdown.
func TestWatcher_CloseWhileStreamSends(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// subCh is written to by the test to simulate incoming stream events.
	subCh := make(chan *configclient.ConfigChange, 64)
	sub := &mockSubscription{ch: subCh, ctx: ctx}

	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{TenantID: "t1", Version: 1}, nil
		},
		subscribeFn: func(sCtx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return sub, nil
		},
	}

	w := &Watcher{
		transport: tr,
		tenantID:  "t1",
		opts: options{
			minBackoff: 5 * time.Millisecond,
			maxBackoff: 10 * time.Millisecond,
			logger:     slog.Default(),
		},
		fields: make(map[string]*fieldEntry),
		done:   make(chan struct{}),
	}

	val := w.String("key", "default")

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Drain Changes so the buffer doesn't fill.
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range val.Changes() {
		}
	}()

	// Flood the stream with changes while concurrently closing.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 200 {
			select {
			case subCh <- &configclient.ConfigChange{
				TenantID:  "t1",
				FieldPath: "key",
				NewValue:  configclient.StringVal(fmt.Sprintf("v%d", i)),
			}:
			default:
			}
		}
	}()

	// Close shortly after starting the flood.
	time.Sleep(2 * time.Millisecond)
	cancel()
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	wg.Wait()

	// Changes channel must be closed after Close returns.
	select {
	case _, ok := <-val.Changes():
		if ok {
			t.Error("expected Changes channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Changes channel was not closed after Close()")
	}

	<-drainDone
}
