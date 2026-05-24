# adminclient

> **Alpha** — API subject to change.

Go client for administrative operations on OpenDecree: schema management, tenant management, field locks, audit queries, and config versioning.

[![Go Reference](https://pkg.go.dev/badge/github.com/opendecree/decree/sdk/adminclient.svg)](https://pkg.go.dev/github.com/opendecree/decree/sdk/adminclient)

## Quickstart

```go
import "github.com/opendecree/decree/sdk/grpctransport"

conn, _ := grpctransport.Dial("localhost:50051", grpctransport.WithInsecure())
client, _ := grpctransport.NewAdminClient(conn,
    grpctransport.WithSubject("admin"),
    grpctransport.WithRole("superadmin"),
)

ctx := context.Background()
```

## Schema management

```go
// Import a schema from YAML and auto-publish it.
schema, _ := client.ImportSchema(ctx, yamlContent, true)

// Or build one programmatically.
schema, _ = client.CreateSchema(ctx, "app-config", []adminclient.Field{
    {Path: "app.name", Type: "string"},
    {Path: "app.debug", Type: "bool", Default: "false"},
}, "initial schema")
schema, _ = client.PublishSchema(ctx, schema.ID, schema.Version)
```

## Tenant management

```go
tenant, _ := client.CreateTenant(ctx, "acme-corp", schema.ID, schema.Version)
tenants, _ := client.ListTenants(ctx, schema.ID)
```

## Field locks

Locks prevent tenants from overriding a field beyond a set of allowed values:

```go
// Lock app.tier to "free" — prevents runtime override above the free plan.
_ = client.LockField(ctx, tenant.ID, "app.tier", "free")

locks, _ := client.ListFieldLocks(ctx, tenant.ID)
```

## Config versioning

```go
versions, _ := client.ListConfigVersions(ctx, tenant.ID)
_ = client.RollbackConfig(ctx, tenant.ID, versions[1].Version, "revert bad deploy")
```

## Config import/export

```go
yaml, _ := client.ExportConfig(ctx, tenant.ID, nil) // nil = latest
_, _ = client.ImportConfig(ctx, tenant.ID, yaml, "seed", adminclient.ImportModeMerge)
```

## Related packages

- [`configclient`](../configclient) — runtime reads and writes for application code
- [`configwatcher`](../configwatcher) — live, auto-refreshing values
- [`grpctransport`](../grpctransport) — gRPC transport and `Dial` helper
