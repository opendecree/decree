package config

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"github.com/zeevdr/central-config-service/internal/auth"
	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
	"github.com/zeevdr/central-config-service/internal/validation"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func testUUID(b byte) pgtype.UUID {
	var id pgtype.UUID
	id.Bytes[0] = b
	id.Valid = true
	return id
}

func newTestService() (*Service, *mockStore, *mockCache, *mockPublisher) {
	store := &mockStore{}
	cache := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	svc := NewService(store, cache, pub, sub, testLogger, nil, nil, nil)
	return svc, store, cache, pub
}

func newTestServiceWithValidation() (*Service, *mockStore) {
	store := &mockStore{}
	cache := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	vf := validation.NewValidatorFactory(store)
	svc := NewService(store, cache, pub, sub, testLogger, nil, nil, vf)
	return svc, store
}

// --- GetConfig ---

func TestGetConfig_CacheHit(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)

	store.On("GetLatestConfigVersion", ctx, tenantID).
		Return(dbstore.ConfigVersion{Version: 5}, nil)
	cache.On("Get", ctx, tenantIDStr, int32(5)).
		Return(map[string]string{"payments.fee": "0.5"}, nil)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantIDStr})

	require.NoError(t, err)
	assert.Len(t, resp.Config.Values, 1)
	assert.Equal(t, "payments.fee", resp.Config.Values[0].FieldPath)
	assert.Equal(t, "0.5", typedValueToDisplayString(resp.Config.Values[0].Value))
	// Should not hit DB.
	store.AssertNotCalled(t, "GetFullConfigAtVersion")
	cache.AssertExpectations(t)
}

func TestGetConfig_CacheMiss(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)

	store.On("GetLatestConfigVersion", ctx, tenantID).
		Return(dbstore.ConfigVersion{Version: 3}, nil)
	cache.On("Get", ctx, tenantIDStr, int32(3)).
		Return(nil, nil)
	store.On("GetFullConfigAtVersion", ctx, dbstore.GetFullConfigAtVersionParams{TenantID: tenantID, Version: 3}).
		Return([]dbstore.GetFullConfigAtVersionRow{
			{FieldPath: "a.b", Value: strPtr("123")},
		}, nil)
	cache.On("Set", ctx, tenantIDStr, int32(3), mock.AnythingOfType("map[string]string"), mock.Anything).
		Return(nil)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantIDStr})

	require.NoError(t, err)
	assert.Len(t, resp.Config.Values, 1)
	cache.AssertCalled(t, "Set", ctx, tenantIDStr, int32(3), mock.AnythingOfType("map[string]string"), mock.Anything)
}

func TestGetConfig_IncludeDescriptions_BypassesCache(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)
	desc := "fee per transaction"

	store.On("GetLatestConfigVersion", ctx, tenantID).
		Return(dbstore.ConfigVersion{Version: 1}, nil)
	store.On("GetFullConfigAtVersion", ctx, dbstore.GetFullConfigAtVersionParams{TenantID: tenantID, Version: 1}).
		Return([]dbstore.GetFullConfigAtVersionRow{
			{FieldPath: "fee", Value: strPtr("0.5"), Description: &desc},
		}, nil)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{
		TenantId:            tenantIDStr,
		IncludeDescriptions: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "fee per transaction", *resp.Config.Values[0].Description)
	// Cache should NOT be read or written.
	cache.AssertNotCalled(t, "Get")
	cache.AssertNotCalled(t, "Set")
}

// --- SetField ---

func TestSetField_Success(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := context.Background()

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)
	versionID := testUUID(2)

	store.On("GetFieldLocks", ctx, tenantID).
		Return([]dbstore.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID).
		Return(dbstore.ConfigVersion{}, pgx.ErrNoRows)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("dbstore.CreateConfigVersionParams")).
		Return(dbstore.ConfigVersion{ID: versionID, TenantID: tenantID, Version: 1, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("dbstore.SetConfigValueParams")).
		Return(nil)
	cache.On("Invalidate", ctx, tenantIDStr).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("dbstore.InsertAuditWriteLogParams")).Return(nil)

	resp, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantIDStr,
		FieldPath: "payments.fee",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
	})

	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.ConfigVersion.Version)
	cache.AssertCalled(t, "Invalidate", ctx, tenantIDStr)
	pub.AssertCalled(t, "Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent"))
}

func TestSetField_ChecksumMismatch(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)
	wrongChecksum := "wrong"

	store.On("GetLatestConfigVersion", ctx, tenantID).
		Return(dbstore.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", ctx, mock.AnythingOfType("dbstore.GetConfigValueAtVersionParams")).
		Return(dbstore.GetConfigValueAtVersionRow{Value: strPtr("old-value")}, nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:         tenantIDStr,
		FieldPath:        "payments.fee",
		Value:            &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
		ExpectedChecksum: &wrongChecksum,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Aborted, status.Code(err))
}

func TestSetField_LockedField(t *testing.T) {
	svc, store, _, _ := newTestService()
	// Use admin context — lock checks only apply to non-superadmin.
	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:     auth.RoleAdmin,
		TenantID: "test-tenant",
	})

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)

	store.On("GetFieldLocks", ctx, tenantID).
		Return([]dbstore.TenantFieldLock{
			{TenantID: tenantID, FieldPath: "payments.fee"},
		}, nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantIDStr,
		FieldPath: "payments.fee",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
	})

	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

// --- GetField ---

func TestGetField_NotFound(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)

	store.On("GetLatestConfigVersion", ctx, tenantID).
		Return(dbstore.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", ctx, mock.AnythingOfType("dbstore.GetConfigValueAtVersionParams")).
		Return(dbstore.GetConfigValueAtVersionRow{}, pgx.ErrNoRows)

	_, err := svc.GetField(ctx, &pb.GetFieldRequest{
		TenantId:  tenantIDStr,
		FieldPath: "nonexistent",
	})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- RollbackToVersion ---

func TestRollbackToVersion_Success(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)
	newVersionID := testUUID(3)

	store.On("GetFullConfigAtVersion", ctx, dbstore.GetFullConfigAtVersionParams{TenantID: tenantID, Version: 2}).
		Return([]dbstore.GetFullConfigAtVersionRow{
			{FieldPath: "a", Value: strPtr("1")},
			{FieldPath: "b", Value: strPtr("2")},
		}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID).
		Return(dbstore.ConfigVersion{Version: 5}, nil)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("dbstore.CreateConfigVersionParams")).
		Return(dbstore.ConfigVersion{ID: newVersionID, TenantID: tenantID, Version: 6, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("dbstore.SetConfigValueParams")).
		Return(nil)
	cache.On("Invalidate", ctx, tenantIDStr).Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("dbstore.InsertAuditWriteLogParams")).Return(nil)

	resp, err := svc.RollbackToVersion(ctx, &pb.RollbackToVersionRequest{
		TenantId: tenantIDStr,
		Version:  2,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(6), resp.ConfigVersion.Version)
	// Should copy 2 values.
	store.AssertNumberOfCalls(t, "SetConfigValue", 2)
}

// --- ExportConfig ---

func TestExportConfig_Success(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)
	schemaID := testUUID(10)
	schemaVersionID := testUUID(11)

	store.On("GetLatestConfigVersion", ctx, tenantID).
		Return(dbstore.ConfigVersion{Version: 3}, nil)
	store.On("GetTenantByID", ctx, tenantID).
		Return(dbstore.Tenant{SchemaID: schemaID, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, dbstore.GetSchemaVersionParams{SchemaID: schemaID, Version: 1}).
		Return(dbstore.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]dbstore.SchemaField{
			{Path: "payments.fee", FieldType: dbstore.FieldTypeNumber},
			{Path: "payments.enabled", FieldType: dbstore.FieldTypeBool},
		}, nil)
	store.On("GetFullConfigAtVersion", ctx, dbstore.GetFullConfigAtVersionParams{TenantID: tenantID, Version: 3}).
		Return([]dbstore.GetFullConfigAtVersionRow{
			{FieldPath: "payments.fee", Value: strPtr("0.025")},
			{FieldPath: "payments.enabled", Value: strPtr("true")},
		}, nil)
	desc := "version 3"
	store.On("GetConfigVersion", ctx, dbstore.GetConfigVersionParams{TenantID: tenantID, Version: 3}).
		Return(dbstore.ConfigVersion{Version: 3, Description: &desc}, nil)

	resp, err := svc.ExportConfig(ctx, &pb.ExportConfigRequest{TenantId: tenantIDStr})

	require.NoError(t, err)
	require.NotEmpty(t, resp.YamlContent)

	// Parse and verify typed values
	doc, err := unmarshalConfigYAML(resp.YamlContent)
	require.NoError(t, err)
	assert.Equal(t, int32(3), doc.Version)
	assert.Equal(t, "version 3", doc.Description)
	assert.Equal(t, 0.025, doc.Values["payments.fee"].Value)
	assert.Equal(t, true, doc.Values["payments.enabled"].Value)
}

// --- ImportConfig ---

func TestImportConfig_Success(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := context.Background()

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)
	schemaID := testUUID(10)
	schemaVersionID := testUUID(11)
	newVersionID := testUUID(20)

	yamlContent := []byte(`
syntax: "v1"
description: "imported config"
values:
  payments.fee:
    value: 0.05
  payments.enabled:
    value: true
`)

	store.On("GetTenantByID", ctx, tenantID).
		Return(dbstore.Tenant{SchemaID: schemaID, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, dbstore.GetSchemaVersionParams{SchemaID: schemaID, Version: 1}).
		Return(dbstore.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]dbstore.SchemaField{
			{Path: "payments.fee", FieldType: dbstore.FieldTypeNumber},
			{Path: "payments.enabled", FieldType: dbstore.FieldTypeBool},
		}, nil)
	store.On("GetFieldLocks", ctx, tenantID).
		Return([]dbstore.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID).
		Return(dbstore.ConfigVersion{Version: 2}, nil)
	store.On("GetConfigValueAtVersion", ctx, mock.AnythingOfType("dbstore.GetConfigValueAtVersionParams")).
		Return(dbstore.GetConfigValueAtVersionRow{}, pgx.ErrNoRows)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("dbstore.CreateConfigVersionParams")).
		Return(dbstore.ConfigVersion{ID: newVersionID, TenantID: tenantID, Version: 3, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("dbstore.SetConfigValueParams")).
		Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("dbstore.InsertAuditWriteLogParams")).
		Return(nil)
	cache.On("Invalidate", ctx, tenantIDStr).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)

	resp, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantIDStr,
		YamlContent: yamlContent,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.ConfigVersion.Version)
	store.AssertNumberOfCalls(t, "SetConfigValue", 2)
	store.AssertNumberOfCalls(t, "InsertAuditWriteLog", 2)
	cache.AssertCalled(t, "Invalidate", ctx, tenantIDStr)
}

// --- ImportConfig with validation ---

func TestImportConfig_ValidationRejectsUnknownField(t *testing.T) {
	svc, store := newTestServiceWithValidation()
	ctx := context.Background()

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)
	schemaID := testUUID(10)
	schemaVersionID := testUUID(11)

	yamlContent := []byte(`
syntax: "v1"
values:
  unknown.field:
    value: "hello"
`)

	store.On("GetTenantByID", ctx, tenantID).
		Return(dbstore.Tenant{SchemaID: schemaID, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, dbstore.GetSchemaVersionParams{SchemaID: schemaID, Version: 1}).
		Return(dbstore.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]dbstore.SchemaField{
			{Path: "known.field", FieldType: dbstore.FieldTypeString},
		}, nil)
	store.On("GetFieldLocks", ctx, tenantID).
		Return([]dbstore.TenantFieldLock{}, nil)

	_, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantIDStr,
		YamlContent: yamlContent,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "not defined")
}

func TestImportConfig_ValidationRejectsConstraintViolation(t *testing.T) {
	svc, store := newTestServiceWithValidation()
	ctx := context.Background()

	tenantID := testUUID(1)
	tenantIDStr := uuidToString(tenantID)
	schemaID := testUUID(10)
	schemaVersionID := testUUID(11)

	// Import an integer value that exceeds max constraint.
	yamlContent := []byte(`
syntax: "v1"
values:
  app.retries:
    value: 99
`)

	minC := float64(0)
	maxC := float64(10)
	constraintsJSON := []byte(`{"min":0,"max":10}`)

	store.On("GetTenantByID", ctx, tenantID).
		Return(dbstore.Tenant{SchemaID: schemaID, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, dbstore.GetSchemaVersionParams{SchemaID: schemaID, Version: 1}).
		Return(dbstore.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]dbstore.SchemaField{
			{Path: "app.retries", FieldType: dbstore.FieldTypeInteger, Constraints: constraintsJSON},
		}, nil)
	store.On("GetFieldLocks", ctx, tenantID).
		Return([]dbstore.TenantFieldLock{}, nil)

	_ = minC
	_ = maxC

	_, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantIDStr,
		YamlContent: yamlContent,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "maximum")
}
