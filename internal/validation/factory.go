package validation

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
)

// Store defines the data access needed by the validator factory.
type Store interface {
	GetTenantByID(ctx context.Context, id pgtype.UUID) (dbstore.Tenant, error)
	GetSchemaVersion(ctx context.Context, arg dbstore.GetSchemaVersionParams) (dbstore.SchemaVersion, error)
	GetSchemaFields(ctx context.Context, schemaVersionID pgtype.UUID) ([]dbstore.SchemaField, error)
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
func (f *ValidatorFactory) GetValidators(ctx context.Context, tenantID pgtype.UUID, tenantIDStr string) (map[string]*FieldValidator, error) {
	if cached, ok := f.cache.Get(tenantIDStr); ok {
		return cached, nil
	}

	tenant, err := f.store.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	sv, err := f.store.GetSchemaVersion(ctx, dbstore.GetSchemaVersionParams{
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
		ft := dbFieldTypeToProto(field.FieldType)
		var constraints *pb.FieldConstraints
		if field.Constraints != nil {
			constraints = &pb.FieldConstraints{}
			_ = json.Unmarshal(field.Constraints, constraints)
		}
		validators[field.Path] = NewFieldValidator(field.Path, ft, field.Nullable, constraints)
	}

	f.cache.Set(tenantIDStr, validators)
	return validators, nil
}

// dbFieldTypeToProto converts a DB field type to proto enum.
func dbFieldTypeToProto(t dbstore.FieldType) pb.FieldType {
	switch t {
	case dbstore.FieldTypeInteger:
		return pb.FieldType_FIELD_TYPE_INT
	case dbstore.FieldTypeNumber:
		return pb.FieldType_FIELD_TYPE_NUMBER
	case dbstore.FieldTypeString:
		return pb.FieldType_FIELD_TYPE_STRING
	case dbstore.FieldTypeBool:
		return pb.FieldType_FIELD_TYPE_BOOL
	case dbstore.FieldTypeTime:
		return pb.FieldType_FIELD_TYPE_TIME
	case dbstore.FieldTypeDuration:
		return pb.FieldType_FIELD_TYPE_DURATION
	case dbstore.FieldTypeUrl:
		return pb.FieldType_FIELD_TYPE_URL
	case dbstore.FieldTypeJson:
		return pb.FieldType_FIELD_TYPE_JSON
	default:
		return pb.FieldType_FIELD_TYPE_UNSPECIFIED
	}
}
