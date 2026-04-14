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

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

// --- GetFields ---

func TestGetFields_Success(t *testing.T) {
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	cache.On("Get", mock.Anything, tenantID1, mock.Anything).Return(nil, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)

	val := "hello"
	chk := "abc"
	store.On("GetConfigValueAtVersion", ctx, mock.Anything).Return(GetConfigValueAtVersionRow{
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
	svc, store, cache, _ := newTestService()
	ctx := context.Background()

	cache.On("Get", mock.Anything, tenantID1, mock.Anything).Return(nil, nil)
	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetConfigValueAtVersion", ctx, mock.Anything).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)

	resp, err := svc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   tenantID1,
		FieldPaths: []string{"missing"},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Values)
}

func TestGetFields_InvalidTenantID(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.GetFields(context.Background(), &pb.GetFieldsRequest{TenantId: ""})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- SetFields ---

func TestSetFields_Success(t *testing.T) {
	svc, store, cache, pub := newTestService()
	ctx := context.Background()

	store.On("GetLatestConfigVersion", ctx, tenantID1).Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetFieldLocks", ctx, tenantID1).Return([]domain.TenantFieldLock{}, nil)
	store.On("GetConfigValueAtVersion", ctx, mock.Anything).Return(GetConfigValueAtVersionRow{}, domain.ErrNotFound)
	store.On("CreateConfigVersion", ctx, mock.Anything).Return(domain.ConfigVersion{
		ID: versionID2, Version: 2, CreatedAt: time.Now(),
	}, nil)
	store.On("SetConfigValue", ctx, mock.Anything).Return(nil)
	store.On("InsertAuditWriteLog", ctx, mock.Anything).Return(nil)
	cache.On("Invalidate", mock.Anything, tenantID1).Return(nil)
	pub.On("Publish", mock.Anything, mock.Anything).Return(nil)

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
	_, err := svc.SetFields(context.Background(), &pb.SetFieldsRequest{TenantId: ""})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- ListVersions ---

func TestListVersions_Success(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

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
	_, err := svc.ListVersions(context.Background(), &pb.ListVersionsRequest{TenantId: ""})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// --- GetVersion ---

func TestGetVersion_Success(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	store.On("GetConfigVersion", ctx, mock.Anything).Return(domain.ConfigVersion{
		ID: versionID2, Version: 2, CreatedBy: "admin", CreatedAt: time.Now(),
	}, nil)

	resp, err := svc.GetVersion(ctx, &pb.GetVersionRequest{TenantId: tenantID1, Version: 2})
	require.NoError(t, err)
	assert.Equal(t, int32(2), resp.ConfigVersion.Version)
}

func TestGetVersion_NotFound(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

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
