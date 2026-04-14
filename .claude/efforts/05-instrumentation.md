# Instrumentation & OpenTelemetry

**Status:** Complete
**Started:** 2026-03-26

---

## Goal

Add configurable observability to the service using OpenTelemetry. Each instrumentation layer is independently toggleable via environment variables, from zero instrumentation (current state) to full traces + metrics.

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `OTEL_ENABLED` | `false` | Master switch — initializes SDK, exporter, and log correlation |
| `OTEL_TRACES_GRPC` | `false` | gRPC server interceptor spans (per-RPC traces) |
| `OTEL_TRACES_DB` | `false` | pgx query spans |
| `OTEL_TRACES_REDIS` | `false` | Redis command spans |
| `OTEL_METRICS` | `false` | Custom business metrics |

Standard OTel env vars (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`, etc.) are respected by the SDK automatically.

Log correlation (trace ID injected into slog output) is enabled whenever `OTEL_ENABLED=true` — no separate flag needed since the codebase already uses `slog.*Context()` everywhere.

## Instrumentation Layers

### 1. SDK + Exporter (`OTEL_ENABLED`)
- Initialize TracerProvider + MeterProvider with OTLP gRPC exporter
- Resource with service name, version
- Graceful shutdown on context cancellation
- slog log correlation: bridge or handler that extracts trace/span IDs from context

### 2. gRPC Traces (`OTEL_TRACES_GRPC`)
- `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`
- Chain OTel interceptors **before** auth in `server.New()` — OTel creates the root span, auth runs within it
- Currently `grpc.ChainUnaryInterceptor(auth)` → becomes `grpc.ChainUnaryInterceptor(otel, auth)`
- Captures: method, status code, duration

### 3. DB Traces (`OTEL_TRACES_DB`)
- `github.com/exaring/otelpgx`
- `storage.NewDB()` currently uses `pgxpool.New(ctx, dsn)` — switch to `pgxpool.NewWithConfig()` and add `otelpgx.NewTracer()` to the pgx config
- Captures: query, params (sanitized), duration
- Both write and read pools instrumented independently

### 4. Redis Traces (`OTEL_TRACES_REDIS`)
- `github.com/redis/go-redis/extra/redisotel/v9`
- Call `redisotel.InstrumentTracing(redisClient)` in main.go after client creation
- Single client shared by cache + pubsub — one instrumentation call covers all
- Captures: command, duration

### 5. Custom Metrics (`OTEL_METRICS`)
- Config read/write counters (per tenant)
- Cache hit/miss ratio
- Import/export counts
- Schema version publish events
- Instrument at the service layer, not storage layer

## Architecture

```
cmd/server/main.go
├── loadConfig()                — parse OTEL_* env vars into telemetry.Config
├── telemetry.Init(cfg)         — SDK setup, returns shutdown func
├── newLogger() or wrap logger  — if OTEL_ENABLED, wrap slog handler with trace correlation
├── storage.NewDB(ctx, w, r, opts...) — conditionally pass otelpgx tracer via functional option
├── redisotel.InstrumentTracing()     — conditionally instrument redis client
├── server.New(cfg)             — conditionally pass otelgrpc interceptors
└── service constructors        — pass meter for custom metrics (if enabled)

internal/telemetry/       — NEW package
├── config.go             — Config struct parsed from env vars
├── provider.go           — Init(Config) → (shutdown func, error); sets global TracerProvider + MeterProvider
├── slog.go               — NewHandler(inner slog.Handler) → slog.Handler with trace/span ID injection
└── metrics.go            — Custom metric definitions and recording helpers
```

## Key Decisions

- **No instrumentation by default** — zero overhead when flags are off
- **Standard OTel SDK** — uses `go.opentelemetry.io/otel` (vanilla, widely adopted)
- **OTLP exporter only** — no Jaeger/Zipkin direct exporters; use OTel Collector as a fan-out
- **Feature flags, not levels** — each layer independent for production tuning
- **Metrics at service layer** — business metrics live in service code, not storage

## Dependencies to Add

```
go.opentelemetry.io/otel
go.opentelemetry.io/otel/sdk
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc
go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
github.com/exaring/otelpgx (or similar pgx OTel integration)
github.com/redis/go-redis/extra/redisotel/v9
```

## Implementation Plan

### Phase 1: Foundation
- [ ] Create `internal/telemetry/` package — config, provider init, slog handler
- [ ] Wire into `cmd/server/main.go` — parse config, init/shutdown
- [ ] Add OTel Collector to `docker-compose.yml` (for local dev)

### Phase 2: Traces
- [ ] gRPC interceptor traces (`OTEL_TRACES_GRPC`)
- [ ] DB traces via pgx hook (`OTEL_TRACES_DB`)
- [ ] Redis traces via redisotel (`OTEL_TRACES_REDIS`)
- [ ] Log correlation — trace IDs in slog output

### Phase 3: Metrics (completed)
- [x] Granular metric flags: OTEL_METRICS_GRPC, OTEL_METRICS_DB_POOL, OTEL_METRICS_CACHE, OTEL_METRICS_CONFIG, OTEL_METRICS_SCHEMA
- [x] DB pool observable gauges (total, acquired, idle connections per pool)
- [x] Cache hit/miss counters
- [x] Config write counter (tenant_id, action labels) + version gauge
- [x] Schema publish counter
- [x] All metric types use nil-receiver pattern — zero overhead when disabled

### Phase 4: Verification
- [ ] E2E test: service emits traces to OTel Collector
- [ ] Verify each flag independently enables/disables its layer
- [ ] Verify zero overhead when `OTEL_ENABLED=false`

## Files to Create/Modify

| File | Action |
|------|--------|
| `internal/telemetry/config.go` | New — env var parsing |
| `internal/telemetry/provider.go` | New — SDK init + shutdown |
| `internal/telemetry/slog.go` | New — trace-correlated slog handler |
| `internal/telemetry/metrics.go` | New — metric definitions |
| `cmd/server/main.go` | Edit — wire telemetry init, conditional instrumentation |
| `internal/storage/postgres.go` | Edit — conditional pgx OTel tracer |
| `internal/server/server.go` | Edit — conditional otelgrpc interceptors |
| `internal/config/service.go` | Edit — record custom metrics |
| `internal/schema/service.go` | Edit — record custom metrics |
| `docker-compose.yml` | Edit — add OTel Collector |
| `go.mod` | Edit — add OTel deps |
