package grpctransport_test

import (
	"crypto/x509"
	"testing"
	"time"

	"google.golang.org/grpc/keepalive"

	"github.com/opendecree/decree/sdk/grpctransport"
)

func TestDial_DefaultTLS(t *testing.T) {
	conn, err := grpctransport.Dial("passthrough:///localhost:9999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = conn.Close()
}

// TODO: There is no test that verifies WithInsecure yields a plaintext
// connection vs a TLS one. grpc.NewClient is lazy — it does not dial until
// the first RPC, so we cannot inspect transport-level security without a live
// server. Add a round-trip integration test (e.g. using an in-process server)
// when the e2e harness is extended to cover transport configuration.
func TestDial_WithInsecure(t *testing.T) {
	conn, err := grpctransport.Dial("passthrough:///localhost:9999", grpctransport.WithInsecure())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = conn.Close()
}

func TestDial_WithCustomCA(t *testing.T) {
	pool := x509.NewCertPool()
	conn, err := grpctransport.Dial("passthrough:///localhost:9999", grpctransport.WithCustomCA(pool))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = conn.Close()
}

// TestDial_DefaultKeepalive verifies that Dial succeeds with the default
// keepalive parameters applied (Time=30s, Timeout=10s, PermitWithoutStream=true).
// grpc.NewClient does not expose the negotiated params directly, so we verify
// that the option is accepted without error (i.e. no panic or validation failure).
func TestDial_DefaultKeepalive(t *testing.T) {
	conn, err := grpctransport.Dial("passthrough:///localhost:9999", grpctransport.WithInsecure())
	if err != nil {
		t.Fatalf("Dial with default keepalive failed: %v", err)
	}
	_ = conn.Close()
}

// TestDial_WithKeepalive verifies that WithKeepalive overrides the defaults
// without error.
func TestDial_WithKeepalive(t *testing.T) {
	custom := keepalive.ClientParameters{
		Time:                60 * time.Second,
		Timeout:             5 * time.Second,
		PermitWithoutStream: false,
	}
	conn, err := grpctransport.Dial("passthrough:///localhost:9999",
		grpctransport.WithInsecure(),
		grpctransport.WithKeepalive(custom),
	)
	if err != nil {
		t.Fatalf("Dial with custom keepalive failed: %v", err)
	}
	_ = conn.Close()
}

// TestDefaultKeepaliveParams verifies the exported default values match the
// documented constants so a future refactor cannot silently change them.
func TestDefaultKeepaliveParams(t *testing.T) {
	params := grpctransport.DefaultKeepalive()
	if got := params.Time; got != 30*time.Second {
		t.Errorf("Time: got %v, want 30s", got)
	}
	if got := params.Timeout; got != 10*time.Second {
		t.Errorf("Timeout: got %v, want 10s", got)
	}
	if !params.PermitWithoutStream {
		t.Error("PermitWithoutStream: got false, want true")
	}
}
