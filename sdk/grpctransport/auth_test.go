package grpctransport_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/grpctransport"
)

func conn() grpc.ClientConnInterface {
	c, _ := grpc.NewClient("passthrough:///localhost:9999", grpc.WithTransportCredentials(insecure.NewCredentials()))
	return c
}

func TestBuildConfig_NoRole_Errors(t *testing.T) {
	_, err := grpctransport.NewConfigTransport(conn())
	if err == nil {
		t.Fatal("expected error when no role or bearer token provided")
	}
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Fatalf("expected ErrRoleRequired, got %v", err)
	}
}

func TestBuildConfig_WithRole_OK(t *testing.T) {
	_, err := grpctransport.NewConfigTransport(conn(),
		grpctransport.WithRole("user"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildConfig_WithBearerToken_NoRoleRequired(t *testing.T) {
	_, err := grpctransport.NewConfigTransport(conn(),
		grpctransport.WithBearerToken("tok"),
	)
	if err != nil {
		t.Fatalf("unexpected error with bearer token and no role: %v", err)
	}
}

func TestNewAdminClient_NoRole_Errors(t *testing.T) {
	_, err := grpctransport.NewAdminClient(conn())
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Fatalf("expected ErrRoleRequired, got %v", err)
	}
}

func TestNewWatcher_NoRole_Errors(t *testing.T) {
	_, err := grpctransport.NewWatcher(conn(), "tenant-id")
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Fatalf("expected ErrRoleRequired, got %v", err)
	}
}

func TestBuildConfig_WithTokenSource_NoRoleRequired(t *testing.T) {
	_, err := grpctransport.NewConfigTransport(conn(),
		grpctransport.WithTokenSource(func(context.Context) (string, error) {
			return "tok", nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error with token source and no role: %v", err)
	}
}

func TestBuildConfig_WithTokenSource_ErrorPropagates(t *testing.T) {
	// Construction succeeds; token source errors surface on RPC calls, not at build time.
	_, err := grpctransport.NewConfigTransport(conn(),
		grpctransport.WithTokenSource(func(context.Context) (string, error) {
			return "", errors.New("token refresh failed")
		}),
	)
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}
}

// TestInsecureConn_WithBearerToken_RPCRejected verifies that gRPC refuses to send
// a bearer token over a plaintext (insecure) connection at RPC call time.
//
// bearerToken.RequireTransportSecurity() returns true, so gRPC must return an
// error containing "transport security" before the RPC leaves the client.
func TestInsecureConn_WithBearerToken_RPCRejected(t *testing.T) {
	// Start an in-process gRPC server using bufconn (no TLS).
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	pb.RegisterConfigServiceServer(srv, &pb.UnimplementedConfigServiceServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop(); _ = lis.Close() })

	// Dial with insecure credentials (no TLS).
	clientConn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = clientConn.Close() })

	transport, err := grpctransport.NewConfigTransport(clientConn,
		grpctransport.WithBearerToken("super-secret-jwt"),
	)
	if err != nil {
		t.Fatalf("NewConfigTransport: %v", err)
	}

	// The RPC must fail because PerRPCCredentials with RequireTransportSecurity=true
	// refuses to attach credentials to a plaintext connection.
	_, rpcErr := transport.GetField(context.Background(), &configclient.GetFieldRequest{
		TenantID:  "tenant-1",
		FieldPath: "a",
	})
	if rpcErr == nil {
		t.Fatal("expected RPC to fail on insecure connection with bearer token, got nil error")
	}
	// gRPC either rejects at the transport layer ("cannot send secure credentials on an
	// insecure connection") or the server rejects the missing token as Unauthenticated.
	// Both outcomes confirm that bearer tokens are not silently sent over plaintext.
	if !errors.Is(rpcErr, configclient.ErrUnauthenticated) {
		errMsg := rpcErr.Error()
		if !strings.Contains(errMsg, "insecure connection") && !strings.Contains(errMsg, "transport security") {
			t.Errorf("expected error to mention insecure connection or transport security, or ErrUnauthenticated; got: %v", rpcErr)
		}
	}
}
