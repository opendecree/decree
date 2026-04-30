<p align="center">
  <img src="https://raw.githubusercontent.com/opendecree/decree/main/assets/logo.png" alt="OpenDecree" width="150">
</p>

<h1 align="center">OpenDecree</h1>

<p align="center">
  <a href="https://github.com/opendecree/decree/actions/workflows/ci.yml"><img src="https://github.com/opendecree/decree/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/opendecree/decree/releases"><img src="https://img.shields.io/github/v/release/opendecree/decree?include_prereleases" alt="Release"></a>
  <a href="https://pkg.go.dev/github.com/opendecree/decree"><img src="https://pkg.go.dev/badge/github.com/opendecree/decree.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/opendecree/decree"><img src="https://goreportcard.com/badge/github.com/opendecree/decree" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License"></a>
  <a href="https://www.repostatus.org/#wip"><img src="https://www.repostatus.org/badges/latest/wip.svg" alt="Project Status: WIP"></a>
</p>

<p align="center">Schema-driven business configuration management for multi-tenant services.</p>

> **Alpha Software** — OpenDecree is under active development. APIs, proto definitions, configuration formats, and behavior may change without notice between versions. Not recommended for production use yet.

<p align="center">
  <img src="https://raw.githubusercontent.com/opendecree/decree/main/assets/demo.gif" alt="OpenDecree quickstart demo: schema → config → watch" width="800">
</p>

## What is this?

OpenDecree manages **business-oriented configuration** — approval rules, fee structures, settlement windows, feature parameters — the kind of config that lives between your infrastructure settings and your application code.

### How is this different?

OpenDecree sits between feature flags (release toggles) and infrastructure config (low-level key-value) — purpose-built for **typed business configuration**.

| Capability | Feature Flags | Infrastructure Config\* | **OpenDecree** |
|---|---|---|---|
| Typed values | bool / variant only | strings only | ✓ native types (int, number, string, bool, time, duration, url, json) |
| Schema validation | ✗ | ✗ | ✓ constraints + JSON Schema, enforced on every write |
| Multi-tenant | limited (segments) | ✗ | ✓ first-class, with field-level locking |
| Audit trail | limited | ✗ | ✓ full who/what/when/why history |
| Real-time updates | ✓ SDK polling/SSE | ✓ watch APIs | ✓ gRPC streaming subscriptions |
| Admin UI | ✓ (vendor-hosted) | ✗ | ✓ open-source admin GUI |
| Versioning + rollback | limited | ✗ | ✓ every change versioned, rollback to any state |

Examples by category: feature flags (LaunchDarkly, ConfigCat, Flagsmith), infrastructure config (etcd, Consul, Spring Cloud Config, AWS AppConfig, Azure App Configuration).

\* Cloud config services (AWS AppConfig, Azure App Configuration) offer limited validation, but lack schema registries, multi-tenancy, and gRPC streaming — and are vendor-locked.

**What makes OpenDecree unique:** no other open-source tool combines schema-first typed configuration with native multi-tenancy, field-level locking, gRPC streaming, and versioned rollback in a single Go binary.

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

| SDK | Language | Protocol | Status | Version | Install |
|-----|----------|----------|--------|---------|---------|
| [decree](sdk/) | ![Go](https://img.shields.io/badge/-Go-00ADD8?logo=go&logoColor=white) | ![gRPC](https://img.shields.io/badge/-gRPC-244c5a?logo=grpc&logoColor=white) | ![alpha](https://img.shields.io/badge/status-alpha-orange) | [![Go release](https://img.shields.io/github/v/release/opendecree/decree?include_prereleases&label=)](https://github.com/opendecree/decree/releases) | `go get github.com/opendecree/decree/sdk/grpctransport@latest` |
| [decree-python](https://github.com/opendecree/decree-python) | ![Python](https://img.shields.io/badge/-Python-3776AB?logo=python&logoColor=white) | ![gRPC](https://img.shields.io/badge/-gRPC-244c5a?logo=grpc&logoColor=white) | ![alpha](https://img.shields.io/badge/status-alpha-orange) | [![PyPI](https://img.shields.io/pypi/v/opendecree?label=)](https://pypi.org/project/opendecree/) | `pip install opendecree` |
| [decree-typescript](https://github.com/opendecree/decree-typescript) | ![TypeScript](https://img.shields.io/badge/-TypeScript-3178C6?logo=typescript&logoColor=white) | ![gRPC](https://img.shields.io/badge/-gRPC-244c5a?logo=grpc&logoColor=white) | ![alpha](https://img.shields.io/badge/status-alpha-orange) | [![npm](https://img.shields.io/npm/v/@opendecree/sdk?label=)](https://www.npmjs.com/package/@opendecree/sdk) | `npm install @opendecree/sdk` |
| [REST API](#rest-api) | Any | ![REST](https://img.shields.io/badge/-REST%2FJSON-0a7ea4?logo=openapiinitiative&logoColor=white) | ![alpha](https://img.shields.io/badge/status-alpha-orange) | — | `curl` |

### Go

The core SDK modules require **Go 1.22+** and have zero external dependencies. The `grpctransport` module (default transport) requires Go 1.24+ because `google.golang.org/grpc` pins that version — if you need a Go 1.22-compatible transport, 👍 or comment on [#90](https://github.com/opendecree/decree/issues/90) to signal demand.

```go
// configclient — application runtime reads and writes
client := grpctransport.NewConfigClient(conn, grpctransport.WithSubject("myapp"))
val, _ := client.GetInt(ctx, tenantID, "payments.retries")

// configwatcher — live typed values with auto-reconnect
w := grpctransport.NewWatcher(conn, tenantID, grpctransport.WithSubject("myapp"))
fee := w.Float("payments.fee", 0.01)
w.Start(ctx)
fmt.Println(fee.Get()) // always fresh
```

```bash
go get github.com/opendecree/decree/sdk/grpctransport@latest  # gRPC transport (pulls in configclient, etc.)
go get github.com/opendecree/decree/sdk/configclient@latest   # core client (no gRPC dependency)
go get github.com/opendecree/decree/sdk/adminclient@latest    # admin operations
go get github.com/opendecree/decree/sdk/configwatcher@latest  # live config watcher
go get github.com/opendecree/decree/sdk/tools@latest          # diff, docgen, validate, seed, dump
```

Go API docs on pkg.go.dev:
[grpctransport](https://pkg.go.dev/github.com/opendecree/decree/sdk/grpctransport) ·
[configclient](https://pkg.go.dev/github.com/opendecree/decree/sdk/configclient) ·
[adminclient](https://pkg.go.dev/github.com/opendecree/decree/sdk/adminclient) ·
[configwatcher](https://pkg.go.dev/github.com/opendecree/decree/sdk/configwatcher) ·
[tools](https://pkg.go.dev/github.com/opendecree/decree/sdk/tools)

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

### Examples

Runnable examples in [`examples/`](examples/) — each is a standalone Go module you can copy into your own project.

| Example | What it shows |
|---------|--------------|
| [quickstart](examples/quickstart/) | Connect and read typed values |
| [feature-flags](examples/feature-flags/) | Live feature toggles with configwatcher |
| [live-config](examples/live-config/) | HTTP server with hot-reloadable config |
| [multi-tenant](examples/multi-tenant/) | Same schema, different tenant values |
| [optimistic-concurrency](examples/optimistic-concurrency/) | Safe concurrent updates with CAS |
| [schema-lifecycle](examples/schema-lifecycle/) | Create, publish, and manage schemas |
| [environment-bootstrap](examples/environment-bootstrap/) | Bootstrap from a single YAML file |
| [config-validation](examples/config-validation/) | Offline validation (no server needed) |

```bash
cd examples && make setup   # start server + seed data
cd quickstart && go run .   # run any example
```

## CLI

```bash
go install github.com/opendecree/decree/cmd/decree@latest

decree schema list
decree schema import --publish decree.schema.yaml  # import + auto-publish

decree tenant create --name acme --schema payroll-service --schema-version 1
decree config set acme payments.fee 0.5%          # use tenant name or UUID
decree config get-all acme
decree config versions acme
decree config rollback acme 2

decree watch acme                                  # live stream
decree lock set acme payments.currency             # lock field
decree audit query --tenant acme --since 24h

# Power tools
decree seed fixtures/billing.yaml                 # bootstrap from a fixture
decree dump acme > backup.yaml                    # full tenant backup
decree diff acme 1 2                              # diff two config versions
decree diff --old v1.yaml --new v2.yaml           # diff two files
decree docgen payroll-service                      # use schema name or UUID
decree validate --schema decree.schema.yaml --config decree.config.yaml
```

Global flags: `--server`, `--subject`, `--role`, `--output table|json|yaml`, `--wait`, `--wait-timeout`

Use `--wait` in Docker/Kubernetes init containers to wait for the server to be ready:

```bash
decree seed examples/seed.yaml --server decree:9090 --wait --wait-timeout 60s
```

## Quick Start

### Try it instantly (no Docker needed)

```bash
go install github.com/opendecree/decree/cmd/server@latest

# Start with in-memory storage — zero dependencies
INSECURE_LISTEN=1 STORAGE_BACKEND=memory HTTP_PORT=8080 decree-server

# Open http://localhost:8080/docs for Swagger UI
# All requests need x-subject header:
curl -H "x-subject: admin" http://localhost:8080/v1/schemas
```

> **TLS required in production.** `INSECURE_LISTEN=1` disables TLS for local development only. See [Transport Security (TLS)](docs/server/configuration.md#transport-security-tls) for production setup.

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
decree schema import --publish examples/config-validation/decree.schema.yaml

# Create a tenant and set config
decree tenant create --name acme --schema <schema-id> --schema-version 1
decree config set <tenant-id> payments.fee "0.5%"
decree config get-all <tenant-id>
```

## Architecture

```mermaid
flowchart LR
    subgraph Clients
        direction TB
        SDKs["🔧 SDKs\nGo · Python · TypeScript"]
        CLI[">_ CLI\ndecree"]
        UI["🖥️ Admin UI\ndecree-ui"]
        Direct["🔗 Direct\ncurl, custom"]
    end

    SDKs & CLI -->|gRPC| GW
    UI & Direct -->|REST| GW

    GW{{"🔐 Gateway\nauth · routing"}}

    subgraph Server["⚙️ OpenDecree"]
        SS["📋 SchemaService\nschemas, tenants"]
        CS["📝 ConfigService\nread, write, subscribe"]
        AS["📊 AuditService\nhistory, usage"]
    end

    GW --> SS & CS & AS

    subgraph Backends["💾 Pluggable Backends"]
        Storage[("Storage")]
        Cache[("Cache")]
        PubSub[("Pub/Sub")]
    end

    SS & CS & AS --> Storage
    CS --> Cache
    CS <--> PubSub
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
| `INSECURE_LISTEN` | Set to `1` to accept plaintext gRPC (local dev only) | disabled |
| `TLS_CERT_FILE` | Path to PEM server certificate | required unless `INSECURE_LISTEN=1` |
| `TLS_KEY_FILE` | Path to PEM server private key | required unless `INSECURE_LISTEN=1` |
| `TLS_CLIENT_CA_FILE` | CA bundle for client certificates (enables mTLS) | disabled |
| `RATE_LIMIT_ENABLED` | Set to `false` to disable rate limiting | `true` |
| `RATE_LIMIT_ANON_RPS` | Requests per second for unauthenticated callers | `10` |
| `RATE_LIMIT_AUTHED_RPS` | Requests per second per authenticated tenant | `100` |
| `RATE_LIMIT_SUPERADMIN_RPS` | Requests per second per superadmin (`0` = unlimited) | `0` |
| `RATE_LIMIT_BURST` | Token bucket burst size (all role classes) | `10` |

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

The schema YAML format authors write to define their config shape is documented in [Schema Format](docs/concepts/schema-format.md). The corresponding [JSON Schema 2020-12 meta-schemas](docs/concepts/meta-schema.md) for editor IntelliSense and CI validation are published at:

- `https://schemas.opendecree.dev/schema/v0.1.0/decree-schema.json` — validates `*.decree.schema.yaml`
- `https://schemas.opendecree.dev/schema/v0.1.0/decree-config.json` — validates `*.decree.config.yaml`

All endpoints that accept a tenant or schema ID also accept the **name slug** — the server resolves automatically. Use UUIDs or human-readable names interchangeably:

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

## Demos

Want to see OpenDecree in action without reading docs? The [demos repo](https://github.com/opendecree/demos) has self-contained, runnable examples — from a 5-minute quickstart to production patterns. Each demo runs with a single `docker compose up`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, build instructions, and contribution guidelines.

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
