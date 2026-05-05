package schema

import (
	"context"

	"github.com/opendecree/decree/internal/storage/domain"
)

// CreateSchemaParams contains parameters for creating a schema.
type CreateSchemaParams struct {
	Name        string
	Description *string
}

// CreateSchemaVersionParams contains parameters for creating a schema version.
type CreateSchemaVersionParams struct {
	SchemaID      string
	Version       int32
	ParentVersion *int32
	Description   *string
	Checksum      string
	// DependentRequired is the JSON-encoded list of cross-field rules.
	// Pass an empty []byte (or nil) when no rules exist; the store will
	// persist `[]` so reads always return well-formed JSON.
	DependentRequired []byte
	// Validations is the JSON-encoded list of CEL validation rules
	// reserved in v0.1.0. Same nil-safe semantics as DependentRequired.
	Validations []byte
}

// GetSchemaVersionParams identifies a specific schema version.
type GetSchemaVersionParams struct {
	SchemaID string
	Version  int32
}

// ListSchemasParams contains pagination parameters for listing schemas.
type ListSchemasParams struct {
	Limit  int32
	Offset int32
}

// CreateSchemaFieldParams contains parameters for creating a schema field.
type CreateSchemaFieldParams struct {
	SchemaVersionID string
	Path            string
	FieldType       domain.FieldType
	Constraints     []byte
	Nullable        bool
	Deprecated      bool
	RedirectTo      *string
	DefaultValue    *string
	Description     *string
	Title           *string
	Example         *string
	Examples        []byte
	ExternalDocs    []byte
	Tags            []string
	Format          *string
	ReadOnly        bool
	WriteOnce       bool
	Sensitive       bool
}

// DeleteSchemaFieldParams identifies a field to delete.
type DeleteSchemaFieldParams struct {
	SchemaVersionID string
	Path            string
}

// PublishSchemaVersionParams identifies a schema version to publish.
type PublishSchemaVersionParams struct {
	SchemaID string
	Version  int32
}

// CreateTenantParams contains parameters for creating a tenant.
type CreateTenantParams struct {
	Name          string
	SchemaID      string
	SchemaVersion int32
}

// ListTenantsParams contains pagination and filtering parameters for listing tenants.
type ListTenantsParams struct {
	Limit            int32
	Offset           int32
	AllowedTenantIDs []string // When non-nil, only return tenants with these IDs.
}

// ListTenantsBySchemaParams contains parameters for listing tenants by schema.
type ListTenantsBySchemaParams struct {
	SchemaID         string
	Limit            int32
	Offset           int32
	AllowedTenantIDs []string // When non-nil, only return tenants with these IDs.
}

// UpdateTenantNameParams contains parameters for updating a tenant's name.
type UpdateTenantNameParams struct {
	ID   string
	Name string
}

// UpdateTenantSchemaVersionParams contains parameters for updating a tenant's schema version.
type UpdateTenantSchemaVersionParams struct {
	ID            string
	SchemaVersion int32
}

// CreateFieldLockParams contains parameters for creating a field lock.
type CreateFieldLockParams struct {
	TenantID     string
	FieldPath    string
	LockedValues []byte
}

// DeleteFieldLockParams identifies a field lock to delete.
type DeleteFieldLockParams struct {
	TenantID  string
	FieldPath string
}

// InsertAuditWriteLogParams contains parameters for inserting a schema/tenant/lock audit entry.
type InsertAuditWriteLogParams struct {
	TenantID   string // empty for global (schema-level) entries
	Actor      string
	Action     string
	ObjectKind string // "schema", "tenant", or "lock"
	FieldPath  *string
	OldValue   *string
	NewValue   *string
	Metadata   []byte
}

// Store defines the data access interface for schema operations.
// Implementations must return [domain.ErrNotFound] when an entity is not found.
type Store interface {
	// RunInTx executes fn within a database transaction.
	// The Store passed to fn is bound to the transaction.
	// If fn returns nil the transaction is committed; otherwise it is rolled back.
	RunInTx(ctx context.Context, fn func(Store) error) error

	// InsertAuditWriteLog writes an admin audit entry transactionally.
	InsertAuditWriteLog(ctx context.Context, arg InsertAuditWriteLogParams) error

	// Schema CRUD.
	CreateSchema(ctx context.Context, arg CreateSchemaParams) (domain.Schema, error)
	GetSchemaByID(ctx context.Context, id string) (domain.Schema, error)
	GetSchemaByName(ctx context.Context, name string) (domain.Schema, error)
	ListSchemas(ctx context.Context, arg ListSchemasParams) ([]domain.Schema, error)
	DeleteSchema(ctx context.Context, id string) error

	// Schema versions.
	CreateSchemaVersion(ctx context.Context, arg CreateSchemaVersionParams) (domain.SchemaVersion, error)
	GetSchemaVersion(ctx context.Context, arg GetSchemaVersionParams) (domain.SchemaVersion, error)
	GetLatestSchemaVersion(ctx context.Context, schemaID string) (domain.SchemaVersion, error)
	PublishSchemaVersion(ctx context.Context, arg PublishSchemaVersionParams) (domain.SchemaVersion, error)

	// Schema fields.
	CreateSchemaField(ctx context.Context, arg CreateSchemaFieldParams) (domain.SchemaField, error)
	GetSchemaFields(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error)
	DeleteSchemaField(ctx context.Context, arg DeleteSchemaFieldParams) error

	// Tenants.
	CreateTenant(ctx context.Context, arg CreateTenantParams) (domain.Tenant, error)
	GetTenantByID(ctx context.Context, id string) (domain.Tenant, error)
	GetTenantByName(ctx context.Context, name string) (domain.Tenant, error)
	ListTenants(ctx context.Context, arg ListTenantsParams) ([]domain.Tenant, error)
	ListTenantsBySchema(ctx context.Context, arg ListTenantsBySchemaParams) ([]domain.Tenant, error)
	UpdateTenantName(ctx context.Context, arg UpdateTenantNameParams) (domain.Tenant, error)
	UpdateTenantSchemaVersion(ctx context.Context, arg UpdateTenantSchemaVersionParams) (domain.Tenant, error)
	DeleteTenant(ctx context.Context, id string) error

	// Field locks.
	CreateFieldLock(ctx context.Context, arg CreateFieldLockParams) error
	DeleteFieldLock(ctx context.Context, arg DeleteFieldLockParams) error
	GetFieldLocks(ctx context.Context, tenantID string) ([]domain.TenantFieldLock, error)
}
