# SDK Instrumentation (OpenTelemetry)

**Status:** Planning
**Started:** 2026-04-12

---

## Goal

Allow users to see SDK operations (get, set, watch) in their application traces. Opt-in — users who don't use OTel pay no dependency cost.

## Scope

### Python SDK (`decree-python`)

Add OTel as an optional dependency with a simple opt-in flag.

```python
# pip install opendecree[otel]
client = ConfigClient("localhost:9090", subject="myapp", otel=True)
```

- Optional extra: `opentelemetry-instrumentation-grpc` in pyproject.toml
- Wire gRPC OTel interceptor when `otel=True` in constructor
- Works for both sync and async clients
- Watcher streams get traced too

### Go SDKs (`decree`)

Docs-only — users already pass their own `grpc.ClientConn`, so they can add OTel interceptors themselves.

```go
import "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

conn, _ := grpc.NewClient(target,
    grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
    grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
)
client := configclient.New(conn, ...)
```

- Add usage examples to Go SDK docs / README
- No code changes needed in the Go SDKs

## What users get

| Signal | What |
|--------|------|
| Traces | Spans for get, set, set_many, watch subscribe — appear in app traces |
| Metrics | gRPC call latency, error rates (from the interceptor) |
| Logs | Trace ID correlation in structured logs |

## Implementation

### Python (medium effort)

- [ ] Add `otel` optional extra to pyproject.toml
- [ ] Add `otel: bool = False` parameter to ConfigClient and AsyncConfigClient
- [ ] Wire `opentelemetry-instrumentation-grpc` interceptor when enabled
- [ ] Test with OTel disabled (default) and enabled
- [ ] Add `docs/instrumentation.md` with setup guide
- [ ] Update README with OTel example

### Go (low effort — docs only)

- [ ] Add OTel interceptor example to configclient README
- [ ] Add OTel interceptor example to configwatcher README
- [ ] Add OTel section to CONTRIBUTING.md or main docs

## Key Decisions

1. **Opt-in only** — OTel is not a runtime dependency unless `opendecree[otel]` is installed
2. **gRPC interceptors** — official packages for both languages, no custom instrumentation needed
3. **Go needs no code changes** — connection is user-owned, interceptors are user-added
4. **Python wires it in constructor** — simpler API than asking users to configure channels
