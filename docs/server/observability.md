# Observability

OpenDecree integrates with OpenTelemetry for distributed tracing, metrics, and log correlation. All observability features are opt-in -- nothing is enabled by default.

## Quick Setup

Enable everything with Docker Compose:

```bash
# Start the service with OTel Collector and Jaeger
docker compose up -d --wait service otel-collector jaeger
```

Then add OTel environment variables to the service. In `docker-compose.yml`, add to the `service` container's `environment`:

```yaml
environment:
  # ... existing vars ...
  OTEL_ENABLED: "true"
  OTEL_TRACES_GRPC: "true"
  OTEL_TRACES_DB: "true"
  OTEL_METRICS_GRPC: "true"
  OTEL_METRICS_CONFIG: "true"
  OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector:4317"
```

View traces at [http://localhost:16686](http://localhost:16686) (Jaeger UI).

## The Master Switch

`OTEL_ENABLED` is the master switch. When set to `true` or `1`, it:

1. Initializes the OpenTelemetry SDK with an OTLP gRPC exporter
2. Creates a `TracerProvider` and `MeterProvider`
3. Wraps the slog JSON handler to inject `trace_id` and `span_id` into every log entry

When `OTEL_ENABLED` is `false` (the default), all other OTel flags are ignored and no telemetry overhead is added.

## Traces

Traces show the full lifecycle of a request across gRPC, database, and Redis.

### gRPC Traces (`OTEL_TRACES_GRPC`)

Creates a span for every incoming gRPC call with:

- RPC method name (e.g., `centralconfig.v1.ConfigService/GetField`)
- gRPC status code
- Duration

This is the top-level span that all other spans nest under.

### Database Traces (`OTEL_TRACES_DB`)

Creates a span for every PostgreSQL query and transaction via pgx instrumentation:

- SQL statement (parameterized)
- Database name
- Duration

Useful for identifying slow queries and understanding how many database calls each RPC makes.

### Redis Traces (`OTEL_TRACES_REDIS`)

Creates a span for every Redis command:

- Command name (GET, SET, PUBLISH, etc.)
- Duration

Shows cache hits/misses and pub/sub publish latency.

### Example Trace

A typical `GetField` trace looks like:

```
[gRPC] ConfigService/GetField (2.1ms)
  └── [Redis] GET config:tenant:field (0.3ms)     ← cache hit
```

A cache miss adds a database span:

```
[gRPC] ConfigService/GetField (5.4ms)
  ├── [Redis] GET config:tenant:field (0.2ms)      ← cache miss
  ├── [PostgreSQL] SELECT ... FROM config_values (3.1ms)
  └── [Redis] SET config:tenant:field (0.4ms)      ← populate cache
```

## Metrics

Metrics provide aggregate counters and gauges for monitoring dashboards and alerting.

### gRPC Metrics (`OTEL_METRICS_GRPC`)

Standard otelgrpc metrics:

- `rpc.server.duration` -- request latency histogram by method and status
- `rpc.server.request.size` -- request message sizes
- `rpc.server.response.size` -- response message sizes
- `rpc.server.requests_per_rpc` -- request count by method

### Database Pool Metrics (`OTEL_METRICS_DB_POOL`)

Connection pool gauges (reported for both write and read pools):

- Total connections
- Acquired (in-use) connections
- Idle connections
- Max connections

Useful for alerting on pool exhaustion.

### Cache Metrics (`OTEL_METRICS_CACHE`)

- `config.cache.hits` -- counter of cache hits
- `ccs.cache.miss` -- counter of cache misses

Monitor your cache hit ratio to tune TTL and capacity.

### Config Metrics (`OTEL_METRICS_CONFIG`)

- `ccs.config.writes` -- counter of config write operations (by tenant)
- `ccs.config.version` -- gauge of the current config version (by tenant)

### Schema Metrics (`OTEL_METRICS_SCHEMA`)

- `ccs.schema.publishes` -- counter of schema publish operations

## Log Correlation

When `OTEL_ENABLED` is true, OpenDecree wraps the slog JSON handler to inject trace context into every log entry:

```json
{
  "time": "2025-06-15T10:30:00Z",
  "level": "INFO",
  "msg": "field updated",
  "tenant_id": "abc-123",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7"
}
```

This lets you correlate logs with traces in your observability backend -- click a trace in Jaeger and find the corresponding log entries, or search logs by trace ID.

## OTel Collector

The Docker Compose stack includes an [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) that receives telemetry from OpenDecree and exports it to backends. The collector config lives at `deploy/otel-collector.yaml`.

The default setup exports traces to Jaeger. To export to other backends (Grafana Tempo, Datadog, etc.), modify the collector config.

### Standard OTel Environment Variables

OpenDecree respects standard OpenTelemetry SDK variables:

| Variable | Default |
|----------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4317` |
| `OTEL_SERVICE_NAME` | `decree` |
| `OTEL_RESOURCE_ATTRIBUTES` | -- |

## Recommended Production Setup

Start with these flags and expand as needed:

```bash
OTEL_ENABLED=true
OTEL_TRACES_GRPC=true        # request-level visibility
OTEL_METRICS_GRPC=true        # latency and error rate
OTEL_METRICS_CONFIG=true      # config activity
OTEL_METRICS_DB_POOL=true     # pool health
OTEL_METRICS_CACHE=true       # cache effectiveness
```

Add `OTEL_TRACES_DB` and `OTEL_TRACES_REDIS` when debugging performance -- they add span overhead per query/command, which may not be needed for routine monitoring.

## Related

- [Server Configuration](configuration.md) -- all OTel environment variables
- [Deployment](deployment.md) -- Docker Compose with OTel stack
