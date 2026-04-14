# Typed Values

OpenDecree enforces type safety at every layer -- from the wire protocol to the database to the SDK. Every config value has a declared type, and the server rejects writes that don't match.

## The 8 Field Types

| Type | Description | Example values |
|------|-------------|----------------|
| `integer` | Whole numbers | `42`, `-1`, `0` |
| `number` | Floating-point numbers | `3.14`, `0.025`, `-99.9` |
| `string` | Free-form text | `"hello"`, `"USD"` |
| `bool` | Boolean | `true`, `false` |
| `time` | Timestamp (RFC 3339) | `2025-01-15T09:30:00Z` |
| `duration` | Go-style duration | `24h`, `30s`, `500ms`, `1h30m` |
| `url` | Absolute URL (always validated) | `https://example.com/hook` |
| `json` | JSON document | `{"key": "value"}` |

## How Types Flow Through the System

Each type has a representation at every layer:

| Layer | integer | number | bool | time | duration | string | url | json |
|-------|---------|--------|------|------|----------|--------|-----|------|
| **YAML** | `42` | `3.14` | `true` | `2025-01-15T09:30:00Z` | `30s` | `hello` | `https://...` | `{"k":"v"}` |
| **Proto** | `int64` | `double` | `bool` | `Timestamp` | `Duration` | `string` | `string` | `string` |
| **Go SDK** | `int64` | `float64` | `bool` | `time.Time` | `time.Duration` | `string` | `string` | `string` |
| **Database** | `"42"` | `"3.14"` | `"true"` | `"2025-01-15T09:30:00Z"` | `"30s"` | `"hello"` | `"https://..."` | `"{\"k\":\"v\"}"` |

The database always stores values as strings. The server handles conversion between string storage and native types on the wire. SDK typed getters convert proto types to native Go types.

## TypedValue in Protocol Buffers

The proto `TypedValue` message uses a `oneof` to carry the native type on the wire:

```protobuf
message TypedValue {
  oneof kind {
    int64                    integer_value  = 1;
    double                   number_value   = 2;
    string                   string_value   = 3;
    bool                     bool_value     = 4;
    google.protobuf.Timestamp time_value    = 5;
    google.protobuf.Duration  duration_value = 6;
    string                   url_value      = 7;
    string                   json_value     = 8;
  }
}
```

This means integer fields carry actual `int64` values, not strings. The server validates that the oneof variant matches the field's declared type -- sending `string_value` for an `integer` field is rejected.

## Null vs. Empty String

OpenDecree distinguishes between **null** (no value) and **empty string** (`""`):

| State | TypedValue | Meaning |
|-------|-----------|---------|
| Null | `value` field absent | Field has no value set |
| Empty string | `string_value: ""` | Field is explicitly set to an empty string |

A field must be declared `nullable: true` in the schema to accept null values. Non-nullable fields must always have a value once set.

In the Go SDK:

```go
// Get returns ("", false) for null, ("", true) for empty string
val, ok := client.Get(ctx, tenantID, "field.path")
if !ok {
    // field is null
}
```

## SDK Typed Getters

The `configclient` SDK provides typed getter methods that handle the proto-to-Go conversion:

```go
intVal, err := client.GetInt(ctx, tenantID, "payments.max_retries")      // int64
floatVal, err := client.GetFloat(ctx, tenantID, "payments.fee_rate")     // float64
boolVal, err := client.GetBool(ctx, tenantID, "payments.enabled")        // bool
timeVal, err := client.GetTime(ctx, tenantID, "payments.cutoff")         // time.Time
durVal, err := client.GetDuration(ctx, tenantID, "payments.timeout")     // time.Duration
strVal, err := client.Get(ctx, tenantID, "payments.currency")            // string
```

The `configwatcher` SDK provides typed reactive fields:

```go
fee := w.Float("payments.fee_rate", 0.01)    // default 0.01
enabled := w.Bool("payments.enabled", false)  // default false
timeout := w.Duration("payments.timeout", 30*time.Second)

fmt.Println(fee.Get())     // always returns float64
fmt.Println(enabled.Get()) // always returns bool
```

## Type Validation

Type checking happens at multiple levels:

1. **Schema definition** -- constraints are validated against the field type (e.g., `minimum` on a `string` is rejected)
2. **Config write** -- the server checks that the TypedValue variant matches the field type
3. **Constraint enforcement** -- the value is checked against any defined constraints (range, pattern, enum, JSON Schema)

For details on constraints, see [Schemas & Fields -- Constraints](schemas-and-fields.md#constraints).

## Related

- [Schemas & Fields](schemas-and-fields.md) -- field definitions and constraints
- [API Reference](../api/api-reference.md) -- TypedValue and FieldType proto definitions
- [SDKs](../sdk.md) -- typed getter documentation
