package adminclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// --- Schema operations ---

func TestGetSchemaVersion_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("GetSchema", mock.Anything, mock.MatchedBy(func(r *pb.GetSchemaRequest) bool {
		return r.Id == "s1" && r.Version != nil && *r.Version == int32(2)
	})).Return(&pb.GetSchemaResponse{
		Schema: &pb.Schema{Id: "s1", Name: "test", Version: 2, CreatedAt: timestamppb.Now()},
	}, nil)

	s, err := client.GetSchemaVersion(context.Background(), "s1", 2)
	require.NoError(t, err)
	assert.Equal(t, int32(2), s.Version)
}

func TestListSchemas_AutoPaginate(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("ListSchemas", mock.Anything, mock.MatchedBy(func(r *pb.ListSchemasRequest) bool {
		return r.PageToken == ""
	})).Return(&pb.ListSchemasResponse{
		Schemas:       []*pb.Schema{{Id: "s1", Name: "a", Version: 1, CreatedAt: timestamppb.Now()}},
		NextPageToken: "page2",
	}, nil).Once()

	ms.On("ListSchemas", mock.Anything, mock.MatchedBy(func(r *pb.ListSchemasRequest) bool {
		return r.PageToken == "page2"
	})).Return(&pb.ListSchemasResponse{
		Schemas: []*pb.Schema{{Id: "s2", Name: "b", Version: 1, CreatedAt: timestamppb.Now()}},
	}, nil).Once()

	schemas, err := client.ListSchemas(context.Background())
	require.NoError(t, err)
	assert.Len(t, schemas, 2)
}

func TestUpdateSchema_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("UpdateSchema", mock.Anything, mock.Anything).Return(&pb.UpdateSchemaResponse{
		Schema: &pb.Schema{Id: "s1", Version: 2, CreatedAt: timestamppb.Now()},
	}, nil)

	s, err := client.UpdateSchema(context.Background(), "s1", []Field{{Path: "new", Type: "STRING"}}, []string{"old"}, "v2")
	require.NoError(t, err)
	assert.Equal(t, int32(2), s.Version)
}

func TestDeleteSchema_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("DeleteSchema", mock.Anything, mock.Anything).Return(&pb.DeleteSchemaResponse{}, nil)
	require.NoError(t, client.DeleteSchema(context.Background(), "s1"))
}

func TestExportSchema_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("ExportSchema", mock.Anything, mock.Anything).Return(&pb.ExportSchemaResponse{
		YamlContent: []byte("syntax: v1"),
	}, nil)

	data, err := client.ExportSchema(context.Background(), "s1", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "syntax")
}

func TestImportSchema_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("ImportSchema", mock.Anything, mock.Anything).Return(&pb.ImportSchemaResponse{
		Schema: &pb.Schema{Id: "s1", Name: "imported", Version: 1, CreatedAt: timestamppb.Now()},
	}, nil)

	s, err := client.ImportSchema(context.Background(), []byte("syntax: v1\nname: imported"))
	require.NoError(t, err)
	assert.Equal(t, "imported", s.Name)
}

func TestImportSchema_AutoPublish(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("ImportSchema", mock.Anything, mock.MatchedBy(func(r *pb.ImportSchemaRequest) bool {
		return r.AutoPublish
	})).Return(&pb.ImportSchemaResponse{
		Schema: &pb.Schema{Id: "s1", Version: 1, Published: true, CreatedAt: timestamppb.Now()},
	}, nil)

	s, err := client.ImportSchema(context.Background(), []byte("yaml"), true)
	require.NoError(t, err)
	assert.True(t, s.Published)
}

func TestPublishSchema_AlreadyPublished(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("PublishSchema", mock.Anything, mock.Anything).
		Return((*pb.PublishSchemaResponse)(nil), status.Error(codes.FailedPrecondition, "already published"))

	_, err := client.PublishSchema(context.Background(), "s1", 1)
	assert.ErrorIs(t, err, ErrFailedPrecondition)
}

// --- Tenant operations ---

func TestGetTenant_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("GetTenant", mock.Anything, mock.Anything).Return(&pb.GetTenantResponse{
		Tenant: &pb.Tenant{Id: "t1", Name: "acme", SchemaId: "s1", SchemaVersion: 1, CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
	}, nil)

	tenant, err := client.GetTenant(context.Background(), "t1")
	require.NoError(t, err)
	assert.Equal(t, "acme", tenant.Name)
}

func TestListTenants_WithSchemaFilter(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("ListTenants", mock.Anything, mock.MatchedBy(func(r *pb.ListTenantsRequest) bool {
		return r.SchemaId != nil && *r.SchemaId == "s1"
	})).Return(&pb.ListTenantsResponse{
		Tenants: []*pb.Tenant{
			{Id: "t1", Name: "acme", SchemaId: "s1", SchemaVersion: 1, CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
		},
	}, nil)

	tenants, err := client.ListTenants(context.Background(), "s1")
	require.NoError(t, err)
	assert.Len(t, tenants, 1)
}

func TestListTenants_NoFilter(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("ListTenants", mock.Anything, mock.MatchedBy(func(r *pb.ListTenantsRequest) bool {
		return r.SchemaId == nil
	})).Return(&pb.ListTenantsResponse{}, nil)

	_, err := client.ListTenants(context.Background(), "")
	require.NoError(t, err)
}

func TestUpdateTenantName(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("UpdateTenant", mock.Anything, mock.MatchedBy(func(r *pb.UpdateTenantRequest) bool {
		return r.Name != nil && *r.Name == "new-name"
	})).Return(&pb.UpdateTenantResponse{
		Tenant: &pb.Tenant{Id: "t1", Name: "new-name", CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
	}, nil)

	tenant, err := client.UpdateTenantName(context.Background(), "t1", "new-name")
	require.NoError(t, err)
	assert.Equal(t, "new-name", tenant.Name)
}

func TestUpdateTenantSchema(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("UpdateTenant", mock.Anything, mock.MatchedBy(func(r *pb.UpdateTenantRequest) bool {
		return r.SchemaVersion != nil && *r.SchemaVersion == int32(2)
	})).Return(&pb.UpdateTenantResponse{
		Tenant: &pb.Tenant{Id: "t1", SchemaVersion: 2, CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
	}, nil)

	tenant, err := client.UpdateTenantSchema(context.Background(), "t1", 2)
	require.NoError(t, err)
	assert.Equal(t, int32(2), tenant.SchemaVersion)
}

func TestDeleteTenant_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("DeleteTenant", mock.Anything, mock.Anything).Return(&pb.DeleteTenantResponse{}, nil)
	require.NoError(t, client.DeleteTenant(context.Background(), "t1"))
}

// --- Lock operations ---

func TestListFieldLocks_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("ListFieldLocks", mock.Anything, mock.Anything).Return(&pb.ListFieldLocksResponse{
		Locks: []*pb.FieldLock{
			{TenantId: "t1", FieldPath: "app.fee", LockedValues: []string{"0.01"}},
		},
	}, nil)

	locks, err := client.ListFieldLocks(context.Background(), "t1")
	require.NoError(t, err)
	assert.Len(t, locks, 1)
	assert.Equal(t, "app.fee", locks[0].FieldPath)
	assert.Equal(t, []string{"0.01"}, locks[0].LockedValues)
}

func TestLockField_WithValues(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.On("LockField", mock.Anything, mock.MatchedBy(func(r *pb.LockFieldRequest) bool {
		return len(r.LockedValues) == 2
	})).Return(&pb.LockFieldResponse{}, nil)

	require.NoError(t, client.LockField(context.Background(), "t1", "app.env", "prod", "staging"))
}

// --- Audit operations ---

func TestGetFieldUsage_Success(t *testing.T) {
	ma := &mockAudit{}
	client := New(nil, nil, ma)

	lastBy := "reader"
	ma.On("GetFieldUsage", mock.Anything, mock.Anything).Return(&pb.GetFieldUsageResponse{
		Stats: &pb.UsageStats{TenantId: "t1", FieldPath: "app.fee", ReadCount: 42, LastReadBy: &lastBy},
	}, nil)

	stats, err := client.GetFieldUsage(context.Background(), "t1", "app.fee", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(42), stats.ReadCount)
}

func TestGetTenantUsage_Success(t *testing.T) {
	ma := &mockAudit{}
	client := New(nil, nil, ma)

	ma.On("GetTenantUsage", mock.Anything, mock.Anything).Return(&pb.GetTenantUsageResponse{
		FieldStats: []*pb.UsageStats{
			{FieldPath: "a", ReadCount: 10},
			{FieldPath: "b", ReadCount: 5},
		},
	}, nil)

	stats, err := client.GetTenantUsage(context.Background(), "t1", nil, nil)
	require.NoError(t, err)
	assert.Len(t, stats, 2)
}

func TestGetUnusedFields_Success(t *testing.T) {
	ma := &mockAudit{}
	client := New(nil, nil, ma)

	ma.On("GetUnusedFields", mock.Anything, mock.Anything).Return(&pb.GetUnusedFieldsResponse{
		FieldPaths: []string{"old.field"},
	}, nil)

	paths, err := client.GetUnusedFields(context.Background(), "t1", time.Now())
	require.NoError(t, err)
	assert.Equal(t, []string{"old.field"}, paths)
}

// --- Audit filters ---

func TestAuditFilters(t *testing.T) {
	req := &pb.QueryWriteLogRequest{}

	WithAuditTenant("t1")(req)
	assert.Equal(t, "t1", *req.TenantId)

	WithAuditActor("admin")(req)
	assert.Equal(t, "admin", *req.Actor)

	WithAuditField("app.fee")(req)
	assert.Equal(t, "app.fee", *req.FieldPath)

	start := time.Now().Add(-time.Hour)
	end := time.Now()
	WithAuditTimeRange(&start, &end)(req)
	assert.NotNil(t, req.StartTime)
	assert.NotNil(t, req.EndTime)
}

// --- Error mapping ---

func TestMapError(t *testing.T) {
	assert.Nil(t, mapError(nil))
	assert.ErrorIs(t, mapError(status.Error(codes.NotFound, "")), ErrNotFound)
	assert.ErrorIs(t, mapError(status.Error(codes.AlreadyExists, "")), ErrAlreadyExists)
	assert.ErrorIs(t, mapError(status.Error(codes.FailedPrecondition, "")), ErrFailedPrecondition)
	assert.Error(t, mapError(status.Error(codes.Internal, "something")))
}

// --- Service not configured ---

func TestServiceNotConfigured_AllMethods(t *testing.T) {
	client := New(nil, nil, nil)
	ctx := context.Background()

	_, err := client.GetSchemaVersion(ctx, "s1", 1)
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.ListSchemas(ctx)
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.UpdateSchema(ctx, "s1", nil, nil, "")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	err = client.DeleteSchema(ctx, "s1")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.PublishSchema(ctx, "s1", 1)
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.ExportSchema(ctx, "s1", nil)
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.ImportSchema(ctx, nil)
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.GetTenant(ctx, "t1")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.ListTenants(ctx, "")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.UpdateTenantName(ctx, "t1", "new")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.UpdateTenantSchema(ctx, "t1", 2)
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	err = client.DeleteTenant(ctx, "t1")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	err = client.LockField(ctx, "t1", "x")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	err = client.UnlockField(ctx, "t1", "x")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.ListFieldLocks(ctx, "t1")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.GetFieldUsage(ctx, "t1", "x", nil, nil)
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.GetTenantUsage(ctx, "t1", nil, nil)
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.GetUnusedFields(ctx, "t1", time.Now())
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.GetConfigVersion(ctx, "t1", 1)
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.RollbackConfig(ctx, "t1", 1, "")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.ExportConfig(ctx, "t1", nil)
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.ImportConfig(ctx, "t1", nil, "")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)
}
