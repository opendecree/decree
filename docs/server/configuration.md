# Server Configuration

OpenDecree is configured entirely through environment variables. No config files needed.

## Server

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `GRPC_PORT` | Port the gRPC server listens on. | `9090` | No |
| `HTTP_PORT` | Port for the REST/JSON gateway + Swagger UI. Empty = disabled. | -- | No |
| `STORAGE_BACKEND` | Storage backend: `postgres` (default) or `memory` (no external deps, data not persisted). | `postgres` | No |
| `DB_WRITE_URL` | PostgreSQL connection string for the primary (read-write) database. Format: `postgres://user:pass@host:5432/dbname?sslmode=disable` | -- | Yes (postgres mode) |
| `DB_READ_URL` | PostgreSQL connection string for the read replica. Used for all read queries. Falls back to `DB_WRITE_URL` if not set. | `DB_WRITE_URL` | No |
| `REDIS_URL` | Redis connection string. Used for config caching and real-time change propagation (pub/sub). Format: `redis://host:6379` | -- | Yes (postgres mode) |
| `ENABLE_SERVICES` | Comma-separated list of services to enable. Valid values: `schema`, `config`, `audit`. | `schema,config,audit` | No |
| `LOG_LEVEL` | Log verbosity. One of: `debug`, `info`, `warn`, `error`. Logs are JSON-formatted to stdout. | `info` | No |
| `USAGE_TRACKING_ENABLED` | Enable automatic recording of config field reads (`GetField`, `GetConfig`, `GetFields`). Set to `false` to disable. | `true` | No |
| `USAGE_FLUSH_INTERVAL` | How often accumulated read counts are flushed to storage. Format: Go duration (e.g., `30s`, `1m`). | `30s` | No |
| `GRPC_MAX_RECV_MSG_BYTES` | Maximum size of an inbound gRPC message, in bytes. Requests above this return `ResourceExhausted`. Set to `0` for the default. | `20971520` (20 MiB) | No |
| `GRPC_MAX_SEND_MSG_BYTES` | Maximum size of an outbound gRPC message, in bytes. Responses above this return `ResourceExhausted` to the client. Set to `0` for the default. | `20971520` (20 MiB) | No |
| `SCHEMA_MAX_FIELDS` | Maximum number of fields per schema accepted by `CreateSchema` and `ImportSchema`. Requests above this return `InvalidArgument`. Set to `0` to disable. | `10000` | No |
| `SCHEMA_MAX_DOC_BYTES` | Maximum serialized YAML document size accepted by `ImportSchema`, in bytes. Requests above this return `InvalidArgument`. Set to `0` to disable. | `5242880` (5 MiB) | No |
| `SCHEMA_COMPILE_TIMEOUT` | Wall-clock cap on a single JSON-Schema compile (per-field constraint). Format: Go duration (e.g., `5s`, `2s`). Set to `0` to disable the timeout. | `5s` | No |
| `SCHEMA_MAX_REF_DEPTH` | Maximum structural nesting depth of a JSON-Schema constraint document. Schemas deeper than this are rejected before compilation. Set to `0` to disable. | `64` | No |

## gRPC Server Options

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `ENABLE_REFLECTION` | Set to `1` to enable gRPC server reflection. Allows tools like `grpcurl` to introspect the server's service schema. **Never enable in production** — reflection exposes the full API to any caller regardless of auth. | disabled | No |

## Transport Security (TLS)

TLS is **required by default** for the gRPC server and the gateway-to-gRPC dial. Set `INSECURE_LISTEN=1` to opt out for local dev only.

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `INSECURE_LISTEN` | Set to `1` to listen in plaintext and have the gateway dial gRPC in plaintext. **Local dev only.** | -- | No |
| `TLS_CERT_FILE` | Path to the server's PEM-encoded TLS certificate. | -- | Yes (unless `INSECURE_LISTEN=1`) |
| `TLS_KEY_FILE` | Path to the server's PEM-encoded private key. | -- | Yes (unless `INSECURE_LISTEN=1`) |
| `TLS_CLIENT_CA_FILE` | Path to PEM CA bundle that signs allowed client certificates. When set, the server requires and verifies client certificates (mTLS). | -- | No |
| `TLS_GATEWAY_CA_FILE` | CA bundle the gateway uses to verify the upstream gRPC server's certificate. When unset, the system root pool is used. | system roots | No |
| `TLS_GATEWAY_SERVER_NAME` | SNI / verification hostname the gateway expects on the upstream gRPC certificate. | dial address host | No |
| `TLS_GATEWAY_CLIENT_CERT_FILE` | Client certificate the gateway presents to the upstream gRPC server (when the server requires mTLS). | -- | No |
| `TLS_GATEWAY_CLIENT_KEY_FILE` | Private key for `TLS_GATEWAY_CLIENT_CERT_FILE`. Must be set together with the cert file. | -- | No |

Example (TLS, no mTLS):

```bash
TLS_CERT_FILE=/etc/decree/tls/server.crt
TLS_KEY_FILE=/etc/decree/tls/server.key
TLS_GATEWAY_CA_FILE=/etc/decree/tls/server.crt   # self-signed: trust the server cert
TLS_GATEWAY_SERVER_NAME=localhost
```

Example (mTLS):

```bash
TLS_CERT_FILE=/etc/decree/tls/server.crt
TLS_KEY_FILE=/etc/decree/tls/server.key
TLS_CLIENT_CA_FILE=/etc/decree/tls/clients-ca.crt
TLS_GATEWAY_CLIENT_CERT_FILE=/etc/decree/tls/gateway.crt
TLS_GATEWAY_CLIENT_KEY_FILE=/etc/decree/tls/gateway.key
```

### Rate Limiting

Rate limiting is **enabled by default** using an in-process token-bucket limiter (per-tenant + per-method). The health check endpoint (`/grpc.health.v1.Health/*`) is always exempt.

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `RATE_LIMIT_ENABLED` | Set to `false` to disable rate limiting entirely. | `true` | No |
| `RATE_LIMIT_ANON_RPS` | Requests per second for unauthenticated callers (shared global bucket per method). | `10` | No |
| `RATE_LIMIT_AUTHED_RPS` | Requests per second per tenant for authenticated callers. | `100` | No |
| `RATE_LIMIT_SUPERADMIN_RPS` | Requests per second per superadmin identity. `0` = unlimited. | `0` | No |
| `RATE_LIMIT_BURST` | Token bucket burst size (applies to all role classes). | `10` | No |

Rejected requests return `codes.ResourceExhausted` with a `RetryInfo` detail (1 s retry hint). The in-process limiter does not share state across replicas; for multi-replica deployments, implement the `Limiter` interface backed by Redis.

### In-Memory Mode

Set `STORAGE_BACKEND=memory` to run without PostgreSQL or Redis. All data is stored in memory and lost on restart. Useful for evaluation, local development, and testing:

```bash
INSECURE_LISTEN=1 STORAGE_BACKEND=memory HTTP_PORT=8080 decree-server
```

### REST/JSON Gateway

Set `HTTP_PORT` to enable the REST API alongside gRPC. The gateway translates HTTP/JSON requests to gRPC automatically. Swagger UI is available at `/docs`:

```bash
# Enable REST gateway on port 8080
HTTP_PORT=8080 decree-server

# Access Swagger UI
open http://localhost:8080/docs
```

### Split Read/Write Database

Setting `DB_READ_URL` to a read replica offloads read queries from the primary. This is useful in read-heavy deployments where config reads vastly outnumber writes.

### Selective Service Enablement

Use `ENABLE_SERVICES` to run different services on different instances:

```bash
# Config-only instance (high read traffic)
ENABLE_SERVICES=config

# Schema + audit instance (admin operations)
ENABLE_SERVICES=schema,audit
```

Each instance must have access to the same PostgreSQL database and Redis instance.

## Authentication

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `JWT_JWKS_URL` | JWKS endpoint URL for JWT validation. Setting this enables JWT auth mode. When unset, the server uses metadata-based auth. | -- | No |
| `JWT_ISSUER` | Expected JWT `iss` claim. When set, tokens with a different issuer are rejected. | -- | No |

When `JWT_JWKS_URL` is not set, the server operates in **metadata auth mode** — identity is passed via gRPC metadata headers (`x-subject`, `x-role`, `x-tenant-id`). See [Auth](../concepts/auth.md) for details on both modes.

## Observability (OpenTelemetry)

All observability flags are opt-in. Set to `true` or `1` to enable.

| Variable | Description | Default |
|----------|-------------|---------|
| `OTEL_ENABLED` | Master switch. Initializes the OTel SDK, OTLP exporter, and enables slog trace correlation (adds `trace_id` and `span_id` to log entries). Required for any other OTel flag to take effect. | `false` |

### Trace Flags

| Variable | What it traces |
|----------|---------------|
| `OTEL_TRACES_GRPC` | gRPC server spans — one span per RPC call with method, status code, and duration. |
| `OTEL_TRACES_DB` | PostgreSQL query spans — one span per query/transaction via pgx instrumentation. |
| `OTEL_TRACES_REDIS` | Redis command spans — one span per Redis command. |

### Metric Flags

| Variable | What it measures |
|----------|-----------------|
| `OTEL_METRICS_GRPC` | gRPC request count, latency histograms, and message sizes (via otelgrpc). |
| `OTEL_METRICS_DB_POOL` | Database connection pool gauges: total, acquired, idle, and max connections. |
| `OTEL_METRICS_CACHE` | Cache hit/miss counters for config value reads. |
| `OTEL_METRICS_CONFIG` | Config write counter and current version gauge per tenant. |
| `OTEL_METRICS_SCHEMA` | Schema publish counter. |

### Standard OTel Variables

OpenDecree respects standard OpenTelemetry SDK environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP exporter endpoint. | `http://localhost:4317` |
| `OTEL_SERVICE_NAME` | Service name reported in traces and metrics. | `decree` |
| `OTEL_RESOURCE_ATTRIBUTES` | Additional resource attributes (e.g., `deployment.environment=prod`). | -- |

See [Observability](observability.md) for setup instructions and trace viewing.

## Example: Minimal Production Config

```bash
GRPC_PORT=9090
HTTP_PORT=8080
DB_WRITE_URL=postgres://decree:secret@db-primary:5432/centralconfig?sslmode=require
DB_READ_URL=postgres://decree:secret@db-replica:5432/centralconfig?sslmode=require
REDIS_URL=redis://redis:6379
JWT_JWKS_URL=https://auth.example.com/.well-known/jwks.json
JWT_ISSUER=https://auth.example.com
LOG_LEVEL=info
OTEL_ENABLED=true
OTEL_TRACES_GRPC=true
OTEL_METRICS_GRPC=true
OTEL_METRICS_CONFIG=true
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
```

## Related

- [Auth](../concepts/auth.md) — auth modes and role system
- [Deployment](deployment.md) — Docker Compose, Helm, and Kubernetes setup
- [Observability](observability.md) — OTel setup and trace viewing
