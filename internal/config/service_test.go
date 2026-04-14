package config

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/validation"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

const (
	tenantID1       = "00000001-0000-0000-0000-000000000000"
	versionID2      = "00000002-0000-0000-0000-000000000000"
	versionID3      = "00000003-0000-0000-0000-000000000000"
	schemaID10      = "0000000a-0000-0000-0000-000000000000"
	schemaVersionID = "0000000b-0000-0000-0000-000000000000"
	versionID20     = "00000014-0000-0000-0000-000000000000"
)

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

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 5}, nil)
	cache.On("Get", ctx, tenantID1, int32(5)).
		Return(map[string]string{"payments.fee": "0.5"}, nil)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})

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

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 3}, nil)
	cache.On("Get", ctx, tenantID1, int32(3)).
		Return(nil, nil)
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 3}).
		Return([]GetFullConfigAtVersionRow{
			{FieldPath: "a.b", Value: strPtr("123")},
		}, nil)
	cache.On("Set", ctx, tenantID1, int32(3), mock.AnythingOfType("map[string]string"), mock.Anything).
		Return(nil)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})

	require.NoError(t, err)
	assert.Len(t, resp.Config.Values, 1)
	cache.AssertCalled(t, "Set", ctx, tenantID1, int32(3), mock.AnythingOfType("map[string]string"), mock.Anything)
}

func TestGetConfig_IncludeDescriptions_BypassesCache(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	desc := "fee per transaction"

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 1}).
		Return([]GetFullConfigAtVersionRow{
			{FieldPath: "fee", Value: strPtr("0.5"), Description: &desc},
		}, nil)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{
		TenantId:            tenantID1,
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

	store.On("GetFieldLocks", ctx, tenantID1).
		Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 1, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	cache.On("Invalidate", ctx, tenantID1).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)

	resp, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.fee",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
	})

	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.ConfigVersion.Version)
	cache.AssertCalled(t, "Invalidate", ctx, tenantID1)
	pub.AssertCalled(t, "Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent"))
}

func TestSetField_ChecksumMismatch(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	wrongChecksum := "wrong"

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", ctx, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{Value: strPtr("old-value")}, nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:         tenantID1,
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

	store.On("GetFieldLocks", ctx, tenantID1).
		Return([]domain.TenantFieldLock{
			{TenantID: tenantID1, FieldPath: "payments.fee"},
		}, nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
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

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", ctx, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)

	_, err := svc.GetField(ctx, &pb.GetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "nonexistent",
	})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- RollbackToVersion ---

func TestRollbackToVersion_Success(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 2}).
		Return([]GetFullConfigAtVersionRow{
			{FieldPath: "a", Value: strPtr("1")},
			{FieldPath: "b", Value: strPtr("2")},
		}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 5}, nil)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID3, TenantID: tenantID1, Version: 6, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	cache.On("Invalidate", ctx, tenantID1).Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)

	resp, err := svc.RollbackToVersion(ctx, &pb.RollbackToVersionRequest{
		TenantId: tenantID1,
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

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 3}, nil)
	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "payments.fee", FieldType: domain.FieldTypeNumber},
			{Path: "payments.enabled", FieldType: domain.FieldTypeBool},
		}, nil)
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 3}).
		Return([]GetFullConfigAtVersionRow{
			{FieldPath: "payments.fee", Value: strPtr("0.025")},
			{FieldPath: "payments.enabled", Value: strPtr("true")},
		}, nil)
	desc := "version 3"
	store.On("GetConfigVersion", ctx, GetConfigVersionParams{TenantID: tenantID1, Version: 3}).
		Return(domain.ConfigVersion{Version: 3, Description: &desc}, nil)

	resp, err := svc.ExportConfig(ctx, &pb.ExportConfigRequest{TenantId: tenantID1})

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

	yamlContent := []byte(`
syntax: "v1"
description: "imported config"
values:
  payments.fee:
    value: 0.05
  payments.enabled:
    value: true
`)

	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "payments.fee", FieldType: domain.FieldTypeNumber},
			{Path: "payments.enabled", FieldType: domain.FieldTypeBool},
		}, nil)
	store.On("GetFieldLocks", ctx, tenantID1).
		Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 2}, nil)
	store.On("GetConfigValueAtVersion", ctx, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID20, TenantID: tenantID1, Version: 3, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).
		Return(nil)
	cache.On("Invalidate", ctx, tenantID1).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)

	resp, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantID1,
		YamlContent: yamlContent,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.ConfigVersion.Version)
	store.AssertNumberOfCalls(t, "SetConfigValue", 2)
	store.AssertNumberOfCalls(t, "InsertAuditWriteLog", 2)
	cache.AssertCalled(t, "Invalidate", ctx, tenantID1)
}

// --- ImportConfig with validation ---

func TestImportConfig_ValidationRejectsUnknownField(t *testing.T) {
	svc, store := newTestServiceWithValidation()
	ctx := context.Background()

	yamlContent := []byte(`
syntax: "v1"
values:
  unknown.field:
    value: "hello"
`)

	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "known.field", FieldType: domain.FieldTypeString},
		}, nil)
	store.On("GetFieldLocks", ctx, tenantID1).
		Return([]domain.TenantFieldLock{}, nil)

	_, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantID1,
		YamlContent: yamlContent,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "not defined")
}

func TestImportConfig_ValidationRejectsConstraintViolation(t *testing.T) {
	svc, store := newTestServiceWithValidation()
	ctx := context.Background()

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

	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "app.retries", FieldType: domain.FieldTypeInteger, Constraints: constraintsJSON},
		}, nil)
	store.On("GetFieldLocks", ctx, tenantID1).
		Return([]domain.TenantFieldLock{}, nil)

	_ = minC
	_ = maxC

	_, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantID1,
		YamlContent: yamlContent,
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "maximum")
}

// --- ImportConfig modes ---

func TestImportConfig_MergeMode_SkipsSameValues(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := context.Background()

	yamlContent := []byte(`
syntax: "v1"
values:
  app.name:
    value: "same"
  app.other:
    value: "changed"
`)

	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "app.name", FieldType: domain.FieldTypeString},
			{Path: "app.other", FieldType: domain.FieldTypeString},
		}, nil)
	store.On("GetFieldLocks", ctx, tenantID1).
		Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)

	// app.name has same value -> should be skipped in merge mode
	store.On("GetConfigValueAtVersion", ctx, mock.MatchedBy(func(p GetConfigValueAtVersionParams) bool {
		return p.FieldPath == "app.name"
	})).Return(GetConfigValueAtVersionRow{Value: strPtr("same")}, nil)

	// app.other has different value -> should be included
	store.On("GetConfigValueAtVersion", ctx, mock.MatchedBy(func(p GetConfigValueAtVersionParams) bool {
		return p.FieldPath == "app.other"
	})).Return(GetConfigValueAtVersionRow{Value: strPtr("old")}, nil)

	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID20, TenantID: tenantID1, Version: 2, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)
	cache.On("Invalidate", ctx, tenantID1).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)

	resp, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantID1,
		YamlContent: yamlContent,
		Mode:        pb.ImportMode_IMPORT_MODE_MERGE,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.ConfigVersion.Version)
	// Only app.other should be set (app.name skipped — same value)
	store.AssertNumberOfCalls(t, "SetConfigValue", 1)
}

func TestImportConfig_DefaultsMode_SkipsExistingValues(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	yamlContent := []byte(`
syntax: "v1"
values:
  app.existing:
    value: "new-from-yaml"
  app.missing:
    value: "default-value"
`)

	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "app.existing", FieldType: domain.FieldTypeString},
			{Path: "app.missing", FieldType: domain.FieldTypeString},
		}, nil)
	store.On("GetFieldLocks", ctx, tenantID1).
		Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)

	// app.existing has a value -> should be skipped in defaults mode
	store.On("GetConfigValueAtVersion", ctx, mock.MatchedBy(func(p GetConfigValueAtVersionParams) bool {
		return p.FieldPath == "app.existing"
	})).Return(GetConfigValueAtVersionRow{Value: strPtr("already-set")}, nil)

	// app.missing has no value -> should be included
	store.On("GetConfigValueAtVersion", ctx, mock.MatchedBy(func(p GetConfigValueAtVersionParams) bool {
		return p.FieldPath == "app.missing"
	})).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)

	newVersionID := versionID20
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: newVersionID, TenantID: tenantID1, Version: 2, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)
	cache := &mockCache{}
	pub := &mockPublisher{}
	svc.cache = cache
	svc.publisher = pub
	cache.On("Invalidate", ctx, tenantID1).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)

	_, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantID1,
		YamlContent: yamlContent,
		Mode:        pb.ImportMode_IMPORT_MODE_DEFAULTS,
	})

	require.NoError(t, err)
	// Only app.missing should be set
	store.AssertNumberOfCalls(t, "SetConfigValue", 1)
}
