package validation

import (
	"context"

	"github.com/opendecree/decree/internal/storage/domain"
)

// SchemaStoreAdapter adapts a schema store (which uses schema-specific
// parameter types) to satisfy the validator [Store] interface.
// This is needed when the schema and config stores are separate instances
// (e.g., in-memory mode) rather than the same database.
type SchemaStoreAdapter struct {
	GetTenantByIDFn    func(ctx context.Context, id string) (domain.Tenant, error)
	GetTenantByNameFn  func(ctx context.Context, name string) (domain.Tenant, error)
	GetSchemaVersionFn func(ctx context.Context, schemaID string, version int32) (domain.SchemaVersion, error)
	GetSchemaFieldsFn  func(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error)
}

func (a *SchemaStoreAdapter) GetTenantByID(ctx context.Context, id string) (domain.Tenant, error) {
	return a.GetTenantByIDFn(ctx, id)
}

func (a *SchemaStoreAdapter) GetTenantByName(ctx context.Context, name string) (domain.Tenant, error) {
	return a.GetTenantByNameFn(ctx, name)
}

func (a *SchemaStoreAdapter) GetSchemaVersion(ctx context.Context, arg domain.SchemaVersionKey) (domain.SchemaVersion, error) {
	return a.GetSchemaVersionFn(ctx, arg.SchemaID, arg.Version)
}

func (a *SchemaStoreAdapter) GetSchemaFields(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error) {
	return a.GetSchemaFieldsFn(ctx, schemaVersionID)
}
