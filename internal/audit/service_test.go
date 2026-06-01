package audit

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/pagination"
	"github.com/opendecree/decree/internal/storage/domain"
)

const (
	testTenantID  = "22222222-2222-2222-2222-222222222222"
	otherTenantID = "00000000-0000-0000-0000-000000000999"
)

// outOfScopeAdminCtx returns context with admin claims scoped to otherTenantID
// — i.e. NOT testTenantID. Used to verify audit RPCs reject callers without
// tenant access.
func outOfScopeAdminCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: []string{otherTenantID},
	})
}

func superadminCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.Claims{Role: auth.RoleSuperAdmin})
}

type mockStore struct{ mock.Mock }

func (m *mockStore) InsertAuditWriteLog(ctx context.Context, arg InsertAuditWriteLogParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockStore) GetAuditWriteLogOrdered(ctx context.Context, tenantID string) ([]domain.AuditWriteLog, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]domain.AuditWriteLog), args.Error(1)
}

func (m *mockStore) QueryAuditWriteLog(ctx context.Context, arg QueryWriteLogParams) ([]domain.AuditWriteLog, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]domain.AuditWriteLog), args.Error(1)
}

func (m *mockStore) GetFieldUsage(ctx context.Context, arg GetFieldUsageParams) ([]domain.UsageStat, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]domain.UsageStat), args.Error(1)
}

func (m *mockStore) GetTenantUsage(ctx context.Context, arg GetTenantUsageParams) ([]domain.TenantUsageRow, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]domain.TenantUsageRow), args.Error(1)
}

func (m *mockStore) GetUnusedFields(ctx context.Context, arg GetUnusedFieldsParams) ([]string, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockStore) UpsertUsageStats(ctx context.Context, arg UpsertUsageStatsParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func newTestService() (*Service, *mockStore) {
	store := &mockStore{}
	svc := NewService(store, slog.Default(), nil)
	return svc, store
}

// --- QueryWriteLog ---

func TestQueryWriteLog_Success(t *testing.T) {
	svc, store := newTestService()
	ctx := superadminCtx()

	store.On("QueryAuditWriteLog", ctx, mock.Anything).Return([]domain.AuditWriteLog{
		{
			ID:        "11111111-1111-1111-1111-111111111111",
			TenantID:  "22222222-2222-2222-2222-222222222222",
			Actor:     "admin",
			Action:    "set_field",
			CreatedAt: time.Now(),
		},
	}, nil)

	resp, err := svc.QueryWriteLog(ctx, &pb.QueryWriteLogRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.Entries, 1)
	assert.Equal(t, "set_field", resp.Entries[0].Action)
}

func TestQueryWriteLog_WithFilters(t *testing.T) {
	svc, store := newTestService()
	ctx := superadminCtx()

	tenantID := "22222222-2222-2222-2222-222222222222"
	actor := "admin"
	fieldPath := "app.fee"
	store.On("QueryAuditWriteLog", ctx, mock.Anything).Return([]domain.AuditWriteLog{}, nil)

	_, err := svc.QueryWriteLog(ctx, &pb.QueryWriteLogRequest{
		TenantId:  &tenantID,
		Actor:     &actor,
		FieldPath: &fieldPath,
		StartTime: timestamppb.Now(),
		EndTime:   timestamppb.Now(),
		PageSize:  10,
	})
	require.NoError(t, err)
}

func TestQueryWriteLog_InvalidTenantID(t *testing.T) {
	svc, _ := newTestService()
	ctx := superadminCtx()

	bad := "not-a-uuid"
	_, err := svc.QueryWriteLog(ctx, &pb.QueryWriteLogRequest{TenantId: &bad})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestQueryWriteLog_DefaultPageSize(t *testing.T) {
	svc, store := newTestService()
	ctx := superadminCtx()

	store.On("QueryAuditWriteLog", ctx, mock.MatchedBy(func(p QueryWriteLogParams) bool {
		return p.Limit == 51 // 50 (default page size) + 1 (to detect next page)
	})).Return([]domain.AuditWriteLog{}, nil)

	_, err := svc.QueryWriteLog(ctx, &pb.QueryWriteLogRequest{})
	require.NoError(t, err)
}

func TestQueryWriteLog_InvalidPageToken(t *testing.T) {
	svc, _ := newTestService()
	ctx := superadminCtx()

	_, err := svc.QueryWriteLog(ctx, &pb.QueryWriteLogRequest{PageToken: "garbage"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestQueryWriteLog_BeyondEnd(t *testing.T) {
	svc, store := newTestService()
	ctx := superadminCtx()

	store.On("QueryAuditWriteLog", ctx, mock.MatchedBy(func(p QueryWriteLogParams) bool {
		return p.Offset == 500
	})).Return([]domain.AuditWriteLog{}, nil)

	token := pagination.EncodePageToken(500)
	resp, err := svc.QueryWriteLog(ctx, &pb.QueryWriteLogRequest{PageToken: token})
	require.NoError(t, err)
	assert.Empty(t, resp.Entries, "expected no entries past end")
	assert.Empty(t, resp.NextPageToken, "expected nil token past end")
}

func TestQueryWriteLog_ZeroPageSize(t *testing.T) {
	svc, store := newTestService()
	ctx := superadminCtx()

	store.On("QueryAuditWriteLog", ctx, mock.MatchedBy(func(p QueryWriteLogParams) bool {
		return p.Limit > 0 // clamped from 0 to default
	})).Return([]domain.AuditWriteLog{}, nil)

	_, err := svc.QueryWriteLog(ctx, &pb.QueryWriteLogRequest{PageSize: 0})
	require.NoError(t, err)
}

func TestQueryWriteLog_KeysetPagination(t *testing.T) {
	svc, store := newTestService()
	ctx := superadminCtx()

	now := time.Now().Truncate(time.Microsecond)
	entry := domain.AuditWriteLog{
		ID:        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		TenantID:  "22222222-2222-2222-2222-222222222222",
		Actor:     "admin",
		Action:    "set_field",
		CreatedAt: now,
	}
	// First page: empty token → KindFirst → Cursor nil in params.
	store.On("QueryAuditWriteLog", ctx, mock.MatchedBy(func(p QueryWriteLogParams) bool {
		return p.Cursor == nil && p.Offset == 0 && p.Limit == 2
	})).Return([]domain.AuditWriteLog{entry, entry}, nil).Once()

	resp, err := svc.QueryWriteLog(ctx, &pb.QueryWriteLogRequest{PageSize: 1})
	require.NoError(t, err)
	require.Len(t, resp.Entries, 1)
	require.NotEmpty(t, resp.NextPageToken, "expected cursor token for full page")

	// Next page token must decode as KindCursor.
	kind, _, _, err := pagination.DecodeTokenKind(resp.NextPageToken)
	require.NoError(t, err)
	assert.Equal(t, pagination.KindCursor, kind)

	// Second page: cursor token → Cursor set in params.
	store.On("QueryAuditWriteLog", ctx, mock.MatchedBy(func(p QueryWriteLogParams) bool {
		return p.Cursor != nil && p.Limit == 2
	})).Return([]domain.AuditWriteLog{}, nil).Once()

	resp2, err := svc.QueryWriteLog(ctx, &pb.QueryWriteLogRequest{
		PageSize:  1,
		PageToken: resp.NextPageToken,
	})
	require.NoError(t, err)
	assert.Empty(t, resp2.Entries)
	assert.Empty(t, resp2.NextPageToken)
}

// --- GetFieldUsage ---

func TestGetFieldUsage_Success(t *testing.T) {
	svc, store := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	lastReadBy := "reader"
	now := time.Now()
	store.On("GetFieldUsage", ctx, mock.Anything).Return([]domain.UsageStat{
		{ReadCount: 10, LastReadBy: &lastReadBy, LastReadAt: &now},
		{ReadCount: 5},
	}, nil)

	resp, err := svc.GetFieldUsage(ctx, &pb.GetFieldUsageRequest{
		TenantId:  "22222222-2222-2222-2222-222222222222",
		FieldPath: "app.fee",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(15), resp.Stats.ReadCount)
	assert.Equal(t, "reader", *resp.Stats.LastReadBy)
}

func TestGetFieldUsage_InvalidTenantID(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.GetFieldUsage(auth.WithoutAuth(context.Background()), &pb.GetFieldUsageRequest{TenantId: "bad"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- GetTenantUsage ---

func TestGetTenantUsage_Success(t *testing.T) {
	svc, store := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetTenantUsage", ctx, mock.Anything).Return([]domain.TenantUsageRow{
		{FieldPath: "app.fee", ReadCount: 10},
		{FieldPath: "app.name", ReadCount: 3},
	}, nil)

	resp, err := svc.GetTenantUsage(ctx, &pb.GetTenantUsageRequest{
		TenantId: "22222222-2222-2222-2222-222222222222",
	})
	require.NoError(t, err)
	assert.Len(t, resp.FieldStats, 2)
}

func TestGetTenantUsage_InvalidTenantID(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.GetTenantUsage(auth.WithoutAuth(context.Background()), &pb.GetTenantUsageRequest{TenantId: "bad"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- GetUnusedFields ---

func TestGetUnusedFields_Success(t *testing.T) {
	svc, store := newTestService()
	ctx := auth.WithoutAuth(context.Background())

	store.On("GetUnusedFields", ctx, mock.Anything).Return([]string{"old.field", "unused.flag"}, nil)

	resp, err := svc.GetUnusedFields(ctx, &pb.GetUnusedFieldsRequest{
		TenantId: "22222222-2222-2222-2222-222222222222",
		Since:    timestamppb.New(time.Now().Add(-24 * time.Hour)),
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"old.field", "unused.flag"}, resp.FieldPaths)
}

func TestGetUnusedFields_InvalidTenantID(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.GetUnusedFields(auth.WithoutAuth(context.Background()), &pb.GetUnusedFieldsRequest{
		TenantId: "bad",
		Since:    timestamppb.Now(),
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetUnusedFields_NilSince(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.GetUnusedFields(auth.WithoutAuth(context.Background()), &pb.GetUnusedFieldsRequest{
		TenantId: testTenantID,
		Since:    nil,
	})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetUnusedFields_SinceCappedForNonSuperadmin(t *testing.T) {
	svc, store := newTestService()
	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: []string{testTenantID},
	})

	store.On("GetUnusedFields", ctx, mock.MatchedBy(func(p GetUnusedFieldsParams) bool {
		earliest := time.Now().Add(-unusedFieldsMaxLookback)
		return !p.Since.Before(earliest.Add(-2*time.Second)) && !p.Since.After(time.Now())
	})).Return([]string{}, nil)

	_, err := svc.GetUnusedFields(ctx, &pb.GetUnusedFieldsRequest{
		TenantId: testTenantID,
		Since:    timestamppb.New(time.Now().Add(-200 * 24 * time.Hour)),
	})
	require.NoError(t, err)
	store.AssertExpectations(t)
}

func TestGetUnusedFields_SuperadminBypassesCap(t *testing.T) {
	svc, store := newTestService()
	ctx := superadminCtx()
	farBack := time.Now().Add(-200 * 24 * time.Hour)

	store.On("GetUnusedFields", ctx, mock.MatchedBy(func(p GetUnusedFieldsParams) bool {
		return p.Since.Before(time.Now().Add(-unusedFieldsMaxLookback))
	})).Return([]string{}, nil)

	_, err := svc.GetUnusedFields(ctx, &pb.GetUnusedFieldsRequest{
		TenantId: testTenantID,
		Since:    timestamppb.New(farBack),
	})
	require.NoError(t, err)
	store.AssertExpectations(t)
}

// --- Tenant access enforcement ---

func TestQueryWriteLog_DeniedForOutOfScopeAdmin(t *testing.T) {
	svc, _ := newTestService()
	tenantID := testTenantID

	_, err := svc.QueryWriteLog(outOfScopeAdminCtx(), &pb.QueryWriteLogRequest{TenantId: &tenantID})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestQueryWriteLog_DeniedForNonSuperAdminWithoutTenantID(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.QueryWriteLog(outOfScopeAdminCtx(), &pb.QueryWriteLogRequest{})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetFieldUsage_DeniedForOutOfScopeAdmin(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.GetFieldUsage(outOfScopeAdminCtx(), &pb.GetFieldUsageRequest{
		TenantId:  testTenantID,
		FieldPath: "app.fee",
	})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetTenantUsage_DeniedForOutOfScopeAdmin(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.GetTenantUsage(outOfScopeAdminCtx(), &pb.GetTenantUsageRequest{
		TenantId: testTenantID,
	})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetUnusedFields_DeniedForOutOfScopeAdmin(t *testing.T) {
	svc, _ := newTestService()

	_, err := svc.GetUnusedFields(outOfScopeAdminCtx(), &pb.GetUnusedFieldsRequest{
		TenantId: testTenantID,
		Since:    timestamppb.Now(),
	})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

// --- VerifyChain ---

func TestVerifyChain_NoClaims(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.VerifyChain(context.Background(), &pb.VerifyChainRequest{TenantId: testTenantID})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestVerifyChain_TenantOutOfScope(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.VerifyChain(outOfScopeAdminCtx(), &pb.VerifyChainRequest{TenantId: testTenantID})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestVerifyChain_TenantSuccess(t *testing.T) {
	svc, store := newTestService()
	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: []string{testTenantID},
	})

	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	h := ComputeEntryHash(ChainInput{
		PreviousHash: "",
		ID:           "id-1",
		TenantID:     testTenantID,
		Actor:        "actor",
		Action:       "set_field",
		ObjectKind:   "field",
		CreatedAt:    ts,
	})
	store.On("GetAuditWriteLogOrdered", ctx, testTenantID).Return([]domain.AuditWriteLog{
		{ID: "id-1", TenantID: testTenantID, Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: h, CreatedAt: ts},
	}, nil)

	resp, err := svc.VerifyChain(ctx, &pb.VerifyChainRequest{TenantId: testTenantID})
	require.NoError(t, err)
	assert.True(t, resp.Ok)
	assert.Equal(t, int32(1), resp.Total)
	assert.Empty(t, resp.Breaks)
}

func TestVerifyChain_GlobalChain_NonSuperAdmin(t *testing.T) {
	svc, _ := newTestService()
	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: []string{testTenantID},
	})
	_, err := svc.VerifyChain(ctx, &pb.VerifyChainRequest{TenantId: ""})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestVerifyChain_GlobalChain_SuperAdmin(t *testing.T) {
	svc, store := newTestService()
	store.On("GetAuditWriteLogOrdered", superadminCtx(), "").Return([]domain.AuditWriteLog{}, nil)

	resp, err := svc.VerifyChain(superadminCtx(), &pb.VerifyChainRequest{TenantId: ""})
	require.NoError(t, err)
	assert.True(t, resp.Ok)
	assert.Equal(t, int32(0), resp.Total)
}

func TestVerifyChain_TenantTamperedEntry(t *testing.T) {
	svc, store := newTestService()
	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: []string{testTenantID},
	})
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	store.On("GetAuditWriteLogOrdered", ctx, testTenantID).Return([]domain.AuditWriteLog{
		{ID: "id-1", TenantID: testTenantID, Actor: "actor", Action: "set_field", ObjectKind: "field", EntryHash: "tampered", CreatedAt: ts},
	}, nil)

	resp, err := svc.VerifyChain(ctx, &pb.VerifyChainRequest{TenantId: testTenantID})
	require.NoError(t, err)
	assert.False(t, resp.Ok)
	require.Len(t, resp.Breaks, 1)
	assert.Equal(t, "tampered", resp.Breaks[0].Got)
}

// --- Helpers ---

func TestIsValidUUID(t *testing.T) {
	assert.True(t, domain.IsUUID("11111111-1111-1111-1111-111111111111"))
	assert.False(t, domain.IsUUID("not-a-uuid"))
	assert.False(t, domain.IsUUID(""))
}

func TestAuditEntryToProto(t *testing.T) {
	fieldPath := "app.fee"
	oldVal := "0.01"
	newVal := "0.02"
	version := int32(3)
	e := domain.AuditWriteLog{
		ID:            "11111111-1111-1111-1111-111111111111",
		TenantID:      "22222222-2222-2222-2222-222222222222",
		Actor:         "admin",
		Action:        "set_field",
		FieldPath:     &fieldPath,
		OldValue:      &oldVal,
		NewValue:      &newVal,
		ConfigVersion: &version,
		CreatedAt:     time.Now(),
	}

	pb := auditEntryToProto(e)
	assert.Equal(t, "admin", pb.Actor)
	assert.Equal(t, "set_field", pb.Action)
	assert.Equal(t, "app.fee", *pb.FieldPath)
	assert.Equal(t, "0.01", *pb.OldValue)
	assert.Equal(t, "0.02", *pb.NewValue)
	assert.Equal(t, int32(3), *pb.ConfigVersion)
}

func TestAuditEntryToProto_ChainEpochAndMetadata(t *testing.T) {
	e := domain.AuditWriteLog{
		ID:         "11111111-1111-1111-1111-111111111112",
		TenantID:   "22222222-2222-2222-2222-222222222222",
		Actor:      "admin",
		Action:     "set_field",
		ObjectKind: "field",
		EntryHash:  "hash1",
		ChainEpoch: 1,
		Metadata:   []byte(`{"request_id":"abc","ip":"1.2.3.4"}`),
		CreatedAt:  time.Now(),
	}

	out := auditEntryToProto(e)
	assert.Equal(t, int32(1), out.ChainEpoch)
	assert.Equal(t, e.Metadata, out.Metadata)
}

func TestAuditEntryToProto_NoMetadata(t *testing.T) {
	e := domain.AuditWriteLog{
		ID:         "11111111-1111-1111-1111-111111111113",
		TenantID:   "22222222-2222-2222-2222-222222222222",
		Actor:      "admin",
		Action:     "set_field",
		ObjectKind: "field",
		ChainEpoch: 0,
		CreatedAt:  time.Now(),
	}

	out := auditEntryToProto(e)
	assert.Equal(t, int32(0), out.ChainEpoch)
	assert.Nil(t, out.Metadata)
}

func TestAuditEntryToProto_InvalidMetadataJSON(t *testing.T) {
	e := domain.AuditWriteLog{
		ID:         "11111111-1111-1111-1111-111111111114",
		TenantID:   "22222222-2222-2222-2222-222222222222",
		Actor:      "admin",
		Action:     "set_field",
		ObjectKind: "field",
		ChainEpoch: 1,
		Metadata:   []byte(`not-valid-json`),
		CreatedAt:  time.Now(),
	}

	// Raw bytes are passed through as-is; even invalid JSON is forwarded.
	out := auditEntryToProto(e)
	assert.Equal(t, int32(1), out.ChainEpoch)
	assert.Equal(t, e.Metadata, out.Metadata)
}

func TestAuditService_RequiresAuth(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	_, err := svc.GetFieldUsage(ctx, &pb.GetFieldUsageRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = svc.GetTenantUsage(ctx, &pb.GetTenantUsageRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = svc.GetUnusedFields(ctx, &pb.GetUnusedFieldsRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
