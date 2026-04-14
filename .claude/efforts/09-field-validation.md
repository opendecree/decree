# Typed Proto Values + Field Validation

**Status:** Complete
**Started:** 2026-03-26

---

## Goal

1. Replace string-encoded config values with a typed `oneof` in proto — wire-level type safety.
2. Validate remaining constraint-based rules (min/max, pattern, enum, JSON Schema) before writes.
3. Strict mode: reject writes to fields not defined in the schema.

## Phase 0: Null Support (completed)

- [x] Proto: `optional string value` for null vs empty string
- [x] DB: nullable `config_values.value`
- [x] Service + SDK + CLI updated for `*string`
- [x] All tests passing

## Phase 1: Typed Proto Values

Replace `optional string value` with a `TypedValue` oneof:

```protobuf
import "google/protobuf/timestamp.proto";
import "google/protobuf/duration.proto";

message TypedValue {
  oneof kind {
    int64 integer_value = 1;
    double number_value = 2;
    string string_value = 3;
    bool bool_value = 4;
    google.protobuf.Timestamp time_value = 5;
    google.protobuf.Duration duration_value = 6;
    string url_value = 7;
    string json_value = 8;
  }
  // no field set = null
}
```

### Changes needed

| Area | Change |
|------|--------|
| `types.proto` | Add `TypedValue` message. Update `ConfigValue.value` to `TypedValue`. Update `ConfigChange.old_value`/`new_value` to `TypedValue`. |
| `config_service.proto` | `SetFieldRequest.value` → `TypedValue`. `FieldUpdate.value` → `TypedValue`. |
| `make generate` | Regenerate proto + sqlc |
| DB storage | Keep `TEXT` column — serialize typed values to string for storage, deserialize on read |
| `internal/config/service.go` | Convert `TypedValue` → string on write, string → `TypedValue` on read (using schema field type) |
| `internal/config/convert.go` | `TypedValue` ↔ string conversion helpers |
| `internal/config/yaml.go` | Update export/import — `TypedValue` replaces typed YAML conversion |
| Cache | Keep `map[string]string` — cache stores string representations |
| SDKs | configclient: type-specific `Set*` methods. configwatcher: already typed via `Value[T]`. adminclient: minimal impact. |
| CLI | `ccs config set` needs type awareness |
| E2E tests | Update for `TypedValue` |

### SDK API changes

```go
// configclient — type-specific setters
func (c *Client) Set(ctx, tenantID, fieldPath string, value *pb.TypedValue) error
func (c *Client) SetString(ctx, tenantID, fieldPath, value string) error
func (c *Client) SetInt(ctx, tenantID, fieldPath string, value int64) error
func (c *Client) SetFloat(ctx, tenantID, fieldPath string, value float64) error
func (c *Client) SetBool(ctx, tenantID, fieldPath string, value bool) error
func (c *Client) SetTime(ctx, tenantID, fieldPath string, value time.Time) error
func (c *Client) SetDuration(ctx, tenantID, fieldPath string, value time.Duration) error
func (c *Client) SetNull(ctx, tenantID, fieldPath string) error

// Get still returns string (for simplicity), GetTyped returns TypedValue
func (c *Client) Get(ctx, tenantID, fieldPath string) (string, error)
func (c *Client) GetTyped(ctx, tenantID, fieldPath string) (*pb.TypedValue, error)
```

## Phase 2: Constraint Validation

With typed proto, type parsing validators are no longer needed. Remaining validations:

| Constraint | Applies to | Check |
|-----------|-----------|-------|
| min/max | integer, number, duration | Range check on the typed value |
| min/max (length) | string | `len(value)` check |
| pattern | string | `regexp.MatchString` |
| enum | any | Value in allowed set |
| JSON Schema | json | `santhosh-tekuri/jsonschema` |
| URL validity | url | `url.Parse`, must be absolute |

### Validator factory + cache (same design as before)

- `ValidatorFactory` builds constraint validators from schema fields
- Per-tenant cache with `sync.RWMutex` + map
- Invalidate on `UpdateTenant` schema version change
- Strict mode: reject unknown field paths

## Dependencies

- `github.com/santhosh-tekuri/jsonschema/v6` — JSON Schema validation

## Implementation Plan

- [x] Phase 0: Null support (optional string → TypedValue)
- [x] Phase 1a: Add `TypedValue` oneof to proto, regenerate
- [x] Phase 1b: Internal conversion layer (TypedValue ↔ string for DB storage)
- [x] Phase 1c: Update ConfigService read/write paths
- [x] Phase 1d: Update SDKs — typed Go getters/setters, no proto leakage
- [x] Phase 1e: Update CLI, e2e tests, unit tests for typed values + null
- [x] Phase 2a: Constraint validators (min/max, pattern, enum, url, json schema) — 19 unit tests
- [x] Phase 2b: Validator factory + per-tenant cache
- [x] Phase 2c: Wire into ConfigService (strict mode) for SetField/SetFields
- [x] Phase 2d: Wire validation into ImportConfig — 2 unit tests + 3 e2e subtests
- [x] Phase 2e: E2e tests for validation — 8 subtests in TestConstraintValidation
- [x] Cache invalidation on UpdateTenant

### Phase 3: Constraint extensions (OAS-aligned)
- [x] Separate `minLength`/`maxLength` for string length (unbundle from `minimum`/`maximum`)
- [x] `minimum`/`maximum` become numeric-only (integer, number, duration)
- [x] Add `exclusiveMinimum`/`exclusiveMaximum` for strict range checks (`>` / `<` instead of `>=` / `<=`)
- [x] Update proto FieldConstraints
- [x] Update validators
- [x] Update schema YAML mapping + JSON Schema
- [x] Update docs
- [x] Tests — 17 unit + 6 e2e subtests + constraint/type validation at schema creation
