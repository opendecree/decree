// Schema Lifecycle demonstrates the full administrative workflow:
// create a schema, add fields, publish, create a tenant, and clean up.
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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/opendecree/decree/sdk/adminclient"
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
		grpctransport.WithSubject("schema-lifecycle-example"),
	)

	// 1. Create a schema with initial fields.
	fmt.Println("Creating schema...")
	schema, err := admin.CreateSchema(ctx, "notifications", []adminclient.Field{
		{Path: "email.enabled", Type: "FIELD_TYPE_BOOL"},
		{Path: "email.from", Type: "FIELD_TYPE_STRING"},
		{Path: "sms.enabled", Type: "FIELD_TYPE_BOOL"},
	}, "Notification channel configuration")
	if err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	fmt.Printf("  Created: %s (v%d, draft)\n", schema.ID, schema.Version)

	// 2. Publish v1 — makes it immutable and assignable to tenants.
	fmt.Println("Publishing v1...")
	published, err := admin.PublishSchema(ctx, schema.ID, 1)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	fmt.Printf("  Published: v%d\n", published.Version)

	// 3. Update — adds a field, creating a new draft version.
	fmt.Println("Updating schema (adding webhook.url)...")
	updated, err := admin.UpdateSchema(ctx, schema.ID,
		[]adminclient.Field{
			{Path: "webhook.url", Type: "FIELD_TYPE_URL"},
		},
		nil,
		"Add webhook support",
	)
	if err != nil {
		return fmt.Errorf("update schema: %w", err)
	}
	fmt.Printf("  Updated: v%d (draft)\n", updated.Version)

	// 4. Publish v2.
	fmt.Println("Publishing v2...")
	_, err = admin.PublishSchema(ctx, schema.ID, updated.Version)
	if err != nil {
		return fmt.Errorf("publish v2: %w", err)
	}
	fmt.Println("  Published: v2")

	// 5. Create a tenant on the latest version.
	fmt.Println("Creating tenant...")
	tenant, err := admin.CreateTenant(ctx, "lifecycle-demo", schema.ID, 2)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	fmt.Printf("  Tenant: %s (schema v%d)\n", tenant.ID, tenant.SchemaVersion)

	// 6. List schemas to show it exists.
	schemas, err := admin.ListSchemas(ctx)
	if err != nil {
		return fmt.Errorf("list schemas: %w", err)
	}
	fmt.Printf("Total schemas: %d\n", len(schemas))

	// 7. Clean up.
	fmt.Println("Cleaning up...")
	admin.DeleteTenant(ctx, tenant.ID)
	admin.DeleteSchema(ctx, schema.ID)
	fmt.Println("Done.")

	return nil
}

func serverAddr() string {
	if v := os.Getenv("DECREE_ADDR"); v != "" {
		return v
	}
	return "localhost:9090"
}
