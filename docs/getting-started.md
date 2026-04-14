# Getting Started

This guide walks you through setting up Central Config Service, creating your first schema and tenant, setting config values, and reading them from a Go application.

## Prerequisites

- Docker and Docker Compose
- Go 1.24+ (for the SDK and CLI)

## 1. Start the service

```bash
git clone https://github.com/zeevdr/central-config-service.git
cd central-config-service

# Start PostgreSQL, Redis, run migrations, and start the service
docker compose up -d --wait service
```

The gRPC service is now available at `localhost:9090`. No JWT setup needed — the service defaults to metadata-based auth.

## 2. Install the CLI

```bash
go install github.com/zeevdr/central-config-service/cmd/ccs@latest
```

Set your identity (required for all operations):

```bash
export CCS_SUBJECT=admin@example.com
```

## 3. Define a schema

A schema defines the structure of your configuration — what fields exist, their types, and constraints. Create a file called `payments.yaml`:

```yaml
syntax: "v1"
name: payments
description: Payment processing configuration

fields:
  payments.enabled:
    type: bool
    description: Whether payment processing is active

  payments.fee_rate:
    type: number
    description: Fee percentage per transaction
    constraints:
      minimum: 0
      maximum: 1

  payments.currency:
    type: string
    description: Default settlement currency
    constraints:
      enum: [USD, EUR, GBP]

  payments.max_retries:
    type: integer
    description: Maximum retry attempts for failed payments
    constraints:
      minimum: 0
      maximum: 10

  payments.timeout:
    type: duration
    description: Payment processing timeout
```

Import and publish it:

```bash
# Import creates the schema as a draft (v1)
ccs schema import payments.yaml

# Note the schema ID from the output, then publish it
ccs schema publish <schema-id> 1
```

Only published schema versions can be assigned to tenants.

## 4. Create a tenant

A tenant is a consumer of configuration — an organization, environment, or service instance bound to a schema version:

```bash
ccs tenant create --name acme --schema <schema-id> --schema-version 1
```

Note the tenant ID from the output — you'll use it for all config operations.

## 5. Set config values

```bash
# Set individual values
ccs config set <tenant-id> payments.enabled true
ccs config set <tenant-id> payments.fee_rate 0.025
ccs config set <tenant-id> payments.currency USD
ccs config set <tenant-id> payments.max_retries 3
ccs config set <tenant-id> payments.timeout 30s

# Or set multiple values at once
ccs config set-many <tenant-id> \
  payments.enabled=true \
  payments.fee_rate=0.025 \
  payments.currency=USD \
  --description "Initial payment config"
```

Read them back:

```bash
ccs config get-all <tenant-id>
```

## 6. Read config from Go

Install the SDK:

```bash
go get github.com/zeevdr/central-config-service/sdk/configclient@latest
```

Read configuration values with typed getters:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
    "github.com/zeevdr/central-config-service/sdk/configclient"
)

func main() {
    // Connect to the service
    conn, err := grpc.NewClient("localhost:9090",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    // Create a config client
    client := configclient.New(
        pb.NewConfigServiceClient(conn),
        configclient.WithSubject("myapp"),
    )

    ctx := context.Background()
    tenantID := "<your-tenant-id>"

    // Read typed values
    enabled, _ := client.GetBool(ctx, tenantID, "payments.enabled")
    feeRate, _ := client.GetFloat(ctx, tenantID, "payments.fee_rate")
    currency, _ := client.Get(ctx, tenantID, "payments.currency")
    retries, _ := client.GetInt(ctx, tenantID, "payments.max_retries")
    timeout, _ := client.GetDuration(ctx, tenantID, "payments.timeout")

    fmt.Printf("Payments enabled: %v\n", enabled)
    fmt.Printf("Fee rate: %.3f\n", feeRate)
    fmt.Printf("Currency: %s\n", currency)
    fmt.Printf("Max retries: %d\n", retries)
    fmt.Printf("Timeout: %s\n", timeout)
}
```

## 7. Use snapshots for consistent reads

When handling a request, you may want all config reads to come from the same version — even if config is being updated concurrently:

```go
// Pin to the current version
snap, _ := client.Snapshot(ctx, tenantID)

// All reads use the same version
fee, _ := snap.Get(ctx, "payments.fee_rate")
currency, _ := snap.Get(ctx, "payments.currency")
// Guaranteed consistent — both from the same config version
```

## 8. Watch for live changes

For long-running services that need to react to config changes in real-time, use the configwatcher SDK:

```bash
go get github.com/zeevdr/central-config-service/sdk/configwatcher@latest
```

```go
import "github.com/zeevdr/central-config-service/sdk/configwatcher"

// Register typed fields with defaults
w := configwatcher.New(conn, tenantID,
    configwatcher.WithSubject("myapp"),
)
feeRate := w.Float("payments.fee_rate", 0.01)
enabled := w.Bool("payments.enabled", false)

// Start watching (loads initial values, then subscribes to changes)
w.Start(ctx)
defer w.Close()

// Read current values (always fresh, never blocks)
fmt.Println(feeRate.Get())   // 0.025
fmt.Println(enabled.Get())   // true

// React to changes
go func() {
    for change := range feeRate.Changes() {
        log.Printf("Fee rate changed: %v → %v", change.Old, change.New)
    }
}()
```

## 9. Version and rollback

Every config change creates a new version. You can list versions and rollback:

```bash
# List versions
ccs config versions <tenant-id>

# Rollback to version 1
ccs config rollback <tenant-id> 1
```

## 10. Export and import

Export your config for backup, review, or migration between environments:

```bash
# Export config as YAML
ccs config export <tenant-id> > config-backup.yaml

# Import into another tenant or environment
ccs config import <other-tenant-id> config-backup.yaml
```

## What's next?

- [Concepts](concepts/overview.md) — understand schemas, tenants, typed values, and versioning in depth
- [API Reference](api/api-reference.md) — full gRPC service and message definitions
- [SDKs](sdk.md) — configclient, adminclient, and configwatcher documentation
- [CLI Reference](cli/ccs.md) — all `ccs` commands
- [Server Configuration](server/configuration.md) — environment variables and deployment
