//go:build example

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/opendecree/decree/sdk/grpctransport"
)

// TestExample verifies the watcher starts and reads initial values.
// It does not wait for live changes (that requires manual interaction).
func TestExample(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpctransport.Dial(serverAddr(), grpctransport.WithInsecure())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	tenantID := os.Getenv("TENANT_ID")
	if tenantID == "" {
		data, err := os.ReadFile("../.tenant-id")
		if err != nil {
			t.Skip("no tenant ID available")
		}
		tenantID = strings.TrimSpace(string(data))
	}

	w, err := grpctransport.NewWatcher(conn, tenantID,
		grpctransport.WithSubject("feature-flags-test"),
		grpctransport.WithRole("superadmin"),
	)
	if err != nil {
		t.Fatalf("create watcher: %v", err)
	}
	darkMode, _ := w.Bool("features.dark_mode", false)
	betaAccess, _ := w.Bool("features.beta_access", false)

	if err := w.Start(ctx); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Close()

	// Verify initial values from seed data.
	if !darkMode.Get() {
		t.Error("expected dark_mode to be true from seed data")
	}
	if betaAccess.Get() {
		t.Error("expected beta_access to be false from seed data")
	}
	fmt.Println("feature-flags: watcher started, initial values verified")
}
