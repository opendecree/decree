// Multi-Tenant demonstrates that tenants share a schema but have
// independent configuration values. This example reads config from
// the seeded tenant and creates a second tenant with different values.
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

	"github.com/opendecree/decree/sdk/configclient"
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

	admin := grpctransport.NewAdminClient(conn,
		grpctransport.WithSubject("multi-tenant-example"),
	)
	cfg := grpctransport.NewConfigClient(conn,
		grpctransport.WithSubject("multi-tenant-example"),
	)

	tenantA := mustTenantID() // "acme-corp" from seed

	// Find the schema that tenant A uses.
	tenantInfo, err := admin.GetTenant(ctx, tenantA)
	if err != nil {
		return fmt.Errorf("get tenant: %w", err)
	}

	// Create a second tenant on the same schema.
	tenantBInfo, err := admin.CreateTenant(ctx, "globex-corp", tenantInfo.SchemaID, tenantInfo.SchemaVersion)
	if err != nil {
		return fmt.Errorf("create tenant B: %w", err)
	}
	tenantB := tenantBInfo.ID
	defer admin.DeleteTenant(ctx, tenantB)

	// Set different values for tenant B.
	if err := cfg.Set(ctx, tenantB, "app.name", "Globex Corp App"); err != nil {
		return fmt.Errorf("set app.name: %w", err)
	}
	if err := cfg.SetInt(ctx, tenantB, "server.rate_limit", 500); err != nil {
		return fmt.Errorf("set server.rate_limit: %w", err)
	}
	if err := cfg.Set(ctx, tenantB, "payments.currency", "EUR"); err != nil {
		return fmt.Errorf("set payments.currency: %w", err)
	}
	if err := cfg.SetFloat(ctx, tenantB, "payments.fee_rate", 0.015); err != nil {
		return fmt.Errorf("set payments.fee_rate: %w", err)
	}

	// Read from both tenants — same fields, different values.
	fmt.Println("=== Tenant A (acme-corp) ===")
	printConfig(ctx, cfg, tenantA)

	fmt.Println()
	fmt.Println("=== Tenant B (globex-corp) ===")
	printConfig(ctx, cfg, tenantB)

	return nil
}

func printConfig(ctx context.Context, cfg *configclient.Client, tenantID string) {
	name, _ := cfg.GetString(ctx, tenantID, "app.name")
	rate, _ := cfg.GetInt(ctx, tenantID, "server.rate_limit")
	currency, _ := cfg.GetString(ctx, tenantID, "payments.currency")
	fee, _ := cfg.GetFloat(ctx, tenantID, "payments.fee_rate")

	fmt.Printf("  app.name:           %s\n", name)
	fmt.Printf("  server.rate_limit:  %d\n", rate)
	fmt.Printf("  payments.currency:  %s\n", currency)
	fmt.Printf("  payments.fee_rate:  %.3f\n", fee)
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
