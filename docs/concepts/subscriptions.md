# Subscriptions

CCS supports real-time change streaming via gRPC server-side streaming. When a config value changes, subscribers receive the update instantly -- no polling required.

## How It Works

The flow from write to subscriber:

```
Writer sets value
       │
       ▼
Server validates and stores ──► PostgreSQL
       │
       ▼
Server publishes change ──────► Redis Pub/Sub
       │
       ▼
Subscriber receives event ────► gRPC stream
```

1. A client writes a config value via `SetField` or `SetFields`
2. The server validates and persists the change
3. The server publishes a change event to Redis pub/sub
4. All active subscribers for that tenant receive the event on their gRPC stream

## Subscribing via the CLI

The simplest way to watch for changes:

```bash
# Watch all changes for a tenant
decree watch <tenant-id>

# Watch specific fields
decree watch <tenant-id> --fields payments.fee_rate,payments.enabled
```

The CLI prints each change as it happens, showing the field path, old value, new value, who made the change, and when.

## Subscribing via gRPC

The `Subscribe` RPC is a server-side streaming call. Each `ConfigChange` event contains:

| Field | Description |
|-------|-------------|
| `tenant_id` | The tenant whose config changed |
| `version` | The new config version number |
| `field_path` | Which field changed |
| `old_value` | Previous value (absent if field was null) |
| `new_value` | New value (absent if set to null) |
| `changed_by` | Who made the change |
| `changed_at` | When the change occurred |

See the [API Reference](../api/api-reference.md) for the full `Subscribe` RPC definition and `ConfigChange` message.

## Field-Path Filtering

You can subscribe to specific fields instead of receiving all changes:

```go
// Only receive changes to fee_rate and enabled
stream, err := configSvc.Subscribe(ctx, &pb.SubscribeRequest{
    TenantId:   tenantID,
    FieldPaths: []string{"payments.fee_rate", "payments.enabled"},
})
```

When `FieldPaths` is empty, you receive all changes for the tenant.

## The configwatcher SDK

For Go applications, the `configwatcher` SDK provides a higher-level abstraction over raw subscriptions. It handles connection management, initial value loading, and typed access:

```go
w := configwatcher.New(conn, tenantID,
    configwatcher.WithSubject("myapp"),
)

// Register typed fields with defaults
feeRate := w.Float("payments.fee_rate", 0.01)
enabled := w.Bool("payments.enabled", false)
timeout := w.Duration("payments.timeout", 30*time.Second)

// Start watching (loads current values, then subscribes)
w.Start(ctx)
defer w.Close()

// Read current values — always fresh, never blocks
fmt.Println(feeRate.Get())   // float64
fmt.Println(enabled.Get())   // bool
fmt.Println(timeout.Get())   // time.Duration
```

### Reacting to Changes

Each watched field exposes a `Changes()` channel:

```go
go func() {
    for change := range feeRate.Changes() {
        log.Printf("Fee rate changed: %v -> %v", change.Old, change.New)
        recalculatePricing(change.New)
    }
}()
```

### Auto-Reconnect

The configwatcher automatically reconnects if the gRPC stream drops. On reconnect, it reloads current values to ensure consistency. Your application code does not need to handle reconnection logic.

## Internal Architecture

Internally, CCS uses Redis pub/sub for change propagation:

- Each tenant has a Redis pub/sub channel
- When the config service writes a value, it publishes the change event to the tenant's channel
- Subscriber goroutines listen on the channel and forward events to their gRPC streams

This architecture means:

- **Horizontal scaling** -- multiple service instances can serve subscribers because Redis pub/sub fans out to all listeners
- **No message persistence** -- if a subscriber is disconnected when a change happens, it misses the event. The configwatcher SDK handles this by reloading state on reconnect.
- **Low latency** -- changes propagate through Redis in sub-millisecond time

## Related

- [Typed Values](typed-values.md) -- how values are represented in change events
- [Versioning](versioning.md) -- version numbers in change events
- [SDKs](../sdk.md) -- configwatcher SDK documentation
- [API Reference](../api/api-reference.md) -- Subscribe RPC definition
