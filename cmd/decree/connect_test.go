package main

import (
	"testing"
)

// TestDialServer_TLS verifies that dialServer succeeds on the non-insecure path,
// meaning grpctransport.Dial is called without WithInsecure and TLS credentials
// (system roots) are used. grpc.NewClient does not actually connect, so this
// test passes without a live server.
func TestDialServer_TLS(t *testing.T) {
	orig := flagServer
	origInsecure := flagInsecure
	t.Cleanup(func() {
		flagServer = orig
		flagInsecure = origInsecure
	})

	flagServer = "passthrough:///localhost:9999"
	flagInsecure = false

	conn, err := dialServer()
	if err != nil {
		t.Fatalf("dialServer (TLS path) unexpected error: %v", err)
	}
	conn.Close()
}

// TestDialServer_Insecure verifies that dialServer succeeds on the insecure path
// (--insecure flag set), which passes WithInsecure() to grpctransport.Dial.
func TestDialServer_Insecure(t *testing.T) {
	orig := flagServer
	origInsecure := flagInsecure
	t.Cleanup(func() {
		flagServer = orig
		flagInsecure = origInsecure
	})

	flagServer = "passthrough:///localhost:9999"
	flagInsecure = true

	conn, err := dialServer()
	if err != nil {
		t.Fatalf("dialServer (insecure path) unexpected error: %v", err)
	}
	conn.Close()
}
