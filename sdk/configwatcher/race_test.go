package configwatcher

import (
	"context"
	"fmt"
	"io"
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

// mockSubscriptionBlocking is a subscription that blocks until context is cancelled.
// Used by race tests that don't need actual stream data.
type mockSubscriptionBlocking struct {
	ctx context.Context
}

func (s *mockSubscriptionBlocking) Recv() (*configclient.ConfigChange, error) {
	<-s.ctx.Done()
	return nil, io.EOF
}
