<p align="center">
  <img src="https://raw.githubusercontent.com/opendecree/decree/main/assets/logo.png" alt="OpenDecree" width="150">
</p>

<h1 align="center">OpenDecree</h1>

<p align="center">
  <a href="https://github.com/opendecree/decree/actions/workflows/ci.yml"><img src="https://github.com/opendecree/decree/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/opendecree/decree/releases"><img src="https://img.shields.io/github/v/release/opendecree/decree" alt="Release"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go" alt="Go"></a>
  <a href="https://goreportcard.com/report/github.com/opendecree/decree"><img src="https://goreportcard.com/badge/github.com/opendecree/decree" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"></a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/server_coverage-85%25-brightgreen" alt="Server Coverage">
  <img src="https://img.shields.io/badge/Go_SDK_coverage-94%25-brightgreen" alt="Go SDK Coverage">
  <img src="https://img.shields.io/badge/Python_SDK_coverage-98%25-brightgreen" alt="Python SDK Coverage">
</p>

<p align="center">Schema-driven business configuration management for multi-tenant services.</p>

> **Alpha Software** — OpenDecree is under active development. APIs, proto definitions, configuration formats, and behavior may change without notice between versions. Not recommended for production use yet.

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

| Language | Package | Install |
|----------|---------|---------|
| **Go** | 4 modules | `go get github.com/opendecree/decree/sdk/configclient@latest` |
| **Python** | [`opendecree`](https://pypi.org/project/opendecree/) | `pip install opendecree` |
| **TypeScript** | [`@opendecree/sdk`](https://www.npmjs.com/package/@opendecree/sdk) | `npm install @opendecree/sdk` |

### Go

```go
// configclient — application runtime reads and writes
client := configclient.New(rpc, configclient.WithSubject("myapp"))
val, _ := client.GetInt(ctx, tenantID, "payments.retries")

// configwatcher — live typed values with auto-reconnect
w := configwatcher.New(conn, tenantID)
fee := w.Float("payments.fee", 0.01)
w.Start(ctx)
fmt.Println(fee.Get()) // always fresh
```

```bash
go get github.com/opendecree/decree/sdk/configclient@latest
go get github.com/opendecree/decree/sdk/adminclient@latest
go get github.com/opendecree/decree/sdk/configwatcher@latest
go get github.com/opendecree/decree/sdk/tools@latest         # diff, docgen, validate, seed, dump
```

### Python

```python
from opendecree import ConfigClient

with ConfigClient("localhost:9090", subject="myapp") as client:
    retries = client.get("tenant-id", "payments.retries", int)

    with client.watch("tenant-id") as watcher:
        fee = watcher.field("payments.fee", float, default=0.01)
        print(fee.value)  # always fresh
```

Docs: [decree-python](https://github.com/opendecree/decree-python)

### TypeScript

```typescript
import { ConfigClient } from "@opendecree/sdk";

const client = new ConfigClient("localhost:9090", { subject: "myapp" });
const retries = await client.get("tenant-id", "payments.retries", Number);

const watcher = client.watch("tenant-id");
const fee = watcher.field("payments.fee", Number, { default: 0.01 });
await watcher.start();
console.log(fee.value); // always fresh
```

Docs: [decree-typescript](https://github.com/opendecree/decree-typescript)

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
decree seed fixtures/billing.yaml                 # bootstrap from a fixture
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
┌───────────────────┐
│      Clients      │
│  Go · Python · TS │
│  CLI · REST · gRPC│
└────────┬──────────┘
         │ gRPC / REST (grpc-gateway)
┌────────▼──────────────────────────┐
│           OpenDecree              │
│                                   │
│  SchemaService · ConfigService    │
│  AuditService  · VersionService   │
│                                   │
│  ┌─────────────────────────────┐  │
│  │    Pluggable Backends       │  │
│  │  Storage: Postgres | Memory │  │
│  │  Cache:   Redis    | Memory │  │
│  │  PubSub:  Redis    | Memory │  │
│  │  OTel:    opt-in             │  │
│  └─────────────────────────────┘  │
└───────────────────────────────────┘
```

Single binary exposing three gRPC services + REST/JSON gateway. All external dependencies (storage, cache, pub/sub) are behind Go interfaces — swap implementations via `STORAGE_BACKEND=memory` for zero-dependency evaluation or testing. Deploy with `ENABLE_SERVICES` to control which services run on each instance.

## Configuration

### Server

| Variable | Description | Default |
|----------|------------|---------|
| `GRPC_PORT` | gRPC listen port | `9090` |
| `HTTP_PORT` | REST/JSON gateway port (disabled if empty) | disabled |
| `STORAGE_BACKEND` | `postgres` or `memory` | `postgres` |
| `DB_WRITE_URL` | PostgreSQL primary connection string | required if postgres |
| `DB_READ_URL` | PostgreSQL read replica connection string | `DB_WRITE_URL` |
| `REDIS_URL` | Redis connection string | required if postgres |
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

The API is defined in Protocol Buffers under [`proto/`](proto/), published to BSR at [`buf.build/opendecree/decree`](https://buf.build/opendecree/decree). Three gRPC services:

- **SchemaService** — create, version, and manage config schemas and tenants
- **ConfigService** — read/write typed config values, subscribe to changes, version management
- **AuditService** — query change history and usage statistics

Values use a `TypedValue` oneof — integer, number, string, bool, timestamp, duration, url, json — with null support.

Generate client stubs in any language from BSR, or use the official SDKs:

| | Repo | Package |
|---|------|---------|
| Go | [this repo](sdk/) | `github.com/opendecree/decree/sdk/*` |
| Python | [decree-python](https://github.com/opendecree/decree-python) | [`opendecree`](https://pypi.org/project/opendecree/) |
| TypeScript | [decree-typescript](https://github.com/opendecree/decree-typescript) | [`@opendecree/sdk`](https://www.npmjs.com/package/@opendecree/sdk) |

## Test Coverage

Coverage badges reflect **business logic only** — infrastructure wrappers that are tested at the integration/e2e level are excluded from the calculation:

| Excluded | Reason |
|----------|--------|
| `store_pg.go` | PostgreSQL store implementations — thin DB wrappers, tested via e2e |
| `redis.go` | Redis cache/pubsub implementations — thin wrappers, tested via e2e |
| `storage/dbstore/` | sqlc-generated query code |
| `storage/postgres.go` | Interface definitions only |
| `telemetry/` | OpenTelemetry provider wiring boilerplate |

Run `./scripts/coverage.sh` to calculate the server coverage, or `./scripts/coverage.sh -v` for a per-function breakdown.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, build instructions, and contribution guidelines.

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
