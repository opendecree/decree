// Optimistic Concurrency demonstrates safe concurrent config updates
// using compare-and-swap (CAS). Two approaches are shown:
//
//  1. GetForUpdate + Set — manual CAS with checksum
//  2. Update — convenience read-modify-write
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
	"strings"

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
	ctx := context.Background()

	conn, err := grpc.NewClient(serverAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	client := grpctransport.NewConfigClient(conn,
		grpctransport.WithSubject("cas-example"),
	)

	tenantID := mustTenantID()

	// --- Approach 1: Manual CAS with GetForUpdate ---

	fmt.Println("=== Manual CAS (GetForUpdate + Set) ===")

	// Read the current value and its checksum.
	locked, err := client.GetForUpdate(ctx, tenantID, "app.name")
	if err != nil {
		return fmt.Errorf("get for update: %w", err)
	}
	fmt.Printf("Current app.name: %q (checksum: %s)\n", locked.Value, locked.Checksum)

	// Update with the checksum — succeeds only if no one else changed it.
	if err := locked.Set(ctx, client, "Acme Corp App v2"); err != nil {
		return fmt.Errorf("cas set: %w", err)
	}
	fmt.Println("Updated to \"Acme Corp App v2\" via CAS")

	// Try again with the stale checksum — fails with ErrChecksumMismatch.
	err = locked.Set(ctx, client, "Acme Corp App v3")
	if err != nil {
		fmt.Printf("Stale CAS correctly rejected: %v\n", err)
	}

	// --- Approach 2: Convenience read-modify-write with Update ---

	fmt.Println()
	fmt.Println("=== Read-Modify-Write (Update) ===")

	// Update performs an atomic read-modify-write: reads the current value,
	// applies the function, and writes back with a checksum guard.
	// Returns ErrChecksumMismatch if another writer modified the value
	// between the read and write — the caller should retry if needed.
	err = client.Update(ctx, tenantID, "app.name", func(current string) (string, error) {
		return current + " (updated)", nil
	})
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}

	final, _ := client.Get(ctx, tenantID, "app.name")
	fmt.Printf("After update: app.name = %q\n", final)

	return nil
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
