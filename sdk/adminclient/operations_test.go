package adminclient

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// --- Schema operations ---

func TestGetSchemaVersion_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.getSchemaFn = func(_ context.Context, r *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
		if r.Id != "s1" || r.Version == nil || *r.Version != int32(2) {
			t.Fatalf("unexpected request: %v", r)
		}
		return &pb.GetSchemaResponse{
			Schema: &pb.Schema{Id: "s1", Name: "test", Version: 2, CreatedAt: timestamppb.Now()},
		}, nil
	}

	s, err := client.GetSchemaVersion(context.Background(), "s1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := s.Version; got != int32(2) {
		t.Errorf("got Version %v, want %v", got, int32(2))
	}
}

func TestListSchemas_AutoPaginate(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.listSchemasFn = func(_ context.Context, r *pb.ListSchemasRequest) (*pb.ListSchemasResponse, error) {
		if r.PageToken == "" {
			return &pb.ListSchemasResponse{
				Schemas:       []*pb.Schema{{Id: "s1", Name: "a", Version: 1, CreatedAt: timestamppb.Now()}},
				NextPageToken: "page2",
			}, nil
		}
		return &pb.ListSchemasResponse{
			Schemas: []*pb.Schema{{Id: "s2", Name: "b", Version: 1, CreatedAt: timestamppb.Now()}},
		}, nil
	}

	schemas, err := client.ListSchemas(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(schemas) != 2 {
		t.Errorf("got len %d, want %d", len(schemas), 2)
	}
}

func TestUpdateSchema_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.updateSchemaFn = func(_ context.Context, _ *pb.UpdateSchemaRequest) (*pb.UpdateSchemaResponse, error) {
		return &pb.UpdateSchemaResponse{
			Schema: &pb.Schema{Id: "s1", Version: 2, CreatedAt: timestamppb.Now()},
		}, nil
	}

	s, err := client.UpdateSchema(context.Background(), "s1", []Field{{Path: "new", Type: "STRING"}}, []string{"old"}, "v2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := s.Version; got != int32(2) {
		t.Errorf("got Version %v, want %v", got, int32(2))
	}
}

func TestDeleteSchema_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.deleteSchemaFn = func(_ context.Context, _ *pb.DeleteSchemaRequest) (*pb.DeleteSchemaResponse, error) {
		return &pb.DeleteSchemaResponse{}, nil
	}
	if err := client.DeleteSchema(context.Background(), "s1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExportSchema_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.exportSchemaFn = func(_ context.Context, _ *pb.ExportSchemaRequest) (*pb.ExportSchemaResponse, error) {
		return &pb.ExportSchemaResponse{
			YamlContent: []byte("syntax: v1"),
		}, nil
	}

	data, err := client.ExportSchema(context.Background(), "s1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "syntax") {
		t.Errorf("expected %q to contain %q", string(data), "syntax")
	}
}

func TestImportSchema_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.importSchemaFn = func(_ context.Context, _ *pb.ImportSchemaRequest) (*pb.ImportSchemaResponse, error) {
		return &pb.ImportSchemaResponse{
			Schema: &pb.Schema{Id: "s1", Name: "imported", Version: 1, CreatedAt: timestamppb.Now()},
		}, nil
	}

	s, err := client.ImportSchema(context.Background(), []byte("syntax: v1\nname: imported"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := s.Name; got != "imported" {
		t.Errorf("got Name %v, want %v", got, "imported")
	}
}

func TestImportSchema_AutoPublish(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.importSchemaFn = func(_ context.Context, r *pb.ImportSchemaRequest) (*pb.ImportSchemaResponse, error) {
		if !r.AutoPublish {
			t.Fatal("expected AutoPublish to be true")
		}
		return &pb.ImportSchemaResponse{
			Schema: &pb.Schema{Id: "s1", Version: 1, Published: true, CreatedAt: timestamppb.Now()},
		}, nil
	}

	s, err := client.ImportSchema(context.Background(), []byte("yaml"), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.Published {
		t.Error("expected Published to be true")
	}
}

func TestPublishSchema_AlreadyPublished(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.publishSchemaFn = func(_ context.Context, _ *pb.PublishSchemaRequest) (*pb.PublishSchemaResponse, error) {
		return nil, status.Error(codes.FailedPrecondition, "already published")
	}

	_, err := client.PublishSchema(context.Background(), "s1", 1)
	if !errors.Is(err, ErrFailedPrecondition) {
		t.Errorf("got error %v, want %v", err, ErrFailedPrecondition)
	}
}

// --- Tenant operations ---

func TestGetTenant_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.getTenantFn = func(_ context.Context, _ *pb.GetTenantRequest) (*pb.GetTenantResponse, error) {
		return &pb.GetTenantResponse{
			Tenant: &pb.Tenant{Id: "t1", Name: "acme", SchemaId: "s1", SchemaVersion: 1, CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
		}, nil
	}

	tenant, err := client.GetTenant(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tenant.Name; got != "acme" {
		t.Errorf("got Name %v, want %v", got, "acme")
	}
}

func TestListTenants_WithSchemaFilter(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.listTenantsFn = func(_ context.Context, r *pb.ListTenantsRequest) (*pb.ListTenantsResponse, error) {
		if r.SchemaId == nil || *r.SchemaId != "s1" {
			t.Fatalf("expected SchemaId filter s1, got %v", r.SchemaId)
		}
		return &pb.ListTenantsResponse{
			Tenants: []*pb.Tenant{
				{Id: "t1", Name: "acme", SchemaId: "s1", SchemaVersion: 1, CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
			},
		}, nil
	}

	tenants, err := client.ListTenants(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tenants) != 1 {
		t.Errorf("got len %d, want %d", len(tenants), 1)
	}
}

func TestListTenants_NoFilter(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.listTenantsFn = func(_ context.Context, r *pb.ListTenantsRequest) (*pb.ListTenantsResponse, error) {
		if r.SchemaId != nil {
			t.Fatalf("expected nil SchemaId, got %v", *r.SchemaId)
		}
		return &pb.ListTenantsResponse{}, nil
	}

	_, err := client.ListTenants(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateTenantName(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.updateTenantFn = func(_ context.Context, r *pb.UpdateTenantRequest) (*pb.UpdateTenantResponse, error) {
		if r.Name == nil || *r.Name != "new-name" {
			t.Fatalf("expected Name new-name, got %v", r.Name)
		}
		return &pb.UpdateTenantResponse{
			Tenant: &pb.Tenant{Id: "t1", Name: "new-name", CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
		}, nil
	}

	tenant, err := client.UpdateTenantName(context.Background(), "t1", "new-name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tenant.Name; got != "new-name" {
		t.Errorf("got Name %v, want %v", got, "new-name")
	}
}

func TestUpdateTenantSchema(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.updateTenantFn = func(_ context.Context, r *pb.UpdateTenantRequest) (*pb.UpdateTenantResponse, error) {
		if r.SchemaVersion == nil || *r.SchemaVersion != int32(2) {
			t.Fatalf("expected SchemaVersion 2, got %v", r.SchemaVersion)
		}
		return &pb.UpdateTenantResponse{
			Tenant: &pb.Tenant{Id: "t1", SchemaVersion: 2, CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
		}, nil
	}

	tenant, err := client.UpdateTenantSchema(context.Background(), "t1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tenant.SchemaVersion; got != int32(2) {
		t.Errorf("got SchemaVersion %v, want %v", got, int32(2))
	}
}

func TestDeleteTenant_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.deleteTenantFn = func(_ context.Context, _ *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error) {
		return &pb.DeleteTenantResponse{}, nil
	}
	if err := client.DeleteTenant(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Lock operations ---

func TestListFieldLocks_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.listFieldLocksFn = func(_ context.Context, _ *pb.ListFieldLocksRequest) (*pb.ListFieldLocksResponse, error) {
		return &pb.ListFieldLocksResponse{
			Locks: []*pb.FieldLock{
				{TenantId: "t1", FieldPath: "app.fee", LockedValues: []string{"0.01"}},
			},
		}, nil
	}

	locks, err := client.ListFieldLocks(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(locks) != 1 {
		t.Errorf("got len %d, want %d", len(locks), 1)
	}
	if got := locks[0].FieldPath; got != "app.fee" {
		t.Errorf("got FieldPath %v, want %v", got, "app.fee")
	}
	if !reflect.DeepEqual(locks[0].LockedValues, []string{"0.01"}) {
		t.Errorf("got LockedValues %v, want %v", locks[0].LockedValues, []string{"0.01"})
	}
}

func TestLockField_WithValues(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)

	ms.lockFieldFn = func(_ context.Context, r *pb.LockFieldRequest) (*pb.LockFieldResponse, error) {
		if len(r.LockedValues) != 2 {
			t.Fatalf("expected 2 locked values, got %d", len(r.LockedValues))
		}
		return &pb.LockFieldResponse{}, nil
	}

	if err := client.LockField(context.Background(), "t1", "app.env", "prod", "staging"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Audit operations ---

func TestGetFieldUsage_Success(t *testing.T) {
	ma := &mockAudit{}
	client := New(nil, nil, ma)

	lastBy := "reader"
	ma.getFieldUsageFn = func(_ context.Context, _ *pb.GetFieldUsageRequest) (*pb.GetFieldUsageResponse, error) {
		return &pb.GetFieldUsageResponse{
			Stats: &pb.UsageStats{TenantId: "t1", FieldPath: "app.fee", ReadCount: 42, LastReadBy: &lastBy},
		}, nil
	}

	stats, err := client.GetFieldUsage(context.Background(), "t1", "app.fee", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := stats.ReadCount; got != int64(42) {
		t.Errorf("got ReadCount %v, want %v", got, int64(42))
	}
}

func TestGetTenantUsage_Success(t *testing.T) {
	ma := &mockAudit{}
	client := New(nil, nil, ma)

	ma.getTenantUsageFn = func(_ context.Context, _ *pb.GetTenantUsageRequest) (*pb.GetTenantUsageResponse, error) {
		return &pb.GetTenantUsageResponse{
			FieldStats: []*pb.UsageStats{
				{FieldPath: "a", ReadCount: 10},
				{FieldPath: "b", ReadCount: 5},
			},
		}, nil
	}

	stats, err := client.GetTenantUsage(context.Background(), "t1", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stats) != 2 {
		t.Errorf("got len %d, want %d", len(stats), 2)
	}
}

func TestGetUnusedFields_Success(t *testing.T) {
	ma := &mockAudit{}
	client := New(nil, nil, ma)

	ma.getUnusedFieldsFn = func(_ context.Context, _ *pb.GetUnusedFieldsRequest) (*pb.GetUnusedFieldsResponse, error) {
		return &pb.GetUnusedFieldsResponse{
			FieldPaths: []string{"old.field"},
		}, nil
	}

	paths, err := client.GetUnusedFields(context.Background(), "t1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(paths, []string{"old.field"}) {
		t.Errorf("got %v, want %v", paths, []string{"old.field"})
	}
}

// --- Audit filters ---

func TestAuditFilters(t *testing.T) {
	req := &pb.QueryWriteLogRequest{}

	WithAuditTenant("t1")(req)
	if got := *req.TenantId; got != "t1" {
		t.Errorf("got TenantId %v, want %v", got, "t1")
	}

	WithAuditActor("admin")(req)
	if got := *req.Actor; got != "admin" {
		t.Errorf("got Actor %v, want %v", got, "admin")
	}

	WithAuditField("app.fee")(req)
	if got := *req.FieldPath; got != "app.fee" {
		t.Errorf("got FieldPath %v, want %v", got, "app.fee")
	}

	start := time.Now().Add(-time.Hour)
	end := time.Now()
	WithAuditTimeRange(&start, &end)(req)
	if req.StartTime == nil {
		t.Error("expected non-nil StartTime")
	}
	if req.EndTime == nil {
		t.Error("expected non-nil EndTime")
	}
}

// --- Error mapping ---

func TestMapError(t *testing.T) {
	if got := mapError(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
	if !errors.Is(mapError(status.Error(codes.NotFound, "")), ErrNotFound) {
		t.Errorf("got error %v, want %v", mapError(status.Error(codes.NotFound, "")), ErrNotFound)
	}
	if !errors.Is(mapError(status.Error(codes.AlreadyExists, "")), ErrAlreadyExists) {
		t.Errorf("got error %v, want %v", mapError(status.Error(codes.AlreadyExists, "")), ErrAlreadyExists)
	}
	if !errors.Is(mapError(status.Error(codes.FailedPrecondition, "")), ErrFailedPrecondition) {
		t.Errorf("got error %v, want %v", mapError(status.Error(codes.FailedPrecondition, "")), ErrFailedPrecondition)
	}
	if err := mapError(status.Error(codes.Internal, "something")); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Service not configured ---

func TestServiceNotConfigured_AllMethods(t *testing.T) {
	client := New(nil, nil, nil)
	ctx := context.Background()

	_, err := client.GetSchemaVersion(ctx, "s1", 1)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ListSchemas(ctx)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.UpdateSchema(ctx, "s1", nil, nil, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	err = client.DeleteSchema(ctx, "s1")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.PublishSchema(ctx, "s1", 1)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ExportSchema(ctx, "s1", nil)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ImportSchema(ctx, nil)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.GetTenant(ctx, "t1")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ListTenants(ctx, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.UpdateTenantName(ctx, "t1", "new")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.UpdateTenantSchema(ctx, "t1", 2)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	err = client.DeleteTenant(ctx, "t1")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	err = client.LockField(ctx, "t1", "x")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	err = client.UnlockField(ctx, "t1", "x")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ListFieldLocks(ctx, "t1")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.GetFieldUsage(ctx, "t1", "x", nil, nil)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.GetTenantUsage(ctx, "t1", nil, nil)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.GetUnusedFields(ctx, "t1", time.Now())
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.GetConfigVersion(ctx, "t1", 1)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.RollbackConfig(ctx, "t1", 1, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ExportConfig(ctx, "t1", nil)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ImportConfig(ctx, "t1", nil, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}
}
