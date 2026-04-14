package validation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

// mockValidationStore implements the validation.Store interface for testing.
type mockValidationStore struct {
	getTenantByIDFn    func(ctx context.Context, id string) (domain.Tenant, error)
	getSchemaVersionFn func(ctx context.Context, arg domain.SchemaVersionKey) (domain.SchemaVersion, error)
	getSchemaFieldsFn  func(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error)
}

func (m *mockValidationStore) GetTenantByID(ctx context.Context, id string) (domain.Tenant, error) {
	return m.getTenantByIDFn(ctx, id)
}

func (m *mockValidationStore) GetSchemaVersion(ctx context.Context, arg domain.SchemaVersionKey) (domain.SchemaVersion, error) {
	return m.getSchemaVersionFn(ctx, arg)
}

func (m *mockValidationStore) GetSchemaFields(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error) {
	return m.getSchemaFieldsFn(ctx, schemaVersionID)
}

const (
	testTenantID        = "t-001"
	testSchemaID        = "s-001"
	testSchemaVersionID = "sv-001"
)

func newMockStore() *mockValidationStore {
	return &mockValidationStore{
		getTenantByIDFn: func(_ context.Context, _ string) (domain.Tenant, error) {
			return domain.Tenant{
				ID:            testTenantID,
				SchemaID:      testSchemaID,
				SchemaVersion: 1,
			}, nil
		},
		getSchemaVersionFn: func(_ context.Context, _ domain.SchemaVersionKey) (domain.SchemaVersion, error) {
			return domain.SchemaVersion{ID: testSchemaVersionID}, nil
		},
		getSchemaFieldsFn: func(_ context.Context, _ string) ([]domain.SchemaField, error) {
			return []domain.SchemaField{
				{Path: "app.name", FieldType: domain.FieldTypeString},
				{Path: "app.retries", FieldType: domain.FieldTypeInteger, Constraints: []byte(`{"min":0,"max":10}`)},
			}, nil
		},
	}
}

// --- NewValidatorFactory ---

func TestNewValidatorFactory(t *testing.T) {
	store := newMockStore()
	f := NewValidatorFactory(store)

	require.NotNil(t, f)
	require.NotNil(t, f.Cache())
}

// --- GetValidators: cache miss → builds validators ---

func TestGetValidators_CacheMiss_BuildsValidators(t *testing.T) {
	store := newMockStore()
	f := NewValidatorFactory(store)
	ctx := context.Background()

	validators, err := f.GetValidators(ctx, testTenantID)
	require.NoError(t, err)
	require.Len(t, validators, 2)

	// Verify app.name validator.
	nameV, ok := validators["app.name"]
	require.True(t, ok)
	assert.Equal(t, pb.FieldType_FIELD_TYPE_STRING, nameV.FieldType())

	// Verify app.retries validator with constraints.
	retriesV, ok := validators["app.retries"]
	require.True(t, ok)
	assert.Equal(t, pb.FieldType_FIELD_TYPE_INT, retriesV.FieldType())

	// Constraints should be applied: value 5 should pass, 99 should fail.
	require.NoError(t, retriesV.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 5}}))
	assert.Error(t, retriesV.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 99}}))
}

// --- GetValidators: cache hit → returns cached ---

func TestGetValidators_CacheHit_ReturnsCached(t *testing.T) {
	store := newMockStore()
	callCount := 0
	store.getTenantByIDFn = func(_ context.Context, _ string) (domain.Tenant, error) {
		callCount++
		return domain.Tenant{
			ID:            testTenantID,
			SchemaID:      testSchemaID,
			SchemaVersion: 1,
		}, nil
	}

	f := NewValidatorFactory(store)
	ctx := context.Background()

	// First call — cache miss.
	v1, err := f.GetValidators(ctx, testTenantID)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second call — cache hit, store should NOT be called again.
	v2, err := f.GetValidators(ctx, testTenantID)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Should return the same map.
	assert.Equal(t, v1, v2)
}

// --- GetValidators: error paths ---

func TestGetValidators_TenantNotFound(t *testing.T) {
	store := newMockStore()
	store.getTenantByIDFn = func(_ context.Context, _ string) (domain.Tenant, error) {
		return domain.Tenant{}, domain.ErrNotFound
	}

	f := NewValidatorFactory(store)
	_, err := f.GetValidators(context.Background(), "unknown-tenant")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetValidators_SchemaVersionError(t *testing.T) {
	store := newMockStore()
	store.getSchemaVersionFn = func(_ context.Context, _ domain.SchemaVersionKey) (domain.SchemaVersion, error) {
		return domain.SchemaVersion{}, errors.New("db error")
	}

	f := NewValidatorFactory(store)
	_, err := f.GetValidators(context.Background(), testTenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestGetValidators_GetSchemaFieldsError(t *testing.T) {
	store := newMockStore()
	store.getSchemaFieldsFn = func(_ context.Context, _ string) ([]domain.SchemaField, error) {
		return nil, errors.New("fields error")
	}

	f := NewValidatorFactory(store)
	_, err := f.GetValidators(context.Background(), testTenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fields error")
}

// --- GetValidators: no constraints ---

func TestGetValidators_FieldWithoutConstraints(t *testing.T) {
	store := newMockStore()
	store.getSchemaFieldsFn = func(_ context.Context, _ string) ([]domain.SchemaField, error) {
		return []domain.SchemaField{
			{Path: "simple.flag", FieldType: domain.FieldTypeBool, Nullable: true},
		}, nil
	}

	f := NewValidatorFactory(store)
	validators, err := f.GetValidators(context.Background(), testTenantID)
	require.NoError(t, err)
	require.Len(t, validators, 1)

	v := validators["simple.flag"]
	require.NotNil(t, v)
	assert.Equal(t, pb.FieldType_FIELD_TYPE_BOOL, v.FieldType())
	// Nullable field should accept nil.
	require.NoError(t, v.Validate(nil))
}

// --- Cache() accessor ---

func TestCache_ReturnsUnderlyingCache(t *testing.T) {
	store := newMockStore()
	f := NewValidatorFactory(store)

	c := f.Cache()
	require.NotNil(t, c)

	// Should be able to manually set/get.
	v := map[string]*FieldValidator{
		"x": NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, false, nil),
	}
	c.Set("manual-tenant", v)

	got, ok := c.Get("manual-tenant")
	require.True(t, ok)
	assert.Len(t, got, 1)

	// Invalidation should work.
	c.Invalidate("manual-tenant")
	_, ok = c.Get("manual-tenant")
	assert.False(t, ok)
}

// --- SchemaStoreAdapter ---

func TestSchemaStoreAdapter_GetTenantByID(t *testing.T) {
	expected := domain.Tenant{ID: "t1", Name: "Test Tenant", SchemaID: "s1", SchemaVersion: 1}
	adapter := &SchemaStoreAdapter{
		GetTenantByIDFn: func(_ context.Context, id string) (domain.Tenant, error) {
			assert.Equal(t, "t1", id)
			return expected, nil
		},
		GetSchemaVersionFn: func(_ context.Context, _ string, _ int32) (domain.SchemaVersion, error) {
			return domain.SchemaVersion{}, nil
		},
		GetSchemaFieldsFn: func(_ context.Context, _ string) ([]domain.SchemaField, error) {
			return nil, nil
		},
	}

	got, err := adapter.GetTenantByID(context.Background(), "t1")
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestSchemaStoreAdapter_GetSchemaVersion(t *testing.T) {
	expected := domain.SchemaVersion{ID: "sv1", SchemaID: "s1", Version: 2}
	adapter := &SchemaStoreAdapter{
		GetTenantByIDFn: func(_ context.Context, _ string) (domain.Tenant, error) {
			return domain.Tenant{}, nil
		},
		GetSchemaVersionFn: func(_ context.Context, schemaID string, version int32) (domain.SchemaVersion, error) {
			assert.Equal(t, "s1", schemaID)
			assert.Equal(t, int32(2), version)
			return expected, nil
		},
		GetSchemaFieldsFn: func(_ context.Context, _ string) ([]domain.SchemaField, error) {
			return nil, nil
		},
	}

	got, err := adapter.GetSchemaVersion(context.Background(), domain.SchemaVersionKey{SchemaID: "s1", Version: 2})
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestSchemaStoreAdapter_GetSchemaFields(t *testing.T) {
	expected := []domain.SchemaField{
		{Path: "a.b", FieldType: domain.FieldTypeString},
	}
	adapter := &SchemaStoreAdapter{
		GetTenantByIDFn: func(_ context.Context, _ string) (domain.Tenant, error) {
			return domain.Tenant{}, nil
		},
		GetSchemaVersionFn: func(_ context.Context, _ string, _ int32) (domain.SchemaVersion, error) {
			return domain.SchemaVersion{}, nil
		},
		GetSchemaFieldsFn: func(_ context.Context, svID string) ([]domain.SchemaField, error) {
			assert.Equal(t, "sv-123", svID)
			return expected, nil
		},
	}

	got, err := adapter.GetSchemaFields(context.Background(), "sv-123")
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestSchemaStoreAdapter_ErrorPropagation(t *testing.T) {
	expectedErr := errors.New("not found")
	adapter := &SchemaStoreAdapter{
		GetTenantByIDFn: func(_ context.Context, _ string) (domain.Tenant, error) {
			return domain.Tenant{}, expectedErr
		},
		GetSchemaVersionFn: func(_ context.Context, _ string, _ int32) (domain.SchemaVersion, error) {
			return domain.SchemaVersion{}, expectedErr
		},
		GetSchemaFieldsFn: func(_ context.Context, _ string) ([]domain.SchemaField, error) {
			return nil, expectedErr
		},
	}

	_, err := adapter.GetTenantByID(context.Background(), "x")
	assert.ErrorIs(t, err, expectedErr)

	_, err = adapter.GetSchemaVersion(context.Background(), domain.SchemaVersionKey{SchemaID: "s", Version: 1})
	assert.ErrorIs(t, err, expectedErr)

	_, err = adapter.GetSchemaFields(context.Background(), "sv")
	assert.ErrorIs(t, err, expectedErr)
}

// --- SchemaStoreAdapter satisfies Store interface ---

func TestSchemaStoreAdapter_ImplementsStore(t *testing.T) {
	adapter := &SchemaStoreAdapter{
		GetTenantByIDFn: func(_ context.Context, _ string) (domain.Tenant, error) {
			return domain.Tenant{}, nil
		},
		GetSchemaVersionFn: func(_ context.Context, _ string, _ int32) (domain.SchemaVersion, error) {
			return domain.SchemaVersion{}, nil
		},
		GetSchemaFieldsFn: func(_ context.Context, _ string) ([]domain.SchemaField, error) {
			return nil, nil
		},
	}

	// Compile-time check that SchemaStoreAdapter implements Store.
	var _ Store = adapter

	// Can be used as a Store in ValidatorFactory.
	f := NewValidatorFactory(adapter)
	require.NotNil(t, f)
}
