package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type noopInterceptor struct{}

func (n *noopInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(ctx, req)
	}
}

func (n *noopInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, ss)
	}
}

func TestNew_Success(t *testing.T) {
	srv, err := New("0", &noopInterceptor{},
		WithEnableServices([]string{"schema", "config"}),
		WithLogger(slog.Default()),
		WithInsecure(),
	)
	require.NoError(t, err)
	assert.NotNil(t, srv.GRPCServer())
	srv.GracefulStop(context.Background())
}

func TestNew_InvalidPort(t *testing.T) {
	_, err := New("99999", &noopInterceptor{},
		WithLogger(slog.Default()),
		WithInsecure(),
	)
	assert.Error(t, err)
}

func TestNew_RequiresTLSOrInsecure(t *testing.T) {
	_, err := New("0", &noopInterceptor{},
		WithLogger(slog.Default()),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TLS config is required")
}

func TestNew_TLSAndInsecureMutuallyExclusive(t *testing.T) {
	_, err := New("0", &noopInterceptor{},
		WithLogger(slog.Default()),
		WithInsecure(),
		WithTLS(&TLSConfig{CertFile: "x", KeyFile: "y"}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestIsServiceEnabled(t *testing.T) {
	srv, err := New("0", &noopInterceptor{},
		WithEnableServices([]string{"schema", "config"}),
		WithLogger(slog.Default()),
		WithInsecure(),
	)
	require.NoError(t, err)
	defer srv.GracefulStop(context.Background())

	assert.True(t, srv.IsServiceEnabled("schema"))
	assert.True(t, srv.IsServiceEnabled("config"))
	assert.False(t, srv.IsServiceEnabled("audit"))
}

func TestSetServiceHealthy(t *testing.T) {
	srv, err := New("0", &noopInterceptor{},
		WithEnableServices([]string{"schema"}),
		WithLogger(slog.Default()),
		WithInsecure(),
	)
	require.NoError(t, err)
	defer srv.GracefulStop(context.Background())

	// Should not panic.
	srv.SetServiceHealthy("centralconfig.v1.SchemaService")
}

// echoServiceDesc registers an "/test.Echo/Echo" unary RPC that returns its
// input verbatim. Used to drive message-size-cap tests without spinning up
// a full proto package.
var echoServiceDesc = grpc.ServiceDesc{
	ServiceName: "test.Echo",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Echo",
			Handler: func(_ any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
				in := &wrapperspb.BytesValue{}
				if err := dec(in); err != nil {
					return nil, err
				}
				return in, nil
			},
		},
	},
}

// startTestServer spins up a Server on a random port with the given size caps,
// registers the echo service, and returns a connected client + cleanup func.
func startTestServer(t *testing.T, recvCap, sendCap int) (*grpc.ClientConn, func()) {
	t.Helper()
	srv, err := New("0", &noopInterceptor{},
		WithLogger(slog.Default()),
		WithMaxRecvMsgBytes(recvCap),
		WithMaxSendMsgBytes(sendCap),
		WithInsecure(),
	)
	require.NoError(t, err)

	srv.grpcServer.RegisterService(&echoServiceDesc, struct{}{})

	addr := srv.listener.Addr().String()
	go func() { _ = srv.Serve(context.Background()) }()

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64*1024*1024),
			grpc.MaxCallSendMsgSize(64*1024*1024),
		),
	)
	require.NoError(t, err)

	cleanup := func() {
		_ = conn.Close()
		srv.GracefulStop(context.Background())
	}
	return conn, cleanup
}

func invokeEcho(ctx context.Context, conn *grpc.ClientConn, payload []byte) (*wrapperspb.BytesValue, error) {
	in := wrapperspb.Bytes(payload)
	out := &wrapperspb.BytesValue{}
	err := conn.Invoke(ctx, "/test.Echo/Echo", in, out)
	return out, err
}

func TestServer_MaxRecvMsgSize_RejectsOversizeRequest(t *testing.T) {
	conn, cleanup := startTestServer(t, 1024, 1024)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := invokeEcho(ctx, conn, make([]byte, 4096))
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestServer_MaxSendMsgSize_RejectsOversizeResponse(t *testing.T) {
	// Server's send cap (1 KB) is hit by a 4 KB echo response;
	// request itself stays under the 64 KB recv cap.
	conn, cleanup := startTestServer(t, 64*1024, 1024)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := invokeEcho(ctx, conn, make([]byte, 4096))
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

func TestServer_UnderCap_OK(t *testing.T) {
	conn, cleanup := startTestServer(t, 64*1024, 64*1024)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := invokeEcho(ctx, conn, make([]byte, 4096))
	require.NoError(t, err)
	assert.Len(t, out.Value, 4096)
}

func TestServer_ZeroCap_AppliesDefault(t *testing.T) {
	// Zero/negative configured caps should fall back to DefaultMaxMsgBytes.
	// 1 KB through is well under the 20 MB default and must succeed.
	conn, cleanup := startTestServer(t, 0, -1)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := invokeEcho(ctx, conn, make([]byte, 1024))
	require.NoError(t, err)
	assert.Len(t, out.Value, 1024)
}

// panicServiceDesc registers a service with a unary handler that panics and
// a streaming handler that panics. Used to drive recovery-interceptor tests.
//
// The unary Handler dispatches through `interceptor` per the gRPC codegen
// contract — without that, configured unary interceptors are bypassed. Stream
// interceptors are wired by the framework, so the stream Handler can panic
// directly.
var panicServiceDesc = grpc.ServiceDesc{
	ServiceName: "test.Panic",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Boom",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				in := &wrapperspb.BytesValue{}
				if err := dec(in); err != nil {
					return nil, err
				}
				inner := func(context.Context, any) (any, error) {
					panic("kaboom-unary")
				}
				if interceptor == nil {
					return inner(ctx, in)
				}
				info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/test.Panic/Boom"}
				return interceptor(ctx, in, info, inner)
			},
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName: "BoomStream",
			Handler: func(_ any, ss grpc.ServerStream) error {
				in := &wrapperspb.BytesValue{}
				if err := ss.RecvMsg(in); err != nil {
					return err
				}
				panic("kaboom-stream")
			},
			ClientStreams: true,
			ServerStreams: true,
		},
	},
}

func startPanicServer(t *testing.T) (*grpc.ClientConn, *bytes.Buffer, func()) {
	t.Helper()
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelError}))

	srv, err := New("0", &noopInterceptor{},
		WithLogger(logger),
		WithInsecure(),
	)
	require.NoError(t, err)
	srv.grpcServer.RegisterService(&panicServiceDesc, struct{}{})

	addr := srv.listener.Addr().String()
	go func() { _ = srv.Serve(context.Background()) }()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	cleanup := func() {
		_ = conn.Close()
		srv.GracefulStop(context.Background())
	}
	return conn, logBuf, cleanup
}

func TestRecovery_Unary_ReturnsInternalAndLogs(t *testing.T) {
	conn, logBuf, cleanup := startPanicServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	in := wrapperspb.Bytes([]byte("ping"))
	out := &wrapperspb.BytesValue{}
	err := conn.Invoke(ctx, "/test.Panic/Boom", in, out)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, genericInternalError, st.Message())
	assert.NotContains(t, st.Message(), "kaboom-unary", "panic value must not leak to client")

	// Log line is JSON-per-line; assert the structured fields rather than substrings.
	var entry map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimRight(logBuf.Bytes(), "\n"), &entry))
	assert.Equal(t, "panic in unary handler", entry["msg"])
	assert.Equal(t, "/test.Panic/Boom", entry["method"])
	assert.Equal(t, "kaboom-unary", entry["panic"])
	assert.Contains(t, entry["stack"], "recovery.go")
}

func TestRecovery_Stream_ReturnsInternalAndLogs(t *testing.T) {
	conn, logBuf, cleanup := startPanicServer(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	desc := &grpc.StreamDesc{StreamName: "BoomStream", ClientStreams: true, ServerStreams: true}
	stream, err := conn.NewStream(ctx, desc, "/test.Panic/BoomStream")
	require.NoError(t, err)
	require.NoError(t, stream.SendMsg(wrapperspb.Bytes([]byte("ping"))))
	require.NoError(t, stream.CloseSend())

	out := &wrapperspb.BytesValue{}
	err = stream.RecvMsg(out)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, genericInternalError, st.Message())
	assert.NotContains(t, st.Message(), "kaboom-stream", "panic value must not leak to client")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimRight(logBuf.Bytes(), "\n"), &entry))
	assert.Equal(t, "panic in stream handler", entry["msg"])
	assert.Equal(t, "/test.Panic/BoomStream", entry["method"])
	assert.Equal(t, "kaboom-stream", entry["panic"])
	assert.Contains(t, entry["stack"], "recovery.go")
}

func TestServe_AndGracefulStop(t *testing.T) {
	srv, err := New("0", &noopInterceptor{},
		WithEnableServices([]string{"schema"}),
		WithLogger(slog.Default()),
		WithInsecure(),
	)
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(context.Background()) }()

	// Give Serve time to start accepting.
	time.Sleep(50 * time.Millisecond)

	srv.GracefulStop(context.Background())
	assert.NoError(t, <-errCh)
}
