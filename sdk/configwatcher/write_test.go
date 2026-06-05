package configwatcher

import (
	"context"
	"errors"
	"testing"

	"github.com/opendecree/decree/sdk/configclient"
)

type writeTransport struct {
	mockTransport
	setFieldFn func(ctx context.Context, req *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error)
}

func (t *writeTransport) SetField(ctx context.Context, req *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error) {
	return t.setFieldFn(ctx, req)
}

func TestSetString_WritesAndUpdatesLocal(t *testing.T) {
	var written *configclient.SetFieldRequest
	tr := &writeTransport{
		mockTransport: mockTransport{
			getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
				return &configclient.GetConfigResponse{
					Values: []configclient.ConfigValue{{FieldPath: "app.env", Value: configclient.StringVal("staging")}},
				}, nil
			},
			subscribeFn: func(ctx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
				return newMockSubscription(ctx), nil
			},
		},
		setFieldFn: func(_ context.Context, req *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error) {
			written = req
			return &configclient.SetFieldResponse{}, nil
		},
	}

	w := New(tr, "tenant1")
	val, err := w.String("app.env", "default")
	if err != nil {
		t.Fatalf("String: %v", err)
	}
	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Close()

	if err := w.SetString(context.Background(), "app.env", "production"); err != nil {
		t.Fatalf("SetString: %v", err)
	}

	// Server was called with correct params.
	if written == nil || written.FieldPath != "app.env" {
		t.Errorf("SetField not called with expected field path, got %+v", written)
	}
	sv, _ := written.Value.StringValue()
	if sv != "production" {
		t.Errorf("SetField value = %q, want %q", sv, "production")
	}

	// Optimistic local update applied.
	if got := val.Get(); got != "production" {
		t.Errorf("local value = %q, want %q", got, "production")
	}
}

func TestSetString_TransportError(t *testing.T) {
	tr := &writeTransport{
		mockTransport: mockTransport{
			getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
				return &configclient.GetConfigResponse{}, nil
			},
			subscribeFn: func(ctx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
				return newMockSubscription(ctx), nil
			},
		},
		setFieldFn: func(_ context.Context, _ *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error) {
			return nil, errors.New("server error")
		},
	}

	w := New(tr, "tenant1")
	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Close()

	err := w.SetString(context.Background(), "app.env", "production")
	if err == nil {
		t.Error("expected error from transport, got nil")
	}
}

func TestSetBool_WriteThroughOptions(t *testing.T) {
	var req *configclient.SetFieldRequest
	tr := &writeTransport{
		mockTransport: mockTransport{
			getConfigFn: func(_ context.Context, _ *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
				return &configclient.GetConfigResponse{}, nil
			},
			subscribeFn: func(ctx context.Context, _ *configclient.SubscribeRequest) (configclient.Subscription, error) {
				return newMockSubscription(ctx), nil
			},
		},
		setFieldFn: func(_ context.Context, r *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error) {
			req = r
			return &configclient.SetFieldResponse{}, nil
		},
	}
	w := New(tr, "tenant1")
	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer w.Close()

	if err := w.SetBool(context.Background(), "app.debug", true,
		WithDescription("enable debug"),
		WithExpectedChecksum("abc123"),
	); err != nil {
		t.Fatalf("SetBool: %v", err)
	}
	if req.Description != "enable debug" {
		t.Errorf("Description = %q, want %q", req.Description, "enable debug")
	}
	if req.ExpectedChecksum == nil || *req.ExpectedChecksum != "abc123" {
		t.Errorf("ExpectedChecksum = %v, want abc123", req.ExpectedChecksum)
	}
}
