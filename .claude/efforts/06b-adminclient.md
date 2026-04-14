# SDK: adminclient

**Status:** Complete
**Parent:** 06-sdk

---

## Goal

Go client for administrative operations: schema management, tenant management, audit queries. Targeted at admin tooling, CI/CD pipelines, and management UIs — not application runtime code.

## API Surface

```go
package adminclient

// New creates an admin client from a gRPC connection.
func New(conn grpc.ClientConnInterface, opts ...Option) *Client

// Schema
func (c *Client) CreateSchema(ctx context.Context, name string, fields []Field, opts ...SchemaOption) (*Schema, error)
func (c *Client) GetSchema(ctx context.Context, id string) (*Schema, error)
func (c *Client) GetSchemaVersion(ctx context.Context, id string, version int32) (*Schema, error)
func (c *Client) ListSchemas(ctx context.Context) ([]*Schema, error)
func (c *Client) UpdateSchema(ctx context.Context, id string, fields []Field, description string) (*Schema, error)
func (c *Client) PublishSchema(ctx context.Context, id string, version int32) (*Schema, error)
func (c *Client) DeleteSchema(ctx context.Context, id string) error
func (c *Client) ExportSchema(ctx context.Context, id string) ([]byte, error)
func (c *Client) ImportSchema(ctx context.Context, yamlContent []byte) (*Schema, error)

// Tenant
func (c *Client) CreateTenant(ctx context.Context, name, schemaID string, schemaVersion int32) (*Tenant, error)
func (c *Client) GetTenant(ctx context.Context, id string) (*Tenant, error)
func (c *Client) ListTenants(ctx context.Context, schemaID string) ([]*Tenant, error)
func (c *Client) UpdateTenant(ctx context.Context, id string, opts ...TenantOption) (*Tenant, error)
func (c *Client) DeleteTenant(ctx context.Context, id string) error

// Field Locks
func (c *Client) LockField(ctx context.Context, tenantID, fieldPath string) error
func (c *Client) UnlockField(ctx context.Context, tenantID, fieldPath string) error
func (c *Client) ListFieldLocks(ctx context.Context, tenantID string) ([]FieldLock, error)

// Audit
func (c *Client) QueryWriteLog(ctx context.Context, opts ...AuditOption) ([]*AuditEntry, error)
func (c *Client) GetFieldUsage(ctx context.Context, tenantID, fieldPath string) (*UsageStats, error)
func (c *Client) GetUnusedFields(ctx context.Context, tenantID string) ([]string, error)
```

## Design Decisions

- **Separate from configclient** — different audience, different import weight. Admin callers don't need config read/write, and config callers don't need schema management.
- **Rich types** — returns `*Schema`, `*Tenant`, `*AuditEntry` structs (SDK-defined, not proto). Cleaner for callers than raw proto messages.
- **Pagination handled internally** — `ListSchemas`, `ListTenants`, `QueryWriteLog` auto-paginate and return all results. Option to set limits if needed.
- **Same auth pattern** as configclient — options for metadata or JWT.

## Implementation Plan

- [ ] Module setup (`sdk/adminclient/go.mod`)
- [ ] Client struct, options, auth (same pattern as configclient)
- [ ] SDK types: Schema, Tenant, AuditEntry, Field, FieldLock, UsageStats
- [ ] Schema operations
- [ ] Tenant operations
- [ ] Field lock operations
- [ ] Audit operations
- [ ] Unit tests

## Files

| File | Description |
|------|-------------|
| `sdk/adminclient/go.mod` | Module definition, depends on api/ |
| `sdk/adminclient/client.go` | Client struct, New(), options |
| `sdk/adminclient/types.go` | SDK-level types (Schema, Tenant, etc.) |
| `sdk/adminclient/schema.go` | Schema CRUD, publish, import/export |
| `sdk/adminclient/tenant.go` | Tenant CRUD |
| `sdk/adminclient/locks.go` | Field lock operations |
| `sdk/adminclient/audit.go` | Audit queries |
| `sdk/adminclient/client_test.go` | Tests |
