package envconfig_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
	envconfig "github.com/opendecree/decree/sdk/contrib/envconfig"
)

// stubTransport returns predetermined field values by path.
type stubTransport struct {
	values map[string]*configclient.TypedValue
	err    error
}

func (s *stubTransport) GetField(_ context.Context, req *configclient.GetFieldRequest) (*configclient.GetFieldResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	v, ok := s.values[req.FieldPath]
	if !ok {
		return nil, errors.New("field not found: " + req.FieldPath)
	}
	return &configclient.GetFieldResponse{FieldPath: req.FieldPath, Value: v}, nil
}

func (s *stubTransport) GetConfig(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
	return &configclient.GetConfigResponse{}, nil
}

func (s *stubTransport) GetFields(_ context.Context, req *configclient.GetFieldsRequest) (*configclient.GetFieldsResponse, error) {
	vals := make([]configclient.ConfigValue, 0, len(req.FieldPaths))
	for _, p := range req.FieldPaths {
		v, ok := s.values[p]
		if ok {
			vals = append(vals, configclient.ConfigValue{FieldPath: p, Value: v})
		}
	}
	return &configclient.GetFieldsResponse{Values: vals}, nil
}

func (s *stubTransport) SetField(_ context.Context, _ *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error) {
	return &configclient.SetFieldResponse{}, nil
}

func (s *stubTransport) SetFields(_ context.Context, _ *configclient.SetFieldsRequest) (*configclient.SetFieldsResponse, error) {
	return &configclient.SetFieldsResponse{}, nil
}

func (s *stubTransport) Subscribe(_ context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
	return nil, nil
}

func newClient(values map[string]*configclient.TypedValue) *configclient.Client {
	return configclient.New(&stubTransport{values: values})
}

func TestProcess_AllTypes(t *testing.T) {
	client := newClient(map[string]*configclient.TypedValue{
		"app.name":    configclient.StringVal("myapp"),
		"app.debug":   configclient.BoolVal(true),
		"app.count":   configclient.IntVal(42),
		"app.rate":    configclient.FloatVal(1.5),
		"app.timeout": configclient.DurationVal(5 * time.Second),
	})

	type Config struct {
		Name    string        `decree:"app.name"`
		Debug   bool          `decree:"app.debug"`
		Count   int64         `decree:"app.count"`
		Rate    float64       `decree:"app.rate"`
		Timeout time.Duration `decree:"app.timeout"`
	}

	var cfg Config
	if err := envconfig.Process(context.Background(), client, "t1", &cfg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if cfg.Name != "myapp" {
		t.Errorf("Name = %q, want %q", cfg.Name, "myapp")
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
	if cfg.Count != 42 {
		t.Errorf("Count = %d, want 42", cfg.Count)
	}
	if cfg.Rate != 1.5 {
		t.Errorf("Rate = %f, want 1.5", cfg.Rate)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", cfg.Timeout)
	}
}

func TestProcess_SkipsUntagged(t *testing.T) {
	client := newClient(map[string]*configclient.TypedValue{})

	type Config struct {
		Name       string `decree:"app.name"`
		Untagged   string
		Skipped    string `decree:"-"`
		unexported string //nolint:unused
	}

	_ = client
	// Process should not error for untagged or "-"-tagged or unexported fields
	client2 := newClient(map[string]*configclient.TypedValue{
		"app.name": configclient.StringVal("x"),
	})
	var cfg Config
	if err := envconfig.Process(context.Background(), client2, "t1", &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcess_NonPointerError(t *testing.T) {
	type Config struct{}
	err := envconfig.Process(context.Background(), nil, "", Config{})
	if err == nil {
		t.Error("expected error for non-pointer, got nil")
	}
}

func TestProcess_FieldError(t *testing.T) {
	stub := &stubTransport{err: errors.New("transport error")}
	client := configclient.New(stub)

	type Config struct {
		Name string `decree:"app.name"`
	}
	var cfg Config
	err := envconfig.Process(context.Background(), client, "t1", &cfg)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
