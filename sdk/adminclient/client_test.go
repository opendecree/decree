package adminclient

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

// --- Mock SchemaService ---

type mockSchema struct{ mock.Mock }

func (m *mockSchema) CreateSchema(ctx context.Context, in *pb.CreateSchemaRequest, opts ...grpc.CallOption) (*pb.CreateSchemaResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.CreateSchemaResponse), args.Error(1)
}

func (m *mockSchema) GetSchema(ctx context.Context, in *pb.GetSchemaRequest, opts ...grpc.CallOption) (*pb.GetSchemaResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetSchemaResponse), args.Error(1)
}

func (m *mockSchema) ListSchemas(ctx context.Context, in *pb.ListSchemasRequest, opts ...grpc.CallOption) (*pb.ListSchemasResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ListSchemasResponse), args.Error(1)
}

func (m *mockSchema) UpdateSchema(ctx context.Context, in *pb.UpdateSchemaRequest, opts ...grpc.CallOption) (*pb.UpdateSchemaResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.UpdateSchemaResponse), args.Error(1)
}

func (m *mockSchema) DeleteSchema(ctx context.Context, in *pb.DeleteSchemaRequest, opts ...grpc.CallOption) (*pb.DeleteSchemaResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.DeleteSchemaResponse), args.Error(1)
}

func (m *mockSchema) PublishSchema(ctx context.Context, in *pb.PublishSchemaRequest, opts ...grpc.CallOption) (*pb.PublishSchemaResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.PublishSchemaResponse), args.Error(1)
}

func (m *mockSchema) CreateTenant(ctx context.Context, in *pb.CreateTenantRequest, opts ...grpc.CallOption) (*pb.CreateTenantResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.CreateTenantResponse), args.Error(1)
}

func (m *mockSchema) GetTenant(ctx context.Context, in *pb.GetTenantRequest, opts ...grpc.CallOption) (*pb.GetTenantResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetTenantResponse), args.Error(1)
}

func (m *mockSchema) ListTenants(ctx context.Context, in *pb.ListTenantsRequest, opts ...grpc.CallOption) (*pb.ListTenantsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ListTenantsResponse), args.Error(1)
}

func (m *mockSchema) UpdateTenant(ctx context.Context, in *pb.UpdateTenantRequest, opts ...grpc.CallOption) (*pb.UpdateTenantResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.UpdateTenantResponse), args.Error(1)
}

func (m *mockSchema) DeleteTenant(ctx context.Context, in *pb.DeleteTenantRequest, opts ...grpc.CallOption) (*pb.DeleteTenantResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.DeleteTenantResponse), args.Error(1)
}

func (m *mockSchema) LockField(ctx context.Context, in *pb.LockFieldRequest, opts ...grpc.CallOption) (*pb.LockFieldResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.LockFieldResponse), args.Error(1)
}

func (m *mockSchema) UnlockField(ctx context.Context, in *pb.UnlockFieldRequest, opts ...grpc.CallOption) (*pb.UnlockFieldResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.UnlockFieldResponse), args.Error(1)
}

func (m *mockSchema) ListFieldLocks(ctx context.Context, in *pb.ListFieldLocksRequest, opts ...grpc.CallOption) (*pb.ListFieldLocksResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ListFieldLocksResponse), args.Error(1)
}

func (m *mockSchema) ExportSchema(ctx context.Context, in *pb.ExportSchemaRequest, opts ...grpc.CallOption) (*pb.ExportSchemaResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ExportSchemaResponse), args.Error(1)
}

func (m *mockSchema) ImportSchema(ctx context.Context, in *pb.ImportSchemaRequest, opts ...grpc.CallOption) (*pb.ImportSchemaResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ImportSchemaResponse), args.Error(1)
}

// --- Mock ConfigService ---

type mockConfig struct{ mock.Mock }

func (m *mockConfig) GetConfig(ctx context.Context, in *pb.GetConfigRequest, opts ...grpc.CallOption) (*pb.GetConfigResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetConfigResponse), args.Error(1)
}

func (m *mockConfig) GetField(ctx context.Context, in *pb.GetFieldRequest, opts ...grpc.CallOption) (*pb.GetFieldResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetFieldResponse), args.Error(1)
}

func (m *mockConfig) GetFields(ctx context.Context, in *pb.GetFieldsRequest, opts ...grpc.CallOption) (*pb.GetFieldsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetFieldsResponse), args.Error(1)
}

func (m *mockConfig) SetField(ctx context.Context, in *pb.SetFieldRequest, opts ...grpc.CallOption) (*pb.SetFieldResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.SetFieldResponse), args.Error(1)
}

func (m *mockConfig) SetFields(ctx context.Context, in *pb.SetFieldsRequest, opts ...grpc.CallOption) (*pb.SetFieldsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.SetFieldsResponse), args.Error(1)
}

func (m *mockConfig) ListVersions(ctx context.Context, in *pb.ListVersionsRequest, opts ...grpc.CallOption) (*pb.ListVersionsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ListVersionsResponse), args.Error(1)
}

func (m *mockConfig) GetVersion(ctx context.Context, in *pb.GetVersionRequest, opts ...grpc.CallOption) (*pb.GetVersionResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetVersionResponse), args.Error(1)
}

func (m *mockConfig) RollbackToVersion(ctx context.Context, in *pb.RollbackToVersionRequest, opts ...grpc.CallOption) (*pb.RollbackToVersionResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.RollbackToVersionResponse), args.Error(1)
}

func (m *mockConfig) Subscribe(ctx context.Context, in *pb.SubscribeRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.SubscribeResponse], error) {
	return nil, nil
}

func (m *mockConfig) ExportConfig(ctx context.Context, in *pb.ExportConfigRequest, opts ...grpc.CallOption) (*pb.ExportConfigResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ExportConfigResponse), args.Error(1)
}

func (m *mockConfig) ImportConfig(ctx context.Context, in *pb.ImportConfigRequest, opts ...grpc.CallOption) (*pb.ImportConfigResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.ImportConfigResponse), args.Error(1)
}

// --- Mock AuditService ---

type mockAudit struct{ mock.Mock }

func (m *mockAudit) QueryWriteLog(ctx context.Context, in *pb.QueryWriteLogRequest, opts ...grpc.CallOption) (*pb.QueryWriteLogResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.QueryWriteLogResponse), args.Error(1)
}

func (m *mockAudit) GetFieldUsage(ctx context.Context, in *pb.GetFieldUsageRequest, opts ...grpc.CallOption) (*pb.GetFieldUsageResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetFieldUsageResponse), args.Error(1)
}

func (m *mockAudit) GetTenantUsage(ctx context.Context, in *pb.GetTenantUsageRequest, opts ...grpc.CallOption) (*pb.GetTenantUsageResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetTenantUsageResponse), args.Error(1)
}

func (m *mockAudit) GetUnusedFields(ctx context.Context, in *pb.GetUnusedFieldsRequest, opts ...grpc.CallOption) (*pb.GetUnusedFieldsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.GetUnusedFieldsResponse), args.Error(1)
}

// --- Tests ---

func TestCreateSchema_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil, WithSubject("admin"))
	ctx := context.Background()

	ms.On("CreateSchema", mock.Anything, mock.Anything).Return(&pb.CreateSchemaResponse{
		Schema: &pb.Schema{Id: "s1", Name: "payments", Version: 1, CreatedAt: timestamppb.Now()},
	}, nil)

	s, err := client.CreateSchema(ctx, "payments", []Field{{Path: "a", Type: "STRING"}}, "test")
	require.NoError(t, err)
	assert.Equal(t, "payments", s.Name)
	assert.Equal(t, int32(1), s.Version)
}

func TestGetSchema_NotFound(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)
	ctx := context.Background()

	ms.On("GetSchema", mock.Anything, mock.Anything).
		Return((*pb.GetSchemaResponse)(nil), status.Error(codes.NotFound, "not found"))

	_, err := client.GetSchema(ctx, "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCreateTenant_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)
	ctx := context.Background()

	ms.On("CreateTenant", mock.Anything, mock.Anything).Return(&pb.CreateTenantResponse{
		Tenant: &pb.Tenant{Id: "t1", Name: "acme", SchemaId: "s1", SchemaVersion: 1, CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
	}, nil)

	tenant, err := client.CreateTenant(ctx, "acme", "s1", 1)
	require.NoError(t, err)
	assert.Equal(t, "acme", tenant.Name)
}

func TestCreateTenant_UnpublishedSchema(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)
	ctx := context.Background()

	ms.On("CreateTenant", mock.Anything, mock.Anything).
		Return((*pb.CreateTenantResponse)(nil), status.Error(codes.FailedPrecondition, "not published"))

	_, err := client.CreateTenant(ctx, "acme", "s1", 1)
	assert.ErrorIs(t, err, ErrFailedPrecondition)
}

func TestLockUnlockField(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)
	ctx := context.Background()

	ms.On("LockField", mock.Anything, mock.Anything).Return(&pb.LockFieldResponse{}, nil)
	ms.On("UnlockField", mock.Anything, mock.Anything).Return(&pb.UnlockFieldResponse{}, nil)

	require.NoError(t, client.LockField(ctx, "t1", "fee"))
	require.NoError(t, client.UnlockField(ctx, "t1", "fee"))
}

func TestListConfigVersions_AutoPaginate(t *testing.T) {
	mc := &mockConfig{}
	client := New(nil, mc, nil)
	ctx := context.Background()

	// Page 1
	mc.On("ListVersions", mock.Anything, mock.MatchedBy(func(r *pb.ListVersionsRequest) bool {
		return r.PageToken == ""
	})).Return(&pb.ListVersionsResponse{
		Versions:      []*pb.ConfigVersion{{Version: 3, CreatedAt: timestamppb.Now()}, {Version: 2, CreatedAt: timestamppb.Now()}},
		NextPageToken: "page2",
	}, nil).Once()

	// Page 2
	mc.On("ListVersions", mock.Anything, mock.MatchedBy(func(r *pb.ListVersionsRequest) bool {
		return r.PageToken == "page2"
	})).Return(&pb.ListVersionsResponse{
		Versions: []*pb.ConfigVersion{{Version: 1, CreatedAt: timestamppb.Now()}},
	}, nil).Once()

	versions, err := client.ListConfigVersions(ctx, "t1")
	require.NoError(t, err)
	assert.Len(t, versions, 3)
}

func TestRollbackConfig_Success(t *testing.T) {
	mc := &mockConfig{}
	client := New(nil, mc, nil)
	ctx := context.Background()

	mc.On("RollbackToVersion", mock.Anything, mock.Anything).Return(&pb.RollbackToVersionResponse{
		ConfigVersion: &pb.ConfigVersion{Version: 5, CreatedAt: timestamppb.Now()},
	}, nil)

	v, err := client.RollbackConfig(ctx, "t1", 2, "rollback to v2")
	require.NoError(t, err)
	assert.Equal(t, int32(5), v.Version)
}

func TestExportImportConfig(t *testing.T) {
	mc := &mockConfig{}
	client := New(nil, mc, nil)
	ctx := context.Background()

	mc.On("ExportConfig", mock.Anything, mock.Anything).Return(&pb.ExportConfigResponse{
		YamlContent: []byte("syntax: v1\nvalues:\n  a:\n    value: x\n"),
	}, nil)

	data, err := client.ExportConfig(ctx, "t1", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "syntax")

	mc.On("ImportConfig", mock.Anything, mock.Anything).Return(&pb.ImportConfigResponse{
		ConfigVersion: &pb.ConfigVersion{Version: 3, CreatedAt: timestamppb.Now()},
	}, nil)

	v, err := client.ImportConfig(ctx, "t1", data, "imported")
	require.NoError(t, err)
	assert.Equal(t, int32(3), v.Version)
}

func TestQueryWriteLog_Success(t *testing.T) {
	ma := &mockAudit{}
	client := New(nil, nil, ma)
	ctx := context.Background()

	ma.On("QueryWriteLog", mock.Anything, mock.Anything).Return(&pb.QueryWriteLogResponse{
		Entries: []*pb.AuditEntry{
			{Id: "e1", TenantId: "t1", Actor: "admin", Action: "set_field", CreatedAt: timestamppb.Now()},
		},
	}, nil)

	entries, err := client.QueryWriteLog(ctx, WithAuditTenant("t1"))
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "set_field", entries[0].Action)
}

func TestServiceNotConfigured(t *testing.T) {
	client := New(nil, nil, nil)
	ctx := context.Background()

	_, err := client.GetSchema(ctx, "s1")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.ListConfigVersions(ctx, "t1")
	assert.ErrorIs(t, err, ErrServiceNotConfigured)

	_, err = client.QueryWriteLog(ctx)
	assert.ErrorIs(t, err, ErrServiceNotConfigured)
}
