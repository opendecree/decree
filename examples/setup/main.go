// Setup seeds the decree server with example data and writes the tenant ID
// to ../.tenant-id so examples can find it automatically.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/tools/seed"
)

func main() {
	addr := "localhost:9090"
	if v := os.Getenv("DECREE_ADDR"); v != "" {
		addr = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	admin := adminclient.New(
		pb.NewSchemaServiceClient(conn),
		pb.NewConfigServiceClient(conn),
		pb.NewAuditServiceClient(conn),
		adminclient.WithSubject("example-setup"),
	)

	data, err := os.ReadFile("../seed.yaml")
	if err != nil {
		log.Fatalf("read seed.yaml: %v", err)
	}

	file, err := seed.ParseFile(data)
	if err != nil {
		log.Fatalf("parse seed.yaml: %v", err)
	}

	result, err := seed.Run(ctx, admin, file, seed.AutoPublish())
	if err != nil {
		log.Fatalf("seed: %v", err)
	}

	if err := os.WriteFile("../.tenant-id", []byte(result.TenantID), 0o644); err != nil {
		log.Fatalf("write .tenant-id: %v", err)
	}

	fmt.Printf("Schema:  %s (v%d, created=%t)\n", result.SchemaID, result.SchemaVersion, result.SchemaCreated)
	fmt.Printf("Tenant:  %s (created=%t)\n", result.TenantID, result.TenantCreated)
	fmt.Printf("Config:  v%d (imported=%t)\n", result.ConfigVersion, result.ConfigImported)
	fmt.Println("Tenant ID written to .tenant-id")
}
