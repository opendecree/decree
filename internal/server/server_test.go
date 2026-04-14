package server

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
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
