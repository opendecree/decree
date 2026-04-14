package validation

import (
	"context"
	"encoding/json"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

// Store defines the read-only data access needed by the validator factory.
// Implementations must return [domain.ErrNotFound] when an entity is not found.
// This interface is a subset of the config.Store — any config store implementation
// automatically satisfies it.
type Store interface {
	GetTenantByID(ctx context.Context, id string) (domain.Tenant, error)
	GetSchemaVersion(ctx context.Context, arg domain.SchemaVersionKey) (domain.SchemaVersion, error)
	GetSchemaFields(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error)
}

// ValidatorFactory builds and caches field validators per tenant.
type ValidatorFactory struct {
	store Store
	cache *ValidatorCache
}

// NewValidatorFactory creates a new validator factory.
func NewValidatorFactory(store Store) *ValidatorFactory {
	return &ValidatorFactory{
		store: store,
		cache: NewValidatorCache(),
	}
}

// Cache returns the underlying cache for invalidation.
func (f *ValidatorFactory) Cache() *ValidatorCache {
	return f.cache
}

// GetValidators returns validators for a tenant's schema fields.
// Results are cached per tenant ID. Returns an error if the tenant or schema is not found.
func (f *ValidatorFactory) GetValidators(ctx context.Context, tenantID string) (map[string]*FieldValidator, error) {
	if cached, ok := f.cache.Get(tenantID); ok {
		return cached, nil
	}

	tenant, err := f.store.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	sv, err := f.store.GetSchemaVersion(ctx, domain.SchemaVersionKey{
		SchemaID: tenant.SchemaID,
		Version:  tenant.SchemaVersion,
	})
	if err != nil {
		return nil, err
	}

	fields, err := f.store.GetSchemaFields(ctx, sv.ID)
	if err != nil {
		return nil, err
	}

	validators := make(map[string]*FieldValidator, len(fields))
	for _, field := range fields {
		ft := field.FieldType.ToProto()
		var constraints *pb.FieldConstraints
		if field.Constraints != nil {
			constraints = &pb.FieldConstraints{}
			_ = json.Unmarshal(field.Constraints, constraints)
		}
		validators[field.Path] = NewFieldValidator(field.Path, ft, field.Nullable, constraints)
	}

	f.cache.Set(tenantID, validators)
	return validators, nil
}
