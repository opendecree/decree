# SDK: configwatcher

**Status:** Complete
**Parent:** 06-sdk

---

## Goal

High-level Go SDK for live, typed configuration values with automatic subscription and reconnect. The "Go way" to consume config — values are always fresh, changes arrive via channels, types are native Go types with null support.

## API Surface

```go
package configwatcher

// New creates a watcher for a tenant's configuration.
// Loads initial config snapshot via configclient, then subscribes for live updates.
func New(conn grpc.ClientConnInterface, tenantID string, opts ...Option) (*Watcher, error)

// Options
func WithSubject(subject string) Option
func WithRole(role string) Option
func WithTenantID(tenantID string) Option        // for auth, separate from watched tenant
func WithBearerToken(token string) Option
func WithReconnectBackoff(min, max time.Duration) Option

// Lifecycle
func (w *Watcher) Start(ctx context.Context) error   // starts subscription goroutine
func (w *Watcher) Close() error                       // stops subscription, closes channels

// Typed field accessors — register interest and get a live value handle.
// Default is used when the field has no value set or is null.
func (w *Watcher) String(fieldPath string, defaultVal string) *Value[string]
func (w *Watcher) Int(fieldPath string, defaultVal int64) *Value[int64]
func (w *Watcher) Float(fieldPath string, defaultVal float64) *Value[float64]
func (w *Watcher) Bool(fieldPath string, defaultVal bool) *Value[bool]
func (w *Watcher) Duration(fieldPath string, defaultVal time.Duration) *Value[time.Duration]

// Raw access (no type conversion)
func (w *Watcher) Raw(fieldPath string, defaultVal string) *Value[string]

// Value[T] is a live, typed config value.
type Value[T any] struct { ... }

func (v *Value[T]) Get() T                          // current value (never blocks)
func (v *Value[T]) GetWithNull() (T, bool)          // value + isSet (false = null/missing)
func (v *Value[T]) Changes() <-chan Change[T]        // channel of changes (buffered)

// Change[T] represents a value transition.
type Change[T any] struct {
    Old     T
    New     T
    WasNull bool   // old value was null/missing
    IsNull  bool   // new value is null/missing
}
```

## Design Decisions

- **Uses configclient** internally for initial snapshot load. Does NOT duplicate read/write logic.
- **Single subscription per watcher** — one gRPC stream per tenant, demuxed to registered Value handles by field path.
- **Typed values via generics** — `Value[T]` with `Get()` that returns the native Go type. Parsing happens on change, not on read (read is lock-free atomic load).
- **Null handling** — `GetWithNull()` returns `(value, isSet)`. `Get()` returns the default when null. Nullable fields are a first-class concept.
- **Auto-reconnect** — on stream failure, reconnects with exponential backoff. Re-fetches full snapshot on reconnect to avoid missed changes.
- **Thread-safe** — `Get()` uses `atomic.Value` or `sync.RWMutex`. Changes channel is buffered.
- **Fields registered before Start()** — register interest with `String()`, `Bool()`, etc., then call `Start()`. The subscription uses the field paths as filters.

## Internal Architecture

```
Watcher
├── configclient.Client          — for initial snapshot (GetAll)
├── pb.ConfigServiceClient       — for Subscribe stream
├── fields map[string]*fieldEntry — registered fields with type converters
├── subscription goroutine       — reads stream, demuxes to fields
└── reconnect logic              — backoff, re-snapshot on reconnect

fieldEntry
├── rawValue atomic.Value        — current string value
├── isSet    atomic.Bool         — null tracking
├── onChange []func(old, new string) — notify Value[T] handles
```

## Type Conversion

| Field accessor | Stored string | Go type | Parse | Format |
|---------------|--------------|---------|-------|--------|
| `String()` | `"USD"` | `string` | identity | identity |
| `Int()` | `"42"` | `int64` | `strconv.ParseInt` | — |
| `Float()` | `"3.14"` | `float64` | `strconv.ParseFloat` | — |
| `Bool()` | `"true"` | `bool` | `strconv.ParseBool` | — |
| `Duration()` | `"24h"` | `time.Duration` | `time.ParseDuration` | — |

Parse errors fall back to the default value and log a warning.

## Implementation Plan

- [ ] Module setup (`sdk/configwatcher/go.mod`, depends on configclient + api)
- [ ] `Value[T]` type with `Get()`, `GetWithNull()`, `Changes()`
- [ ] Type conversion helpers (string → T for each supported type)
- [ ] Watcher struct with field registration (`String`, `Int`, `Bool`, etc.)
- [ ] Initial snapshot load via configclient
- [ ] Subscription goroutine with field-path demux
- [ ] Auto-reconnect with exponential backoff + re-snapshot
- [ ] `Close()` lifecycle management
- [ ] Unit tests (mock stream, type conversion, null handling, reconnect)

## Files

| File | Description |
|------|-------------|
| `sdk/configwatcher/go.mod` | Module definition, depends on configclient + api |
| `sdk/configwatcher/watcher.go` | Watcher struct, New(), Start(), Close(), field registration |
| `sdk/configwatcher/value.go` | Value[T], Change[T], Get(), GetWithNull(), Changes() |
| `sdk/configwatcher/types.go` | Type parsers (string→int64, string→bool, etc.), null handling |
| `sdk/configwatcher/subscription.go` | Stream management, demux, reconnect with backoff |
| `sdk/configwatcher/watcher_test.go` | Tests |
