package configwatcher

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"github.com/zeevdr/central-config-service/sdk/configclient"
)

func sp(s string) *string { return &s }

func sv(s string) *pb.TypedValue {
	return &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: s}}
}

// --- Value unit tests ---

func TestValue_Get_Default(t *testing.T) {
	v := newValue(42, parseInt)
	assert.Equal(t, int64(42), v.Get())

	val, ok := v.GetWithNull()
	assert.Equal(t, int64(42), val)
	assert.False(t, ok)
}

func TestValue_Update_Set(t *testing.T) {
	v := newValue(0.0, parseFloat)
	v.update("3.14", true)

	assert.Equal(t, 3.14, v.Get())
	val, ok := v.GetWithNull()
	assert.Equal(t, 3.14, val)
	assert.True(t, ok)
}

func TestValue_Update_Null(t *testing.T) {
	v := newValue("default", parseString)
	v.update("hello", true)
	assert.Equal(t, "hello", v.Get())

	v.update("", false) // null
	assert.Equal(t, "default", v.Get())
	_, ok := v.GetWithNull()
	assert.False(t, ok)
}

func TestValue_Update_ParseError(t *testing.T) {
	v := newValue(int64(99), parseInt)
	v.update("not-a-number", true)

	// Falls back to default on parse error.
	assert.Equal(t, int64(99), v.Get())
	_, ok := v.GetWithNull()
	assert.False(t, ok)
}

func TestValue_Changes_Channel(t *testing.T) {
	v := newValue(false, parseBool)
	v.update("true", true)

	select {
	case ch := <-v.Changes():
		assert.True(t, ch.WasNull)
		assert.False(t, ch.IsNull)
		assert.False(t, ch.Old)
		assert.True(t, ch.New)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected change on channel")
	}
}

func TestValue_Duration(t *testing.T) {
	v := newValue(time.Second, parseDuration)
	v.update("24h", true)
	assert.Equal(t, 24*time.Hour, v.Get())
}

// --- Mock gRPC client for watcher integration tests ---

type mockRPC struct {
	mock.Mock
}

func (m *mockRPC) GetConfig(ctx context.Context, in *pb.GetConfigRequest, opts ...grpc.CallOption) (*pb.GetConfigResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetConfigResponse), args.Error(1)
}

func (m *mockRPC) GetField(ctx context.Context, in *pb.GetFieldRequest, opts ...grpc.CallOption) (*pb.GetFieldResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetFieldResponse), args.Error(1)
}

func (m *mockRPC) GetFields(ctx context.Context, in *pb.GetFieldsRequest, opts ...grpc.CallOption) (*pb.GetFieldsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetFieldsResponse), args.Error(1)
}

func (m *mockRPC) SetField(ctx context.Context, in *pb.SetFieldRequest, opts ...grpc.CallOption) (*pb.SetFieldResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.SetFieldResponse), args.Error(1)
}

func (m *mockRPC) SetFields(ctx context.Context, in *pb.SetFieldsRequest, opts ...grpc.CallOption) (*pb.SetFieldsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.SetFieldsResponse), args.Error(1)
}

func (m *mockRPC) ListVersions(ctx context.Context, in *pb.ListVersionsRequest, opts ...grpc.CallOption) (*pb.ListVersionsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ListVersionsResponse), args.Error(1)
}

func (m *mockRPC) GetVersion(ctx context.Context, in *pb.GetVersionRequest, opts ...grpc.CallOption) (*pb.GetVersionResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetVersionResponse), args.Error(1)
}

func (m *mockRPC) RollbackToVersion(ctx context.Context, in *pb.RollbackToVersionRequest, opts ...grpc.CallOption) (*pb.RollbackToVersionResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.RollbackToVersionResponse), args.Error(1)
}

func (m *mockRPC) Subscribe(ctx context.Context, in *pb.SubscribeRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.SubscribeResponse], error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(grpc.ServerStreamingClient[pb.SubscribeResponse]), args.Error(1)
}

func (m *mockRPC) ExportConfig(ctx context.Context, in *pb.ExportConfigRequest, opts ...grpc.CallOption) (*pb.ExportConfigResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ExportConfigResponse), args.Error(1)
}

func (m *mockRPC) ImportConfig(ctx context.Context, in *pb.ImportConfigRequest, opts ...grpc.CallOption) (*pb.ImportConfigResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ImportConfigResponse), args.Error(1)
}

// mockStream simulates a gRPC server stream.
type mockStream struct {
	ch     chan *pb.SubscribeResponse
	ctx    context.Context
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
	rpc := &mockRPC{}

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

	// Mock snapshot (GetConfig for configclient.GetAll).
	rpc.On("GetConfig", mock.Anything, mock.Anything).Return(&pb.GetConfigResponse{
		Config: &pb.Config{TenantId: "t1", Version: 1, Values: []*pb.ConfigValue{
			{FieldPath: "payments.fee", Value: sv("0.025")},
			{FieldPath: "payments.enabled", Value: sv("true")},
		}},
	}, nil)

	// Mock Subscribe stream.
	ctx, cancel := context.WithCancel(context.Background())
	stream := newMockStream(ctx)
	rpc.On("Subscribe", mock.Anything, mock.Anything).Return(stream, nil)

	err := w.Start(ctx)
	require.NoError(t, err)

	// Verify initial snapshot values.
	assert.Equal(t, 0.025, fee.Get())
	assert.True(t, enabled.Get())

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
	assert.Equal(t, 0.05, fee.Get())

	cancel()
	_ = w.Close()
}

// newConfigClientFromRPC creates a configclient.Client from a mock RPC
// without needing a real grpc.ClientConn.
func newConfigClientFromRPC(rpc pb.ConfigServiceClient) *configclient.Client {
	return configclient.New(rpc)
}
