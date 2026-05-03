# Migration Guide

OpenDecree is **alpha** — the API is unstable and breaking changes ship without backward-compatibility shims. This document records the breaks so SDK consumers and embedders know what to update when bumping versions.

## v0.10.0-alpha.2 — functional options for service constructors

PRs [#235], [#236], [#249], [#254] replaced the struct-config pattern across every public constructor with the `With...()` functional-options pattern already used in `sdk/configclient`, `sdk/configwatcher`, `sdk/grpctransport`, and `internal/storage`. Required dependencies stay positional; only optional knobs become options.

The old `Config` / `ServiceConfig` / `RecorderConfig` / `GatewayConfig` structs are **removed** — there is no shim. Call sites must be rewritten.

[#235]: https://github.com/opendecree/decree/pull/235
[#236]: https://github.com/opendecree/decree/pull/236
[#249]: https://github.com/opendecree/decree/pull/249
[#254]: https://github.com/opendecree/decree/pull/254

### `internal/server`

```go
// Before
srv := server.New(server.Config{
    GRPCPort:           50051,
    Auth:               authInterceptor,
    Logger:             logger,
    EnableServices:     []string{"schema", "config"},
    GRPCServerOptions:  grpcOpts,
    MaxRecvMsgBytes:    8 << 20,
    MaxSendMsgBytes:    8 << 20,
    TLS:                tlsConfig,
})

// After
srv := server.New(50051, authInterceptor,
    server.WithLogger(logger),
    server.WithEnableServices("schema", "config"),
    server.WithGRPCServerOptions(grpcOpts...),
    server.WithMaxRecvMsgBytes(8<<20),
    server.WithMaxSendMsgBytes(8<<20),
    server.WithTLS(tlsConfig),
    // server.WithInsecure() to opt out of TLS in dev
)
```

Options: `WithLogger`, `WithEnableServices`, `WithGRPCServerOptions`, `WithMaxRecvMsgBytes`, `WithMaxSendMsgBytes`, `WithTLS`, `WithInsecure`.

### `internal/server` — gateway

Gateway-side options carry a `Gateway` prefix to disambiguate from the server-side ones in the same package.

```go
// Before
gw, err := server.NewGateway(ctx, server.GatewayConfig{
    HTTPPort:        8080,
    GRPCAddr:        "127.0.0.1:50051",
    Logger:          logger,
    OpenAPISpec:     spec,
    MaxRecvMsgBytes: 8 << 20,
    MaxSendMsgBytes: 8 << 20,
    TLS:             tlsConfig,
})

// After
gw, err := server.NewGateway(ctx, 8080, "127.0.0.1:50051",
    server.WithGatewayLogger(logger),
    server.WithOpenAPISpec(spec),
    server.WithGatewayMaxRecvMsgBytes(8<<20),
    server.WithGatewayMaxSendMsgBytes(8<<20),
    server.WithGatewayTLS(tlsConfig),
    // server.WithGatewayInsecure() to opt out of TLS in dev
)
```

Options: `WithGatewayLogger`, `WithOpenAPISpec`, `WithGatewayMaxRecvMsgBytes`, `WithGatewayMaxSendMsgBytes`, `WithGatewayTLS`, `WithGatewayInsecure`.

### `internal/audit`

```go
// Before
rec := audit.NewUsageRecorder(store, audit.RecorderConfig{
    FlushInterval: 5 * time.Second,
    Logger:        logger,
})

// After
rec := audit.NewUsageRecorder(store,
    audit.WithFlushInterval(5*time.Second),
    audit.WithLogger(logger),
)
```

Options: `WithFlushInterval`, `WithLogger`. The `RecorderConfig` struct is removed.

### `internal/auth`

The previous positional `issuer` and `logger` (where `issuer` was frequently empty) become options. The option type is `auth.InterceptorOption` so future constructors in `auth` (e.g. `NewMetadataInterceptor`) can add their own without conflict.

```go
// Before
i, err := auth.NewInterceptor(ctx, jwksURL, issuer, logger)

// After
i, err := auth.NewInterceptor(ctx, jwksURL,
    auth.WithIssuer(issuer),
    auth.WithLogger(logger),
)
```

Options: `WithIssuer`, `WithLogger`.

### `internal/config`

Four required dependencies stay positional; the rest are nil-safe and become options.

```go
// Before
svc := config.NewService(store, cache, publisher, subscriber, config.ServiceConfig{
    Logger:       logger,
    CacheMetrics: cm,
    Metrics:      m,
    Validators:   vs,
    Recorder:     rec,
})

// After
svc := config.NewService(store, cache, publisher, subscriber,
    config.WithLogger(logger),
    config.WithCacheMetrics(cm),
    config.WithMetrics(m),
    config.WithValidators(vs),
    config.WithRecorder(rec),
)
```

Options: `WithLogger`, `WithCacheMetrics`, `WithMetrics`, `WithValidators`, `WithRecorder`. The `ServiceConfig` struct is removed.

### `sdk/adminclient`

All four transports (Schema, Config, Audit, Server) are independently optional per the long-standing nil-allowed contract (methods on a nil-transport service return `ErrServiceNotConfigured`). The `nil, nil, nil` placeholders disappear.

```go
// Before
admin := adminclient.New(nil, nil, ma, nil)

// After
admin := adminclient.New(adminclient.WithAuditTransport(ma))
```

Options: `WithSchemaTransport`, `WithConfigTransport`, `WithAuditTransport`, `WithServerTransport`.

### `internal/schema`

```go
// Before
svc, err := schema.NewService(store, schema.ServiceConfig{
    Validators: vs,
    Logger:     logger,
})

// After
svc, err := schema.NewService(store,
    schema.WithValidators(vs),
    schema.WithLogger(logger),
    schema.WithLimits(schema.Limits{MaxFields: 10000, MaxDocBytes: 5 << 20}),
)
```

Options include the existing `Validators` / `Logger` plus the new `WithLimits`. The `ServiceConfig` struct is removed.

### Why the change

- Optional knobs no longer require the caller to construct (and zero-fill) a config struct.
- Required dependencies are positional and cannot be silently omitted by passing a partial struct literal.
- Forward-compatible: new optional fields can be added without breaking existing call sites.
- Consistent with the SDK transport packages, which already followed this pattern.

The pattern rule is uniform across the codebase: **required arguments stay positional; only optional arguments use `With...()` options.**
