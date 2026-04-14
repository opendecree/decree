# SDK Instrumentation — Design Context

## Goal

Allow users to see SDK operations (get, set, watch) in their application traces. Opt-in — users who don't use OTel pay no dependency cost.

## Scope

### Python SDK (`decree-python`)

Optional extra: `pip install opendecree[otel]`. Constructor flag `otel=True` wires gRPC OTel interceptor. Works for sync, async, and watcher streams.

### Go SDKs (`decree`)

Docs-only — users pass their own `grpc.ClientConn`, add OTel interceptors themselves:
```go
conn, _ := grpc.NewClient(target,
    grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
    grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()),
)
```

## What users get

| Signal | What |
|--------|------|
| Traces | Spans for get, set, set_many, watch |
| Metrics | gRPC call latency, error rates |
| Logs | Trace ID correlation |

## Key Decisions

1. **Opt-in only** — OTel is not a runtime dependency unless explicitly installed
2. **gRPC interceptors** — official packages, no custom instrumentation
3. **Go needs no code changes** — connection is user-owned
4. **Python wires in constructor** — simpler than asking users to configure channels
