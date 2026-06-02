package configwatcher_test

import (
	"context"
	"fmt"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/configwatcher"
)

// fakeTransport implements configclient.Transport for documentation examples.
type fakeTransport struct{}

func (f *fakeTransport) GetField(_ context.Context, _ *configclient.GetFieldRequest) (*configclient.GetFieldResponse, error) {
	return &configclient.GetFieldResponse{Value: configclient.StringVal("production")}, nil
}

func (f *fakeTransport) GetConfig(_ context.Context, req *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
	return &configclient.GetConfigResponse{TenantID: req.TenantID, Version: 1}, nil
}

func (f *fakeTransport) GetFields(_ context.Context, _ *configclient.GetFieldsRequest) (*configclient.GetFieldsResponse, error) {
	return &configclient.GetFieldsResponse{}, nil
}

func (f *fakeTransport) SetField(_ context.Context, _ *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error) {
	return &configclient.SetFieldResponse{}, nil
}

func (f *fakeTransport) SetFields(_ context.Context, _ *configclient.SetFieldsRequest) (*configclient.SetFieldsResponse, error) {
	return &configclient.SetFieldsResponse{}, nil
}

func (f *fakeTransport) Subscribe(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
	return nil, fmt.Errorf("not implemented")
}

func ExampleWatcher() {
	w := configwatcher.New(&fakeTransport{}, "tenant-1")

	// Register typed fields before calling Start.
	maxConn, _ := w.Int("limits.max_connections", 100)
	debug, _ := w.Bool("app.debug", false)

	// Before Start is called, Get returns the registered default.
	fmt.Println(maxConn.Get())
	fmt.Println(debug.Get())
	// Output:
	// 100
	// false
}

func ExampleWatcher_Start() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := configwatcher.New(&fakeTransport{}, "tenant-1")
	timeout, _ := w.String("jobs.timeout", "30s")

	// Start loads a snapshot and streams live updates in the background.
	if err := w.Start(ctx); err != nil {
		fmt.Println("start error:", err)
	}
	defer w.Close()

	// Get returns the current value; defaults are returned until the first snapshot.
	fmt.Println(timeout.Get())
	// Output: 30s
}

func ExampleValue_Changes() {
	w := configwatcher.New(&fakeTransport{}, "tenant-1")
	maxConn, _ := w.Int("limits.max_connections", 100)

	// Changes channel receives a notification on every update.
	// Read without blocking — no update has arrived yet.
	select {
	case change := <-maxConn.Changes():
		fmt.Println("updated to", change.New)
	default:
		fmt.Println("no update yet")
	}
	// Output: no update yet
}

func ExampleValue_GetWithNull() {
	w := configwatcher.New(&fakeTransport{}, "tenant-1")
	flag, _ := w.Bool("feature.enabled", false)

	val, ok := flag.GetWithNull()
	fmt.Println(val, ok) // default; field not yet received from server
	// Output: false false
}

func ExampleWithReconnectBackoff() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := configwatcher.New(&fakeTransport{}, "tenant-1",
		configwatcher.WithReconnectBackoff(100*time.Millisecond, 10*time.Second),
	)
	flag, _ := w.Bool("feature.enabled", false)

	if err := w.Start(ctx); err != nil {
		fmt.Println("start error:", err)
	}
	defer w.Close()

	fmt.Println(flag.Get())
	// Output: false
}
