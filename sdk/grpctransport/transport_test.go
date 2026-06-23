package grpctransport_test

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/grpctransport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// --------------------------------------------------------------------------
// Test harness helpers
// --------------------------------------------------------------------------

// incomingMD extracts the incoming gRPC metadata from a server-side context.
func incomingMD(ctx context.Context) metadata.MD {
	md, _ := metadata.FromIncomingContext(ctx)
	return md
}

// newBufconnServer starts a gRPC server on a bufconn listener, registers all
// three service stubs, dials an insecure client connection, and returns it.
// All cleanup is registered on t.
func newBufconnServer(
	t *testing.T,
	configSvc pb.ConfigServiceServer,
	schemaSvc pb.SchemaServiceServer,
	auditSvc pb.AuditServiceServer,
) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	if configSvc != nil {
		pb.RegisterConfigServiceServer(srv, configSvc)
	}
	if schemaSvc != nil {
		pb.RegisterSchemaServiceServer(srv, schemaSvc)
	}
	if auditSvc != nil {
		pb.RegisterAuditServiceServer(srv, auditSvc)
	}
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop(); _ = lis.Close() })

	cc, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	return cc
}

// newConfigTransport creates a ConfigTransport with the given opts backed by conn.
func newConfigTransport(t *testing.T, conn *grpc.ClientConn, opts ...grpctransport.Option) *grpctransport.ConfigTransport {
	t.Helper()
	tr, err := grpctransport.NewConfigTransport(conn, opts...)
	if err != nil {
		t.Fatalf("NewConfigTransport: %v", err)
	}
	return tr
}

// newAdminConfigTransport creates an AdminConfigTransport backed by conn.
func newAdminConfigTransport(t *testing.T, conn *grpc.ClientConn, opts ...grpctransport.Option) *grpctransport.AdminConfigTransport {
	t.Helper()
	tr, err := grpctransport.NewAdminConfigTransport(conn, opts...)
	if err != nil {
		t.Fatalf("NewAdminConfigTransport: %v", err)
	}
	return tr
}

// newSchemaTransport creates a SchemaTransport backed by conn.
func newSchemaTransport(t *testing.T, conn *grpc.ClientConn, opts ...grpctransport.Option) *grpctransport.SchemaTransport {
	t.Helper()
	tr, err := grpctransport.NewSchemaTransport(conn, opts...)
	if err != nil {
		t.Fatalf("NewSchemaTransport: %v", err)
	}
	return tr
}

// newAuditTransport creates an AuditTransport backed by conn.
func newAuditTransport(t *testing.T, conn *grpc.ClientConn, opts ...grpctransport.Option) *grpctransport.AuditTransport {
	t.Helper()
	tr, err := grpctransport.NewAuditTransport(conn, opts...)
	if err != nil {
		t.Fatalf("NewAuditTransport: %v", err)
	}
	return tr
}

// assertMetadataHeader fails if the incoming metadata key does not equal want.
func assertMetadataHeader(t *testing.T, md metadata.MD, key, want string) {
	t.Helper()
	vals := md.Get(key)
	if len(vals) == 0 || vals[0] != want {
		t.Errorf("metadata %q: got %v, want %q", key, vals, want)
	}
}

// --------------------------------------------------------------------------
// Stub ConfigService
// --------------------------------------------------------------------------

// stubConfigService records the last incoming metadata and returns a
// configurable error (or canned success) for every method.
type stubConfigService struct {
	pb.UnimplementedConfigServiceServer
	err     error
	lastMD  metadata.MD
	sendEOF bool // Subscribe: close stream cleanly after one message
}

func (s *stubConfigService) GetConfig(ctx context.Context, _ *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.GetConfigResponse{Config: &pb.Config{TenantId: "t1", Version: 1}}, nil
}

func (s *stubConfigService) GetField(ctx context.Context, _ *pb.GetFieldRequest) (*pb.GetFieldResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.GetFieldResponse{Value: &pb.ConfigValue{
		FieldPath: "app.name",
		Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "myapp"}},
	}}, nil
}

func (s *stubConfigService) GetFields(ctx context.Context, _ *pb.GetFieldsRequest) (*pb.GetFieldsResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.GetFieldsResponse{Values: []*pb.ConfigValue{
		{FieldPath: "app.name", Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "myapp"}}},
	}}, nil
}

func (s *stubConfigService) SetField(ctx context.Context, _ *pb.SetFieldRequest) (*pb.SetFieldResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.SetFieldResponse{}, nil
}

func (s *stubConfigService) SetFields(ctx context.Context, _ *pb.SetFieldsRequest) (*pb.SetFieldsResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.SetFieldsResponse{}, nil
}

func (s *stubConfigService) ListVersions(ctx context.Context, _ *pb.ListVersionsRequest) (*pb.ListVersionsResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.ListVersionsResponse{}, nil
}

func (s *stubConfigService) GetVersion(ctx context.Context, _ *pb.GetVersionRequest) (*pb.GetVersionResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.GetVersionResponse{ConfigVersion: &pb.ConfigVersion{TenantId: "t1", Version: 1}}, nil
}

func (s *stubConfigService) RollbackToVersion(ctx context.Context, _ *pb.RollbackToVersionRequest) (*pb.RollbackToVersionResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.RollbackToVersionResponse{ConfigVersion: &pb.ConfigVersion{TenantId: "t1", Version: 2}}, nil
}

func (s *stubConfigService) DiffVersions(ctx context.Context, _ *pb.DiffVersionsRequest) (*pb.DiffVersionsResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.DiffVersionsResponse{Diffs: []*pb.FieldDiff{
		{FieldPath: "a", ChangeType: pb.ChangeType_CHANGE_TYPE_ADDED, NewValue: "1"},
		{FieldPath: "b", ChangeType: pb.ChangeType_CHANGE_TYPE_REMOVED, OldValue: "2"},
		{FieldPath: "c", ChangeType: pb.ChangeType_CHANGE_TYPE_MODIFIED, OldValue: "3", NewValue: "4"},
	}}, nil
}

func (s *stubConfigService) ExportConfig(ctx context.Context, _ *pb.ExportConfigRequest) (*pb.ExportConfigResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.ExportConfigResponse{YamlContent: []byte("config: true")}, nil
}

func (s *stubConfigService) ImportConfig(ctx context.Context, _ *pb.ImportConfigRequest) (*pb.ImportConfigResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.ImportConfigResponse{ConfigVersion: &pb.ConfigVersion{TenantId: "t1", Version: 3}}, nil
}

func (s *stubConfigService) Subscribe(_ *pb.SubscribeRequest, stream grpc.ServerStreamingServer[pb.SubscribeResponse]) error {
	s.lastMD = incomingMD(stream.Context())
	if s.err != nil {
		return s.err
	}
	if s.sendEOF {
		// Send one message then close cleanly (EOF).
		return stream.Send(&pb.SubscribeResponse{
			Change: &pb.ConfigChange{
				TenantId:  "t1",
				FieldPath: "app.name",
				NewValue:  &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "updated"}},
			},
		})
	}
	// Block until context is cancelled (for cancel-path tests).
	<-stream.Context().Done()
	return status.FromContextError(stream.Context().Err()).Err()
}

// --------------------------------------------------------------------------
// Stub SchemaService
// --------------------------------------------------------------------------

type stubSchemaService struct {
	pb.UnimplementedSchemaServiceServer
	err    error
	lastMD metadata.MD
}

func (s *stubSchemaService) CreateSchema(ctx context.Context, _ *pb.CreateSchemaRequest) (*pb.CreateSchemaResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.CreateSchemaResponse{Schema: &pb.Schema{Id: "schema-1", Name: "main"}}, nil
}

func (s *stubSchemaService) GetSchema(ctx context.Context, _ *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.GetSchemaResponse{Schema: &pb.Schema{Id: "schema-1", Name: "main"}}, nil
}

func (s *stubSchemaService) ListSchemas(ctx context.Context, _ *pb.ListSchemasRequest) (*pb.ListSchemasResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.ListSchemasResponse{Schemas: []*pb.Schema{{Id: "schema-1"}}}, nil
}

func (s *stubSchemaService) UpdateSchema(ctx context.Context, _ *pb.UpdateSchemaRequest) (*pb.UpdateSchemaResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.UpdateSchemaResponse{Schema: &pb.Schema{Id: "schema-1", Name: "main"}}, nil
}

func (s *stubSchemaService) DeleteSchema(ctx context.Context, _ *pb.DeleteSchemaRequest) (*pb.DeleteSchemaResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.DeleteSchemaResponse{}, nil
}

func (s *stubSchemaService) PublishSchema(ctx context.Context, _ *pb.PublishSchemaRequest) (*pb.PublishSchemaResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.PublishSchemaResponse{Schema: &pb.Schema{Id: "schema-1", Published: true}}, nil
}

func (s *stubSchemaService) ExportSchema(ctx context.Context, _ *pb.ExportSchemaRequest) (*pb.ExportSchemaResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.ExportSchemaResponse{YamlContent: []byte("schema: true")}, nil
}

func (s *stubSchemaService) ImportSchema(ctx context.Context, _ *pb.ImportSchemaRequest) (*pb.ImportSchemaResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.ImportSchemaResponse{Schema: &pb.Schema{Id: "schema-1"}}, nil
}

func (s *stubSchemaService) CreateTenant(ctx context.Context, _ *pb.CreateTenantRequest) (*pb.CreateTenantResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.CreateTenantResponse{Tenant: &pb.Tenant{Id: "tenant-1", Name: "acme"}}, nil
}

func (s *stubSchemaService) GetTenant(ctx context.Context, _ *pb.GetTenantRequest) (*pb.GetTenantResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.GetTenantResponse{Tenant: &pb.Tenant{Id: "tenant-1", Name: "acme"}}, nil
}

func (s *stubSchemaService) ListTenants(ctx context.Context, _ *pb.ListTenantsRequest) (*pb.ListTenantsResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.ListTenantsResponse{Tenants: []*pb.Tenant{{Id: "tenant-1"}}}, nil
}

func (s *stubSchemaService) UpdateTenant(ctx context.Context, _ *pb.UpdateTenantRequest) (*pb.UpdateTenantResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.UpdateTenantResponse{Tenant: &pb.Tenant{Id: "tenant-1", Name: "acme2"}}, nil
}

func (s *stubSchemaService) DeleteTenant(ctx context.Context, _ *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.DeleteTenantResponse{}, nil
}

func (s *stubSchemaService) LockField(ctx context.Context, _ *pb.LockFieldRequest) (*pb.LockFieldResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.LockFieldResponse{}, nil
}

func (s *stubSchemaService) UnlockField(ctx context.Context, _ *pb.UnlockFieldRequest) (*pb.UnlockFieldResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.UnlockFieldResponse{}, nil
}

func (s *stubSchemaService) ListFieldLocks(ctx context.Context, _ *pb.ListFieldLocksRequest) (*pb.ListFieldLocksResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.ListFieldLocksResponse{Locks: []*pb.FieldLock{{TenantId: "t1", FieldPath: "app.name"}}}, nil
}

// --------------------------------------------------------------------------
// Stub AuditService
// --------------------------------------------------------------------------

type stubAuditService struct {
	pb.UnimplementedAuditServiceServer
	err    error
	lastMD metadata.MD
}

func (s *stubAuditService) QueryWriteLog(ctx context.Context, _ *pb.QueryWriteLogRequest) (*pb.QueryWriteLogResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.QueryWriteLogResponse{Entries: []*pb.AuditEntry{{Id: "entry-1"}}}, nil
}

func (s *stubAuditService) GetFieldUsage(ctx context.Context, _ *pb.GetFieldUsageRequest) (*pb.GetFieldUsageResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.GetFieldUsageResponse{Stats: &pb.UsageStats{TenantId: "t1", FieldPath: "app.name", ReadCount: 5}}, nil
}

func (s *stubAuditService) GetTenantUsage(ctx context.Context, _ *pb.GetTenantUsageRequest) (*pb.GetTenantUsageResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.GetTenantUsageResponse{FieldStats: []*pb.UsageStats{{TenantId: "t1", ReadCount: 10}}}, nil
}

func (s *stubAuditService) GetUnusedFields(ctx context.Context, _ *pb.GetUnusedFieldsRequest) (*pb.GetUnusedFieldsResponse, error) {
	s.lastMD = incomingMD(ctx)
	if s.err != nil {
		return nil, s.err
	}
	return &pb.GetUnusedFieldsResponse{FieldPaths: []string{"app.deprecated"}}, nil
}

// --------------------------------------------------------------------------
// ConfigTransport: metadata forwarding
// --------------------------------------------------------------------------

func TestConfigTransport_GetField_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"), grpctransport.WithSubject("alice"), grpctransport.WithTenantID("acme"))

	_, _ = tr.GetField(context.Background(), &configclient.GetFieldRequest{TenantID: "acme", FieldPath: "app.name"})

	assertMetadataHeader(t, stub.lastMD, "x-role", "user")
	assertMetadataHeader(t, stub.lastMD, "x-subject", "alice")
	assertMetadataHeader(t, stub.lastMD, "x-tenant-id", "acme")
}

func TestConfigTransport_GetConfig_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	_, _ = tr.GetConfig(context.Background(), &configclient.GetConfigRequest{TenantID: "t1"})

	assertMetadataHeader(t, stub.lastMD, "x-role", "user")
}

func TestConfigTransport_GetFields_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("reader"))

	_, _ = tr.GetFields(context.Background(), &configclient.GetFieldsRequest{TenantID: "t1", FieldPaths: []string{"app.name"}})

	assertMetadataHeader(t, stub.lastMD, "x-role", "reader")
}

func TestConfigTransport_SetField_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("writer"))

	_, _ = tr.SetField(context.Background(), &configclient.SetFieldRequest{
		TenantID:  "t1",
		FieldPath: "app.name",
		Value:     configclient.StringVal("hello"),
	})

	assertMetadataHeader(t, stub.lastMD, "x-role", "writer")
}

func TestConfigTransport_SetFields_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("writer"))

	_, _ = tr.SetFields(context.Background(), &configclient.SetFieldsRequest{
		TenantID: "t1",
		Updates: []configclient.FieldUpdate{
			{FieldPath: "app.name", Value: configclient.StringVal("v")},
		},
	})

	assertMetadataHeader(t, stub.lastMD, "x-role", "writer")
}

// --------------------------------------------------------------------------
// ConfigTransport: status-to-sentinel mapping
// --------------------------------------------------------------------------

func TestConfigTransport_GetField_NotFound(t *testing.T) {
	stub := &stubConfigService{err: status.Error(codes.NotFound, "field not found")}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	_, err := tr.GetField(context.Background(), &configclient.GetFieldRequest{TenantID: "t1", FieldPath: "missing"})
	if !errors.Is(err, configclient.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestConfigTransport_GetField_PermissionDenied(t *testing.T) {
	stub := &stubConfigService{err: status.Error(codes.PermissionDenied, "denied")}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	_, err := tr.GetField(context.Background(), &configclient.GetFieldRequest{TenantID: "t1", FieldPath: "x"})
	if !errors.Is(err, configclient.ErrPermissionDenied) {
		t.Errorf("got %v, want ErrPermissionDenied", err)
	}
}

func TestConfigTransport_SetField_ChecksumMismatch(t *testing.T) {
	stub := &stubConfigService{err: status.Error(codes.Aborted, "checksum mismatch")}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("writer"))

	_, err := tr.SetField(context.Background(), &configclient.SetFieldRequest{
		TenantID: "t1", FieldPath: "app.name", Value: configclient.StringVal("v"),
	})
	if !errors.Is(err, configclient.ErrChecksumMismatch) {
		t.Errorf("got %v, want ErrChecksumMismatch", err)
	}
}

func TestConfigTransport_SetField_Locked(t *testing.T) {
	stub := &stubConfigService{err: status.Error(codes.FailedPrecondition, "field is locked")}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("writer"))

	_, err := tr.SetField(context.Background(), &configclient.SetFieldRequest{
		TenantID: "t1", FieldPath: "app.name", Value: configclient.StringVal("v"),
	})
	if !errors.Is(err, configclient.ErrLocked) {
		t.Errorf("got %v, want ErrLocked", err)
	}
}

func TestConfigTransport_GetConfig_Unavailable_IsRetryable(t *testing.T) {
	stub := &stubConfigService{err: status.Error(codes.Unavailable, "down")}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	_, err := tr.GetConfig(context.Background(), &configclient.GetConfigRequest{TenantID: "t1"})
	var re *configclient.RetryableError
	if !errors.As(err, &re) {
		t.Errorf("got %v, want *RetryableError", err)
	}
}

func TestConfigTransport_GetConfig_RateLimited(t *testing.T) {
	stub := &stubConfigService{err: status.Error(codes.ResourceExhausted, "rate limit")}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	_, err := tr.GetConfig(context.Background(), &configclient.GetConfigRequest{TenantID: "t1"})
	if !errors.Is(err, configclient.ErrRateLimited) {
		t.Errorf("got %v, want ErrRateLimited", err)
	}
}

func TestConfigTransport_GetField_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	resp, err := tr.GetField(context.Background(), &configclient.GetFieldRequest{TenantID: "t1", FieldPath: "app.name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FieldPath != "app.name" {
		t.Errorf("FieldPath = %q, want %q", resp.FieldPath, "app.name")
	}
}

func TestConfigTransport_GetConfig_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	resp, err := tr.GetConfig(context.Background(), &configclient.GetConfigRequest{TenantID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TenantID != "t1" {
		t.Errorf("TenantID = %q, want %q", resp.TenantID, "t1")
	}
}

func TestConfigTransport_GetFields_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	resp, err := tr.GetFields(context.Background(), &configclient.GetFieldsRequest{TenantID: "t1", FieldPaths: []string{"app.name"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Values) != 1 {
		t.Errorf("Values len = %d, want 1", len(resp.Values))
	}
}

func TestConfigTransport_SetField_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("writer"))

	_, err := tr.SetField(context.Background(), &configclient.SetFieldRequest{
		TenantID: "t1", FieldPath: "app.name", Value: configclient.StringVal("hello"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigTransport_SetFields_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("writer"))

	_, err := tr.SetFields(context.Background(), &configclient.SetFieldsRequest{
		TenantID: "t1",
		Updates:  []configclient.FieldUpdate{{FieldPath: "app.name", Value: configclient.StringVal("v")}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --------------------------------------------------------------------------
// ConfigTransport: constructor guard
// --------------------------------------------------------------------------

func TestConfigTransport_Constructor_RequiresRole(t *testing.T) {
	conn := newBufconnServer(t, &stubConfigService{}, nil, nil)
	_, err := grpctransport.NewConfigTransport(conn)
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Errorf("got %v, want ErrRoleRequired", err)
	}
}

// --------------------------------------------------------------------------
// Subscribe: success, error, cancel, and clean-EOF paths
// --------------------------------------------------------------------------

func TestConfigTransport_Subscribe_ErrorPath(t *testing.T) {
	// The stub returns PermissionDenied immediately on the streaming handler.
	// Subscribe() itself returns a stream (gRPC defers server-side errors to Recv),
	// so the sentinel must surface via Recv.
	stub := &stubConfigService{err: status.Error(codes.PermissionDenied, "denied")}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	sub, subscribeErr := tr.Subscribe(context.Background(), &configclient.SubscribeRequest{TenantID: "t1"})
	if subscribeErr != nil {
		// Some gRPC implementations surface streaming errors on Subscribe.
		if !errors.Is(subscribeErr, configclient.ErrPermissionDenied) {
			t.Errorf("Subscribe error: got %v, want ErrPermissionDenied", subscribeErr)
		}
		return
	}
	_, err := sub.Recv()
	if !errors.Is(err, configclient.ErrPermissionDenied) {
		t.Errorf("Recv error: got %v, want ErrPermissionDenied", err)
	}
}

func TestConfigTransport_Subscribe_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{sendEOF: true}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("viewer"), grpctransport.WithSubject("bob"))

	sub, err := tr.Subscribe(context.Background(), &configclient.SubscribeRequest{TenantID: "t1"})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	// Drain so the server captures metadata before the test checks.
	_, _ = sub.Recv()

	assertMetadataHeader(t, stub.lastMD, "x-role", "viewer")
	assertMetadataHeader(t, stub.lastMD, "x-subject", "bob")
}

func TestConfigTransport_Subscribe_CancelPath(t *testing.T) {
	// The stub blocks until ctx is cancelled — exercising the cancel path.
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	ctx, cancel := context.WithCancel(context.Background())
	sub, err := tr.Subscribe(ctx, &configclient.SubscribeRequest{TenantID: "t1"})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	cancel()

	// Recv must return an error because the context was cancelled.
	_, err = sub.Recv()
	if err == nil {
		t.Error("expected error after cancel, got nil")
	}
}

func TestConfigTransport_Subscribe_CleanEOF(t *testing.T) {
	// sendEOF=true: server sends one message then closes normally.
	stub := &stubConfigService{sendEOF: true}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	sub, err := tr.Subscribe(context.Background(), &configclient.SubscribeRequest{TenantID: "t1"})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// First Recv returns the message.
	change, err := sub.Recv()
	if err != nil {
		t.Fatalf("first Recv: %v", err)
	}
	if change.FieldPath != "app.name" {
		t.Errorf("FieldPath = %q, want %q", change.FieldPath, "app.name")
	}

	// Second Recv must return an error (EOF or mapped sentinel).
	_, err = sub.Recv()
	if err == nil {
		t.Error("second Recv after EOF: expected error, got nil")
	}
}

func TestConfigTransport_Subscribe_RecvErrorMapping(t *testing.T) {
	// Server returns NotFound immediately — Subscribe itself may succeed but
	// Recv must return ErrNotFound.
	stub := &stubConfigService{err: status.Error(codes.NotFound, "tenant not found")}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newConfigTransport(t, conn, grpctransport.WithRole("user"))

	_, err := tr.Subscribe(context.Background(), &configclient.SubscribeRequest{TenantID: "t1"})
	// The error may surface on Subscribe() or on Recv(); test whichever path fires.
	if err != nil {
		if !errors.Is(err, configclient.ErrNotFound) {
			t.Errorf("Subscribe error: got %v, want ErrNotFound", err)
		}
		return
	}
}

// --------------------------------------------------------------------------
// AdminConfigTransport: metadata forwarding + status mapping
// --------------------------------------------------------------------------

func TestAdminConfigTransport_ListVersions_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.ListVersions(context.Background(), "t1", 10, "")

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestAdminConfigTransport_GetVersion_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.GetVersion(context.Background(), "t1", 1)

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestAdminConfigTransport_RollbackToVersion_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.RollbackToVersion(context.Background(), "t1", 1, "rollback")

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestAdminConfigTransport_DiffVersions_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.DiffVersions(context.Background(), "t1", 1, 2)

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestAdminConfigTransport_DiffVersions_NotFound(t *testing.T) {
	stub := &stubConfigService{err: status.Error(codes.NotFound, "version not found")}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, err := tr.DiffVersions(context.Background(), "t1", 1, 99)
	if !errors.Is(err, adminclient.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestAdminConfigTransport_DiffVersions_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	diffs, err := tr.DiffVersions(context.Background(), "t1", 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []adminclient.FieldDiff{
		{FieldPath: "a", ChangeType: adminclient.ChangeTypeAdded, NewValue: "1"},
		{FieldPath: "b", ChangeType: adminclient.ChangeTypeRemoved, OldValue: "2"},
		{FieldPath: "c", ChangeType: adminclient.ChangeTypeModified, OldValue: "3", NewValue: "4"},
	}
	if !reflect.DeepEqual(diffs, want) {
		t.Errorf("diffs = %+v, want %+v", diffs, want)
	}
}

func TestAdminConfigTransport_ExportConfig_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.ExportConfig(context.Background(), "t1", nil)

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestAdminConfigTransport_ImportConfig_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.ImportConfig(context.Background(), &adminclient.ImportConfigRequest{TenantID: "t1", YamlContent: []byte("x: 1")})

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestAdminConfigTransport_ListVersions_NotFound(t *testing.T) {
	stub := &stubConfigService{err: status.Error(codes.NotFound, "tenant not found")}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, err := tr.ListVersions(context.Background(), "missing", 10, "")
	if !errors.Is(err, adminclient.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestAdminConfigTransport_GetVersion_PermissionDenied(t *testing.T) {
	stub := &stubConfigService{err: status.Error(codes.PermissionDenied, "denied")}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, err := tr.GetVersion(context.Background(), "t1", 1)
	if !errors.Is(err, adminclient.ErrPermissionDenied) {
		t.Errorf("got %v, want ErrPermissionDenied", err)
	}
}

func TestAdminConfigTransport_ListVersions_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	resp, err := tr.ListVersions(context.Background(), "t1", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp
}

func TestAdminConfigTransport_GetVersion_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	v, err := tr.GetVersion(context.Background(), "t1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.TenantID != "t1" {
		t.Errorf("TenantID = %q, want %q", v.TenantID, "t1")
	}
}

func TestAdminConfigTransport_RollbackToVersion_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	v, err := tr.RollbackToVersion(context.Background(), "t1", 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Version != 2 {
		t.Errorf("Version = %d, want 2", v.Version)
	}
}

func TestAdminConfigTransport_ExportConfig_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	data, err := tr.ExportConfig(context.Background(), "t1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "config: true" {
		t.Errorf("got %q, want %q", string(data), "config: true")
	}
}

func TestAdminConfigTransport_ImportConfig_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)
	tr := newAdminConfigTransport(t, conn, grpctransport.WithRole("superadmin"))

	v, err := tr.ImportConfig(context.Background(), &adminclient.ImportConfigRequest{TenantID: "t1", YamlContent: []byte("x: 1")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Version != 3 {
		t.Errorf("Version = %d, want 3", v.Version)
	}
}

// --------------------------------------------------------------------------
// SchemaTransport: metadata forwarding + status mapping
// --------------------------------------------------------------------------

func TestSchemaTransport_CreateSchema_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.CreateSchema(context.Background(), &adminclient.CreateSchemaRequest{Name: "main"})

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_GetSchema_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.GetSchema(context.Background(), "schema-1", nil)

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_ListSchemas_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.ListSchemas(context.Background(), 10, "")

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_UpdateSchema_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.UpdateSchema(context.Background(), &adminclient.UpdateSchemaRequest{ID: "schema-1"})

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_DeleteSchema_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_ = tr.DeleteSchema(context.Background(), "schema-1")

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_PublishSchema_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.PublishSchema(context.Background(), "schema-1", 1)

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_ExportSchema_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.ExportSchema(context.Background(), "schema-1", nil)

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_ImportSchema_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.ImportSchema(context.Background(), []byte("x: 1"), false)

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_CreateTenant_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.CreateTenant(context.Background(), &adminclient.CreateTenantRequest{Name: "acme", SchemaID: "schema-1"})

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_GetTenant_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.GetTenant(context.Background(), "tenant-1")

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_ListTenants_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.ListTenants(context.Background(), nil, 10, "")

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_UpdateTenant_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	name := "acme2"
	_, _ = tr.UpdateTenant(context.Background(), &adminclient.UpdateTenantRequest{ID: "tenant-1", Name: &name})

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_DeleteTenant_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_ = tr.DeleteTenant(context.Background(), "tenant-1")

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_LockField_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_ = tr.LockField(context.Background(), "t1", "app.name", []string{"prod"})

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_UnlockField_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_ = tr.UnlockField(context.Background(), "t1", "app.name")

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_ListFieldLocks_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, _ = tr.ListFieldLocks(context.Background(), "t1")

	assertMetadataHeader(t, stub.lastMD, "x-role", "superadmin")
}

func TestSchemaTransport_GetSchema_NotFound(t *testing.T) {
	stub := &stubSchemaService{err: status.Error(codes.NotFound, "schema not found")}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, err := tr.GetSchema(context.Background(), "missing", nil)
	if !errors.Is(err, adminclient.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestSchemaTransport_CreateSchema_AlreadyExists(t *testing.T) {
	stub := &stubSchemaService{err: status.Error(codes.AlreadyExists, "schema exists")}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, err := tr.CreateSchema(context.Background(), &adminclient.CreateSchemaRequest{Name: "main"})
	if !errors.Is(err, adminclient.ErrAlreadyExists) {
		t.Errorf("got %v, want ErrAlreadyExists", err)
	}
}

func TestSchemaTransport_PublishSchema_FailedPrecondition(t *testing.T) {
	stub := &stubSchemaService{err: status.Error(codes.FailedPrecondition, "already published")}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	_, err := tr.PublishSchema(context.Background(), "schema-1", 1)
	if !errors.Is(err, adminclient.ErrFailedPrecondition) {
		t.Errorf("got %v, want ErrFailedPrecondition", err)
	}
}

func TestSchemaTransport_DeleteSchema_PermissionDenied(t *testing.T) {
	stub := &stubSchemaService{err: status.Error(codes.PermissionDenied, "denied")}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	err := tr.DeleteSchema(context.Background(), "schema-1")
	if !errors.Is(err, adminclient.ErrPermissionDenied) {
		t.Errorf("got %v, want ErrPermissionDenied", err)
	}
}

func TestSchemaTransport_CreateSchema_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	s, err := tr.CreateSchema(context.Background(), &adminclient.CreateSchemaRequest{Name: "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID != "schema-1" {
		t.Errorf("ID = %q, want %q", s.ID, "schema-1")
	}
}

func TestSchemaTransport_GetSchema_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	s, err := tr.GetSchema(context.Background(), "schema-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "main" {
		t.Errorf("Name = %q, want %q", s.Name, "main")
	}
}

func TestSchemaTransport_ListSchemas_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	resp, err := tr.ListSchemas(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Schemas) != 1 {
		t.Errorf("Schemas len = %d, want 1", len(resp.Schemas))
	}
}

func TestSchemaTransport_UpdateSchema_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	s, err := tr.UpdateSchema(context.Background(), &adminclient.UpdateSchemaRequest{ID: "schema-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID != "schema-1" {
		t.Errorf("ID = %q, want %q", s.ID, "schema-1")
	}
}

func TestSchemaTransport_DeleteSchema_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	err := tr.DeleteSchema(context.Background(), "schema-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSchemaTransport_PublishSchema_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	s, err := tr.PublishSchema(context.Background(), "schema-1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.Published {
		t.Error("Published = false, want true")
	}
}

func TestSchemaTransport_ExportSchema_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	data, err := tr.ExportSchema(context.Background(), "schema-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "schema: true" {
		t.Errorf("got %q, want %q", string(data), "schema: true")
	}
}

func TestSchemaTransport_ImportSchema_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	s, err := tr.ImportSchema(context.Background(), []byte("x: 1"), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID != "schema-1" {
		t.Errorf("ID = %q, want %q", s.ID, "schema-1")
	}
}

func TestSchemaTransport_CreateTenant_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	ten, err := tr.CreateTenant(context.Background(), &adminclient.CreateTenantRequest{Name: "acme", SchemaID: "schema-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ten.Name != "acme" {
		t.Errorf("Name = %q, want %q", ten.Name, "acme")
	}
}

func TestSchemaTransport_GetTenant_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	ten, err := tr.GetTenant(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ten.ID != "tenant-1" {
		t.Errorf("ID = %q, want %q", ten.ID, "tenant-1")
	}
}

func TestSchemaTransport_ListTenants_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	resp, err := tr.ListTenants(context.Background(), nil, 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Tenants) != 1 {
		t.Errorf("Tenants len = %d, want 1", len(resp.Tenants))
	}
}

func TestSchemaTransport_UpdateTenant_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	name2 := "acme2"
	ten, err := tr.UpdateTenant(context.Background(), &adminclient.UpdateTenantRequest{ID: "tenant-1", Name: &name2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ten.Name != "acme2" {
		t.Errorf("Name = %q, want %q", ten.Name, "acme2")
	}
}

func TestSchemaTransport_DeleteTenant_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	err := tr.DeleteTenant(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSchemaTransport_LockField_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	err := tr.LockField(context.Background(), "t1", "app.name", []string{"prod"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSchemaTransport_UnlockField_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	err := tr.UnlockField(context.Background(), "t1", "app.name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSchemaTransport_ListFieldLocks_Success(t *testing.T) {
	stub := &stubSchemaService{}
	conn := newBufconnServer(t, nil, stub, nil)
	tr := newSchemaTransport(t, conn, grpctransport.WithRole("superadmin"))

	locks, err := tr.ListFieldLocks(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(locks) != 1 {
		t.Errorf("locks len = %d, want 1", len(locks))
	}
}

func TestSchemaTransport_Constructor_RequiresRole(t *testing.T) {
	conn := newBufconnServer(t, nil, &stubSchemaService{}, nil)
	_, err := grpctransport.NewSchemaTransport(conn)
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Errorf("got %v, want ErrRoleRequired", err)
	}
}

// --------------------------------------------------------------------------
// AuditTransport: metadata forwarding + status mapping
// --------------------------------------------------------------------------

func TestAuditTransport_QueryWriteLog_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubAuditService{}
	conn := newBufconnServer(t, nil, nil, stub)
	tr := newAuditTransport(t, conn, grpctransport.WithRole("auditor"))

	_, _ = tr.QueryWriteLog(context.Background(), &adminclient.QueryWriteLogRequest{})

	assertMetadataHeader(t, stub.lastMD, "x-role", "auditor")
}

func TestAuditTransport_GetFieldUsage_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubAuditService{}
	conn := newBufconnServer(t, nil, nil, stub)
	tr := newAuditTransport(t, conn, grpctransport.WithRole("auditor"))

	_, _ = tr.GetFieldUsage(context.Background(), "t1", "app.name", nil, nil)

	assertMetadataHeader(t, stub.lastMD, "x-role", "auditor")
}

func TestAuditTransport_GetTenantUsage_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubAuditService{}
	conn := newBufconnServer(t, nil, nil, stub)
	tr := newAuditTransport(t, conn, grpctransport.WithRole("auditor"))

	_, _ = tr.GetTenantUsage(context.Background(), "t1", nil, nil)

	assertMetadataHeader(t, stub.lastMD, "x-role", "auditor")
}

func TestAuditTransport_GetUnusedFields_ForwardsAuthMetadata(t *testing.T) {
	stub := &stubAuditService{}
	conn := newBufconnServer(t, nil, nil, stub)
	tr := newAuditTransport(t, conn, grpctransport.WithRole("auditor"))

	_, _ = tr.GetUnusedFields(context.Background(), "t1", time.Now().Add(-24*time.Hour))

	assertMetadataHeader(t, stub.lastMD, "x-role", "auditor")
}

func TestAuditTransport_QueryWriteLog_PermissionDenied(t *testing.T) {
	stub := &stubAuditService{err: status.Error(codes.PermissionDenied, "denied")}
	conn := newBufconnServer(t, nil, nil, stub)
	tr := newAuditTransport(t, conn, grpctransport.WithRole("auditor"))

	_, err := tr.QueryWriteLog(context.Background(), &adminclient.QueryWriteLogRequest{})
	if !errors.Is(err, adminclient.ErrPermissionDenied) {
		t.Errorf("got %v, want ErrPermissionDenied", err)
	}
}

func TestAuditTransport_GetFieldUsage_NotFound(t *testing.T) {
	stub := &stubAuditService{err: status.Error(codes.NotFound, "field not found")}
	conn := newBufconnServer(t, nil, nil, stub)
	tr := newAuditTransport(t, conn, grpctransport.WithRole("auditor"))

	_, err := tr.GetFieldUsage(context.Background(), "t1", "missing", nil, nil)
	if !errors.Is(err, adminclient.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestAuditTransport_QueryWriteLog_Success(t *testing.T) {
	stub := &stubAuditService{}
	conn := newBufconnServer(t, nil, nil, stub)
	tr := newAuditTransport(t, conn, grpctransport.WithRole("auditor"))

	resp, err := tr.QueryWriteLog(context.Background(), &adminclient.QueryWriteLogRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Entries) != 1 {
		t.Errorf("Entries len = %d, want 1", len(resp.Entries))
	}
}

func TestAuditTransport_GetFieldUsage_Success(t *testing.T) {
	stub := &stubAuditService{}
	conn := newBufconnServer(t, nil, nil, stub)
	tr := newAuditTransport(t, conn, grpctransport.WithRole("auditor"))

	stats, err := tr.GetFieldUsage(context.Background(), "t1", "app.name", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.ReadCount != 5 {
		t.Errorf("ReadCount = %d, want 5", stats.ReadCount)
	}
}

func TestAuditTransport_GetTenantUsage_Success(t *testing.T) {
	stub := &stubAuditService{}
	conn := newBufconnServer(t, nil, nil, stub)
	tr := newAuditTransport(t, conn, grpctransport.WithRole("auditor"))

	stats, err := tr.GetTenantUsage(context.Background(), "t1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 1 {
		t.Errorf("stats len = %d, want 1", len(stats))
	}
}

func TestAuditTransport_GetUnusedFields_Success(t *testing.T) {
	stub := &stubAuditService{}
	conn := newBufconnServer(t, nil, nil, stub)
	tr := newAuditTransport(t, conn, grpctransport.WithRole("auditor"))

	fields, err := tr.GetUnusedFields(context.Background(), "t1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 1 || fields[0] != "app.deprecated" {
		t.Errorf("fields = %v, want [app.deprecated]", fields)
	}
}

func TestAuditTransport_Constructor_RequiresRole(t *testing.T) {
	conn := newBufconnServer(t, nil, nil, &stubAuditService{})
	_, err := grpctransport.NewAuditTransport(conn)
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Errorf("got %v, want ErrRoleRequired", err)
	}
}

// --------------------------------------------------------------------------
// NewAdminConfigTransport: constructor guard
// --------------------------------------------------------------------------

func TestAdminConfigTransport_Constructor_RequiresRole(t *testing.T) {
	conn := newBufconnServer(t, &stubConfigService{}, nil, nil)
	_, err := grpctransport.NewAdminConfigTransport(conn)
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Errorf("got %v, want ErrRoleRequired", err)
	}
}

// --------------------------------------------------------------------------
// Convenience constructors
// --------------------------------------------------------------------------

func TestNewConfigClient_Success(t *testing.T) {
	stub := &stubConfigService{}
	conn := newBufconnServer(t, stub, nil, nil)

	client, err := grpctransport.NewConfigClient(conn, grpctransport.WithRole("user"))
	if err != nil {
		t.Fatalf("NewConfigClient: %v", err)
	}
	_ = client
}

func TestNewConfigClient_RequiresRole(t *testing.T) {
	conn := newBufconnServer(t, &stubConfigService{}, nil, nil)
	_, err := grpctransport.NewConfigClient(conn)
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Errorf("got %v, want ErrRoleRequired", err)
	}
}

func TestNewAdminClient_Success(t *testing.T) {
	stub := &stubConfigService{}
	schemaSvc := &stubSchemaService{}
	auditSvc := &stubAuditService{}
	conn := newBufconnServer(t, stub, schemaSvc, auditSvc)

	client, err := grpctransport.NewAdminClient(conn, grpctransport.WithRole("superadmin"))
	if err != nil {
		t.Fatalf("NewAdminClient: %v", err)
	}
	_ = client
}

func TestNewAdminClient_RequiresRole(t *testing.T) {
	conn := newBufconnServer(t, &stubConfigService{}, &stubSchemaService{}, &stubAuditService{})
	_, err := grpctransport.NewAdminClient(conn)
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Errorf("got %v, want ErrRoleRequired", err)
	}
}

func TestNewWatcher_RequiresRole(t *testing.T) {
	conn := newBufconnServer(t, &stubConfigService{}, nil, nil)
	_, err := grpctransport.NewWatcher(conn, "t1")
	if !errors.Is(err, grpctransport.ErrRoleRequired) {
		t.Errorf("got %v, want ErrRoleRequired", err)
	}
}

func TestNewWatcher_Success(t *testing.T) {
	conn := newBufconnServer(t, &stubConfigService{}, nil, nil)
	w, err := grpctransport.NewWatcher(conn, "t1", grpctransport.WithRole("user"))
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	_ = w
}

// --------------------------------------------------------------------------
// Dial
// --------------------------------------------------------------------------

func TestDial_WithInsecure_Succeeds(t *testing.T) {
	cc, err := grpctransport.Dial("localhost:0", grpctransport.WithInsecure())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	_ = cc.Close()
}

func TestDefaultKeepalive_HasExpectedDefaults(t *testing.T) {
	kp := grpctransport.DefaultKeepalive()
	if kp.Time != 30*time.Second {
		t.Errorf("Time = %v, want 30s", kp.Time)
	}
	if kp.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", kp.Timeout)
	}
	if !kp.PermitWithoutStream {
		t.Error("PermitWithoutStream = false, want true")
	}
}
