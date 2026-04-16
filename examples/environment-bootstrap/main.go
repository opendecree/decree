// Environment Bootstrap demonstrates the seed tool — bootstrapping a
// complete environment (schema + tenant + config + locks) from a single
// YAML file. The operation is idempotent: run it multiple times safely.
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
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/opendecree/decree/sdk/grpctransport"
	"github.com/opendecree/decree/sdk/tools/seed"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(serverAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	admin := grpctransport.NewAdminClient(conn,
		grpctransport.WithSubject("bootstrap-example"),
	)

	// Parse the seed file.
	data, err := os.ReadFile("env.yaml")
	if err != nil {
		return fmt.Errorf("read env.yaml: %w", err)
	}
	file, err := seed.ParseFile(data)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	// Run the seed — creates schema, tenant, config, and locks.
	fmt.Println("Seeding environment...")
	result, err := seed.Run(ctx, admin, file, seed.AutoPublish())
	if err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	fmt.Printf("  Schema:  %s (v%d, created=%t)\n", result.SchemaID, result.SchemaVersion, result.SchemaCreated)
	fmt.Printf("  Tenant:  %s (created=%t)\n", result.TenantID, result.TenantCreated)
	fmt.Printf("  Config:  v%d (imported=%t)\n", result.ConfigVersion, result.ConfigImported)
	fmt.Printf("  Locks:   %d applied\n", result.LocksApplied)

	// Clean up.
	admin.DeleteTenant(ctx, result.TenantID)
	admin.DeleteSchema(ctx, result.SchemaID)

	return nil
}

func serverAddr() string {
	if v := os.Getenv("DECREE_ADDR"); v != "" {
		return v
	}
	return "localhost:9090"
}
