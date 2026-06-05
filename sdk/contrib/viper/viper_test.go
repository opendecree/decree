package vipercontrib_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
	vipercontrib "github.com/opendecree/decree/sdk/contrib/viper"
	"github.com/spf13/viper"
)

// fakeTransport implements configclient.Transport for tests.
type fakeTransport struct {
	values []configclient.ConfigValue
	err    error
}

func (f *fakeTransport) GetField(_ context.Context, req *configclient.GetFieldRequest) (*configclient.GetFieldResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, v := range f.values {
		if v.FieldPath == req.FieldPath {
			return &configclient.GetFieldResponse{FieldPath: v.FieldPath, Value: v.Value}, nil
		}
	}
	return nil, configclient.ErrNotFound
}

func (f *fakeTransport) GetConfig(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &configclient.GetConfigResponse{
		TenantID: "test-tenant",
		Version:  1,
		Values:   f.values,
	}, nil
}

func (f *fakeTransport) GetFields(_ context.Context, req *configclient.GetFieldsRequest) (*configclient.GetFieldsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]configclient.ConfigValue, 0, len(req.FieldPaths))
	for _, path := range req.FieldPaths {
		for _, v := range f.values {
			if v.FieldPath == path {
				out = append(out, v)
				break
			}
		}
	}
	return &configclient.GetFieldsResponse{Values: out}, nil
}

func (f *fakeTransport) SetField(_ context.Context, _ *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error) {
	return &configclient.SetFieldResponse{}, nil
}

func (f *fakeTransport) SetFields(_ context.Context, _ *configclient.SetFieldsRequest) (*configclient.SetFieldsResponse, error) {
	return &configclient.SetFieldsResponse{}, nil
}

func (f *fakeTransport) Subscribe(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
	return nil, configclient.ErrNotFound
}

// TestNew verifies that New returns a non-nil Provider.
func TestNew(t *testing.T) {
	client := configclient.New(&fakeTransport{})
	p := vipercontrib.New(client, "tenant-1")
	if p == nil {
		t.Fatal("New returned nil")
	}
}

// TestNew_WithTimeout verifies that WithTimeout is accepted without error.
func TestNew_WithTimeout(t *testing.T) {
	client := configclient.New(&fakeTransport{})
	p := vipercontrib.New(client, "tenant-1", vipercontrib.WithTimeout(10*time.Second))
	if p == nil {
		t.Fatal("New returned nil")
	}
}

// TestFetch_JSONOutput verifies that the provider serialises config values to
// a valid JSON object that Viper can parse.
func TestFetch_JSONOutput(t *testing.T) {
	tr := &fakeTransport{
		values: []configclient.ConfigValue{
			{FieldPath: "app.name", Value: configclient.StringVal("myapp")},
			{FieldPath: "app.debug", Value: configclient.StringVal("true")},
			{FieldPath: "payments.fee", Value: configclient.StringVal("0.5")},
		},
	}
	client := configclient.New(tr)
	p := vipercontrib.New(client, "tenant-1")

	v := viper.New()
	v.SetConfigType("json")

	// Register the provider under a test name.
	const providerName = "decree-test"
	vipercontrib.Register(providerName, p)

	if err := v.AddRemoteProvider(providerName, "decree://local", ""); err != nil {
		t.Fatalf("AddRemoteProvider: %v", err)
	}
	if err := v.ReadRemoteConfig(); err != nil {
		t.Fatalf("ReadRemoteConfig: %v", err)
	}

	if got := v.GetString("app.name"); got != "myapp" {
		t.Errorf("app.name: got %q, want %q", got, "myapp")
	}
	if got := v.GetString("app.debug"); got != "true" {
		t.Errorf("app.debug: got %q, want %q", got, "true")
	}
	if got := v.GetString("payments.fee"); got != "0.5" {
		t.Errorf("payments.fee: got %q, want %q", got, "0.5")
	}
}

// TestFetch_EmptyConfig verifies that an empty config produces an empty JSON
// object without error.
func TestFetch_EmptyConfig(t *testing.T) {
	tr := &fakeTransport{values: nil}
	client := configclient.New(tr)
	p := vipercontrib.New(client, "tenant-1")

	v := viper.New()
	v.SetConfigType("json")

	const providerName = "decree-empty"
	vipercontrib.Register(providerName, p)

	if err := v.AddRemoteProvider(providerName, "decree://local", ""); err != nil {
		t.Fatalf("AddRemoteProvider: %v", err)
	}
	if err := v.ReadRemoteConfig(); err != nil {
		t.Fatalf("ReadRemoteConfig: %v", err)
	}
	// No keys should be set.
	if keys := v.AllKeys(); len(keys) != 0 {
		t.Errorf("expected 0 keys, got %v", keys)
	}
}

// TestFetch_TransportError verifies that ReadRemoteConfig propagates transport errors.
func TestFetch_TransportError(t *testing.T) {
	tr := &fakeTransport{err: configclient.ErrNotFound}
	client := configclient.New(tr)
	p := vipercontrib.New(client, "tenant-1")

	v := viper.New()
	v.SetConfigType("json")

	const providerName = "decree-err"
	vipercontrib.Register(providerName, p)

	if err := v.AddRemoteProvider(providerName, "decree://local", ""); err != nil {
		t.Fatalf("AddRemoteProvider: %v", err)
	}
	if err := v.ReadRemoteConfig(); err == nil {
		t.Fatal("expected error from ReadRemoteConfig, got nil")
	}
}

// TestFetch_JSONShape verifies the raw JSON produced by the provider uses
// nested objects for dot-separated field paths so that Viper's hierarchical
// lookup works correctly.
func TestFetch_JSONShape(t *testing.T) {
	tr := &fakeTransport{
		values: []configclient.ConfigValue{
			{FieldPath: "feature.flag", Value: configclient.StringVal("on")},
		},
	}
	client := configclient.New(tr)
	p := vipercontrib.New(client, "tenant-1")

	const providerName = "decree-shape"
	vipercontrib.Register(providerName, p)

	v := viper.New()
	v.SetConfigType("json")

	if err := v.AddRemoteProvider(providerName, "decree://local", ""); err != nil {
		t.Fatalf("AddRemoteProvider: %v", err)
	}
	if err := v.ReadRemoteConfig(); err != nil {
		t.Fatalf("ReadRemoteConfig: %v", err)
	}

	// Verify the raw JSON uses nested objects, not flat dotted keys.
	reader, err := viper.RemoteConfig.Get(nil)
	if err != nil {
		t.Fatalf("RemoteConfig.Get: %v", err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	// Expect nested: {"feature": {"flag": "on"}}
	feature, ok := m["feature"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested object at key %q, got: %v", "feature", m)
	}
	if flag, ok := feature["flag"]; !ok || flag != "on" {
		t.Errorf("expected feature.flag=%q, got: %v", "on", feature)
	}

	// Viper should also resolve it via GetString.
	if got := v.GetString("feature.flag"); got != "on" {
		t.Errorf("GetString(feature.flag): got %q, want %q", got, "on")
	}
}
