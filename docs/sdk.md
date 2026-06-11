# SDKs

> **Alpha** — OpenDecree is under active development. SDK APIs may change between releases. The authoritative, version-pinned API for each package is on pkg.go.dev.

Six Go SDK packages, each an independent module. Full API reference is hosted on pkg.go.dev.

- **`configclient`** — runtime config reads and writes for application code.
- **`adminclient`** — schema, tenant, audit, and config admin operations.
- **`configwatcher`** — live typed configuration values with automatic subscription.
- **`tools`** — reusable power tools (diff, docgen, validate, seed, dump).
- **`grpctransport`** — gRPC connection management and the `Transport` the clients run on.
- **`retry`** — the shared retry policy (exponential backoff) used by the clients.

The `sdk/contrib/` directory additionally holds optional configuration-loader integrations (`envconfig`, `koanf`, `viper`).

## Connecting

The clients run on a `Transport`. The `grpctransport` package dials the server and wraps the connection into a client:

```go
import "github.com/opendecree/decree/sdk/grpctransport"

// TLS with system roots by default; use WithInsecure() for a local plaintext server.
conn, err := grpctransport.Dial("localhost:9090", grpctransport.WithInsecure())
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

client, err := grpctransport.NewConfigClient(conn)
if err != nil {
    log.Fatal(err)
}
```

`grpctransport` also provides `NewAdminClient(conn, ...)` and `NewWatcher(conn, tenantID, ...)`. A single `*grpc.ClientConn` can back multiple clients; the caller owns it and must `Close` it when done.

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

## grpctransport

gRPC connection management and the gRPC implementation of the `Transport` the clients depend on.

- **Install:** `go get github.com/opendecree/decree/sdk/grpctransport@latest`
- **API Reference:** [pkg.go.dev/github.com/opendecree/decree/sdk/grpctransport](https://pkg.go.dev/github.com/opendecree/decree/sdk/grpctransport)

`Dial(target, ...)` returns a `*grpc.ClientConn` (TLS with system roots by default; `WithInsecure()`, `WithCustomCA()`, and `WithKeepalive()` adjust it). The convenience constructors `NewConfigClient`, `NewAdminClient`, and `NewWatcher` wrap a connection into the matching client. For a custom connection — for example one with interceptors — construct the `*grpc.ClientConn` yourself and pass it to one of those constructors.

## retry

The shared retry policy used by the clients: exponential backoff with optional jitter, applied to transient (retryable) errors. Enable it on a client with that client's `WithRetry` option. The package also exposes `Config`, `IsRetryable`, and backoff helpers for use in custom tooling.

- **Install:** `go get github.com/opendecree/decree/sdk/retry@latest`
- **API Reference:** [pkg.go.dev/github.com/opendecree/decree/sdk/retry](https://pkg.go.dev/github.com/opendecree/decree/sdk/retry)

## OpenTelemetry instrumentation (Go)

The Go SDKs run on a `*grpc.ClientConn` that callers can construct themselves. Wire an OTel interceptor at connection time, then hand the connection to a `grpctransport` constructor:

```go
import (
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    "github.com/opendecree/decree/sdk/grpctransport"
)

conn, err := grpc.NewClient(
    "localhost:9090",
    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
if err != nil {
    log.Fatal(err)
}
client, err := grpctransport.NewConfigClient(conn)
```

`otelgrpc.NewClientHandler()` uses the stats handler API (preferred over the deprecated interceptors) and records spans for every RPC. The same connection works with `NewAdminClient` and `NewWatcher`.

Install: `go get go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc`
