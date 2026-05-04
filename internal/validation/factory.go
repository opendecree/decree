package validation

import (
	"context"
	"encoding/json"
	"sync"

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
	store      Store
	cache      *ValidatorCache
	rulesCache sync.Map // tenantID → []byte (raw dependent_required JSON)
	limits     Limits
}

// NewValidatorFactory creates a new validator factory. Pass [WithLimits]
// to override the JSON-Schema compile defaults.
func NewValidatorFactory(store Store, opts ...Option) *ValidatorFactory {
	o := resolveOptions(opts)
	return &ValidatorFactory{
		store:  store,
		cache:  NewValidatorCache(0),
		limits: o.limits,
	}
}

// Cache returns the underlying cache for invalidation.
func (f *ValidatorFactory) Cache() *ValidatorCache {
	return f.cache
}

// InvalidateRules drops the cached dependentRequired bytes for a tenant.
// Call this alongside Cache().Invalidate() whenever a tenant's schema
// version changes.
func (f *ValidatorFactory) InvalidateRules(tenantID string) {
	f.rulesCache.Delete(tenantID)
}

// GetDependentRequired returns the raw JSON-encoded dependentRequired rules
// for a tenant's bound schema version. Returns nil bytes for "no rules";
// callers should treat that as a no-op. Cached per tenant; invalidate via
// InvalidateRules when the tenant's schema binding changes.
//
// Returns []byte rather than the decoded proto type so the validation
// package does not have to import internal/schema for the unmarshal helper
// (avoiding a circular import). Decode at the call site.
func (f *ValidatorFactory) GetDependentRequired(ctx context.Context, tenantID string) ([]byte, error) {
	if v, ok := f.rulesCache.Load(tenantID); ok {
		return v.([]byte), nil
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
	raw := sv.DependentRequired
	f.rulesCache.Store(tenantID, raw)
	return raw, nil
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
		validators[field.Path] = NewFieldValidator(field.Path, ft, field.Nullable, field.Sensitive, constraints, WithLimits(f.limits))
	}

	f.cache.Set(tenantID, validators)
	return validators, nil
}
