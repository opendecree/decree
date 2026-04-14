# Server Configuration

CCS is configured entirely through environment variables. No config files needed.

## Server

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `GRPC_PORT` | Port the gRPC server listens on. | `9090` | No |
| `DB_WRITE_URL` | PostgreSQL connection string for the primary (read-write) database. Format: `postgres://user:pass@host:5432/dbname?sslmode=disable` | -- | Yes |
| `DB_READ_URL` | PostgreSQL connection string for the read replica. Used for all read queries (GetField, GetAllFields, list operations). Falls back to `DB_WRITE_URL` if not set. | `DB_WRITE_URL` | No |
| `REDIS_URL` | Redis connection string. Used for config caching and real-time change propagation (pub/sub). Format: `redis://host:6379` or `redis://host:6379/0?password=secret` | -- | Yes |
| `ENABLE_SERVICES` | Comma-separated list of services to enable. Valid values: `schema`, `config`, `audit`. Allows deploying separate instances for different workloads. | `schema,config,audit` | No |
| `LOG_LEVEL` | Log verbosity. One of: `debug`, `info`, `warn`, `error`. Logs are JSON-formatted to stdout. | `info` | No |

### Split Read/Write Database

Setting `DB_READ_URL` to a read replica offloads read queries from the primary. This is useful in read-heavy deployments where config reads vastly outnumber writes. The write URL is used for all mutations (SetField, SetFields, rollback, schema changes).

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

When `JWT_JWKS_URL` is not set, the server operates in **metadata auth mode** -- identity is passed via gRPC metadata headers (`x-subject`, `x-role`, `x-tenant-id`). See [Auth](../concepts/auth.md) for details on both modes.

## Observability (OpenTelemetry)

All observability flags are opt-in. Set to `true` or `1` to enable.

| Variable | Description | Default |
|----------|-------------|---------|
| `OTEL_ENABLED` | Master switch. Initializes the OTel SDK, OTLP exporter, and enables slog trace correlation (adds `trace_id` and `span_id` to log entries). Required for any other OTel flag to take effect. | `false` |

### Trace Flags

| Variable | What it traces |
|----------|---------------|
| `OTEL_TRACES_GRPC` | gRPC server spans -- one span per RPC call with method, status code, and duration. |
| `OTEL_TRACES_DB` | PostgreSQL query spans -- one span per query/transaction via pgx instrumentation. |
| `OTEL_TRACES_REDIS` | Redis command spans -- one span per Redis command. |

### Metric Flags

| Variable | What it measures |
|----------|-----------------|
| `OTEL_METRICS_GRPC` | gRPC request count, latency histograms, and message sizes (via otelgrpc). |
| `OTEL_METRICS_DB_POOL` | Database connection pool gauges: total, acquired, idle, and max connections. |
| `OTEL_METRICS_CACHE` | Cache hit/miss counters for config value reads. |
| `OTEL_METRICS_CONFIG` | Config write counter and current version gauge per tenant. |
| `OTEL_METRICS_SCHEMA` | Schema publish counter. |

### Standard OTel Variables

CCS respects standard OpenTelemetry SDK environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP exporter endpoint. | `http://localhost:4317` |
| `OTEL_SERVICE_NAME` | Service name reported in traces and metrics. | `decree` |
| `OTEL_RESOURCE_ATTRIBUTES` | Additional resource attributes (e.g., `deployment.environment=prod`). | -- |

See [Observability](observability.md) for setup instructions and trace viewing.

## Example: Minimal Production Config

```bash
GRPC_PORT=9090
DB_WRITE_URL=postgres://ccs:secret@db-primary:5432/centralconfig?sslmode=require
DB_READ_URL=postgres://ccs:secret@db-replica:5432/centralconfig?sslmode=require
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

- [Auth](../concepts/auth.md) -- auth modes and role system
- [Deployment](deployment.md) -- Docker Compose and Kubernetes setup
- [Observability](observability.md) -- OTel setup and trace viewing
