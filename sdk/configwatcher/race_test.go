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
				_, _ = registerField(w, fmt.Sprintf("field-%d", g), int64(0), parseInt)
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

	_, _ = w.String("app.name", "default")

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

	_, _ = w.String("app.name", "default")

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

		_, _ = w.String("app.name", "default")

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

// TestWatcher_StartCloseConcurrent_SnapshotFails is a variant of
// TestWatcher_StartCloseConcurrent that uses a getConfigFn which always returns
// an error. This exercises the failed-snapshot rollback branch in watcher.go
// (w.started = false, fresh cancelReady channel, close(old)).
//
// Start is expected to return an error on every iteration — that is not a
// failure. The assertions verify that:
//   - no panic or data race occurs (run with -race);
//   - after both goroutines finish, w.started is false (either rollback ran or
//     Close won the race before started was set);
//   - w.done is either closed (subscriptionLoop ran, which it should not on a
//     snapshot failure) or w.started == false.
//
// Run with: go test -race -run TestWatcher_StartCloseConcurrent ./sdk/configwatcher/
func TestWatcher_StartCloseConcurrent_SnapshotFails(t *testing.T) {
	const iterations = 200

	for range iterations {
		ctx, cancel := context.WithCancel(context.Background())

		tr := &mockTransport{
			getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
				return nil, fmt.Errorf("simulated snapshot failure")
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

		_, _ = w.String("app.name", "default")

		// ready gates Start and Close to begin at the same instant.
		ready := make(chan struct{})

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			<-ready
			// Error is expected (snapshot always fails); not a test failure.
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

		// After both goroutines finish, w.started must be false: either the
		// rollback path reset it, or Close won before Start set it to true.
		w.mu.RLock()
		started := w.started
		w.mu.RUnlock()
		if started {
			t.Error("w.started is true after a failed snapshot — rollback did not run")
		}

		// w.done must be either already closed (subscriptionLoop ran, which
		// should not happen on snapshot failure) or still open with started==false.
		// Either way, started==false already asserted above. Just confirm no leak:
		// if done is still open, that is fine because the loop was never launched.
		select {
		case <-w.done:
			// subscriptionLoop ran and exited — unexpected but not a data race.
		default:
			// done is still open: loop was never launched (expected path).
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

	val, err := w.String("key", "default")
	if err != nil {
		t.Fatalf("String: %v", err)
	}

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

// TestWatcher_CloseDoesNotHangOnFailedStart verifies that Close returns promptly
// when it races a Start whose snapshot call fails.
//
// Before the fix, Start's rollback path replaced w.cancelReady with a new channel
// without closing the old one. A concurrent Close that had already captured the old
// channel would block on it forever. The fix closes the old channel during rollback
// so Close is always unblocked.
//
// The test is deterministic: startedReady is closed by the snapshot function right
// after Start has set w.started=true (and before loadSnapshot returns), guaranteeing
// that Close captures the cancelReady channel that the rollback is about to swap out.
//
// Run with: go test -race -timeout 30s -run TestWatcher_CloseDoesNotHangOnFailedStart ./sdk/configwatcher/
func TestWatcher_CloseDoesNotHangOnFailedStart(t *testing.T) {
	const iterations = 200

	for range iterations {
		// startedReady is closed by the snapshot fn once w.started=true is set;
		// this gates the Close goroutine so it races the rollback window.
		startedReady := make(chan struct{})
		// unblockSnapshot is closed by the Close goroutine to release loadSnapshot.
		unblockSnapshot := make(chan struct{})

		tr := &mockTransport{
			getConfigFn: func(ctx context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
				// Notify Close goroutine that w.started=true has been set.
				close(startedReady)
				// Block until Close goroutine has called Close() and is waiting.
				select {
				case <-unblockSnapshot:
				case <-ctx.Done():
				}
				return nil, fmt.Errorf("snapshot failed")
			},
			subscribeFn: func(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
				panic("subscribe must never be reached on a failed snapshot")
			},
		}

		w := &Watcher{
			transport: tr,
			tenantID:  "t1",
			opts:      options{minBackoff: 5 * time.Millisecond, maxBackoff: 10 * time.Millisecond},
			fields:    make(map[string]*fieldEntry),
			done:      make(chan struct{}),
		}

		var wg sync.WaitGroup
		wg.Add(2)

		// Goroutine 1: calls Start; the snapshot fn will fail.
		go func() {
			defer wg.Done()
			_ = w.Start(context.Background())
		}()

		// Goroutine 2: waits until Start is inside loadSnapshot, then calls Close.
		closeDone := make(chan struct{})
		go func() {
			defer wg.Done()
			defer close(closeDone)

			// Wait until w.started=true has been committed inside Start.
			select {
			case <-startedReady:
			case <-time.After(5 * time.Second):
				t.Errorf("timed out waiting for Start to enter loadSnapshot")
				return
			}

			// Allow Start to proceed and fail while we race it with Close.
			close(unblockSnapshot)

			// This must not block forever (regression for #807).
			_ = w.Close()
		}()

		// Close must return within a generous deadline.
		select {
		case <-closeDone:
			// Good: Close returned promptly.
		case <-time.After(5 * time.Second):
			t.Fatal("Close blocked forever while racing a failed Start (deadlock regression — #807)")
		}

		wg.Wait()
	}
}
