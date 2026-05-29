package validation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

func mustMarshalRules(t *testing.T, rules []*pb.ValidationRule) []byte {
	t.Helper()
	b, err := json.Marshal(rules)
	require.NoError(t, err)
	return b
}

func TestGetValidations_CacheMiss_DecodesRules(t *testing.T) {
	rules := []*pb.ValidationRule{{Rule: "self.app.retries > 0", Message: "must be positive"}}
	store := newMockStore()
	store.getSchemaVersionFn = func(_ context.Context, _ domain.SchemaVersionKey) (domain.SchemaVersion, error) {
		return domain.SchemaVersion{ID: testSchemaVersionID, Validations: mustMarshalRules(t, rules)}, nil
	}

	f := NewValidatorFactory(store)
	got, err := f.GetValidations(context.Background(), testTenantID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "self.app.retries > 0", got[0].GetRule())
}

func TestGetValidations_CacheHit_SkipsStore(t *testing.T) {
	calls := 0
	store := newMockStore()
	store.getTenantByIDFn = func(_ context.Context, _ string) (domain.Tenant, error) {
		calls++
		return domain.Tenant{ID: testTenantID, SchemaID: testSchemaID, SchemaVersion: 1}, nil
	}

	f := NewValidatorFactory(store)
	ctx := context.Background()
	_, err := f.GetValidations(ctx, testTenantID)
	require.NoError(t, err)
	_, err = f.GetValidations(ctx, testTenantID)
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "second call should be served from cache")
}

func TestGetValidations_TenantError(t *testing.T) {
	store := newMockStore()
	store.getTenantByIDFn = func(_ context.Context, _ string) (domain.Tenant, error) {
		return domain.Tenant{}, domain.ErrNotFound
	}
	f := NewValidatorFactory(store)
	_, err := f.GetValidations(context.Background(), testTenantID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetValidations_SchemaVersionError(t *testing.T) {
	store := newMockStore()
	store.getSchemaVersionFn = func(_ context.Context, _ domain.SchemaVersionKey) (domain.SchemaVersion, error) {
		return domain.SchemaVersion{}, errors.New("sv error")
	}
	f := NewValidatorFactory(store)
	_, err := f.GetValidations(context.Background(), testTenantID)
	assert.ErrorContains(t, err, "sv error")
}

func TestGetCelArtifacts_NoRules_ReturnsNil(t *testing.T) {
	store := newMockStore() // default schema version has no Validations
	f := NewValidatorFactory(store)
	env, progs, err := f.GetCelArtifacts(context.Background(), testTenantID)
	require.NoError(t, err)
	assert.Nil(t, env)
	assert.Nil(t, progs)
}

func TestGetCelArtifacts_WithRules_BuildsAndCaches(t *testing.T) {
	rules := []*pb.ValidationRule{{Rule: "self.app.retries > 0", Message: "positive"}}
	store := newMockStore()
	store.getSchemaVersionFn = func(_ context.Context, _ domain.SchemaVersionKey) (domain.SchemaVersion, error) {
		return domain.SchemaVersion{ID: testSchemaVersionID, SchemaID: testSchemaID, Version: 1, Validations: mustMarshalRules(t, rules)}, nil
	}

	f := NewValidatorFactory(store)
	ctx := context.Background()
	env, progs, err := f.GetCelArtifacts(ctx, testTenantID)
	require.NoError(t, err)
	require.NotNil(t, env)
	require.Len(t, progs, 1)

	// Second call hits the env/programs cache and returns the same env.
	env2, progs2, err := f.GetCelArtifacts(ctx, testTenantID)
	require.NoError(t, err)
	assert.Same(t, env, env2)
	assert.Len(t, progs2, 1)
}

func TestGetCelArtifacts_PropagatesValidationsError(t *testing.T) {
	store := newMockStore()
	store.getTenantByIDFn = func(_ context.Context, _ string) (domain.Tenant, error) {
		return domain.Tenant{}, domain.ErrNotFound
	}
	f := NewValidatorFactory(store)
	_, _, err := f.GetCelArtifacts(context.Background(), testTenantID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDecodeValidations(t *testing.T) {
	assert.Nil(t, decodeValidations(nil))
	assert.Nil(t, decodeValidations([]byte{}))
	assert.Nil(t, decodeValidations([]byte("{not json")))

	rules := decodeValidations([]byte(`[{"rule":"self.x > 0","message":"m"}]`))
	require.Len(t, rules, 1)
	assert.Equal(t, "self.x > 0", rules[0].GetRule())
}

func TestGetDependentRequired_CacheHit(t *testing.T) {
	calls := 0
	store := newMockStore()
	store.getTenantByIDFn = func(_ context.Context, _ string) (domain.Tenant, error) {
		calls++
		return domain.Tenant{ID: testTenantID, SchemaID: testSchemaID, SchemaVersion: 1}, nil
	}
	store.getSchemaVersionFn = func(_ context.Context, _ domain.SchemaVersionKey) (domain.SchemaVersion, error) {
		return domain.SchemaVersion{ID: testSchemaVersionID, DependentRequired: []byte(`[{"trigger_field":"a"}]`)}, nil
	}

	f := NewValidatorFactory(store)
	ctx := context.Background()
	raw, err := f.GetDependentRequired(ctx, testTenantID)
	require.NoError(t, err)
	assert.NotEmpty(t, raw)

	raw2, err := f.GetDependentRequired(ctx, testTenantID)
	require.NoError(t, err)
	assert.Equal(t, raw, raw2)
	assert.Equal(t, 1, calls, "second call should be served from cache")
}

func TestSchemaStoreAdapter_GetTenantByName(t *testing.T) {
	expected := domain.Tenant{ID: "t9", Name: "by-name"}
	adapter := &SchemaStoreAdapter{
		GetTenantByNameFn: func(_ context.Context, name string) (domain.Tenant, error) {
			assert.Equal(t, "by-name", name)
			return expected, nil
		},
	}
	got, err := adapter.GetTenantByName(context.Background(), "by-name")
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}
