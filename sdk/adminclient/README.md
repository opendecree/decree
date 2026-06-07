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
schema, _ := client.ImportSchema(ctx, yamlContent, adminclient.WithAutoPublish())

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

Locks prevent non-superadmin users from modifying a field. Omit the values to lock the whole field, or pass specific enum values to lock only those (a block-list — the listed values can no longer be set; an empty list locks the entire field):

```go
// Lock the entire app.tier field — non-superadmins can no longer change it.
_ = client.LockField(ctx, tenant.ID, "app.tier")

// Or lock specific enum values only — here, block setting app.tier to "enterprise".
_ = client.LockField(ctx, tenant.ID, "app.tier", "enterprise")

locks, _ := client.ListFieldLocks(ctx, tenant.ID)
```

## Config versioning

```go
versions, _ := client.ListConfigVersions(ctx, tenant.ID)
newVer, _ := client.RollbackConfig(ctx, tenant.ID, versions[1].Version, "revert bad deploy")
_ = newVer
```

## Config import/export

```go
yaml, _ := client.ExportConfig(ctx, tenant.ID, nil) // nil = latest
// Mode defaults to ImportModeMerge. Pass WithImportMode to override.
_, _ = client.ImportConfig(ctx, tenant.ID, yaml, "seed")
_, _ = client.ImportConfig(ctx, tenant.ID, yaml, "replace", adminclient.WithImportMode(adminclient.ImportModeReplace))
```

## Related packages

- [`configclient`](../configclient) — runtime reads and writes for application code
- [`configwatcher`](../configwatcher) — live, auto-refreshing values
- [`grpctransport`](../grpctransport) — gRPC transport and `Dial` helper
