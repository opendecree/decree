<p align="center">
  <img src="https://raw.githubusercontent.com/opendecree/decree/main/assets/logo.png" alt="OpenDecree" width="150">
</p>

<h1 align="center">OpenDecree</h1>

<p align="center">
  <a href="https://github.com/opendecree/decree/actions/workflows/ci.yml"><img src="https://github.com/opendecree/decree/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go" alt="Go"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"></a>
</p>

<p align="center">Schema-driven business configuration management for multi-tenant services.</p>

## What is this?

OpenDecree manages **business-oriented configuration** — approval rules, fee structures, settlement windows, feature parameters — the kind of config that lives between your infrastructure settings and your application code.

### How is this different?

**Feature flag tools** (LaunchDarkly, ConfigCat, Flagsmith) focus on boolean/multivariate flags for release management — not structured business configuration with schemas.

**Infrastructure config tools** (etcd, Consul, Spring Cloud Config) are low-level key-value stores without typed schemas, validation, or multi-tenancy.

**Cloud config services** (AWS AppConfig, Azure App Configuration) offer some validation but lack schema registries, gRPC APIs, real-time subscriptions, and are vendor-locked.

**What makes OpenDecree unique:**

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
go get github.com/opendecree/decree/sdk/configclient@latest
go get github.com/opendecree/decree/sdk/adminclient@latest
go get github.com/opendecree/decree/sdk/configwatcher@latest
go get github.com/opendecree/decree/sdk/tools@latest         # diff, docgen, validate, seed, dump
```

## CLI

```bash
go install github.com/opendecree/decree/cmd/decree@latest

decree schema list
decree schema import --publish schema.yaml      # import + auto-publish

decree tenant create --name acme --schema <id> --schema-version 1
decree config set <tenant-id> payments.fee 0.5%
decree config get-all <tenant-id>
decree config versions <tenant-id>
decree config rollback <tenant-id> 2

decree watch <tenant-id>                          # live stream
decree lock set <tenant-id> payments.currency     # lock field
decree audit query --tenant <tenant-id> --since 24h

# Power tools
decree seed environments/dev.yaml                 # bootstrap everything from one file
decree dump <tenant-id> > backup.yaml             # full tenant backup
decree diff <tenant-id> 1 2                       # diff two config versions
decree diff --old v1.yaml --new v2.yaml           # diff two files
decree docgen <schema-id>                          # generate markdown docs
decree validate --schema s.yaml --config c.yaml   # offline validation
```

Global flags: `--server`, `--subject`, `--role`, `--output table|json|yaml`

## Quick Start

### Try it instantly (no Docker needed)

```bash
go install github.com/opendecree/decree/cmd/server@latest

# Start with in-memory storage — zero dependencies
STORAGE_BACKEND=memory HTTP_PORT=8080 decree-server

# Open http://localhost:8080/docs for Swagger UI
# All requests need x-subject header:
curl -H "x-subject: admin" http://localhost:8080/v1/schemas
```

### Docker Compose (production-like)

```bash
git clone https://github.com/opendecree/decree.git
cd decree

# Start the full stack (PostgreSQL + Redis + migrations + service)
docker compose up -d --wait service

# gRPC at localhost:9090, REST/JSON at localhost:8080
# No JWT required — metadata auth is the default
```

### REST API

The entire gRPC API is also available as REST/JSON (via grpc-gateway). Set `HTTP_PORT` to enable:

```bash
# Version check
curl http://localhost:8080/v1/version

# List schemas
curl -H "x-subject: admin" http://localhost:8080/v1/schemas

# Create a schema
curl -X POST http://localhost:8080/v1/schemas \
  -H "Content-Type: application/json" \
  -H "x-subject: admin" \
  -d '{"name":"payments","fields":[{"path":"fee","type":7}]}'

# Get config
curl -H "x-subject: admin" http://localhost:8080/v1/tenants/{id}/config
```

OpenAPI spec: [`docs/api/openapi.swagger.json`](docs/api/openapi.swagger.json)

### Using the CLI

```bash
# Set auth identity
export DECREE_SUBJECT=admin@example.com

# Create and publish a schema
decree schema import --publish examples/schema.yaml

# Create a tenant and set config
decree tenant create --name acme --schema <schema-id> --schema-version 1
decree config set <tenant-id> payments.fee "0.5%"
decree config get-all <tenant-id>
```

## Architecture

```
┌──────────┐     gRPC      ┌────────────────────────┐
│  Clients ├──────────────►│      OpenDecree        │
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
| `HTTP_PORT` | REST/JSON gateway port (disabled if empty) | disabled |
| `STORAGE_BACKEND` | `postgres` or `memory` | `postgres` |
| `DB_WRITE_URL` | PostgreSQL primary connection string | required (postgres) |
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
