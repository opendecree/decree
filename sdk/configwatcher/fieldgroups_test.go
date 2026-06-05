package configwatcher

import (
	"context"
	"testing"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
)

func TestNewGroup_FillAllTypes(t *testing.T) {
	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{
				Values: []configclient.ConfigValue{
					{FieldPath: "app.name", Value: configclient.StringVal("myapp")},
					{FieldPath: "app.debug", Value: configclient.BoolVal(true)},
					{FieldPath: "app.count", Value: configclient.IntVal(42)},
					{FieldPath: "app.rate", Value: configclient.FloatVal(1.5)},
					{FieldPath: "app.timeout", Value: configclient.DurationVal(5 * time.Second)},
				},
			}, nil
		},
		subscribeFn: func(ctx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return newMockSubscription(ctx), nil
		},
	}

	type Config struct {
		Name    string        `decree:"app.name"`
		Debug   bool          `decree:"app.debug"`
		Count   int64         `decree:"app.count"`
		Rate    float64       `decree:"app.rate"`
		Timeout time.Duration `decree:"app.timeout"`
		Ignored string
		Skipped string `decree:"-"`
	}

	w := New(tr, "t1")
	g, err := w.NewGroup(&Config{})
	if err != nil {
		t.Fatalf("NewGroup: %v", err)
	}
	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Close()

	var cfg Config
	if err := g.Fill(&cfg); err != nil {
		t.Fatalf("Fill: %v", err)
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

func TestNewGroup_NonPointerError(t *testing.T) {
	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{}, nil
		},
		subscribeFn: func(ctx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return newMockSubscription(ctx), nil
		},
	}
	w := New(tr, "t1")
	type S struct{}
	_, err := w.NewGroup(S{})
	if err == nil {
		t.Error("expected error for non-pointer, got nil")
	}
}

func TestGroup_Fill_TypeMismatch(t *testing.T) {
	tr := &mockTransport{
		getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
			return &configclient.GetConfigResponse{}, nil
		},
		subscribeFn: func(ctx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
			return newMockSubscription(ctx), nil
		},
	}
	w := New(tr, "t1")
	type A struct{ X string `decree:"x"` }
	type B struct{ X string `decree:"x"` }

	g, err := w.NewGroup(&A{})
	if err != nil {
		t.Fatalf("NewGroup: %v", err)
	}
	var b B
	if err := g.Fill(&b); err == nil {
		t.Error("expected type mismatch error, got nil")
	}
}
