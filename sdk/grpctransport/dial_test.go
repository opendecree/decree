package grpctransport_test

import (
	"crypto/x509"
	"testing"

	"github.com/opendecree/decree/sdk/grpctransport"
)

func TestDial_DefaultTLS(t *testing.T) {
	conn, err := grpctransport.Dial("passthrough:///localhost:9999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	conn.Close()
}

func TestDial_WithInsecure(t *testing.T) {
	conn, err := grpctransport.Dial("passthrough:///localhost:9999", grpctransport.WithInsecure())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	conn.Close()
}

func TestDial_WithCustomCA(t *testing.T) {
	pool := x509.NewCertPool()
	conn, err := grpctransport.Dial("passthrough:///localhost:9999", grpctransport.WithCustomCA(pool))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	conn.Close()
}
