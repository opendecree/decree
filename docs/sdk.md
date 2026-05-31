# SDKs

Three Go SDK packages, each an independent module. Full API reference is hosted on pkg.go.dev.

## configclient

Runtime config reads and writes for application code.

- **Install:** `go get github.com/opendecree/decree/sdk/configclient@latest`
- **API Reference:** [pkg.go.dev/github.com/opendecree/decree/sdk/configclient](https://pkg.go.dev/github.com/opendecree/decree/sdk/configclient)

Features: Get, GetAll, GetFields, Set, SetMany, typed getters/setters (GetInt, SetBool, etc.), Snapshot for pinned-version reads, GetForUpdate + Update for optimistic concurrency, null support, opt-in retry with exponential backoff (`WithRetry`).

## adminclient

Schema, tenant, audit, and config admin operations for tooling and CI/CD.

- **Install:** `go get github.com/opendecree/decree/sdk/adminclient@latest`
- **API Reference:** [pkg.go.dev/github.com/opendecree/decree/sdk/adminclient](https://pkg.go.dev/github.com/opendecree/decree/sdk/adminclient)

Features: Schema CRUD, publish, import/export (YAML), tenant CRUD, field locks, config versioning, rollback, audit log queries, usage stats.

## configwatcher

Live typed configuration values with automatic subscription and reconnect.

- **Install:** `go get github.com/opendecree/decree/sdk/configwatcher@latest`
- **API Reference:** [pkg.go.dev/github.com/opendecree/decree/sdk/configwatcher](https://pkg.go.dev/github.com/opendecree/decree/sdk/configwatcher)

Features: `Value[T]` generic type with `Get()`, `GetWithNull()`, `Changes()` channel. Typed accessors (String, Int, Float, Bool, Duration). Auto-reconnect with exponential backoff. Thread-safe.

## tools

Reusable power tools for config management — importable as Go packages for integration into servers, CI, or custom tooling.

- **Install:** `go get github.com/opendecree/decree/sdk/tools@latest`
- **API Reference:** [pkg.go.dev/github.com/opendecree/decree/sdk/tools](https://pkg.go.dev/github.com/opendecree/decree/sdk/tools)

Packages: `diff` (config version diffing), `docgen` (schema → markdown), `validate` (offline YAML validation), `seed` (bootstrap from YAML), `dump` (full tenant backup). Offline tools have zero gRPC/proto dependencies.

## OpenTelemetry instrumentation (Go)

The Go SDKs accept a `grpc.ClientConn` that callers construct themselves. Wire an OTel interceptor at connection time:

```go
import (
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
    "google.golang.org/grpc"
    "github.com/opendecree/decree/sdk/configclient"
)

conn, err := grpc.NewClient(
    "localhost:9090",
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
// conn is passed to the SDK client
client := configclient.New(conn, ...)
```

`otelgrpc.NewClientHandler()` uses the stats handler API (preferred over deprecated interceptors) and records spans for every RPC. The same pattern applies to `adminclient` and the watcher's underlying connection.

Install: `go get go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`
