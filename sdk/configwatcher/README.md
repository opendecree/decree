# configwatcher

> **Alpha** — API subject to change.

Live, auto-refreshing configuration values for Go applications. Registers typed `Value` handles before start, loads a full snapshot on `Start`, then streams server-pushed updates in the background.

[![Go Reference](https://pkg.go.dev/badge/github.com/opendecree/decree/sdk/configwatcher.svg)](https://pkg.go.dev/github.com/opendecree/decree/sdk/configwatcher)

## Quickstart

```go
import "github.com/opendecree/decree/sdk/grpctransport"

conn, _ := grpctransport.Dial("localhost:50051", grpctransport.WithInsecure())
w, _ := grpctransport.NewWatcher(conn, tenantID,
    grpctransport.WithSubject("myapp"),
    grpctransport.WithRole("user"),
)

// Register fields before Start — each returns a live *Value[T].
maxConn := w.Int("limits.max_connections", 100)
debug   := w.Bool("app.debug", false)
timeout := w.Duration("jobs.timeout", 30*time.Second)

// Start loads the snapshot and begins streaming updates.
if err := w.Start(ctx); err != nil {
    log.Fatal(err)
}
defer w.Close()

// Get never blocks and is safe for concurrent use.
fmt.Println(maxConn.Get())   // → 100 (or server value)
fmt.Println(timeout.Get())   // → 30s (or server value)
```

## Observing changes

```go
for change := range debug.Changes() {
    log.Printf("app.debug changed: %v → %v", change.Old, change.New)
}
```

The `Changes()` channel is closed when `Close` is called.

## Typed field methods

| Method | Go type |
|--------|---------|
| `String(path, default)` | `string` |
| `Int(path, default)` | `int64` |
| `Float(path, default)` | `float64` |
| `Bool(path, default)` | `bool` |
| `Duration(path, default)` | `time.Duration` |
| `Time(path, default)` | `time.Time` |

## Related packages

- [`configclient`](../configclient) — one-shot reads/writes without a persistent stream
- [`adminclient`](../adminclient) — schema management, tenant ops, locks, audit
- [`grpctransport`](../grpctransport) — gRPC transport and `Dial` helper
