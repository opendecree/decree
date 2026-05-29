package configclient

import (
	"context"
	"fmt"
	"testing"
)

// staticTransport returns a fixed response with zero overhead.
type staticTransport struct {
	fieldResp  *GetFieldResponse
	configResp *GetConfigResponse
}

func (s *staticTransport) GetField(_ context.Context, _ *GetFieldRequest) (*GetFieldResponse, error) {
	return s.fieldResp, nil
}

func (s *staticTransport) GetConfig(_ context.Context, _ *GetConfigRequest) (*GetConfigResponse, error) {
	return s.configResp, nil
}

func (s *staticTransport) GetFields(_ context.Context, req *GetFieldsRequest) (*GetFieldsResponse, error) {
	vals := make([]ConfigValue, len(req.FieldPaths))
	for i, fp := range req.FieldPaths {
		vals[i] = ConfigValue{FieldPath: fp, Value: StringVal("v")}
	}
	return &GetFieldsResponse{Values: vals}, nil
}

func (s *staticTransport) SetField(_ context.Context, _ *SetFieldRequest) (*SetFieldResponse, error) {
	return &SetFieldResponse{}, nil
}

func (s *staticTransport) SetFields(_ context.Context, _ *SetFieldsRequest) (*SetFieldsResponse, error) {
	return &SetFieldsResponse{}, nil
}

func (s *staticTransport) Subscribe(_ context.Context, _ *SubscribeRequest) (Subscription, error) {
	return nil, nil
}

func BenchmarkGet(b *testing.B) {
	tr := &staticTransport{
		fieldResp: &GetFieldResponse{
			FieldPath: "app.timeout",
			Value:     StringVal("30s"),
		},
	}
	client := New(tr)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Get(ctx, "tenant-1", "app.timeout")
	}
}

func BenchmarkConfigToMap(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		resp := makeConfigResponse(n)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = configToMap(resp)
			}
		})
	}
}

func makeConfigResponse(n int) *GetConfigResponse {
	vals := make([]ConfigValue, n)
	for i := range vals {
		vals[i] = ConfigValue{
			FieldPath: fmt.Sprintf("field.%d", i),
			Value:     StringVal(fmt.Sprintf("value-%d", i)),
		}
	}
	return &GetConfigResponse{TenantID: "t1", Values: vals}
}
