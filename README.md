# Central Config Service

Schema-driven business configuration management for multi-tenant services.

## What is this?

Central Config Service manages **business-oriented configuration** — approval rules, fee structures, settlement windows, feature parameters — the kind of config that lives between your infrastructure settings and your application code.

### How is this different?

**Feature flag tools** (LaunchDarkly, ConfigCat, Flagsmith) focus on boolean/multivariate flags for release management — not structured business configuration with schemas.

**Infrastructure config tools** (etcd, Consul, Spring Cloud Config) are low-level key-value stores without typed schemas, validation, or multi-tenancy.

**Cloud config services** (AWS AppConfig, Azure App Configuration) offer some validation but lack schema registries, gRPC APIs, real-time subscriptions, and are vendor-locked.

**What makes Central Config Service unique:**

No existing open-source tool combines a schema-first approach to typed configuration with native multi-tenancy, constraint validation, field-level locking, gRPC streaming, and versioned rollback — all in a single Go binary.

### Features

- **Typed values** — native proto types (integer, number, string, bool, timestamp, duration, url, json) with wire-level type safety
- **Schema validation** — constraints (min/max, pattern, enum, JSON Schema) enforced on every write
- **Multi-tenancy** — apply schemas to tenants with role-based access and field-level locking
- **Versioned configs** — every change creates a version; rollback to any previous state
- **Real-time subscriptions** — gRPC streaming pushes changes to consumers instantly
- **Audit trail** — full history of who changed what, when, and why
- **Import/Export** — portable schemas and configs in YAML format
- **Optimistic concurrency** — safe read-modify-write with checksum validation
- **Null support** — null and empty string are distinct values

## SDKs

Three Go SDK packages, each an independent module:

```go
// configclient — application runtime reads and writes
client := configclient.New(rpc, configclient.WithSubject("myapp"))
val, _ := client.GetInt(ctx, tenantID, "payments.retries")
client.SetBool(ctx, tenantID, "payments.enabled", true)

// Snapshot for consistent reads within a flow
snap, _ := client.Snapshot(ctx, tenantID)
fee, _ := snap.Get(ctx, "payments.fee")
currency, _ := snap.Get(ctx, "payments.currency")

// Optimistic concurrency (compare-and-swap)
client.Update(ctx, tenantID, "counter", func(current string) (string, error) {
    n, _ := strconv.Atoi(current)
    return strconv.Itoa(n + 1), nil
})

// adminclient — schema, tenant, audit management
admin := adminclient.New(schemaSvc, configSvc, auditSvc)
schema, _ := admin.CreateSchema(ctx, "payments", fields, "")
admin.PublishSchema(ctx, schema.ID, 1)

// configwatcher — live typed values with auto-reconnect
w := configwatcher.New(conn, tenantID)
fee := w.Float("payments.fee", 0.01)
enabled := w.Bool("payments.enabled", false)
w.Start(ctx)
fmt.Println(fee.Get())       // always fresh
for change := range fee.Changes() { ... }
```

Install only what you need:
```bash
go get github.com/zeevdr/central-config-service/sdk/configclient@latest
go get github.com/zeevdr/central-config-service/sdk/adminclient@latest
go get github.com/zeevdr/central-config-service/sdk/configwatcher@latest
```

## CLI

```bash
go install github.com/zeevdr/central-config-service/cmd/ccs@latest

ccs schema list
ccs schema import schema.yaml
ccs schema publish <schema-id> 1

ccs tenant create --name acme --schema <id> --schema-version 1
ccs config set <tenant-id> payments.fee 0.5%
ccs config get-all <tenant-id>
ccs config versions <tenant-id>
ccs config rollback <tenant-id> 2

ccs watch <tenant-id>                          # live stream
ccs lock set <tenant-id> payments.currency     # lock field
ccs audit query --tenant <tenant-id> --since 24h
```

Global flags: `--server`, `--subject`, `--role`, `--output table|json|yaml`

## Quick Start

### Docker Compose (local development)

```bash
git clone https://github.com/zeevdr/central-config-service.git
cd central-config-service

# Start the full stack (PostgreSQL + Redis + migrations + service)
docker compose up -d --wait service

# The gRPC service is now available at localhost:9090
# No JWT required — metadata auth is the default
```

### Using the CLI

```bash
# Set auth identity
export CCS_SUBJECT=admin@example.com

# Create and publish a schema
ccs schema import examples/schema.yaml
ccs schema publish <schema-id> 1

# Create a tenant and set config
ccs tenant create --name acme --schema <schema-id> --schema-version 1
ccs config set <tenant-id> payments.fee "0.5%"
ccs config get-all <tenant-id>
```

## Architecture

```
┌──────────┐     gRPC      ┌────────────────────────┐
│  Clients ├──────────────►│  Central Config Service │
└──────────┘               │                        │
                           │  ┌── SchemaService     │
                           │  ├── ConfigService     │
                           │  └── AuditService      │
                           └───┬──────────┬─────────┘
                               │          │
                          ┌────▼───┐  ┌───▼────┐
                          │ Postgres│  │ Redis  │
                          │        │  │ Cache + │
                          │        │  │ Pub/Sub │
                          └────────┘  └────────┘
```

Single binary exposing three gRPC services. Deploy with `ENABLE_SERVICES` to control which services run on each instance — scale read-heavy config instances independently from schema management.

## Configuration

### Server

| Variable | Description | Default |
|----------|------------|---------|
| `GRPC_PORT` | gRPC listen port | `9090` |
| `DB_WRITE_URL` | PostgreSQL primary connection string | required |
| `DB_READ_URL` | PostgreSQL read replica connection string | `DB_WRITE_URL` |
| `REDIS_URL` | Redis connection string | required |
| `ENABLE_SERVICES` | Services to enable: `schema`, `config`, `audit` | all |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` |

### Authentication

JWT is **opt-in**. By default, the service uses metadata-based auth:

| Variable | Description | Default |
|----------|------------|---------|
| `JWT_JWKS_URL` | JWKS endpoint — enables JWT validation | disabled |
| `JWT_ISSUER` | Expected JWT issuer | optional |

Without JWT, pass identity via gRPC metadata headers:
- `x-subject` (required) — actor identity
- `x-role` — `superadmin` (default), `admin`, or `user`
- `x-tenant-id` — required for non-superadmin roles

### Observability (all opt-in)

| Variable | Description |
|----------|------------|
| `OTEL_ENABLED` | Master switch — initializes SDK + slog trace correlation |
| `OTEL_TRACES_GRPC` | gRPC server spans |
| `OTEL_TRACES_DB` | PostgreSQL query spans |
| `OTEL_TRACES_REDIS` | Redis command spans |
| `OTEL_METRICS_GRPC` | gRPC request count/latency |
| `OTEL_METRICS_DB_POOL` | Connection pool gauges |
| `OTEL_METRICS_CACHE` | Cache hit/miss counters |
| `OTEL_METRICS_CONFIG` | Config write counter + version gauge |
| `OTEL_METRICS_SCHEMA` | Schema publish counter |

Standard OTel variables (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`) are respected by the SDK.

## API

The API is defined in Protocol Buffers under [`proto/`](proto/). Three gRPC services:

- **SchemaService** — create, version, and manage config schemas and tenants
- **ConfigService** — read/write typed config values, subscribe to changes, version management
- **AuditService** — query change history and usage statistics

Values use a `TypedValue` oneof — integer, number, string, bool, timestamp, duration, url, json — with null support.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, build instructions, and contribution guidelines.

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
