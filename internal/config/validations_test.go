package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

// validationsJSON is the wire encoding the schema package emits for one
// cross-field rule.
const validationsJSON = `[{"path":"payments","rule":"self.payments.min_amount < self.payments.max_amount","message":"min must be less than max"}]`

// setupValidationsService wires the validator factory with a tenant whose
// schema declares one CEL validation rule.
func setupValidationsService(t *testing.T) (*Service, *mockStore) {
	t.Helper()
	svc, store := newTestServiceWithValidation()
	store.On("GetTenantByID", mock.Anything, tenantID1).Return(domain.Tenant{
		ID:            tenantID1,
		Name:          "tenant-one",
		SchemaID:      schemaID10,
		SchemaVersion: 1,
	}, nil)
	store.On("GetSchemaVersion", mock.Anything, domain.SchemaVersionKey{
		SchemaID: schemaID10,
		Version:  1,
	}).Return(domain.SchemaVersion{
		ID:          schemaVersionID,
		SchemaID:    schemaID10,
		Version:     1,
		Validations: []byte(validationsJSON),
	}, nil)
	store.On("GetSchemaFields", mock.Anything, schemaVersionID).Return([]domain.SchemaField{
		{Path: "payments.min_amount", FieldType: domain.FieldTypeNumber},
		{Path: "payments.max_amount", FieldType: domain.FieldTypeNumber},
	}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound).Maybe()
	return svc, store
}

func TestSetField_Validations_RuleFires_Rejected(t *testing.T) {
	svc, store := setupValidationsService(t)
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 2}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	// Post-merge snapshot: min equals max — violates `min < max`.
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  2,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "payments.min_amount", Value: strPtr("100")},
		{FieldPath: "payments.max_amount", Value: strPtr("100")},
	}, nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.min_amount",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 100}},
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "min must be less than max")
}

func TestSetField_Validations_RulePasses_Allowed(t *testing.T) {
	svc, store := setupValidationsService(t)
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 2}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  2,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "payments.min_amount", Value: strPtr("10")},
		{FieldPath: "payments.max_amount", Value: strPtr("100")},
	}, nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).
		Return(nil)
	svc.cache.(*mockCache).On("Invalidate", ctx, tenantID1).Return(nil)
	svc.publisher.(*mockPublisher).On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).
		Return(nil)

	resp, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.min_amount",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 10}},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.ConfigVersion.Version)
}

func TestSetFields_Validations_PostMergeStateChecked(t *testing.T) {
	// SetFields must evaluate the rule against the FINAL state — an
	// intermediate-violating ordering is tolerated as long as the
	// post-merge snapshot satisfies the rule.
	svc, store := setupValidationsService(t)
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 2}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  2,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "payments.min_amount", Value: strPtr("10")},
		{FieldPath: "payments.max_amount", Value: strPtr("100")},
	}, nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).
		Return(nil)
	svc.cache.(*mockCache).On("Invalidate", ctx, tenantID1).Return(nil)
	svc.publisher.(*mockPublisher).On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).
		Return(nil).Maybe()

	resp, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
		TenantId: tenantID1,
		Updates: []*pb.FieldUpdate{
			{FieldPath: "payments.min_amount", Value: &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 10}}},
			{FieldPath: "payments.max_amount", Value: &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 100}}},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.ConfigVersion.Version)
}

func TestValidatorFactory_GetCelArtifacts_RebuildsEnvAfterInvalidate(t *testing.T) {
	svc, _ := setupValidationsService(t)
	envBefore, programsBefore, err := svc.validators.GetCelArtifacts(context.Background(), tenantID1)
	require.NoError(t, err)
	require.NotNil(t, envBefore)
	require.Len(t, programsBefore, 1)

	svc.validators.InvalidateRules(tenantID1)

	envAfter, programsAfter, err := svc.validators.GetCelArtifacts(context.Background(), tenantID1)
	require.NoError(t, err)
	require.NotNil(t, envAfter)
	require.Len(t, programsAfter, 1)
	// The factory drops its per-tenant env so the next call rebuilds it.
	// Underlying programs are keyed by (schemaID, schemaVersion, ruleIndex)
	// in the shared celpkg.Cache and are reused as long as the schema
	// version has not moved — that is the correct behaviour for an
	// invalidation that does not bump the version.
	assert.NotSame(t, envBefore, envAfter, "env must be rebuilt after invalidation")
	assert.Same(t, programsBefore[0], programsAfter[0],
		"programs are pinned to (schema, version, ruleIndex) and stay cached across tenant invalidation")
}
