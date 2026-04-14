package schema

import (
	"context"

	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
)

// Store defines the data access interface for schema operations.
type Store interface {
	// Schema CRUD.
	CreateSchema(ctx context.Context, arg dbstore.CreateSchemaParams) (dbstore.Schema, error)
	GetSchemaByID(ctx context.Context, id pgUUID) (dbstore.Schema, error)
	GetSchemaByName(ctx context.Context, name string) (dbstore.Schema, error)
	ListSchemas(ctx context.Context, arg dbstore.ListSchemasParams) ([]dbstore.Schema, error)
	DeleteSchema(ctx context.Context, id pgUUID) error

	// Schema versions.
	CreateSchemaVersion(ctx context.Context, arg dbstore.CreateSchemaVersionParams) (dbstore.SchemaVersion, error)
	GetSchemaVersion(ctx context.Context, arg dbstore.GetSchemaVersionParams) (dbstore.SchemaVersion, error)
	GetLatestSchemaVersion(ctx context.Context, schemaID pgUUID) (dbstore.SchemaVersion, error)
	PublishSchemaVersion(ctx context.Context, arg dbstore.PublishSchemaVersionParams) (dbstore.SchemaVersion, error)

	// Schema fields.
	CreateSchemaField(ctx context.Context, arg dbstore.CreateSchemaFieldParams) (dbstore.SchemaField, error)
	GetSchemaFields(ctx context.Context, schemaVersionID pgUUID) ([]dbstore.SchemaField, error)
	DeleteSchemaField(ctx context.Context, arg dbstore.DeleteSchemaFieldParams) error

	// Tenants.
	CreateTenant(ctx context.Context, arg dbstore.CreateTenantParams) (dbstore.Tenant, error)
	GetTenantByID(ctx context.Context, id pgUUID) (dbstore.Tenant, error)
	ListTenants(ctx context.Context, arg dbstore.ListTenantsParams) ([]dbstore.Tenant, error)
	ListTenantsBySchema(ctx context.Context, arg dbstore.ListTenantsBySchemaParams) ([]dbstore.Tenant, error)
	UpdateTenantName(ctx context.Context, arg dbstore.UpdateTenantNameParams) (dbstore.Tenant, error)
	UpdateTenantSchemaVersion(ctx context.Context, arg dbstore.UpdateTenantSchemaVersionParams) (dbstore.Tenant, error)
	DeleteTenant(ctx context.Context, id pgUUID) error

	// Field locks.
	CreateFieldLock(ctx context.Context, arg dbstore.CreateFieldLockParams) error
	DeleteFieldLock(ctx context.Context, arg dbstore.DeleteFieldLockParams) error
	GetFieldLocks(ctx context.Context, tenantID pgUUID) ([]dbstore.TenantFieldLock, error)
}
