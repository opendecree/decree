# SDK: configclient

**Status:** Complete
**Parent:** 06-sdk

---

## Goal

Simple, ergonomic Go client for reading and writing config values. Thin wrapper over the generated ConfigService gRPC client. Targeted at application code that consumes configuration.

## API Surface

```go
package configclient

// New creates a config client from a gRPC connection.
func New(conn grpc.ClientConnInterface, opts ...Option) *Client

// Options
func WithSubject(subject string) Option          // x-subject header
func WithRole(role string) Option                // x-role header (default: superadmin)
func WithTenantID(tenantID string) Option        // x-tenant-id header
func WithBearerToken(token string) Option        // JWT bearer token (alternative to metadata)

// Read
func (c *Client) Get(ctx context.Context, tenantID, fieldPath string) (string, error)
func (c *Client) GetAll(ctx context.Context, tenantID string) (map[string]string, error)
func (c *Client) GetFields(ctx context.Context, tenantID string, paths []string) (map[string]string, error)
func (c *Client) GetVersion(ctx context.Context, tenantID string, version int32) (map[string]string, error)

// Write
func (c *Client) Set(ctx context.Context, tenantID, fieldPath, value string) error
func (c *Client) SetMany(ctx context.Context, tenantID string, values map[string]string, description string) error

// Import/Export
func (c *Client) Export(ctx context.Context, tenantID string) ([]byte, error)
func (c *Client) Import(ctx context.Context, tenantID string, yamlContent []byte, description string) error

// Versioning
func (c *Client) ListVersions(ctx context.Context, tenantID string) ([]Version, error)
func (c *Client) Rollback(ctx context.Context, tenantID string, version int32) error
```

## Design Decisions

- **Connection passed in** — caller manages gRPC dial lifecycle. SDK doesn't own connections.
- **Auth via options** — metadata headers injected via unary interceptor attached per-call. Supports both metadata mode and JWT.
- **Simple return types** — `string` for values, `map[string]string` for bulk. No custom value types at this layer (that's configwatcher's job).
- **Errors** — unwrap gRPC status codes into sentinel errors: `ErrNotFound`, `ErrLocked`, `ErrChecksumMismatch`.

## Implementation Plan

- [x] Module setup (`sdk/configclient/go.mod`)
- [x] Client struct with functional options
- [x] Auth metadata injection (per-call metadata via WithSubject/WithRole/WithTenantID/WithBearerToken)
- [x] Read methods (Get, GetAll, GetFields)
- [x] Snapshot reads (Snapshot, AtVersion — pinned version for flow consistency)
- [x] Write methods (Set, SetMany)
- [x] Optimistic concurrency (GetForUpdate + LockedValue.Set, Update convenience CAS)
- [x] Error sentinel types (ErrNotFound, ErrLocked, ErrChecksumMismatch, ErrAlreadyExists)
- [x] Unit tests — 11 tests with mock gRPC client

Note: Import/Export, ListVersions, Rollback moved to adminclient (06b).

## Files

| File | Description |
|------|-------------|
| `sdk/configclient/go.mod` | Module definition, depends on api/ |
| `sdk/configclient/client.go` | Client struct, New(), options |
| `sdk/configclient/read.go` | Get, GetAll, GetFields, GetVersion |
| `sdk/configclient/write.go` | Set, SetMany, Import, Export, Rollback |
| `sdk/configclient/errors.go` | Sentinel errors |
| `sdk/configclient/client_test.go` | Tests |
