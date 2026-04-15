package configwatcher

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/configclient"
)

func sv(s string) *pb.TypedValue {
	return &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: s}}
}

// --- Value unit tests ---

func TestValue_Get_Default(t *testing.T) {
	v := newValue(42, parseInt)
	if got := v.Get(); got != int64(42) {
		t.Errorf("got %v, want %v", got, int64(42))
	}

	val, ok := v.GetWithNull()
	if got := val; got != int64(42) {
		t.Errorf("got %v, want %v", got, int64(42))
	}
	if ok {
		t.Error("expected false for null flag on default value")
	}
}

func TestValue_Update_Set(t *testing.T) {
	v := newValue(0.0, parseFloat)
	v.update("3.14", true)

	if got := v.Get(); got != 3.14 {
		t.Errorf("got %v, want %v", got, 3.14)
	}
	val, ok := v.GetWithNull()
	if got := val; got != 3.14 {
		t.Errorf("got %v, want %v", got, 3.14)
	}
	if !ok {
		t.Error("expected true for non-null value")
	}
}

func TestValue_Update_Null(t *testing.T) {
	v := newValue("default", parseString)
	v.update("hello", true)
	if got := v.Get(); got != "hello" {
		t.Errorf("got %v, want %v", got, "hello")
	}

	v.update("", false) // null
	if got := v.Get(); got != "default" {
		t.Errorf("got %v, want %v", got, "default")
	}
	_, ok := v.GetWithNull()
	if ok {
		t.Error("expected false for null value")
	}
}

func TestValue_Update_ParseError(t *testing.T) {
	v := newValue(int64(99), parseInt)
	v.update("not-a-number", true)

	// Falls back to default on parse error.
	if got := v.Get(); got != int64(99) {
		t.Errorf("got %v, want %v", got, int64(99))
	}
	_, ok := v.GetWithNull()
	if ok {
		t.Error("expected false after parse error fallback")
	}
}

func TestValue_Changes_Channel(t *testing.T) {
	v := newValue(false, parseBool)
	v.update("true", true)

	select {
	case ch := <-v.Changes():
		if !ch.WasNull {
			t.Error("expected WasNull to be true")
		}
		if ch.IsNull {
			t.Error("expected IsNull to be false")
		}
		if ch.Old {
			t.Error("expected Old to be false")
		}
		if !ch.New {
			t.Error("expected New to be true")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected change on channel")
	}
}

func TestValue_Duration(t *testing.T) {
	v := newValue(time.Second, parseDuration)
	v.update("24h", true)
	if got := v.Get(); got != 24*time.Hour {
		t.Errorf("got %v, want %v", got, 24*time.Hour)
	}
}

// --- Hand-written mock gRPC client for watcher integration tests ---

type mockRPC struct {
	mu              sync.Mutex
	getConfigResp   *pb.GetConfigResponse
	getConfigErr    error
	subscribeStream grpc.ServerStreamingClient[pb.SubscribeResponse]
	subscribeErr    error
}

func (m *mockRPC) GetConfig(_ context.Context, _ *pb.GetConfigRequest, _ ...grpc.CallOption) (*pb.GetConfigResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getConfigResp, m.getConfigErr
}

func (m *mockRPC) GetField(_ context.Context, _ *pb.GetFieldRequest, _ ...grpc.CallOption) (*pb.GetFieldResponse, error) {
	return nil, nil
}

func (m *mockRPC) GetFields(_ context.Context, _ *pb.GetFieldsRequest, _ ...grpc.CallOption) (*pb.GetFieldsResponse, error) {
	return nil, nil
}

func (m *mockRPC) SetField(_ context.Context, _ *pb.SetFieldRequest, _ ...grpc.CallOption) (*pb.SetFieldResponse, error) {
	return nil, nil
}

func (m *mockRPC) SetFields(_ context.Context, _ *pb.SetFieldsRequest, _ ...grpc.CallOption) (*pb.SetFieldsResponse, error) {
	return nil, nil
}

func (m *mockRPC) ListVersions(_ context.Context, _ *pb.ListVersionsRequest, _ ...grpc.CallOption) (*pb.ListVersionsResponse, error) {
	return nil, nil
}

func (m *mockRPC) GetVersion(_ context.Context, _ *pb.GetVersionRequest, _ ...grpc.CallOption) (*pb.GetVersionResponse, error) {
	return nil, nil
}

func (m *mockRPC) RollbackToVersion(_ context.Context, _ *pb.RollbackToVersionRequest, _ ...grpc.CallOption) (*pb.RollbackToVersionResponse, error) {
	return nil, nil
}

func (m *mockRPC) Subscribe(_ context.Context, _ *pb.SubscribeRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[pb.SubscribeResponse], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.subscribeStream, m.subscribeErr
}

func (m *mockRPC) ExportConfig(_ context.Context, _ *pb.ExportConfigRequest, _ ...grpc.CallOption) (*pb.ExportConfigResponse, error) {
	return nil, nil
}

func (m *mockRPC) ImportConfig(_ context.Context, _ *pb.ImportConfigRequest, _ ...grpc.CallOption) (*pb.ImportConfigResponse, error) {
	return nil, nil
}

// mockStream simulates a gRPC server stream.
type mockStream struct {
	ch  chan *pb.SubscribeResponse
	ctx context.Context
	grpc.ClientStream
}

func newMockStream(ctx context.Context) *mockStream {
	return &mockStream{ch: make(chan *pb.SubscribeResponse, 16), ctx: ctx}
}

func (s *mockStream) Recv() (*pb.SubscribeResponse, error) {
	select {
	case <-s.ctx.Done():
		return nil, io.EOF
	case msg, ok := <-s.ch:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil
	}
}

func (s *mockStream) send(change *pb.ConfigChange) {
	s.ch <- &pb.SubscribeResponse{Change: change}
}

// --- Watcher integration tests ---

func TestWatcher_SnapshotAndStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stream := newMockStream(ctx)

	rpc := &mockRPC{
		getConfigResp: &pb.GetConfigResponse{
			Config: &pb.Config{TenantId: "t1", Version: 1, Values: []*pb.ConfigValue{
				{FieldPath: "payments.fee", Value: sv("0.025")},
				{FieldPath: "payments.enabled", Value: sv("true")},
			}},
		},
		subscribeStream: stream,
	}

	// Build watcher manually with injected mock.
	w := &Watcher{
		rpc:      rpc,
		tenantID: "t1",
		opts:     options{role: "superadmin", minBackoff: 10 * time.Millisecond, maxBackoff: 50 * time.Millisecond},
		fields:   make(map[string]*fieldEntry),
		done:     make(chan struct{}),
	}
	// Wire configclient with same mock RPC.
	w.configClient = newConfigClientFromRPC(rpc)

	fee := w.Float("payments.fee", 0.01)
	enabled := w.Bool("payments.enabled", false)

	err := w.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify initial snapshot values.
	if got := fee.Get(); got != 0.025 {
		t.Errorf("got %v, want %v", got, 0.025)
	}
	if !enabled.Get() {
		t.Error("expected enabled to be true after snapshot")
	}

	// Simulate a stream change.
	stream.send(&pb.ConfigChange{
		TenantId:  "t1",
		FieldPath: "payments.fee",
		OldValue:  sv("0.025"),
		NewValue:  sv("0.05"),
	})

	// Wait for change to propagate.
	select {
	case ch := <-fee.Changes():
		// First change is from snapshot load, second from stream.
		// The snapshot load fires a change too.
		_ = ch
	case <-time.After(100 * time.Millisecond):
	}

	// Read updated value.
	time.Sleep(10 * time.Millisecond) // let stream update propagate
	if got := fee.Get(); got != 0.05 {
		t.Errorf("got %v, want %v", got, 0.05)
	}

	cancel()
	_ = w.Close()
}

// newConfigClientFromRPC creates a configclient.Client from a mock RPC
// without needing a real grpc.ClientConn.
func newConfigClientFromRPC(rpc pb.ConfigServiceClient) *configclient.Client {
	return configclient.New(rpc)
}
