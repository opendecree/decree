// Quickstart demonstrates connecting to OpenDecree and reading typed
// configuration values. This is the simplest possible example.
//
// Run:
//
//	go run .
//
// Requires a running decree server with seeded data (see ../README.md).
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

	// Connect to the decree server.
	conn, err := grpc.NewClient(serverAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	// Create a config client.
	client := grpctransport.NewConfigClient(conn,
		grpctransport.WithSubject("quickstart-example"),
	)

	tenantID := mustTenantID()

	// Read typed values — no string parsing needed.
	name, err := client.GetString(ctx, tenantID, "app.name")
	if err != nil {
		return fmt.Errorf("get app.name: %w", err)
	}
	fmt.Println("app.name:          ", name)

	debug, err := client.GetBool(ctx, tenantID, "app.debug")
	if err != nil {
		return fmt.Errorf("get app.debug: %w", err)
	}
	fmt.Println("app.debug:         ", debug)

	rateLimit, err := client.GetInt(ctx, tenantID, "server.rate_limit")
	if err != nil {
		return fmt.Errorf("get server.rate_limit: %w", err)
	}
	fmt.Println("server.rate_limit: ", rateLimit)

	timeout, err := client.GetDuration(ctx, tenantID, "server.timeout")
	if err != nil {
		return fmt.Errorf("get server.timeout: %w", err)
	}
	fmt.Println("server.timeout:    ", timeout)

	feeRate, err := client.GetFloat(ctx, tenantID, "payments.fee_rate")
	if err != nil {
		return fmt.Errorf("get payments.fee_rate: %w", err)
	}
	fmt.Println("payments.fee_rate: ", feeRate)

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
