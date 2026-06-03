package config

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/audit"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/pubsub"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/validation"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func superadminCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.Claims{Role: auth.RoleSuperAdmin})
}

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
	c := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	svc := NewService(store, c, pub, sub,
		WithLogger(testLogger),
	)
	return svc, store, c, pub
}

func newTestServiceWithValidation() (*Service, *mockStore) {
	store := &mockStore{}
	c := &mockCache{}
	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	vf := validation.NewValidatorFactory(store)
	svc := NewService(store, c, pub, sub,
		WithLogger(testLogger),
		WithValidators(vf),
	)
	return svc, store
}

// --- GetConfig ---

func TestGetConfig_CacheHit(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 5}, nil)
	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
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
	ctx := auth.WithoutAuth(context.Background())

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
	setupNoSensitiveFields(store)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})

	require.NoError(t, err)
	assert.Len(t, resp.Config.Values, 1)
	cache.AssertCalled(t, "Set", ctx, tenantID1, int32(3), mock.AnythingOfType("map[string]string"), mock.Anything)
}

func TestGetConfig_IncludeDescriptions_BypassesCache(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	desc := "fee per transaction"

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 1}).
		Return([]GetFullConfigAtVersionRow{
			{FieldPath: "fee", Value: strPtr("0.5"), Description: &desc},
		}, nil)
	setupNoSensitiveFields(store)

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

func TestSetField_EmptyFieldPath(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := superadminCtx()

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	store.AssertNotCalled(t, "SetConfigValue")
}

func TestSetFields_EmptyUpdates(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := superadminCtx()
	_, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
		TenantId: tenantID1,
		Updates:  nil,
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSetFields_EmptyFieldPath(t *testing.T) {
	tests := []struct {
		name     string
		updates  []*pb.FieldUpdate
		wantCode codes.Code
	}{
		{
			"one update empty field_path",
			[]*pb.FieldUpdate{
				{FieldPath: "", Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "v"}}},
			},
			codes.InvalidArgument,
		},
		{
			"second update empty field_path",
			[]*pb.FieldUpdate{
				{FieldPath: "a.b", Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "v"}}},
				{FieldPath: "", Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "v"}}},
			},
			codes.InvalidArgument,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, store, _, _ := newTestService()
			ctx := superadminCtx()

			store.On("GetTenantByID", ctx, tenantID1).
				Return(domain.Tenant{ID: tenantID1, Name: "t"}, nil)
			store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)

			_, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
				TenantId: tenantID1,
				Updates:  tc.updates,
			})

			assert.Equal(t, tc.wantCode, status.Code(err))
		})
	}
}

func TestSetField_Success(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

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
	setupNoSensitiveFields(store)

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

// TestSetField_PreCommitInvalidationClosesStaleWindow is a regression test for
// #431. A GetConfig read that executes after the tx commits but before the
// post-commit invalidation fires can return stale data. The fix adds a
// pre-commit invalidation so the cache is cleared before the commit. This test
// asserts that Invalidate fires exactly twice — once pre-commit and once
// post-commit — for a successful SetField.
func TestSetField_PreCommitInvalidationClosesStaleWindow(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 1, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)
	setupNoSensitiveFields(store)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.fee",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
	})
	require.NoError(t, err)
	cache.AssertNumberOfCalls(t, "Invalidate", 2)
}

func TestSetField_ChecksumMismatch(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := superadminCtx()

	wrongChecksum := "wrong"

	// getOrCreateVersion (outside tx) + txLatestVersion (inside tx).
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	// getCurrentValue (outside tx) + checkChecksumAtVersion (inside tx).
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{Value: strPtr("old-value")}, nil)
	setupNoSensitiveFields(store)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:         tenantID1,
		FieldPath:        "payments.fee",
		Value:            &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
		ExpectedChecksum: &wrongChecksum,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Aborted, status.Code(err))
}

func TestSetField_ChecksumFirstWrite(t *testing.T) {
	// When no versions exist yet (txLockedVersion == 0), an ExpectedChecksum
	// is ignored and the write proceeds — there is nothing to mismatch against.
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	checksum := "any-value"

	// Both outside-tx and inside-tx GetLatestConfigVersion return ErrNotFound.
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 1, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", mock.Anything, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	store.On("InsertAuditWriteLog", mock.Anything, mock.AnythingOfType("config.InsertAuditWriteLogParams")).
		Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)
	setupNoSensitiveFields(store)

	resp, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:         tenantID1,
		FieldPath:        "app.name",
		Value:            &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "x"}},
		ExpectedChecksum: &checksum,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.ConfigVersion.Version)
}

func TestSetField_ChecksumDBError(t *testing.T) {
	// When GetConfigValueAtVersion returns an unexpected error (not ErrNotFound),
	// checkChecksumAtVersion surfaces it as codes.Internal.
	svc, store, cache, _ := newTestService()
	ctx := superadminCtx()

	checksum := "abc"
	dbErr := errors.New("db exploded")

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	// Outside-tx getCurrentValue: ErrNotFound (field doesn't exist for audit).
	// Inside-tx checkChecksumAtVersion: real db error.
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound).Once()
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, dbErr).Once()
	setupNoSensitiveFields(store)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:         tenantID1,
		FieldPath:        "app.name",
		Value:            &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "x"}},
		ExpectedChecksum: &checksum,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestSetField_TxLatestVersionDBError(t *testing.T) {
	// When GetLatestConfigVersion fails inside the tx with a non-ErrNotFound
	// error, SetField returns codes.Internal.
	svc, store, cache, _ := newTestService()
	ctx := superadminCtx()

	dbErr := errors.New("db exploded")

	// Outside-tx: getOrCreateVersion succeeds.
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil).Once()
	// Outside-tx: getCurrentValue.
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	setupNoSensitiveFields(store)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	// Inside-tx: txLatestVersion fails.
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{}, dbErr).Once()

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "app.name",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "x"}},
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestSetField_ChecksumFieldNotYetWritten(t *testing.T) {
	// When a version exists but the field has never been written, ErrNotFound
	// from GetConfigValueAtVersion inside the tx means there is nothing to
	// mismatch — the write proceeds regardless of ExpectedChecksum.
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	checksum := "any"

	// Outside-tx: version 1 exists.
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	// getCurrentValue (outside tx): field not found.
	// checkChecksumAtVersion (inside tx): field also not found → returns nil.
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 2, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", mock.Anything, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	store.On("InsertAuditWriteLog", mock.Anything, mock.AnythingOfType("config.InsertAuditWriteLogParams")).
		Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)
	setupNoSensitiveFields(store)

	resp, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:         tenantID1,
		FieldPath:        "app.name",
		Value:            &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "x"}},
		ExpectedChecksum: &checksum,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.ConfigVersion.Version)
}

func TestSetField_ChecksumMatchSucceeds(t *testing.T) {
	// When the expected checksum matches the stored checksum, the write proceeds.
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	storedChecksum := "correct-checksum"

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{Value: strPtr("old"), Checksum: &storedChecksum}, nil)
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 2, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", mock.Anything, mock.AnythingOfType("config.SetConfigValueParams")).
		Return(nil)
	store.On("InsertAuditWriteLog", mock.Anything, mock.AnythingOfType("config.InsertAuditWriteLogParams")).
		Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)
	setupNoSensitiveFields(store)

	resp, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:         tenantID1,
		FieldPath:        "app.name",
		Value:            &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "new"}},
		ExpectedChecksum: &storedChecksum,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.ConfigVersion.Version)
}

func TestSetField_LockedField(t *testing.T) {
	svc, store, _, _ := newTestService()
	// Use admin context — lock checks only apply to non-superadmin.
	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: []string{"test-tenant"},
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
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)

	_, err := svc.GetField(ctx, &pb.GetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "nonexistent",
	})

	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestSetField_VersionConflictRetried(t *testing.T) {
	// First tx attempt returns ErrVersionConflict; second attempt succeeds.
	// The retry must be transparent — caller sees success, not codes.Aborted.
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", mock.Anything, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	// Pre-tx + 2 tx attempts = 3 total GetLatestConfigVersion calls.
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 5}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	setupNoSensitiveFields(store)

	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{}, ErrVersionConflict).Once()
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 6, CreatedBy: "unknown"}, nil).Once()

	store.On("SetConfigValue", mock.Anything, mock.AnythingOfType("config.SetConfigValueParams")).Return(nil)
	store.On("InsertAuditWriteLog", mock.Anything, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)

	resp, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "app.env",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "prod"}},
	})

	require.NoError(t, err, "single version conflict must be retried transparently")
	assert.Equal(t, int32(6), resp.ConfigVersion.Version)
	store.AssertNumberOfCalls(t, "CreateConfigVersion", 2)
}

func TestSetField_VersionConflictExhausted(t *testing.T) {
	// All 4 attempts (initial + 3 retries) return ErrVersionConflict.
	// The service must return codes.Aborted, not codes.Internal.
	svc, store, cache, _ := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", mock.Anything, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 5}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	setupNoSensitiveFields(store)

	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{}, ErrVersionConflict)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "app.env",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "prod"}},
	})

	require.Error(t, err)
	assert.Equal(t, codes.Aborted, status.Code(err))
	store.AssertNumberOfCalls(t, "CreateConfigVersion", 4) // initial + 3 retries
}

// --- RollbackToVersion ---

func TestRollbackToVersion_Success(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", mock.Anything, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 2}).
		Return([]GetFullConfigAtVersionRow{
			{FieldPath: "a", Value: strPtr("1")},
			{FieldPath: "b", Value: strPtr("2")},
		}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 5}, nil)
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID3, TenantID: tenantID1, Version: 6, CreatedBy: "unknown"}, nil)
	store.On("BulkSetConfigValues", mock.Anything, mock.Anything).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	store.On("InsertAuditWriteLog", mock.Anything, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)
	// getFieldWriteAttrsMap needs schema store calls.
	setupNoSensitiveFields(store)

	resp, err := svc.RollbackToVersion(ctx, &pb.RollbackToVersionRequest{
		TenantId: tenantID1,
		Version:  2,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(6), resp.ConfigVersion.Version)
	store.AssertCalled(t, "BulkSetConfigValues", mock.Anything, mock.Anything)
}

func TestRollbackToVersion_FieldLocksError_ReturnsInternal(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := superadminCtx()

	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 2}).
		Return([]GetFullConfigAtVersionRow{{FieldPath: "a", Value: strPtr("1")}}, nil)
	store.On("GetFieldLocks", mock.Anything, tenantID1).Return([]domain.TenantFieldLock(nil), errors.New("db down"))

	_, err := svc.RollbackToVersion(ctx, &pb.RollbackToVersionRequest{TenantId: tenantID1, Version: 2})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestRollbackToVersion_ValidateFieldError_ReturnsInvalidArgument(t *testing.T) {
	// Uses a service with validators so fieldTypeMap is non-nil and validateField
	// can return an error when a restored value violates a constraint.
	svc, store := newTestServiceWithValidation()
	ctx := superadminCtx()

	store.On("GetFieldLocks", mock.Anything, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 2}).
		Return([]GetFullConfigAtVersionRow{{FieldPath: "a", Value: strPtr("1")}}, nil)
	// Simulate fieldTypeMap failure: GetTenantByID returns error → fieldTypeMap propagates it.
	store.On("GetTenantByID", mock.Anything, tenantID1).Return(domain.Tenant{}, errors.New("db down"))

	_, err := svc.RollbackToVersion(ctx, &pb.RollbackToVersionRequest{TenantId: tenantID1, Version: 2})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestRollbackToVersion_VersionConflictReturnsAborted(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", mock.Anything, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 2}).
		Return([]GetFullConfigAtVersionRow{{FieldPath: "a", Value: strPtr("1")}}, nil)
	// GetLatestConfigVersion is now called inside the tx per attempt (4 total).
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).Return(domain.ConfigVersion{Version: 5}, nil)
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{}, ErrVersionConflict)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	// getFieldWriteAttrsMap needs schema store calls.
	setupNoSensitiveFields(store)

	_, err := svc.RollbackToVersion(ctx, &pb.RollbackToVersionRequest{
		TenantId: tenantID1,
		Version:  2,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Aborted, status.Code(err), "version conflict during rollback must return Aborted after retry budget exhausted")
	store.AssertNumberOfCalls(t, "CreateConfigVersion", 4)
}

func TestRollbackToVersion_VersionConflictRetried(t *testing.T) {
	// First attempt gets ErrVersionConflict; second attempt succeeds.
	svc, store, cache, _ := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", mock.Anything, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 2}).
		Return([]GetFullConfigAtVersionRow{{FieldPath: "a", Value: strPtr("1")}}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).Return(domain.ConfigVersion{Version: 5}, nil)
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{}, ErrVersionConflict).Once()
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID3, TenantID: tenantID1, Version: 6, CreatedBy: "unknown"}, nil).Once()
	store.On("BulkSetConfigValues", mock.Anything, mock.Anything).Return(nil)
	store.On("InsertAuditWriteLog", mock.Anything, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	// getFieldWriteAttrsMap needs schema store calls.
	setupNoSensitiveFields(store)

	resp, err := svc.RollbackToVersion(ctx, &pb.RollbackToVersionRequest{
		TenantId: tenantID1,
		Version:  2,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(6), resp.ConfigVersion.Version)
	store.AssertNumberOfCalls(t, "CreateConfigVersion", 2)
}

func TestRollbackToVersion_LockedField_ReturnsFailedPrecondition(t *testing.T) {
	// FieldLockGuard bypasses superadmin, so use an admin ctx with tenant access.
	svc, store, _, _ := newTestService()
	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: []string{tenantID1},
	})

	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 2}).
		Return([]GetFullConfigAtVersionRow{{FieldPath: "payments.fee", Value: strPtr("0.5")}}, nil)
	store.On("GetFieldLocks", mock.Anything, tenantID1).
		Return([]domain.TenantFieldLock{{TenantID: tenantID1, FieldPath: "payments.fee"}}, nil)
	// getFieldWriteAttrsMap needs schema store calls.
	setupNoSensitiveFields(store)

	_, err := svc.RollbackToVersion(ctx, &pb.RollbackToVersionRequest{TenantId: tenantID1, Version: 2})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err), "rollback to a locked field must return FailedPrecondition")
}

func TestRollbackToVersion_InvalidValue_ReturnsInvalidArgument(t *testing.T) {
	// Restore a value that violates the schema constraint (integer > max).
	svc, store := newTestServiceWithValidation()
	ctx := superadminCtx()

	constraintsJSON := []byte(`{"min":0,"max":10}`)
	store.On("GetFieldLocks", mock.Anything, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 2}).
		Return([]GetFullConfigAtVersionRow{{FieldPath: "app.retries", Value: strPtr("99")}}, nil)
	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", mock.Anything, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", mock.Anything, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "app.retries", FieldType: domain.FieldTypeInteger, Constraints: constraintsJSON},
		}, nil)

	_, err := svc.RollbackToVersion(ctx, &pb.RollbackToVersionRequest{TenantId: tenantID1, Version: 2})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "rollback with a value exceeding max constraint must return InvalidArgument")
}

// --- ExportConfig ---

func TestExportConfig_Success(t *testing.T) {
	svc, store := newTestServiceWithValidation()
	ctx := auth.WithoutAuth(context.Background())

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
	ctx := superadminCtx()

	yamlContent := []byte(`
spec_version: "v1"
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
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID20, TenantID: tenantID1, Version: 3, CreatedBy: "unknown"}, nil)
	store.On("BulkSetConfigValues", ctx, mock.MatchedBy(func(args []SetConfigValueParams) bool {
		return len(args) == 2
	})).Return(nil)
	store.On("BulkInsertAuditWriteLog", ctx, mock.MatchedBy(func(args []InsertAuditWriteLogParams) bool {
		return len(args) == 2
	})).Return(nil)
	cache.On("Invalidate", ctx, tenantID1).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)

	resp, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantID1,
		YamlContent: yamlContent,
	})

	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.ConfigVersion.Version)
	store.AssertCalled(t, "BulkSetConfigValues", ctx, mock.Anything)
	store.AssertCalled(t, "BulkInsertAuditWriteLog", ctx, mock.Anything)
	cache.AssertCalled(t, "Invalidate", ctx, tenantID1)
}

func TestImportConfig_VersionConflictReturnsAborted(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := superadminCtx()

	yamlContent := []byte(`spec_version: "v1"
values:
  app.env:
    value: "prod"
`)

	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", mock.Anything, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", mock.Anything, schemaVersionID).
		Return([]domain.SchemaField{{Path: "app.env", FieldType: domain.FieldTypeString}}, nil)
	store.On("GetFieldLocks", mock.Anything, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{}, ErrVersionConflict)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)

	_, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantID1,
		YamlContent: yamlContent,
		Mode:        pb.ImportMode_IMPORT_MODE_REPLACE,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Aborted, status.Code(err), "version conflict during import must return Aborted, not Internal")
}

// --- ImportConfig with validation ---

func TestImportConfig_ValidationRejectsUnknownField(t *testing.T) {
	svc, store := newTestServiceWithValidation()
	ctx := superadminCtx()

	yamlContent := []byte(`
spec_version: "v1"
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
	ctx := superadminCtx()

	// Import an integer value that exceeds max constraint.
	yamlContent := []byte(`
spec_version: "v1"
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
	ctx := superadminCtx()

	yamlContent := []byte(`
spec_version: "v1"
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
	store.On("GetConfigValueAtVersion", mock.Anything, mock.MatchedBy(func(p GetConfigValueAtVersionParams) bool {
		return p.FieldPath == "app.name"
	})).Return(GetConfigValueAtVersionRow{Value: strPtr("same")}, nil)

	// app.other has different value -> should be included
	store.On("GetConfigValueAtVersion", mock.Anything, mock.MatchedBy(func(p GetConfigValueAtVersionParams) bool {
		return p.FieldPath == "app.other"
	})).Return(GetConfigValueAtVersionRow{Value: strPtr("old")}, nil)

	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID20, TenantID: tenantID1, Version: 2, CreatedBy: "unknown"}, nil)
	store.On("BulkSetConfigValues", ctx, mock.MatchedBy(func(args []SetConfigValueParams) bool {
		return len(args) == 1
	})).Return(nil)
	store.On("BulkInsertAuditWriteLog", ctx, mock.MatchedBy(func(args []InsertAuditWriteLogParams) bool {
		return len(args) == 1
	})).Return(nil)
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
	store.AssertCalled(t, "BulkSetConfigValues", ctx, mock.Anything)
}

func TestImportConfig_DefaultsMode_SkipsExistingValues(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := superadminCtx()

	yamlContent := []byte(`
spec_version: "v1"
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

	// DEFAULTS filter: one bulk snapshot fetch instead of N per-field calls.
	// app.existing is present in the snapshot → skipped by the filter.
	// app.missing is absent from the snapshot → included.
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  1,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "app.existing", Value: strPtr("already-set")},
	}, nil)

	// changeG old-value lookup: only app.missing survives the filter.
	store.On("GetConfigValueAtVersion", mock.Anything, mock.MatchedBy(func(p GetConfigValueAtVersionParams) bool {
		return p.FieldPath == "app.missing"
	})).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)

	newVersionID := versionID20
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: newVersionID, TenantID: tenantID1, Version: 2, CreatedBy: "unknown"}, nil)
	store.On("BulkSetConfigValues", ctx, mock.MatchedBy(func(args []SetConfigValueParams) bool {
		return len(args) == 1
	})).Return(nil)
	store.On("BulkInsertAuditWriteLog", ctx, mock.MatchedBy(func(args []InsertAuditWriteLogParams) bool {
		return len(args) == 1
	})).Return(nil)
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
	// Only app.missing should be set — app.existing was already in the snapshot.
	store.AssertCalled(t, "BulkSetConfigValues", ctx, mock.Anything)
	// Verify exactly one bulk snapshot fetch was made (no N+1).
	store.AssertNumberOfCalls(t, "GetFullConfigAtVersion", 1)
}

func TestImportConfig_DefaultsMode_AllExistingReturnsAlreadyExists(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := superadminCtx()

	yamlContent := []byte(`
spec_version: "v1"
values:
  app.foo:
    value: "from-yaml"
  app.bar:
    value: "from-yaml"
`)

	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "app.foo", FieldType: domain.FieldTypeString},
			{Path: "app.bar", FieldType: domain.FieldTypeString},
		}, nil)
	store.On("GetFieldLocks", ctx, tenantID1).
		Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 3}, nil)

	// Both fields exist in the snapshot — nothing should be written.
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  3,
	}).Return([]GetFullConfigAtVersionRow{
		{FieldPath: "app.foo", Value: strPtr("existing-foo")},
		{FieldPath: "app.bar", Value: strPtr("existing-bar")},
	}, nil)

	_, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantID1,
		YamlContent: yamlContent,
		Mode:        pb.ImportMode_IMPORT_MODE_DEFAULTS,
	})

	require.Error(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Code(err), "all fields exist → AlreadyExists")
	store.AssertNumberOfCalls(t, "GetFullConfigAtVersion", 1)
	store.AssertNotCalled(t, "CreateConfigVersion")
}

func TestImportConfig_DefaultsMode_NoExistingConfig_IncludesAll(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := superadminCtx()

	yamlContent := []byte(`
spec_version: "v1"
values:
  app.alpha:
    value: "a"
  app.beta:
    value: "b"
`)

	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "app.alpha", FieldType: domain.FieldTypeString},
			{Path: "app.beta", FieldType: domain.FieldTypeString},
		}, nil)
	store.On("GetFieldLocks", ctx, tenantID1).
		Return([]domain.TenantFieldLock{}, nil)
	// Version 0 means no existing config.
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)

	// No snapshot fetch should occur when latestVersion == 0.
	newVersionID := versionID20
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: newVersionID, TenantID: tenantID1, Version: 1, CreatedBy: "unknown"}, nil)
	store.On("BulkSetConfigValues", ctx, mock.MatchedBy(func(args []SetConfigValueParams) bool {
		return len(args) == 2
	})).Return(nil)
	store.On("BulkInsertAuditWriteLog", ctx, mock.MatchedBy(func(args []InsertAuditWriteLogParams) bool {
		return len(args) == 2
	})).Return(nil)
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
	// All fields included (no existing config).
	store.AssertCalled(t, "BulkSetConfigValues", ctx, mock.Anything)
	// No bulk snapshot fetch — latestVersion was 0.
	store.AssertNotCalled(t, "GetFullConfigAtVersion")
}

func TestImportConfig_DefaultsMode_SnapshotFetchError_FallsBackToAll(t *testing.T) {
	// When GetFullConfigAtVersion returns an error, filterByImportMode falls
	// back to including all values (safe degradation) and the import proceeds.
	svc, store, _, _ := newTestService()
	ctx := superadminCtx()

	yamlContent := []byte(`
spec_version: "v1"
values:
  app.alpha:
    value: "a"
  app.beta:
    value: "b"
`)

	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "app.alpha", FieldType: domain.FieldTypeString},
			{Path: "app.beta", FieldType: domain.FieldTypeString},
		}, nil)
	store.On("GetFieldLocks", ctx, tenantID1).
		Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 2}, nil)

	// Snapshot fetch fails — should log a warning and include all values.
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  2,
	}).Return([]GetFullConfigAtVersionRow{}, errors.New("db connection lost"))

	// Both fields are included (fallback) — changeG old-value lookups follow.
	store.On("GetConfigValueAtVersion", mock.Anything, mock.MatchedBy(func(p GetConfigValueAtVersionParams) bool {
		return p.TenantID == tenantID1
	})).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)

	newVersionID := versionID20
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: newVersionID, TenantID: tenantID1, Version: 3, CreatedBy: "unknown"}, nil)
	store.On("BulkSetConfigValues", ctx, mock.MatchedBy(func(args []SetConfigValueParams) bool {
		return len(args) == 2 // both fields written despite fetch error
	})).Return(nil)
	store.On("BulkInsertAuditWriteLog", ctx, mock.MatchedBy(func(args []InsertAuditWriteLogParams) bool {
		return len(args) == 2
	})).Return(nil)
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
	// Both fields written — fallback included all values despite the fetch error.
	store.AssertCalled(t, "BulkSetConfigValues", ctx, mock.Anything)
	store.AssertNumberOfCalls(t, "GetFullConfigAtVersion", 1)
}

// --- Usage Recording ---

func TestGetField_RecordsUsage(t *testing.T) {
	store := &mockStore{}
	c := &mockCache{}
	auditStore := audit.NewMemoryStore()
	recorder := audit.NewUsageRecorder(auditStore,
		audit.WithFlushInterval(time.Hour),
		audit.WithLogger(testLogger),
	)
	svc := NewService(store, c, nil, nil,
		WithLogger(testLogger),
		WithRecorder(recorder),
	)
	ctx := auth.WithoutAuth(context.Background())

	mockNoSensitiveFields(store)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{FieldPath: "app.fee", Value: strPtr("0.5")}, nil)

	_, err := svc.GetField(ctx, &pb.GetFieldRequest{TenantId: tenantID1, FieldPath: "app.fee"})
	require.NoError(t, err)

	require.NoError(t, recorder.Flush(context.Background()))

	stats, err := auditStore.GetFieldUsage(context.Background(), audit.GetFieldUsageParams{
		TenantID:  tenantID1,
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(1), stats[0].ReadCount)
}

func TestGetConfig_RecordsUsage(t *testing.T) {
	store := &mockStore{}
	c := &mockCache{}
	auditStore := audit.NewMemoryStore()
	recorder := audit.NewUsageRecorder(auditStore,
		audit.WithFlushInterval(time.Hour),
		audit.WithLogger(testLogger),
	)
	svc := NewService(store, c, nil, nil,
		WithLogger(testLogger),
		WithRecorder(recorder),
	)
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	c.On("Get", ctx, tenantID1, int32(1)).Return(nil, nil)
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 1}).
		Return([]GetFullConfigAtVersionRow{
			{FieldPath: "a.x", Value: strPtr("1")},
			{FieldPath: "a.y", Value: strPtr("2")},
		}, nil)
	c.On("Set", ctx, tenantID1, int32(1), mock.AnythingOfType("map[string]string"), mock.Anything).
		Return(nil)
	setupNoSensitiveFields(store)

	_, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})
	require.NoError(t, err)

	require.NoError(t, recorder.Flush(context.Background()))

	for _, path := range []string{"a.x", "a.y"} {
		stats, err := auditStore.GetFieldUsage(context.Background(), audit.GetFieldUsageParams{
			TenantID:  tenantID1,
			FieldPath: path,
		})
		require.NoError(t, err)
		require.Len(t, stats, 1, "path %s", path)
		assert.Equal(t, int64(1), stats[0].ReadCount)
	}
}

func TestGetFields_RecordsUsage(t *testing.T) {
	store := &mockStore{}
	c := &mockCache{}
	auditStore := audit.NewMemoryStore()
	recorder := audit.NewUsageRecorder(auditStore,
		audit.WithFlushInterval(time.Hour),
		audit.WithLogger(testLogger),
	)
	svc := NewService(store, c, nil, nil,
		WithLogger(testLogger),
		WithRecorder(recorder),
	)
	ctx := auth.WithoutAuth(context.Background())

	mockNoSensitiveFields(store)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	// Each requested path returns a different row.
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{TenantID: tenantID1, FieldPath: "b.x", Version: 1}).
		Return(GetConfigValueAtVersionRow{FieldPath: "b.x", Value: strPtr("v1")}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{TenantID: tenantID1, FieldPath: "b.y", Version: 1}).
		Return(GetConfigValueAtVersionRow{FieldPath: "b.y", Value: strPtr("v2")}, nil)

	_, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"b.x", "b.y"},
	})
	require.NoError(t, err)

	require.NoError(t, recorder.Flush(context.Background()))

	stats, err := auditStore.GetTenantUsage(context.Background(), audit.GetTenantUsageParams{
		TenantID: tenantID1,
	})
	require.NoError(t, err)
	assert.Len(t, stats, 2)
}

func TestGetConfig_CacheHit_RecordsUsage(t *testing.T) {
	store := &mockStore{}
	c := &mockCache{}
	auditStore := audit.NewMemoryStore()
	recorder := audit.NewUsageRecorder(auditStore,
		audit.WithFlushInterval(time.Hour),
		audit.WithLogger(testLogger),
	)
	svc := NewService(store, c, nil, nil,
		WithLogger(testLogger),
		WithRecorder(recorder),
	)
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 5}, nil)
	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	c.On("Get", ctx, tenantID1, int32(5)).
		Return(map[string]string{"a.x": "1", "a.y": "2"}, nil)

	_, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})
	require.NoError(t, err)

	require.NoError(t, recorder.Flush(context.Background()))

	stats, err := auditStore.GetTenantUsage(context.Background(), audit.GetTenantUsageParams{
		TenantID: tenantID1,
	})
	require.NoError(t, err)
	assert.Len(t, stats, 2)
}

func TestGetField_NilRecorder_NoPanic(t *testing.T) {
	// Default test service has nil recorder — verify reads don't panic.
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	mockNoSensitiveFields(store)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{FieldPath: "x", Value: strPtr("1")}, nil)

	_, err := svc.GetField(ctx, &pb.GetFieldRequest{TenantId: tenantID1, FieldPath: "x"})
	require.NoError(t, err)
}

// --- Fail-closed validator lookup (issue #285) ---

// TestValidateField_ValidatorLookupError asserts that validateField returns
// codes.Internal (fail-closed) when the validator store is unavailable,
// rather than silently skipping validation (fail-open).
func TestValidateField_ValidatorLookupError(t *testing.T) {
	svc, store := newTestServiceWithValidation()
	ctx := auth.WithoutAuth(context.Background())

	storeErr := errors.New("db connection lost")
	store.On("GetTenantByID", ctx, tenantID1).Return(domain.Tenant{}, storeErr)

	tv := &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hello"}}
	err := svc.validateField(ctx, tenantID1, "some.field", tv)

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err), "must fail-closed with codes.Internal, not silently skip")
}

// TestFieldTypeMap_ValidatorLookupError asserts that fieldTypeMap propagates
// a validator-store error rather than returning nil (which would silently
// coerce all response types to STRING).
func TestFieldTypeMap_ValidatorLookupError(t *testing.T) {
	svc, store := newTestServiceWithValidation()
	ctx := auth.WithoutAuth(context.Background())

	storeErr := errors.New("db connection lost")
	store.On("GetTenantByID", ctx, tenantID1).Return(domain.Tenant{}, storeErr)

	types, err := svc.fieldTypeMap(ctx, tenantID1)

	require.Error(t, err)
	assert.Nil(t, types)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// TestSetField_ValidatorLookupError asserts that SetField returns codes.Internal
// when the validator store is unavailable (end-to-end fail-closed path).
func TestSetField_ValidatorLookupError(t *testing.T) {
	svc, store := newTestServiceWithValidation()
	ctx := superadminCtx()

	storeErr := errors.New("schema store unavailable")
	store.On("GetTenantByID", ctx, tenantID1).Return(domain.Tenant{}, storeErr)
	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)

	tv := &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "v"}}
	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "app.flag",
		Value:     tv,
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// --- Sensitive field redaction ---

func sensitiveSchemaSetup(store *mockStore, ctx context.Context) {
	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "app.secret", FieldType: domain.FieldTypeString, Sensitive: true},
			{Path: "app.name", FieldType: domain.FieldTypeString, Sensitive: false},
		}, nil)
}

func TestGetConfig_RedactsSensitiveFields(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	cache.On("Get", ctx, tenantID1, int32(1)).Return(nil, nil)
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 1}).
		Return([]GetFullConfigAtVersionRow{
			{FieldPath: "app.secret", Value: strPtr("s3cr3t")},
			{FieldPath: "app.name", Value: strPtr("myapp")},
		}, nil)

	var capturedCacheMap map[string]string
	cache.On("Set", ctx, tenantID1, int32(1), mock.MatchedBy(func(m map[string]string) bool {
		capturedCacheMap = m
		return true
	}), mock.Anything).Return(nil)

	sensitiveSchemaSetup(store, ctx)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})

	require.NoError(t, err)
	vals := make(map[string]string, len(resp.Config.Values))
	for _, v := range resp.Config.Values {
		vals[v.FieldPath] = typedValueToDisplayString(v.Value)
	}
	assert.Equal(t, redactedSentinel, vals["app.secret"])
	assert.Equal(t, "myapp", vals["app.name"])

	// Cache must also store the sentinel, not the raw value.
	require.NotNil(t, capturedCacheMap)
	assert.Equal(t, redactedSentinel, capturedCacheMap["app.secret"])
	assert.Equal(t, "myapp", capturedCacheMap["app.name"])
}

func TestSetField_AuditRedactsSensitiveField(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "app.secret", Version: 1,
	}).Return(GetConfigValueAtVersionRow{Value: strPtr("old-secret")}, nil)

	sensitiveSchemaSetup(store, ctx)

	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 2, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).Return(nil)
	cache.On("Invalidate", ctx, tenantID1).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)

	var capturedAudit InsertAuditWriteLogParams
	store.On("InsertAuditWriteLog", ctx, mock.MatchedBy(func(p InsertAuditWriteLogParams) bool {
		capturedAudit = p
		return true
	})).Return(nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "app.secret",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "new-secret"}},
	})

	require.NoError(t, err)
	assert.Equal(t, redactedSentinel, derefString(capturedAudit.OldValue))
	assert.Equal(t, redactedSentinel, derefString(capturedAudit.NewValue))
}

func TestSetField_PublishRedactsSensitiveField(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "app.secret", Version: 1,
	}).Return(GetConfigValueAtVersionRow{Value: strPtr("old-secret")}, nil)

	sensitiveSchemaSetup(store, ctx)

	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 2, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", ctx, mock.AnythingOfType("config.SetConfigValueParams")).Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)
	cache.On("Invalidate", ctx, tenantID1).Return(nil)

	var capturedEvent pubsub.ConfigChangeEvent
	pub.On("Publish", ctx, mock.MatchedBy(func(e pubsub.ConfigChangeEvent) bool {
		capturedEvent = e
		return true
	})).Return(nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "app.secret",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "new-secret"}},
	})

	require.NoError(t, err)
	require.Len(t, capturedEvent.Changes, 1)
	assert.Equal(t, redactedSentinel, capturedEvent.Changes[0].OldValue)
	assert.Equal(t, redactedSentinel, capturedEvent.Changes[0].NewValue)
}

func TestExportConfig_RedactsSensitiveFields(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetFullConfigAtVersion", ctx, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 1}).
		Return([]GetFullConfigAtVersionRow{
			{FieldPath: "app.secret", Value: strPtr("s3cr3t")},
			{FieldPath: "app.name", Value: strPtr("myapp")},
		}, nil)
	desc := "v1"
	store.On("GetConfigVersion", ctx, GetConfigVersionParams{TenantID: tenantID1, Version: 1}).
		Return(domain.ConfigVersion{Version: 1, Description: &desc}, nil)

	// getSensitiveFieldSet call
	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "app.secret", FieldType: domain.FieldTypeString, Sensitive: true},
			{Path: "app.name", FieldType: domain.FieldTypeString, Sensitive: false},
		}, nil)

	resp, err := svc.ExportConfig(ctx, &pb.ExportConfigRequest{TenantId: tenantID1})

	require.NoError(t, err)
	yaml := string(resp.YamlContent)
	assert.NotContains(t, yaml, "s3cr3t")
	assert.Contains(t, yaml, redactedSentinel)
	assert.Contains(t, yaml, "myapp")
}

func TestCrossFieldError(t *testing.T) {
	inner := errors.New("inner error")
	e := &crossFieldError{kind: crossFieldKindDependentRequired, err: inner}
	assert.Equal(t, "inner error", e.Error())
	assert.Equal(t, inner, e.Unwrap())

	e2 := &crossFieldError{kind: crossFieldKindValidation, err: errors.New("cel rule failed")}
	assert.Equal(t, "cel rule failed", e2.Error())
}

func TestWithCacheMetrics_Option(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, &mockCache{}, &mockPublisher{}, &mockSubscriber{},
		WithCacheMetrics(nil))
	assert.NotNil(t, svc)
}

func TestWithMetrics_Option(t *testing.T) {
	store := &mockStore{}
	svc := NewService(store, &mockCache{}, &mockPublisher{}, &mockSubscriber{},
		WithMetrics(nil))
	assert.NotNil(t, svc)
}

func TestConfigService_RequiresAuth(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := context.Background()

	_, err := svc.GetConfig(ctx, &pb.GetConfigRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = svc.GetField(ctx, &pb.GetFieldRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = svc.GetFields(ctx, &pb.GetFieldsRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = svc.ListVersions(ctx, &pb.ListVersionsRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = svc.GetVersion(ctx, &pb.GetVersionRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = svc.ExportConfig(ctx, &pb.ExportConfigRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	stream := &mockServerStream{ctx: ctx}
	err = svc.Subscribe(&pb.SubscribeRequest{}, stream)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// --- Coverage for new lines added in #431 ---

// TestGetConfig_TenantNotFound covers the GetTenantByID error branch in
// fetchAndCacheConfig → getSensitiveFieldSet (service.go).
func TestGetConfig_TenantNotFound(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	cache.On("Get", mock.Anything, tenantID1, int32(1)).Return(nil, nil)
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 1}).
		Return([]GetFullConfigAtVersionRow{{FieldPath: "a.b", Value: strPtr("v")}}, nil)
	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{}, errors.New("db error"))

	_, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// TestSetField_PreInvalidateFails_WriteProceeds covers the WarnContext branch
// when pre-commit cache invalidation fails in SetField (service.go:503-505).
// The write must still succeed even when the pre-commit invalidation fails.
func TestSetField_PreInvalidateFails_WriteProceeds(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 1, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", mock.Anything, mock.AnythingOfType("config.SetConfigValueParams")).Return(nil)
	store.On("InsertAuditWriteLog", mock.Anything, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)
	// Pre-commit fails, post-commit succeeds — write must not be blocked.
	cache.On("Invalidate", mock.Anything, tenantID1).Return(errors.New("redis down")).Once()
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil).Once()
	pub.On("Publish", mock.Anything, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)
	setupNoSensitiveFields(store)

	resp, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.fee",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.ConfigVersion.Version)
	cache.AssertNumberOfCalls(t, "Invalidate", 2)
}

// TestSetFields_PreInvalidateFails_WriteProceeds covers the WarnContext branch
// when pre-commit cache invalidation fails in SetFields (service.go:667-669).
func TestSetFields_PreInvalidateFails_WriteProceeds(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", mock.Anything, mock.Anything).
		Return(domain.ConfigVersion{ID: versionID2, Version: 2}, nil)
	store.On("BulkSetConfigValues", mock.Anything, mock.Anything).Return(nil)
	store.On("BulkInsertAuditWriteLog", mock.Anything, mock.Anything).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(errors.New("redis down")).Once()
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil).Once()
	pub.On("Publish", mock.Anything, mock.Anything).Return(nil)
	setupNoSensitiveFields(store)

	resp, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
		TenantId: tenantID1,
		Updates: []*pb.FieldUpdate{
			{FieldPath: "app.name", Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "test"}}},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.ConfigVersion.Version)
	cache.AssertNumberOfCalls(t, "Invalidate", 2)
}

// TestRollbackToVersion_PreInvalidateFails_WriteProceeds covers the WarnContext
// branch when pre-commit cache invalidation fails in RollbackToVersion
// (service.go:863-865).
func TestRollbackToVersion_PreInvalidateFails_WriteProceeds(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", mock.Anything, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{TenantID: tenantID1, Version: 2}).
		Return([]GetFullConfigAtVersionRow{{FieldPath: "a", Value: strPtr("1")}}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 5}, nil)
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID3, TenantID: tenantID1, Version: 6, CreatedBy: "unknown"}, nil)
	store.On("BulkSetConfigValues", mock.Anything, mock.Anything).Return(nil)
	store.On("InsertAuditWriteLog", mock.Anything, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(errors.New("redis down")).Once()
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil).Once()
	// getFieldWriteAttrsMap needs schema store calls.
	setupNoSensitiveFields(store)

	resp, err := svc.RollbackToVersion(ctx, &pb.RollbackToVersionRequest{
		TenantId: tenantID1,
		Version:  2,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(6), resp.ConfigVersion.Version)
	cache.AssertNumberOfCalls(t, "Invalidate", 2)
}

// TestImportConfig_PreInvalidateFails_WriteProceeds covers the WarnContext
// branch when pre-commit cache invalidation fails in ImportConfig
// (service.go:1220-1222).
func TestImportConfig_PreInvalidateFails_WriteProceeds(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	yamlContent := []byte(`
spec_version: "v1"
values:
  payments.fee:
    value: 0.05
`)

	store.On("GetTenantByID", ctx, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", ctx, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", ctx, schemaVersionID).
		Return([]domain.SchemaField{{Path: "payments.fee", FieldType: domain.FieldTypeNumber}}, nil)
	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).
		Return(domain.ConfigVersion{Version: 2}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.AnythingOfType("config.GetConfigValueAtVersionParams")).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", ctx, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID20, TenantID: tenantID1, Version: 3, CreatedBy: "unknown"}, nil)
	store.On("BulkSetConfigValues", ctx, mock.Anything).Return(nil)
	store.On("BulkInsertAuditWriteLog", ctx, mock.Anything).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(errors.New("redis down")).Once()
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil).Once()
	pub.On("Publish", ctx, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)

	resp, err := svc.ImportConfig(ctx, &pb.ImportConfigRequest{
		TenantId:    tenantID1,
		YamlContent: yamlContent,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), resp.ConfigVersion.Version)
	cache.AssertNumberOfCalls(t, "Invalidate", 2)
}

// TestSetField_SensitiveFields_TenantNotFound covers the GetTenantByID error
// path in getSensitiveFieldSet (service.go:1393-1395). The SetField must
// propagate the NotFound status.
func TestSetField_SensitiveFields_TenantNotFound(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := superadminCtx()

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{}, errors.New("db error"))

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.fee",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// TestSetField_SensitiveFields_SchemaVersionError covers the GetSchemaVersion
// error path in getSensitiveFieldSetFromTenant (service.go:1375-1377).
func TestSetField_SensitiveFields_SchemaVersionError(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := superadminCtx()

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", mock.Anything, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{}, errors.New("db error"))

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.fee",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// TestSetField_SensitiveFields_SchemaFieldsError covers the GetSchemaFields
// error path in getSensitiveFieldSetFromTenant (service.go:1379-1381).
func TestSetField_SensitiveFields_SchemaFieldsError(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := superadminCtx()

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", mock.Anything, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil)
	store.On("GetSchemaFields", mock.Anything, schemaVersionID).
		Return([]domain.SchemaField(nil), errors.New("db error"))

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "payments.fee",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "0.5"}},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// --- read_only / write_once enforcement ---

// setupWriteAttrService creates a service with a schema that declares one
// read-only field ("config.frozen") and one write-once field ("config.init").
func setupWriteAttrService(t *testing.T) (*Service, *mockStore, *mockCache, *mockPublisher) {
	t.Helper()
	svc, store, c, pub := newTestService()
	// Schema store calls used by getSchemaFieldSets.
	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{SchemaID: schemaID10, SchemaVersion: 1}, nil).Maybe()
	store.On("GetSchemaVersion", mock.Anything, domain.SchemaVersionKey{SchemaID: schemaID10, Version: 1}).
		Return(domain.SchemaVersion{ID: schemaVersionID}, nil).Maybe()
	store.On("GetSchemaFields", mock.Anything, schemaVersionID).
		Return([]domain.SchemaField{
			{Path: "config.frozen", FieldType: domain.FieldTypeString, ReadOnly: true},
			{Path: "config.init", FieldType: domain.FieldTypeString, WriteOnce: true},
		}, nil).Maybe()
	return svc, store, c, pub
}

func TestSetField_ReadOnly_Rejected(t *testing.T) {
	svc, store, _, _ := setupWriteAttrService(t)
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "config.frozen",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "new-value"}},
	})

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "read-only")
}

func TestSetField_WriteOnce_SecondWrite_Rejected(t *testing.T) {
	// Write-once field already has a value — the second write must be rejected.
	svc, store, _, _ := setupWriteAttrService(t)
	ctx := superadminCtx()

	existingVal := "initial-value"

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	// getCurrentValue returns the existing value for the write-once check.
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{Value: &existingVal}, nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "config.init",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "second-value"}},
	})

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "write-once")
}

func TestSetField_WriteOnce_FirstWrite_Allowed(t *testing.T) {
	// Write-once field has no existing value — the first write must succeed.
	svc, store, cache, pub := setupWriteAttrService(t)
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{ID: versionID2, TenantID: tenantID1, Version: 1, CreatedBy: "unknown"}, nil)
	store.On("SetConfigValue", mock.Anything, mock.AnythingOfType("config.SetConfigValueParams")).Return(nil)
	store.On("InsertAuditWriteLog", mock.Anything, mock.AnythingOfType("config.InsertAuditWriteLogParams")).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("pubsub.ConfigChangeEvent")).Return(nil)

	resp, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "config.init",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "first-value"}},
	})

	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.ConfigVersion.Version)
}

func TestSetFields_ReadOnly_Rejected(t *testing.T) {
	svc, store, _, _ := setupWriteAttrService(t)
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)

	_, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
		TenantId: tenantID1,
		Updates: []*pb.FieldUpdate{
			{FieldPath: "config.frozen", Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "x"}}},
		},
	})

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "read-only")
}

func TestSetFields_WriteOnce_SecondWrite_Rejected(t *testing.T) {
	svc, store, _, _ := setupWriteAttrService(t)
	ctx := superadminCtx()
	existing := "initial"

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{Value: &existing}, nil)

	_, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
		TenantId: tenantID1,
		Updates: []*pb.FieldUpdate{
			{FieldPath: "config.init", Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "second"}}},
		},
	})

	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "write-once")
}
