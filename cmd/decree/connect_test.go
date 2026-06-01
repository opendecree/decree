package main

import (
	"bytes"
	"io"
	"os"
	"strings"
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

// TestDialServer_TLS_NoWarning verifies that no warning is printed to stderr
// when --insecure is not set (the default, secure path).
func TestDialServer_TLS_NoWarning(t *testing.T) {
	orig := flagServer
	origInsecure := flagInsecure
	t.Cleanup(func() {
		flagServer = orig
		flagInsecure = origInsecure
	})

	flagServer = "passthrough:///localhost:9999"
	flagInsecure = false

	// Capture stderr.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	conn, dialErr := dialServer()
	w.Close()

	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck

	if dialErr != nil {
		t.Fatalf("dialServer (TLS path) unexpected error: %v", dialErr)
	}
	conn.Close()

	if buf.Len() != 0 {
		t.Errorf("expected no stderr output on TLS path, got: %q", buf.String())
	}
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

// TestDialServer_Insecure_PrintsWarning verifies that a warning is printed to
// stderr when --insecure is set.
func TestDialServer_Insecure_PrintsWarning(t *testing.T) {
	orig := flagServer
	origInsecure := flagInsecure
	t.Cleanup(func() {
		flagServer = orig
		flagInsecure = origInsecure
	})

	flagServer = "passthrough:///localhost:9999"
	flagInsecure = true

	// Capture stderr.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	origStderr := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	conn, dialErr := dialServer()
	w.Close()

	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck

	if dialErr != nil {
		t.Fatalf("dialServer (insecure path) unexpected error: %v", dialErr)
	}
	conn.Close()

	got := buf.String()
	if !strings.Contains(got, "Warning") {
		t.Errorf("expected warning on stderr for --insecure path, got: %q", got)
	}
	if !strings.Contains(got, "--insecure") {
		t.Errorf("expected warning to mention --insecure, got: %q", got)
	}
	if !strings.Contains(got, "not encrypted") {
		t.Errorf("expected warning to mention encryption, got: %q", got)
	}
}

// TestFlagInsecure_DefaultIsFalse verifies the --insecure flag defaults to false,
// ensuring TLS is used by default.
func TestFlagInsecure_DefaultIsFalse(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("insecure")
	if f == nil {
		t.Fatal("--insecure flag not registered on rootCmd")
	}
	if f.DefValue != "false" {
		t.Errorf("--insecure default = %q, want %q", f.DefValue, "false")
	}
}
