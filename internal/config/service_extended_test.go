package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/pagination"
	"github.com/opendecree/decree/internal/pubsub"
	"github.com/opendecree/decree/internal/storage/domain"
)

func encodeVersionOffset(offset int32) string { return pagination.EncodePageToken(offset) }

// mockNoSensitiveFields stubs the three store calls that getSensitiveFieldSet
// makes (GetTenantByID → GetSchemaVersion → GetSchemaFields) to return an
// empty sensitive-field set. Call this in any test that exercises GetField or
// GetFields but does not specifically test sensitive-value redaction.
func mockNoSensitiveFields(store *mockStore) {
	store.On("GetTenantByID", mock.Anything, mock.AnythingOfType("string")).
		Return(domain.Tenant{ID: tenantID1, SchemaID: "s1", SchemaVersion: 1}, nil).Maybe()
	store.On("GetSchemaVersion", mock.Anything, mock.Anything).
		Return(domain.SchemaVersion{ID: "sv1"}, nil).Maybe()
	store.On("GetSchemaFields", mock.Anything, mock.Anything).
		Return([]domain.SchemaField{}, nil).Maybe()
}

// --- GetFields ---

func TestGetFields_Success(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	mockNoSensitiveFields(store)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)

	val := "hello"
	chk := "abc"
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).Return(GetConfigValueAtVersionRow{
		FieldPath: "app.name", Value: &val, Checksum: &chk,
	}, nil)

	resp, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"app.name"},
	})
	require.NoError(t, err)
	assert.Len(t, resp.Values, 1)
	assert.Equal(t, "app.name", resp.Values[0].FieldPath)
}

func TestGetFields_SkipsMissing(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	mockNoSensitiveFields(store)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)

	resp, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"missing"},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Values)
}

func TestGetFields_InvalidTenantID(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.GetFields(auth.WithoutAuth(context.Background()), &pb.GetFieldsRequest{TenantId: ""})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestGetFields_PreservesOrder verifies the per-field fan-out preserves
// request order regardless of completion order. Per-path mock matchers let us
// assert each value lands in the right slot.
func TestGetFields_PreservesOrder(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	mockNoSensitiveFields(store)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)

	paths := []string{"a.one", "a.two", "a.three", "a.four", "a.five"}
	for _, p := range paths {
		val := "v-" + p
		store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
			TenantID: tenantID1, FieldPath: p, Version: 1,
		}).Return(GetConfigValueAtVersionRow{FieldPath: p, Value: &val}, nil)
	}

	resp, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: paths,
	})
	require.NoError(t, err)
	require.Len(t, resp.Values, len(paths))
	for i, p := range paths {
		assert.Equal(t, p, resp.Values[i].FieldPath, "slot %d", i)
	}
}

// TestGetFields_MixedMissingAndPresent verifies that NotFound rows are
// dropped while present rows retain their request order.
func TestGetFields_MixedMissingAndPresent(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	mockNoSensitiveFields(store)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)

	present := "x"
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "have.first", Version: 1,
	}).Return(GetConfigValueAtVersionRow{FieldPath: "have.first", Value: &present}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "missing", Version: 1,
	}).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "have.second", Version: 1,
	}).Return(GetConfigValueAtVersionRow{FieldPath: "have.second", Value: &present}, nil)

	resp, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"have.first", "missing", "have.second"},
	})
	require.NoError(t, err)
	require.Len(t, resp.Values, 2)
	assert.Equal(t, "have.first", resp.Values[0].FieldPath)
	assert.Equal(t, "have.second", resp.Values[1].FieldPath)
}

// TestGetFields_PropagatesError verifies a non-NotFound store error
// surfaces as Internal and aborts the group.
func TestGetFields_PropagatesError(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	mockNoSensitiveFields(store)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	val := "ok"
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "ok.path", Version: 1,
	}).Return(GetConfigValueAtVersionRow{FieldPath: "ok.path", Value: &val}, nil).Maybe()
	store.On("GetConfigValueAtVersion", mock.Anything, GetConfigValueAtVersionParams{
		TenantID: tenantID1, FieldPath: "boom", Version: 1,
	}).Return(GetConfigValueAtVersionRow{}, errors.New("db exploded"))

	_, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"ok.path", "boom"},
	})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// --- Sensitive field redaction ---

func TestGetField_SensitiveValueRedacted(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	// Field is sensitive — GetTenantByID → GetSchemaVersion → GetSchemaFields returns sensitive=true.
	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{ID: tenantID1, SchemaID: "s1", SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", mock.Anything, mock.Anything).
		Return(domain.SchemaVersion{ID: "sv1"}, nil)
	store.On("GetSchemaFields", mock.Anything, mock.Anything).
		Return([]domain.SchemaField{{Path: "app.secret", Sensitive: true}}, nil)

	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{FieldPath: "app.secret", Value: strPtr("my-token")}, nil)

	resp, err := svc.GetField(ctx, &pb.GetFieldRequest{TenantId: tenantID1, FieldPath: "app.secret"})
	require.NoError(t, err)
	assert.Equal(t, redactedSentinel, resp.Value.Value.GetStringValue())
}

func TestGetField_SensitiveFieldSetError_ReturnsInternal(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{FieldPath: "app.fee", Value: strPtr("1")}, nil)
	// GetTenantByID fails → getSensitiveFieldSet returns error.
	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{}, errors.New("db down"))

	_, err := svc.GetField(ctx, &pb.GetFieldRequest{TenantId: tenantID1, FieldPath: "app.fee"})
	require.Error(t, err)
}

func TestGetFields_SensitiveValueRedacted(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetTenantByID", mock.Anything, tenantID1).
		Return(domain.Tenant{ID: tenantID1, SchemaID: "s1", SchemaVersion: 1}, nil)
	store.On("GetSchemaVersion", mock.Anything, mock.Anything).
		Return(domain.SchemaVersion{ID: "sv1"}, nil)
	store.On("GetSchemaFields", mock.Anything, mock.Anything).
		Return([]domain.SchemaField{{Path: "app.secret", Sensitive: true}}, nil)

	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{FieldPath: "app.secret", Value: strPtr("my-token")}, nil)

	resp, err := svc.GetFields(ctx, &pb.GetFieldsRequest{TenantId: tenantID1, FieldPaths: []string{"app.secret"}})
	require.NoError(t, err)
	require.Len(t, resp.Values, 1)
	assert.Equal(t, redactedSentinel, resp.Values[0].Value.GetStringValue())
}

// --- Slug resolution tests ---

func TestGetConfig_ByTenantName(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	// Resolve "acme" slug → tenant UUID
	store.On("GetTenantByName", mock.Anything, "acme").Return(domain.Tenant{
		ID: tenantID1, Name: "acme",
	}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).Return(domain.ConfigVersion{
		TenantID: tenantID1, Version: 1,
	}, nil)
	// Return a cached result so we skip the DB path
	store.On("GetTenantByID", mock.Anything, tenantID1).Return(domain.Tenant{ID: tenantID1, SchemaVersion: 0}, nil)
	cached := map[string]string{"key": "val"}
	cache.On("Get", mock.Anything, tenantID1, int32(1)).Return(cached, nil)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: "acme"})
	require.NoError(t, err)
	assert.Equal(t, tenantID1, resp.Config.TenantId)
	store.AssertCalled(t, "GetTenantByName", mock.Anything, "acme")
}

func TestGetConfig_ByTenantUUID(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	// UUID goes directly to version lookup — no name resolution
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).Return(domain.ConfigVersion{
		TenantID: tenantID1, Version: 1,
	}, nil)
	store.On("GetTenantByID", mock.Anything, tenantID1).Return(domain.Tenant{ID: tenantID1, SchemaVersion: 0}, nil)
	cached := map[string]string{"key": "val"}
	cache.On("Get", mock.Anything, tenantID1, int32(1)).Return(cached, nil)

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})
	require.NoError(t, err)
	assert.Equal(t, tenantID1, resp.Config.TenantId)
	store.AssertNotCalled(t, "GetTenantByName", mock.Anything, mock.Anything)
}

func TestGetConfig_TenantNameNotFound(t *testing.T) {
	svc, store, _, _ := newTestService()
	store.On("GetTenantByName", mock.Anything, "nonexistent").Return(domain.Tenant{}, domain.ErrNotFound)

	_, err := svc.GetConfig(auth.WithoutAuth(context.Background()), &pb.GetConfigRequest{TenantId: "nonexistent"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- Batch publish ---

// TestSetField_PublishesOneEvent verifies that a single-field update emits
// exactly one pubsub event carrying a one-element Changes slice.
func TestSetField_PublishesOneEvent(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", mock.Anything, mock.Anything).Return(domain.ConfigVersion{
		ID: versionID2, Version: 2, CreatedAt: time.Now(),
	}, nil)
	store.On("SetConfigValue", mock.Anything, mock.Anything).Return(nil)
	store.On("InsertAuditWriteLog", mock.Anything, mock.Anything).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	setupNoSensitiveFields(store)

	var captured pubsub.ConfigChangeEvent
	pub.On("Publish", mock.Anything, mock.AnythingOfType("pubsub.ConfigChangeEvent")).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(pubsub.ConfigChangeEvent)
		}).
		Return(nil)

	_, err := svc.SetField(ctx, &pb.SetFieldRequest{
		TenantId:  tenantID1,
		FieldPath: "app.name",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hello"}},
	})
	require.NoError(t, err)

	pub.AssertNumberOfCalls(t, "Publish", 1)
	require.Len(t, captured.Changes, 1)
	assert.Equal(t, "app.name", captured.Changes[0].FieldPath)
	assert.Equal(t, "hello", captured.Changes[0].NewValue)
}

// TestSetFields_PublishesOneEventWithAllChanges verifies that a multi-field
// update emits exactly one pubsub event carrying all changes in order.
func TestSetFields_PublishesOneEventWithAllChanges(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", mock.Anything, mock.Anything).Return(domain.ConfigVersion{
		ID: versionID2, Version: 2, CreatedAt: time.Now(),
	}, nil)
	store.On("BulkSetConfigValues", mock.Anything, mock.Anything).Return(nil)
	store.On("BulkInsertAuditWriteLog", mock.Anything, mock.Anything).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	setupNoSensitiveFields(store)

	var captured pubsub.ConfigChangeEvent
	pub.On("Publish", mock.Anything, mock.AnythingOfType("pubsub.ConfigChangeEvent")).
		Run(func(args mock.Arguments) {
			captured = args.Get(1).(pubsub.ConfigChangeEvent)
		}).
		Return(nil)

	_, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
		TenantId: tenantID1,
		Updates: []*pb.FieldUpdate{
			{FieldPath: "app.name", Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "v1"}}},
			{FieldPath: "app.port", Value: &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 9090}}},
		},
	})
	require.NoError(t, err)

	// Exactly one publish for two field changes.
	pub.AssertNumberOfCalls(t, "Publish", 1)
	require.Len(t, captured.Changes, 2)
	assert.Equal(t, "app.name", captured.Changes[0].FieldPath)
	assert.Equal(t, "app.port", captured.Changes[1].FieldPath)
}

// TestSubscribe_BatchEventExpandedToPerFieldMessages verifies that a single
// batched pubsub event with multiple fields fans out to one gRPC message per field.
func TestSubscribe_BatchEventExpandedToPerFieldMessages(t *testing.T) {
	svc, _, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent, 1)
	cancel := func() {}

	ctx, ctxCancel := context.WithCancel(auth.WithoutAuth(context.Background()))
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	now := time.Now()
	ch <- pubsub.ConfigChangeEvent{
		TenantID: tenantID1,
		Version:  3,
		Changes: []pubsub.FieldChange{
			{FieldPath: "app.name", OldValue: "old", NewValue: "new"},
			{FieldPath: "app.port", OldValue: "8080", NewValue: "9090"},
		},
		ChangedBy: "admin",
		ChangedAt: now,
	}
	close(ch)

	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: tenantID1}, stream)
	require.NoError(t, err)

	// One pubsub event with two fields → two gRPC messages.
	require.Len(t, stream.sent, 2)
	assert.Equal(t, "app.name", stream.sent[0].Change.FieldPath)
	assert.Equal(t, int32(3), stream.sent[0].Change.Version)
	assert.Equal(t, "app.port", stream.sent[1].Change.FieldPath)
	assert.Equal(t, int32(3), stream.sent[1].Change.Version)

	ctxCancel()
}

// --- SetFields ---

func TestSetFields_Success(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := superadminCtx()

	// GetFieldLocks is called with the original ctx before context wrapping.
	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	// Subsequent store calls use a context wrapped with the lock cache; use
	// mock.Anything so the matcher is not sensitive to context wrapper identity.
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", mock.Anything, mock.Anything).Return(domain.ConfigVersion{
		ID: versionID2, Version: 2, CreatedAt: time.Now(),
	}, nil)
	store.On("BulkSetConfigValues", mock.Anything, mock.Anything).Return(nil)
	store.On("BulkInsertAuditWriteLog", mock.Anything, mock.Anything).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
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
}

func TestSetFields_InvalidTenantID(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.SetFields(superadminCtx(), &pb.SetFieldsRequest{TenantId: ""})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSetFields_ChecksumMismatch(t *testing.T) {
	// Exercises the per-field checksum loop inside the SetFields transaction.
	svc, store, cache, _ := newTestService()
	ctx := superadminCtx()

	storedChecksum := "actual-stored"
	clientChecksum := "client-expected-different"

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	// getOrCreateVersion + txLatestVersion (two calls to the same mock).
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	// getCurrentValue (outside tx) + checkChecksumAtVersion (inside tx): both
	// return the stored checksum, which differs from the client's expectation.
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).
		Return(GetConfigValueAtVersionRow{Value: strPtr("old"), Checksum: &storedChecksum}, nil)
	setupNoSensitiveFields(store)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)

	_, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
		TenantId: tenantID1,
		Updates: []*pb.FieldUpdate{
			{
				FieldPath:        "app.name",
				Value:            &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "x"}},
				ExpectedChecksum: &clientChecksum,
			},
		},
	})

	require.Error(t, err)
	assert.Equal(t, codes.Aborted, status.Code(err))
}

func TestSetFields_VersionConflictReturnsAborted(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := superadminCtx()

	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", mock.Anything, mock.Anything).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	setupNoSensitiveFields(store)
	store.On("CreateConfigVersion", mock.Anything, mock.AnythingOfType("config.CreateConfigVersionParams")).
		Return(domain.ConfigVersion{}, ErrVersionConflict)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)

	_, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
		TenantId: tenantID1,
		Updates: []*pb.FieldUpdate{
			{FieldPath: "app.name", Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "x"}}},
		},
	})

	require.Error(t, err)
	assert.Equal(t, codes.Aborted, status.Code(err), "version conflict must return Aborted, not Internal")
}

// --- ListVersions ---

func TestListVersions_Success(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("ListConfigVersions", ctx, mock.Anything).Return([]domain.ConfigVersion{
		{ID: versionID2, Version: 2, CreatedAt: time.Now()},
		{ID: versionID3, Version: 1, CreatedAt: time.Now()},
	}, nil)

	resp, err := svc.ListVersions(ctx, &pb.ListVersionsRequest{TenantId: tenantID1})
	require.NoError(t, err)
	assert.Len(t, resp.Versions, 2)
}

func TestListVersions_InvalidTenantID(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.ListVersions(auth.WithoutAuth(context.Background()), &pb.ListVersionsRequest{TenantId: ""})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestListVersions_InvalidPageToken(t *testing.T) {
	svc, _, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())
	_, err := svc.ListVersions(ctx, &pb.ListVersionsRequest{TenantId: tenantID1, PageToken: "garbage"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestListVersions_BeyondEnd(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("ListConfigVersions", ctx, mock.MatchedBy(func(p ListConfigVersionsParams) bool {
		return p.TenantID == tenantID1 && p.Offset == 500
	})).Return([]domain.ConfigVersion{}, nil)

	token := encodeVersionOffset(500)
	resp, err := svc.ListVersions(ctx, &pb.ListVersionsRequest{TenantId: tenantID1, PageToken: token})
	require.NoError(t, err)
	assert.Empty(t, resp.Versions, "expected no versions past end")
	assert.Empty(t, resp.NextPageToken, "expected nil token past end")
}

func TestListVersions_ZeroPageSize(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("ListConfigVersions", ctx, mock.MatchedBy(func(p ListConfigVersionsParams) bool {
		return p.Limit > 0 // clamped from 0 to default
	})).Return([]domain.ConfigVersion{
		{ID: versionID2, Version: 1, CreatedAt: time.Now()},
	}, nil)

	resp, err := svc.ListVersions(ctx, &pb.ListVersionsRequest{TenantId: tenantID1, PageSize: 0})
	require.NoError(t, err)
	assert.Len(t, resp.Versions, 1)
}

// --- GetVersion ---

func TestGetVersion_Success(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetConfigVersion", ctx, mock.Anything).Return(domain.ConfigVersion{
		ID: versionID2, Version: 2, CreatedBy: "admin", CreatedAt: time.Now(),
	}, nil)

	resp, err := svc.GetVersion(ctx, &pb.GetVersionRequest{TenantId: tenantID1, Version: 2})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.ConfigVersion.Version)
}

func TestGetVersion_NotFound(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetConfigVersion", ctx, mock.Anything).Return(domain.ConfigVersion{}, domain.ErrNotFound)

	_, err := svc.GetVersion(ctx, &pb.GetVersionRequest{TenantId: tenantID1, Version: 99})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

// --- convert.go: typedValueToString coverage ---

func TestTypedValueToString_AllTypes(t *testing.T) {
	tests := []struct {
		name     string
		tv       *pb.TypedValue
		expected *string
	}{
		{"nil", nil, nil},
		{"string", &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hi"}}, strPtr("hi")},
		{"integer", &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}, strPtr("42")},
		{"number", &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 3.14}}, strPtr("3.14")},
		{"bool", &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}, strPtr("true")},
		{"url", &pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://x.com"}}, strPtr("https://x.com")},
		{"json", &pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `{}`}}, strPtr("{}")},
		{"empty kind", &pb.TypedValue{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typedValueToString(tt.tv)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

// --- Subscribe ---

// mockServerStream implements grpc.ServerStreamingServer[pb.SubscribeResponse] for testing.
type mockServerStream struct {
	ctx  context.Context
	sent []*pb.SubscribeResponse
}

func (m *mockServerStream) Send(resp *pb.SubscribeResponse) error {
	m.sent = append(m.sent, resp)
	return nil
}

func (m *mockServerStream) Context() context.Context { return m.ctx }

func (m *mockServerStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockServerStream) SendHeader(metadata.MD) error { return nil }
func (m *mockServerStream) SetTrailer(metadata.MD)       {}
func (m *mockServerStream) SendMsg(any) error            { return nil }
func (m *mockServerStream) RecvMsg(any) error            { return nil }

func TestSubscribe_InvalidTenantID(t *testing.T) {
	svc, _, _, _ := newTestService()

	stream := &mockServerStream{ctx: auth.WithoutAuth(context.Background())}
	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: ""}, stream)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSubscribe_SubscribeError(t *testing.T) {
	svc, _, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ctx := auth.WithoutAuth(context.Background())
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", ctx, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(nil), context.CancelFunc(func() {}), errors.New("subscribe failed"))

	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: tenantID1}, stream)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestSubscribe_ForwardsEvents(t *testing.T) {
	svc, _, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent, 2)
	cancel := func() {}

	ctx, ctxCancel := context.WithCancel(auth.WithoutAuth(context.Background()))
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	now := time.Now()
	ch <- pubsub.ConfigChangeEvent{
		TenantID:  tenantID1,
		Version:   1,
		Changes:   []pubsub.FieldChange{{FieldPath: "app.name", OldValue: "", NewValue: "hello"}},
		ChangedBy: "admin",
		ChangedAt: now,
	}
	ch <- pubsub.ConfigChangeEvent{
		TenantID:  tenantID1,
		Version:   2,
		Changes:   []pubsub.FieldChange{{FieldPath: "app.port", OldValue: "8080", NewValue: "9090"}},
		ChangedBy: "admin",
		ChangedAt: now,
	}

	// Close the channel so the for loop exits after draining.
	close(ch)

	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: tenantID1}, stream)
	require.NoError(t, err)

	require.Len(t, stream.sent, 2)
	assert.Equal(t, "app.name", stream.sent[0].Change.FieldPath)
	assert.Equal(t, int32(1), stream.sent[0].Change.Version)
	assert.Equal(t, "admin", stream.sent[0].Change.ChangedBy)
	assert.Equal(t, "app.port", stream.sent[1].Change.FieldPath)
	assert.Equal(t, int32(2), stream.sent[1].Change.Version)

	// No leak — cancel context just for cleanup.
	ctxCancel()
}

func TestSubscribe_FiltersByFieldPaths(t *testing.T) {
	svc, _, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent, 3)
	cancel := func() {}

	ctx := auth.WithoutAuth(context.Background())
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	now := time.Now()
	ch <- pubsub.ConfigChangeEvent{TenantID: tenantID1, Version: 1, Changes: []pubsub.FieldChange{{FieldPath: "app.name", NewValue: "v1"}}, ChangedAt: now}
	ch <- pubsub.ConfigChangeEvent{TenantID: tenantID1, Version: 2, Changes: []pubsub.FieldChange{{FieldPath: "app.port", NewValue: "9090"}}, ChangedAt: now}
	ch <- pubsub.ConfigChangeEvent{TenantID: tenantID1, Version: 3, Changes: []pubsub.FieldChange{{FieldPath: "app.name", NewValue: "v2"}}, ChangedAt: now}
	close(ch)

	err := svc.Subscribe(&pb.SubscribeRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"app.name"},
	}, stream)
	require.NoError(t, err)

	// Only "app.name" events should be forwarded.
	require.Len(t, stream.sent, 2)
	assert.Equal(t, "app.name", stream.sent[0].Change.FieldPath)
	assert.Equal(t, "app.name", stream.sent[1].Change.FieldPath)
}

func TestSubscribe_ContextCancellation(t *testing.T) {
	svc, _, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent) // unbuffered — blocks
	cancelCalled := false
	cancel := func() { cancelCalled = true }

	ctx, ctxCancel := context.WithCancel(auth.WithoutAuth(context.Background()))
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	// Cancel the context immediately so the select hits ctx.Done().
	ctxCancel()

	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: tenantID1}, stream)
	require.NoError(t, err)
	assert.Empty(t, stream.sent)
	assert.True(t, cancelCalled, "subscriber cancel function should be called via defer")
}

func TestSubscribe_SendError(t *testing.T) {
	svc, _, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent, 1)
	cancel := func() {}

	stream := &errServerStream{ctx: auth.WithoutAuth(context.Background()), sendErr: errors.New("stream broken")}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	ch <- pubsub.ConfigChangeEvent{TenantID: tenantID1, Version: 1, Changes: []pubsub.FieldChange{{FieldPath: "x", NewValue: "y"}}, ChangedAt: time.Now()}
	close(ch)

	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: tenantID1}, stream)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream broken")
}

// errServerStream is a mock stream that returns an error on Send.
type errServerStream struct {
	ctx     context.Context
	sendErr error
}

func (m *errServerStream) Send(*pb.SubscribeResponse) error { return m.sendErr }
func (m *errServerStream) Context() context.Context         { return m.ctx }
func (m *errServerStream) SetHeader(metadata.MD) error      { return nil }
func (m *errServerStream) SendHeader(metadata.MD) error     { return nil }
func (m *errServerStream) SetTrailer(metadata.MD)           {}
func (m *errServerStream) SendMsg(any) error                { return nil }
func (m *errServerStream) RecvMsg(any) error                { return nil }

func ptr[T any](v T) *T { return &v }

func TestSubscribe_ReplayStoreError(t *testing.T) {
	svc, store, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent)
	cancel := func() {}

	ctx := auth.WithoutAuth(context.Background())
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{}, nil)

	store.On("GetConfigValuesSince", mock.Anything, mock.Anything).
		Return([]ConfigValueSince(nil), errors.New("db error"))

	setupNoSensitiveFields(store)

	startVersion := int32(1)
	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: tenantID1, StartVersion: &startVersion}, stream)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestSubscribe_ReplaysMissedEvents(t *testing.T) {
	svc, store, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent) // never sends — only replay matters
	cancel := func() {}

	ctx, ctxCancel := context.WithCancel(auth.WithoutAuth(context.Background()))
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	now := time.Now()

	// GetLatestConfigVersion returns version 4.
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 4}, nil)

	// GetConfigValuesSince returns two deltas for versions 3 and 4.
	store.On("GetConfigValuesSince", mock.Anything, GetConfigValuesSinceParams{
		TenantID:     tenantID1,
		StartVersion: 3,
	}).Return([]ConfigValueSince{
		{FieldPath: "app.name", Value: ptr("v3"), Version: 3, CreatedBy: "alice", ChangedAt: now},
		{FieldPath: "app.name", Value: ptr("v4"), Version: 4, CreatedBy: "bob", ChangedAt: now},
	}, nil)

	setupNoSensitiveFields(store)

	// Cancel after replay so Subscribe returns.
	ctxCancel()

	startVersion := int32(3)
	err := svc.Subscribe(&pb.SubscribeRequest{
		TenantId:     tenantID1,
		StartVersion: &startVersion,
	}, stream)
	require.NoError(t, err)

	// Both replay events should be sent before live streaming begins.
	require.Len(t, stream.sent, 2)
	assert.Equal(t, "app.name", stream.sent[0].Change.FieldPath)
	assert.Equal(t, int32(3), stream.sent[0].Change.Version)
	assert.Equal(t, "alice", stream.sent[0].Change.ChangedBy)
	assert.Equal(t, "app.name", stream.sent[1].Change.FieldPath)
	assert.Equal(t, int32(4), stream.sent[1].Change.Version)
	assert.Equal(t, "bob", stream.sent[1].Change.ChangedBy)
}

func TestSubscribe_WatermarkDeduplicatesReplayedVersions(t *testing.T) {
	svc, store, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	// Live channel delivers version 4 — same as the last replayed version.
	ch := make(chan pubsub.ConfigChangeEvent, 2)
	cancel := func() {}

	ctx := auth.WithoutAuth(context.Background())
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	now := time.Now()

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 4}, nil)

	store.On("GetConfigValuesSince", mock.Anything, GetConfigValuesSinceParams{
		TenantID:     tenantID1,
		StartVersion: 3,
	}).Return([]ConfigValueSince{
		{FieldPath: "app.name", Value: ptr("v4"), Version: 4, CreatedBy: "alice", ChangedAt: now},
	}, nil)

	setupNoSensitiveFields(store)

	// Pubsub delivers version 4 (duplicate of replay) and version 5 (new).
	ch <- pubsub.ConfigChangeEvent{TenantID: tenantID1, Version: 4, Changes: []pubsub.FieldChange{{FieldPath: "app.name", NewValue: "v4-dup"}}, ChangedAt: now}
	ch <- pubsub.ConfigChangeEvent{TenantID: tenantID1, Version: 5, Changes: []pubsub.FieldChange{{FieldPath: "app.name", NewValue: "v5"}}, ChangedAt: now}
	close(ch)

	startVersion := int32(3)
	err := svc.Subscribe(&pb.SubscribeRequest{
		TenantId:     tenantID1,
		StartVersion: &startVersion,
	}, stream)
	require.NoError(t, err)

	// 1 replay + 1 live (version 4 duplicate must be suppressed, version 5 forwarded).
	require.Len(t, stream.sent, 2)
	assert.Equal(t, int32(4), stream.sent[0].Change.Version) // replay
	assert.Equal(t, int32(5), stream.sent[1].Change.Version) // live, new
}

// TestSubscribe_VersionLookupError ensures that when GetLatestConfigVersion
// fails during replay setup the Subscribe call returns codes.Internal and does
// not swallow the error (which would leave watermark at 0 and deliver all live
// events as duplicates).
func TestSubscribe_VersionLookupError(t *testing.T) {
	svc, store, _, _ := newTestService()
	sub := &mockSubscriber{}
	svc.subscriber = sub

	ch := make(chan pubsub.ConfigChangeEvent)
	cancel := func() {}

	ctx := auth.WithoutAuth(context.Background())
	stream := &mockServerStream{ctx: ctx}

	sub.On("Subscribe", mock.Anything, tenantID1).
		Return((<-chan pubsub.ConfigChangeEvent)(ch), context.CancelFunc(cancel), nil)

	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{}, errors.New("db unavailable"))

	setupNoSensitiveFields(store)

	startVersion := int32(1)
	err := svc.Subscribe(&pb.SubscribeRequest{TenantId: tenantID1, StartVersion: &startVersion}, stream)
	assert.Equal(t, codes.Internal, status.Code(err))
	// No events must have been sent.
	assert.Empty(t, stream.sent)
}
