# configclient

> **Alpha** — API subject to change.

Go client for reading and writing OpenDecree configuration values at application runtime. Wraps a pluggable `Transport` with typed accessors and optimistic concurrency helpers.

[![Go Reference](https://pkg.go.dev/badge/github.com/opendecree/decree/sdk/configclient.svg)](https://pkg.go.dev/github.com/opendecree/decree/sdk/configclient)

## Quickstart

```go
import (
    "github.com/opendecree/decree/sdk/grpctransport"
)

conn, _ := grpctransport.Dial("localhost:50051", grpctransport.WithInsecure())
client, _ := grpctransport.NewConfigClient(conn,
    grpctransport.WithSubject("myapp"),
    grpctransport.WithRole("user"),
)

ctx := context.Background()

// Typed reads — no string parsing needed.
name, _     := client.GetString(ctx, tenantID, "app.name")
debug, _    := client.GetBool(ctx, tenantID, "app.debug")
timeout, _  := client.GetDuration(ctx, tenantID, "jobs.timeout")

// Typed writes.
_ = client.SetBool(ctx, tenantID, "app.debug", true)

// Batch write.
_ = client.SetMany(ctx, tenantID, map[string]string{
    "app.name":  "MyApp",
    "app.debug": "false",
}, "initial seed")
```

## Typed accessors

| Read method | Write method | Go type |
|-------------|--------------|---------|
| `GetString` | `Set` (string overload) | `string` |
| `GetInt` | `SetInt` | `int64` |
| `GetFloat` | `SetFloat` | `float64` |
| `GetBool` | `SetBool` | `bool` |
| `GetTime` | `SetTime` | `time.Time` |
| `GetDuration` | `SetDuration` | `time.Duration` |

Note: there is no `SetString` convenience method — use `Set(ctx, tenantID, path, value)` for string writes.

## Optimistic concurrency

Use `GetForUpdate` + `LockedValue.Set` to perform a read-modify-write without losing concurrent updates:

```go
lv, _ := client.GetForUpdate(ctx, tenantID, "counters.visits")
_ = lv.Set(ctx, strconv.Itoa(visits+1))
```

Or use the higher-level helper that retries on conflict:

```go
_ = client.Update(ctx, tenantID, "counters.visits", func(cur string) (string, error) {
    n, _ := strconv.Atoi(cur)
    return strconv.Itoa(n+1), nil
})
```

## Related packages

- [`configwatcher`](../configwatcher) — live, auto-refreshing values via subscription stream
- [`adminclient`](../adminclient) — schema management, tenant ops, locks, audit
- [`grpctransport`](../grpctransport) — gRPC `Transport` implementation and `Dial` helper
