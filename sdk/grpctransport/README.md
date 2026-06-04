# grpctransport

> **Alpha** — API subject to change.

gRPC transport implementations for the OpenDecree Go SDK. Provides `Dial`, `NewConfigClient`, `NewAdminClient`, and `NewWatcher` — the usual entry points for connecting to an OpenDecree server.

[![Go Reference](https://pkg.go.dev/badge/github.com/opendecree/decree/sdk/grpctransport.svg)](https://pkg.go.dev/github.com/opendecree/decree/sdk/grpctransport)

## Quickstart

```go
import "github.com/opendecree/decree/sdk/grpctransport"

// Production: TLS with system roots.
conn, err := grpctransport.Dial("api.example.com:443")

// Local dev: insecure (plaintext).
conn, err := grpctransport.Dial("localhost:50051", grpctransport.WithInsecure())

// Read config values.
client, err := grpctransport.NewConfigClient(conn,
    grpctransport.WithSubject("myapp"),
    grpctransport.WithRole("user"),
)

// Admin operations (schema, tenants, locks).
admin, err := grpctransport.NewAdminClient(conn,
    grpctransport.WithSubject("admin"),
    grpctransport.WithRole("superadmin"),
)

// Live-updating config values.
watcher, err := grpctransport.NewWatcher(conn, tenantID,
    grpctransport.WithSubject("myapp"),
    grpctransport.WithRole("user"),
)
```

## TLS options

| Option | Use case |
|--------|----------|
| _(none)_ | Production — TLS with system certificate roots |
| `WithCustomCA(pool)` | Private CA or internal mTLS |
| `WithInsecure()` | Local development / testing only |

## Auth options

| Option | Use case |
|--------|----------|
| `WithRole(r)` | Metadata-header auth (default mode) |
| `WithSubject(s)` | Identifies the calling service |
| `WithTenantID(id)` | Pins a tenant via the `x-tenant-id` header |
| `WithBearerToken(t)` | Static JWT bearer token (opt-in) |
| `WithTokenSource(fn)` | Dynamic token source; called on every RPC — use for OAuth2 / short-lived JWTs |

`WithRole`, `WithBearerToken`, or `WithTokenSource` is required; construction returns an error if omitted.
When `WithBearerToken` or `WithTokenSource` is set, the `x-subject`, `x-role`, and `x-tenant-id` headers are not sent.

## gRPC status → SDK error mapping

The transport translates gRPC status codes to typed sentinel errors so callers can use `errors.Is`:

| gRPC status | configclient error | adminclient error |
|-------------|-------------------|-------------------|
| `NotFound` | `ErrNotFound` | `ErrNotFound` |
| `PermissionDenied` | `ErrPermissionDenied` | `ErrPermissionDenied` |
| `Unauthenticated` | `ErrUnauthenticated` | `ErrUnauthenticated` |
| `FailedPrecondition` | `ErrLocked` | `ErrFailedPrecondition` |
| `Aborted` | `ErrChecksumMismatch` | — |
| `AlreadyExists` | `ErrAlreadyExists` | `ErrAlreadyExists` |
| `InvalidArgument` | `InvalidArgumentError` | `InvalidArgumentError` |
| `ResourceExhausted` | `ErrRateLimited` (wrapped) | `ErrRateLimited` (wrapped) |
| `Unavailable` / `DeadlineExceeded` | `*RetryableError` | `*RetryableError` |

## Related packages

- [`configclient`](../configclient) — typed reads and writes
- [`adminclient`](../adminclient) — schema, tenant, lock, and audit operations
- [`configwatcher`](../configwatcher) — live, auto-refreshing values
