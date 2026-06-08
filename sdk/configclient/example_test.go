package configclient_test

import (
	"context"
	"fmt"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
)

// fakeTransport is a minimal Transport for documentation examples.
type fakeTransport struct{}

func (f *fakeTransport) GetField(_ context.Context, req *configclient.GetFieldRequest) (*configclient.GetFieldResponse, error) {
	return &configclient.GetFieldResponse{
		FieldPath: req.FieldPath,
		Value:     configclient.StringVal("production"),
	}, nil
}

func (f *fakeTransport) GetConfig(_ context.Context, req *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
	return &configclient.GetConfigResponse{
		TenantID: req.TenantID,
		Version:  1,
		Values: []configclient.ConfigValue{
			{FieldPath: "app.name", Value: configclient.StringVal("MyApp")},
			{FieldPath: "app.debug", Value: configclient.BoolVal(false)},
		},
	}, nil
}

func (f *fakeTransport) GetFields(_ context.Context, req *configclient.GetFieldsRequest) (*configclient.GetFieldsResponse, error) {
	values := make([]configclient.ConfigValue, len(req.FieldPaths))
	for i, p := range req.FieldPaths {
		values[i] = configclient.ConfigValue{FieldPath: p, Value: configclient.StringVal("value")}
	}
	return &configclient.GetFieldsResponse{Values: values}, nil
}

func (f *fakeTransport) SetField(_ context.Context, _ *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error) {
	return &configclient.SetFieldResponse{ConfigVersion: &configclient.ConfigVersion{Version: 2}}, nil
}

func (f *fakeTransport) SetFields(_ context.Context, _ *configclient.SetFieldsRequest) (*configclient.SetFieldsResponse, error) {
	return &configclient.SetFieldsResponse{ConfigVersion: &configclient.ConfigVersion{Version: 2}}, nil
}

func (f *fakeTransport) Subscribe(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
	return nil, fmt.Errorf("not implemented")
}

func ExampleClient_Get() {
	client := configclient.New(&fakeTransport{})
	ctx := context.Background()

	env, err := client.Get(ctx, "tenant-1", "app.environment")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(env)
	// Output: production
}

func ExampleClient_Set() {
	client := configclient.New(&fakeTransport{})
	ctx := context.Background()

	version, err := client.Set(ctx, "tenant-1", "app.environment", "staging")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("version:", version.Version)
	// Output: version: 2
}

func ExampleClient_GetAll() {
	client := configclient.New(&fakeTransport{})
	ctx := context.Background()

	values, err := client.GetAll(ctx, "tenant-1")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(len(values), "fields")
	// Output: 2 fields
}

func ExampleClient_SetMany() {
	client := configclient.New(&fakeTransport{})
	ctx := context.Background()

	_, err := client.SetMany(ctx, "tenant-1", map[string]string{
		"app.name":  "MyApp",
		"app.debug": "false",
	}, "bulk update")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("ok")
	// Output: ok
}

func ExampleClient_Snapshot() {
	client := configclient.New(&fakeTransport{})
	ctx := context.Background()

	// Snapshot pins all reads to the current version — consistent across calls.
	snap, err := client.Snapshot(ctx, "tenant-1")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(snap.Version() > 0)

	val, err := snap.Get(ctx, "app.environment")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(val)
	// Output:
	// true
	// production
}

func ExampleClient_Update() {
	client := configclient.New(&fakeTransport{})
	ctx := context.Background()

	// Atomic read-modify-write: append a suffix to the current value.
	_, err := client.Update(ctx, "tenant-1", "app.environment", func(current string) (string, error) {
		return current + "-updated", nil
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("ok")
	// Output: ok
}

func ExampleWithRetry() {
	client := configclient.New(&fakeTransport{}, configclient.WithRetry(configclient.RetryConfig{
		MaxAttempts:    5,
		InitialBackoff: 50 * time.Millisecond,
		Jitter:         true,
	}))
	ctx := context.Background()

	val, err := client.GetString(ctx, "tenant-1", "app.environment")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(val)
	// Output: production
}

func ExampleTypedValue() {
	tv := configclient.IntVal(42)
	fmt.Println(tv.Kind() == configclient.KindInteger)

	n, ok := tv.IntValue()
	fmt.Println(n, ok)
	fmt.Println(tv.String())
	// Output:
	// true
	// 42 true
	// 42
}
