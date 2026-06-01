package grpctransport_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/grpctransport"
)

// stubServerService is an in-process ServerService that always returns the
// configured error (or a canned response when err is nil).
type stubServerService struct {
	pb.UnimplementedServerServiceServer
	err error
}

func (s *stubServerService) GetServerInfo(_ context.Context, _ *pb.GetServerInfoRequest) (*pb.GetServerInfoResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &pb.GetServerInfoResponse{
		Version:  "v1.2.3",
		Commit:   "abc123",
		Features: map[string]bool{"schemas": true},
	}, nil
}

// newServerTransportWithStub starts an in-process gRPC server backed by stub,
// dials it with insecure credentials, and returns a ServerTransport connected to it.
func newServerTransportWithStub(t *testing.T, stub pb.ServerServiceServer) *grpctransport.ServerTransport {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	pb.RegisterServerServiceServer(srv, stub)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop(); lis.Close() })

	cc, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { cc.Close() })

	return grpctransport.NewServerTransport(cc)
}

func TestServerTransport_GetServerInfo_UnavailableIsRetryable(t *testing.T) {
	stub := &stubServerService{err: status.Error(codes.Unavailable, "service down")}
	transport := newServerTransportWithStub(t, stub)

	_, err := transport.GetServerInfo(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var re *adminclient.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("got %v, want *adminclient.RetryableError", err)
	}
}

func TestServerTransport_GetServerInfo_DeadlineExceededIsRetryable(t *testing.T) {
	stub := &stubServerService{err: status.Error(codes.DeadlineExceeded, "deadline")}
	transport := newServerTransportWithStub(t, stub)

	_, err := transport.GetServerInfo(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var re *adminclient.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("got %v, want *adminclient.RetryableError", err)
	}
}

func TestServerTransport_GetServerInfo_UnavailableIsRetryableViaIsRetryable(t *testing.T) {
	stub := &stubServerService{err: status.Error(codes.Unavailable, "service down")}
	transport := newServerTransportWithStub(t, stub)

	_, err := transport.GetServerInfo(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !adminclient.IsRetryable(err) {
		t.Errorf("adminclient.IsRetryable(%v) = false, want true", err)
	}
}

func TestServerTransport_GetServerInfo_Success(t *testing.T) {
	stub := &stubServerService{}
	transport := newServerTransportWithStub(t, stub)

	info, err := transport.GetServerInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Version != "v1.2.3" {
		t.Errorf("Version = %q, want %q", info.Version, "v1.2.3")
	}
	if info.Commit != "abc123" {
		t.Errorf("Commit = %q, want %q", info.Commit, "abc123")
	}
}

func TestServerTransport_GetServerInfo_NotFoundIsNotRetryable(t *testing.T) {
	stub := &stubServerService{err: status.Error(codes.NotFound, "not found")}
	transport := newServerTransportWithStub(t, stub)

	_, err := transport.GetServerInfo(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if adminclient.IsRetryable(err) {
		t.Errorf("adminclient.IsRetryable(%v) = true, want false for NotFound", err)
	}
	if !errors.Is(err, adminclient.ErrNotFound) {
		t.Errorf("got %v, want adminclient.ErrNotFound", err)
	}
}
