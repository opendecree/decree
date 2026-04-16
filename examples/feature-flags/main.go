// Feature Flags demonstrates using configwatcher to react to live
// configuration changes — the core "feature flag" pattern.
//
// Run this example, then use the decree CLI or Admin GUI to toggle
// features.dark_mode or features.beta_access and watch the output change.
//
// Run:
//
//	go run .
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/opendecree/decree/sdk/grpctransport"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	conn, err := grpc.NewClient(serverAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	tenantID := mustTenantID()

	// Create a watcher and register boolean feature flags with defaults.
	w := grpctransport.NewWatcher(conn, tenantID,
		grpctransport.WithSubject("feature-flags-example"),
	)
	darkMode := w.Bool("features.dark_mode", false)
	betaAccess := w.Bool("features.beta_access", false)

	// Start loads the current values and subscribes to live changes.
	if err := w.Start(ctx); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}
	defer w.Close()

	fmt.Println("Feature flags loaded:")
	fmt.Println("  dark_mode:   ", darkMode.Get())
	fmt.Println("  beta_access: ", betaAccess.Get())
	fmt.Println()
	fmt.Println("Watching for changes... (Ctrl+C to stop)")
	fmt.Println("Try: decree config set <tenant-id> features.dark_mode false")

	// React to changes via the Changes channel.
	for {
		select {
		case change, ok := <-darkMode.Changes():
			if !ok {
				return nil
			}
			fmt.Printf("[%s] dark_mode changed: %v → %v\n",
				time.Now().Format("15:04:05"), change.Old, change.New)
		case change, ok := <-betaAccess.Changes():
			if !ok {
				return nil
			}
			fmt.Printf("[%s] beta_access changed: %v → %v\n",
				time.Now().Format("15:04:05"), change.Old, change.New)
		case <-ctx.Done():
			return nil
		}
	}
}

func serverAddr() string {
	if v := os.Getenv("DECREE_ADDR"); v != "" {
		return v
	}
	return "localhost:9090"
}

func mustTenantID() string {
	if v := os.Getenv("TENANT_ID"); v != "" {
		return v
	}
	data, err := os.ReadFile("../.tenant-id")
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	log.Fatal("Set TENANT_ID env var or run 'make setup' from the examples directory")
	return ""
}
