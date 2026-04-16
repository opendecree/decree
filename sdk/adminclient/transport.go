package adminclient

import (
	"context"
	"time"
)

// SchemaTransport abstracts schema and tenant management operations.
type SchemaTransport interface {
	CreateSchema(ctx context.Context, req *CreateSchemaRequest) (*Schema, error)
	GetSchema(ctx context.Context, id string, version *int32) (*Schema, error)
	ListSchemas(ctx context.Context, pageSize int32, pageToken string) (*ListSchemasResponse, error)
	UpdateSchema(ctx context.Context, req *UpdateSchemaRequest) (*Schema, error)
	PublishSchema(ctx context.Context, id string, version int32) (*Schema, error)
	DeleteSchema(ctx context.Context, id string) error
	ExportSchema(ctx context.Context, id string, version *int32) ([]byte, error)
	ImportSchema(ctx context.Context, yamlContent []byte, autoPublish bool) (*Schema, error)

	CreateTenant(ctx context.Context, req *CreateTenantRequest) (*Tenant, error)
	GetTenant(ctx context.Context, id string) (*Tenant, error)
	ListTenants(ctx context.Context, schemaID *string, pageSize int32, pageToken string) (*ListTenantsResponse, error)
	UpdateTenant(ctx context.Context, req *UpdateTenantRequest) (*Tenant, error)
	DeleteTenant(ctx context.Context, id string) error

	LockField(ctx context.Context, tenantID, fieldPath string, lockedValues []string) error
	UnlockField(ctx context.Context, tenantID, fieldPath string) error
	ListFieldLocks(ctx context.Context, tenantID string) ([]FieldLock, error)
}

// ConfigTransport abstracts config versioning and import/export operations.
type ConfigTransport interface {
	ListVersions(ctx context.Context, tenantID string, pageSize int32, pageToken string) (*ListVersionsResponse, error)
	GetVersion(ctx context.Context, tenantID string, version int32) (*Version, error)
	RollbackToVersion(ctx context.Context, tenantID string, version int32, description string) (*Version, error)
	ExportConfig(ctx context.Context, tenantID string, version *int32) ([]byte, error)
	ImportConfig(ctx context.Context, req *ImportConfigRequest) (*Version, error)
}

// AuditTransport abstracts audit log operations.
type AuditTransport interface {
	QueryWriteLog(ctx context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error)
	GetFieldUsage(ctx context.Context, tenantID, fieldPath string, start, end *time.Time) (*UsageStats, error)
	GetTenantUsage(ctx context.Context, tenantID string, start, end *time.Time) ([]*UsageStats, error)
	GetUnusedFields(ctx context.Context, tenantID string, since time.Time) ([]string, error)
}

// --- Request types ---

// CreateSchemaRequest is the input for [SchemaTransport.CreateSchema].
type CreateSchemaRequest struct {
	Name        string
	Fields      []Field
	Description string
}

// UpdateSchemaRequest is the input for [SchemaTransport.UpdateSchema].
type UpdateSchemaRequest struct {
	ID                 string
	AddOrModify        []Field
	RemoveFields       []string
	VersionDescription string
}

// CreateTenantRequest is the input for [SchemaTransport.CreateTenant].
type CreateTenantRequest struct {
	Name          string
	SchemaID      string
	SchemaVersion int32
}

// UpdateTenantRequest is the input for [SchemaTransport.UpdateTenant].
type UpdateTenantRequest struct {
	ID            string
	Name          *string
	SchemaVersion *int32
}

// ImportConfigRequest is the input for [ConfigTransport.ImportConfig].
type ImportConfigRequest struct {
	TenantID    string
	YamlContent []byte
	Description string
	Mode        ImportMode
}

// QueryWriteLogRequest is the input for [AuditTransport.QueryWriteLog].
type QueryWriteLogRequest struct {
	TenantID  *string
	Actor     *string
	FieldPath *string
	StartTime *time.Time
	EndTime   *time.Time
	PageSize  int32
	PageToken string
}

// --- Response types (paginated) ---

// ListSchemasResponse is the output of [SchemaTransport.ListSchemas].
type ListSchemasResponse struct {
	Schemas       []*Schema
	NextPageToken string
}

// ListTenantsResponse is the output of [SchemaTransport.ListTenants].
type ListTenantsResponse struct {
	Tenants       []*Tenant
	NextPageToken string
}

// ListVersionsResponse is the output of [ConfigTransport.ListVersions].
type ListVersionsResponse struct {
	Versions      []*Version
	NextPageToken string
}

// QueryWriteLogResponse is the output of [AuditTransport.QueryWriteLog].
type QueryWriteLogResponse struct {
	Entries       []*AuditEntry
	NextPageToken string
}
