package configclient_test

import (
	"context"
	"fmt"

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
	return &configclient.SetFieldResponse{}, nil
}

func (f *fakeTransport) SetFields(_ context.Context, _ *configclient.SetFieldsRequest) (*configclient.SetFieldsResponse, error) {
	return &configclient.SetFieldsResponse{}, nil
}

func (f *fakeTransport) Subscribe(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
	return nil, fmt.Errorf("not implemented")
}

func ExampleClient_Get() {
	client := configclient.New(&fakeTransport{})
	ctx := context.Background()

	env, err := client.GetString(ctx, "tenant-1", "app.environment")
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

	err := client.Set(ctx, "tenant-1", "app.environment", "staging")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("ok")
	// Output: ok
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

	err := client.SetMany(ctx, "tenant-1", map[string]string{
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
