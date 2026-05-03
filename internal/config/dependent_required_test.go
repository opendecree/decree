package config

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

// dependentRequiredJSON is the wire encoding the schema package emits.
const dependentRequiredJSON = `[{"trigger_field":"payments.refunds_enabled","dependent_fields":["payments.refund_window"]}]`

// setupDependentRequiredService wires the validator factory with a tenant
// whose schema declares one dependentRequired rule:
//
//	payments.refunds_enabled → [payments.refund_window]
func setupDependentRequiredService(t *testing.T) (*Service, *mockStore) {
	t.Helper()
	svc, store := newTestServiceWithValidation()
	store.On("GetTenantByID", mock.Anything, tenantID1).Return(domain.Tenant{
		ID:            tenantID1,
		SchemaID:      schemaID10,
		SchemaVersion: 1,
	}, nil)
	store.On("GetSchemaVersion", mock.Anything, domain.SchemaVersionKey{
		SchemaID: schemaID10,
		Version:  1,
	}).Return(domain.SchemaVersion{
		ID:                schemaVersionID,
		SchemaID:          schemaID10,
		Version:           1,
		DependentRequired: []byte(dependentRequiredJSON),
	}, nil)
	// validateField (per-field type/constraint check) also reaches into the
	// validator factory, which calls GetSchemaFields. Mock the schema's
	// field set so writes can pass field-level validation.
	store.On("GetSchemaFields", mock.Anything, schemaVersionID).Return([]domain.SchemaField{
		{Path: "payments.refunds_enabled", FieldType: domain.FieldTypeBool, Nullable: true},
		{Path: "payments.refund_window", FieldType: domain.FieldTypeDuration, Nullable: true},
		{Path: "payments.fee", FieldType: domain.FieldTypeString, Nullable: true},
	}, nil)
	// SetField records an "old value" for audit via getCurrentValue, which
	// goes through GetConfigValueAtVersion. Default to NotFound so the audit
	// trail records empty old values; tests can override per case.
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound).Maybe()
	return svc, store
}

// TestSetField_DependentRequired_TriggerSetWithoutDependent_Rejected covers
// the core failure mode: setting the trigger to a non-null value while the
// dependent path is null must return InvalidArgument.
func TestSetField_DependentRequired_TriggerSetWithoutDependent_Rejected(t *testing.T) {
	svc, store := setupDependentRequiredService(t)
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 1}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	// Snapshot at the new version: trigger is set (we just wrote it), dependent absent.
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  1,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "payments.refunds_enabled", Value: strPtr("true")},
	}, nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.refunds_enabled",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}},
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "payments.refund_window")
}

// TestSetField_DependentRequired_BothPresent_Allowed verifies the rule is
// satisfied when both trigger and dependent are present in the post-merge
// snapshot.
func TestSetField_DependentRequired_BothPresent_Allowed(t *testing.T) {
	svc, store := setupDependentRequiredService(t)
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
		{FieldPath: "payments.refunds_enabled", Value: strPtr("true")},
		{FieldPath: "payments.refund_window", Value: strPtr("30s")},
	}, nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).
		Return(nil)
	// Cache + publish are post-tx; service obtains them from svc fields.
	// The default newTestServiceWithValidation wires real cache/publisher mocks.
	svc.cache.(*mockCache).On("Invalidate", ctx, tenantID1).Return(nil)
	svc.publisher.(*mockPublisher).On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).
		Return(nil)

	resp, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.refunds_enabled",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.ConfigVersion.Version)
}

// TestSetField_DependentRequired_TriggerAbsent_Allowed verifies the rule
// does NOT fire when the trigger is null in the post-merge snapshot, even
// if the dependent is also absent.
func TestSetField_DependentRequired_TriggerAbsent_Allowed(t *testing.T) {
	svc, store := setupDependentRequiredService(t)
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 1}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	// Writing some other field; trigger never set; dependent never set.
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  1,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "payments.fee", Value: strPtr("0.5")},
	}, nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).
		Return(nil)
	svc.cache.(*mockCache).On("Invalidate", ctx, tenantID1).Return(nil)
	svc.publisher.(*mockPublisher).On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).
		Return(nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.fee",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
	})
	require.NoError(t, err)
}

// TestSetField_DependentRequired_TriggerSetToNull_Allowed verifies that
// setting the trigger to null clears the requirement — a null write
// produces a row with Value == nil in the snapshot, which the presence
// builder treats as absent.
func TestSetField_DependentRequired_TriggerSetToNull_Allowed(t *testing.T) {
	svc, store := setupDependentRequiredService(t)
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 2}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	// Trigger null in snapshot (the SetConfigValue stored Value=nil because TypedValue is nil/null).
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  2,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "payments.refunds_enabled", Value: nil},
	}, nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).
		Return(nil)
	svc.cache.(*mockCache).On("Invalidate", ctx, tenantID1).Return(nil)
	svc.publisher.(*mockPublisher).On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).
		Return(nil)

	// Null TypedValue (nil) — clears the trigger.
	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.refunds_enabled",
		Value:     nil,
	})
	require.NoError(t, err)
}

// TestSetFields_DependentRequired_AggregateCheck verifies the multi-field
// path runs the check once over the post-merge snapshot, not per field.
func TestSetFields_DependentRequired_AggregateCheck(t *testing.T) {
	svc, store := setupDependentRequiredService(t)
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 1}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil).Twice()
	// Snapshot reflects both writes — trigger AND dependent set together.
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  1,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "payments.refunds_enabled", Value: strPtr("true")},
		{FieldPath: "payments.refund_window", Value: strPtr("30s")},
	}, nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).
		Return(nil).Twice()
	svc.cache.(*mockCache).On("Invalidate", ctx, tenantID1).Return(nil)
	svc.publisher.(*mockPublisher).On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).
		Return(nil)

	_, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
		TenantId: tenantID1,
		Updates: []*pb.FieldUpdate{
			{
				FieldPath: "payments.refunds_enabled",
				Value:     &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}},
			},
			{
				FieldPath: "payments.refund_window",
				Value:     &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(30 * time.Second)}},
			},
		},
	})
	require.NoError(t, err)
	// One snapshot read for one rule check.
	store.AssertNumberOfCalls(t, "GetFullConfigAtVersion", 1)
}

// TestEnforceDependentRequiredInTx_NoRules_NoSnapshotRead verifies that the
// helper short-circuits without touching the store when the rules slice is
// empty. Important for the hot path of writes against schemas without
// dependentRequired.
func TestEnforceDependentRequiredInTx_NoRules_NoSnapshotRead(t *testing.T) {
	svc, _, _, _ := newTestService()
	store := &mockStore{}
	err := svc.enforceDependentRequiredInTx(context.Background(), store, tenantID1, 5, nil)
	require.NoError(t, err)
	store.AssertNotCalled(t, "GetFullConfigAtVersion")
}

// TestRollbackToVersion_DependentRequired_Rejected verifies the rollback
// path runs the cross-field check against the snapshot built from the
// rollback target. A target version that satisfied the rules at write time
// can still violate them after a schema upgrade introduces new rules — in
// which case rollback must be rejected, not silently produce inconsistent
// state.
func TestRollbackToVersion_DependentRequired_Rejected(t *testing.T) {
	svc, store := setupDependentRequiredService(t)
	ctx := superadminCtx()

	// Rollback target (version 2) had only the trigger set; dependent never existed.
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  2,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "payments.refunds_enabled", Value: strPtr("true")},
	}, nil).Once()
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 5}, nil)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID3, TenantID: tenantID1, Version: 6}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	// Post-rollback snapshot mirrors the target — same violation.
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  6,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "payments.refunds_enabled", Value: strPtr("true")},
	}, nil).Once()

	_, err := svc.RollbackToVersion(ctx, &pb.RollbackToVersionRequest{
		TenantId: tenantID1,
		Version:  2,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "payments.refund_window")
}

// TestImportConfig_DependentRequired_Rejected verifies a YAML import that
// introduces a trigger value without its required dependent is rejected
// with InvalidArgument.
func TestImportConfig_DependentRequired_Rejected(t *testing.T) {
	svc, store := setupDependentRequiredService(t)
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 1}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).
		Return(nil).Maybe()
	// Post-import snapshot: trigger set, dependent absent.
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  1,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "payments.refunds_enabled", Value: strPtr("true")},
	}, nil)

	yamlContent := []byte(`
spec_version: "v1"
values:
  payments.refunds_enabled:
    value: true
`)
	_, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantID1,
		YamlContent: yamlContent,
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "payments.refund_window")
}
