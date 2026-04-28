package server

import (
	"context"
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
	srv, err := New(Config{
		GRPCPort:        "0", // OS-assigned port
		EnableServices:  []string{"schema", "config"},
		Logger:          slog.Default(),
		AuthInterceptor: &noopInterceptor{},
	})
	require.NoError(t, err)
	assert.NotNil(t, srv.GRPCServer())
	srv.GracefulStop(context.Background())
}

func TestNew_InvalidPort(t *testing.T) {
	_, err := New(Config{
		GRPCPort:        "99999",
		Logger:          slog.Default(),
		AuthInterceptor: &noopInterceptor{},
	})
	assert.Error(t, err)
}

func TestIsServiceEnabled(t *testing.T) {
	srv, err := New(Config{
		GRPCPort:        "0",
		EnableServices:  []string{"schema", "config"},
		Logger:          slog.Default(),
		AuthInterceptor: &noopInterceptor{},
	})
	require.NoError(t, err)
	defer srv.GracefulStop(context.Background())

	assert.True(t, srv.IsServiceEnabled("schema"))
	assert.True(t, srv.IsServiceEnabled("config"))
	assert.False(t, srv.IsServiceEnabled("audit"))
}

func TestSetServiceHealthy(t *testing.T) {
	srv, err := New(Config{
		GRPCPort:        "0",
		EnableServices:  []string{"schema"},
		Logger:          slog.Default(),
		AuthInterceptor: &noopInterceptor{},
	})
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
	srv, err := New(Config{
		GRPCPort:        "0",
		Logger:          slog.Default(),
		AuthInterceptor: &noopInterceptor{},
		MaxRecvMsgBytes: recvCap,
		MaxSendMsgBytes: sendCap,
	})
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

func TestServe_AndGracefulStop(t *testing.T) {
	srv, err := New(Config{
		GRPCPort:        "0",
		EnableServices:  []string{"schema"},
		Logger:          slog.Default(),
		AuthInterceptor: &noopInterceptor{},
	})
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(context.Background()) }()

	// Give Serve time to start accepting.
	time.Sleep(50 * time.Millisecond)

	srv.GracefulStop(context.Background())
	assert.NoError(t, <-errCh)
}
