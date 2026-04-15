package adminclient

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// --- Mock SchemaService ---

type mockSchema struct {
	pb.SchemaServiceClient

	createSchemaFn   func(ctx context.Context, in *pb.CreateSchemaRequest) (*pb.CreateSchemaResponse, error)
	getSchemaFn      func(ctx context.Context, in *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error)
	listSchemasFn    func(ctx context.Context, in *pb.ListSchemasRequest) (*pb.ListSchemasResponse, error)
	updateSchemaFn   func(ctx context.Context, in *pb.UpdateSchemaRequest) (*pb.UpdateSchemaResponse, error)
	deleteSchemaFn   func(ctx context.Context, in *pb.DeleteSchemaRequest) (*pb.DeleteSchemaResponse, error)
	publishSchemaFn  func(ctx context.Context, in *pb.PublishSchemaRequest) (*pb.PublishSchemaResponse, error)
	createTenantFn   func(ctx context.Context, in *pb.CreateTenantRequest) (*pb.CreateTenantResponse, error)
	getTenantFn      func(ctx context.Context, in *pb.GetTenantRequest) (*pb.GetTenantResponse, error)
	listTenantsFn    func(ctx context.Context, in *pb.ListTenantsRequest) (*pb.ListTenantsResponse, error)
	updateTenantFn   func(ctx context.Context, in *pb.UpdateTenantRequest) (*pb.UpdateTenantResponse, error)
	deleteTenantFn   func(ctx context.Context, in *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error)
	lockFieldFn      func(ctx context.Context, in *pb.LockFieldRequest) (*pb.LockFieldResponse, error)
	unlockFieldFn    func(ctx context.Context, in *pb.UnlockFieldRequest) (*pb.UnlockFieldResponse, error)
	listFieldLocksFn func(ctx context.Context, in *pb.ListFieldLocksRequest) (*pb.ListFieldLocksResponse, error)
	exportSchemaFn   func(ctx context.Context, in *pb.ExportSchemaRequest) (*pb.ExportSchemaResponse, error)
	importSchemaFn   func(ctx context.Context, in *pb.ImportSchemaRequest) (*pb.ImportSchemaResponse, error)
}

func (m *mockSchema) CreateSchema(ctx context.Context, in *pb.CreateSchemaRequest, opts ...grpc.CallOption) (*pb.CreateSchemaResponse, error) {
	return m.createSchemaFn(ctx, in)
}

func (m *mockSchema) GetSchema(ctx context.Context, in *pb.GetSchemaRequest, opts ...grpc.CallOption) (*pb.GetSchemaResponse, error) {
	return m.getSchemaFn(ctx, in)
}

func (m *mockSchema) ListSchemas(ctx context.Context, in *pb.ListSchemasRequest, opts ...grpc.CallOption) (*pb.ListSchemasResponse, error) {
	return m.listSchemasFn(ctx, in)
}

func (m *mockSchema) UpdateSchema(ctx context.Context, in *pb.UpdateSchemaRequest, opts ...grpc.CallOption) (*pb.UpdateSchemaResponse, error) {
	return m.updateSchemaFn(ctx, in)
}

func (m *mockSchema) DeleteSchema(ctx context.Context, in *pb.DeleteSchemaRequest, opts ...grpc.CallOption) (*pb.DeleteSchemaResponse, error) {
	return m.deleteSchemaFn(ctx, in)
}

func (m *mockSchema) PublishSchema(ctx context.Context, in *pb.PublishSchemaRequest, opts ...grpc.CallOption) (*pb.PublishSchemaResponse, error) {
	return m.publishSchemaFn(ctx, in)
}

func (m *mockSchema) CreateTenant(ctx context.Context, in *pb.CreateTenantRequest, opts ...grpc.CallOption) (*pb.CreateTenantResponse, error) {
	return m.createTenantFn(ctx, in)
}

func (m *mockSchema) GetTenant(ctx context.Context, in *pb.GetTenantRequest, opts ...grpc.CallOption) (*pb.GetTenantResponse, error) {
	return m.getTenantFn(ctx, in)
}

func (m *mockSchema) ListTenants(ctx context.Context, in *pb.ListTenantsRequest, opts ...grpc.CallOption) (*pb.ListTenantsResponse, error) {
	return m.listTenantsFn(ctx, in)
}

func (m *mockSchema) UpdateTenant(ctx context.Context, in *pb.UpdateTenantRequest, opts ...grpc.CallOption) (*pb.UpdateTenantResponse, error) {
	return m.updateTenantFn(ctx, in)
}

func (m *mockSchema) DeleteTenant(ctx context.Context, in *pb.DeleteTenantRequest, opts ...grpc.CallOption) (*pb.DeleteTenantResponse, error) {
	return m.deleteTenantFn(ctx, in)
}

func (m *mockSchema) LockField(ctx context.Context, in *pb.LockFieldRequest, opts ...grpc.CallOption) (*pb.LockFieldResponse, error) {
	return m.lockFieldFn(ctx, in)
}

func (m *mockSchema) UnlockField(ctx context.Context, in *pb.UnlockFieldRequest, opts ...grpc.CallOption) (*pb.UnlockFieldResponse, error) {
	return m.unlockFieldFn(ctx, in)
}

func (m *mockSchema) ListFieldLocks(ctx context.Context, in *pb.ListFieldLocksRequest, opts ...grpc.CallOption) (*pb.ListFieldLocksResponse, error) {
	return m.listFieldLocksFn(ctx, in)
}

func (m *mockSchema) ExportSchema(ctx context.Context, in *pb.ExportSchemaRequest, opts ...grpc.CallOption) (*pb.ExportSchemaResponse, error) {
	return m.exportSchemaFn(ctx, in)
}

func (m *mockSchema) ImportSchema(ctx context.Context, in *pb.ImportSchemaRequest, opts ...grpc.CallOption) (*pb.ImportSchemaResponse, error) {
	return m.importSchemaFn(ctx, in)
}

// --- Mock ConfigService ---

type mockConfig struct {
	pb.ConfigServiceClient

	getConfigFn    func(ctx context.Context, in *pb.GetConfigRequest) (*pb.GetConfigResponse, error)
	getFieldFn     func(ctx context.Context, in *pb.GetFieldRequest) (*pb.GetFieldResponse, error)
	getFieldsFn    func(ctx context.Context, in *pb.GetFieldsRequest) (*pb.GetFieldsResponse, error)
	setFieldFn     func(ctx context.Context, in *pb.SetFieldRequest) (*pb.SetFieldResponse, error)
	setFieldsFn    func(ctx context.Context, in *pb.SetFieldsRequest) (*pb.SetFieldsResponse, error)
	listVersionsFn func(ctx context.Context, in *pb.ListVersionsRequest) (*pb.ListVersionsResponse, error)
	getVersionFn   func(ctx context.Context, in *pb.GetVersionRequest) (*pb.GetVersionResponse, error)
	rollbackFn     func(ctx context.Context, in *pb.RollbackToVersionRequest) (*pb.RollbackToVersionResponse, error)
	exportConfigFn func(ctx context.Context, in *pb.ExportConfigRequest) (*pb.ExportConfigResponse, error)
	importConfigFn func(ctx context.Context, in *pb.ImportConfigRequest) (*pb.ImportConfigResponse, error)
}

func (m *mockConfig) GetConfig(ctx context.Context, in *pb.GetConfigRequest, opts ...grpc.CallOption) (*pb.GetConfigResponse, error) {
	return m.getConfigFn(ctx, in)
}

func (m *mockConfig) GetField(ctx context.Context, in *pb.GetFieldRequest, opts ...grpc.CallOption) (*pb.GetFieldResponse, error) {
	return m.getFieldFn(ctx, in)
}

func (m *mockConfig) GetFields(ctx context.Context, in *pb.GetFieldsRequest, opts ...grpc.CallOption) (*pb.GetFieldsResponse, error) {
	return m.getFieldsFn(ctx, in)
}

func (m *mockConfig) SetField(ctx context.Context, in *pb.SetFieldRequest, opts ...grpc.CallOption) (*pb.SetFieldResponse, error) {
	return m.setFieldFn(ctx, in)
}

func (m *mockConfig) SetFields(ctx context.Context, in *pb.SetFieldsRequest, opts ...grpc.CallOption) (*pb.SetFieldsResponse, error) {
	return m.setFieldsFn(ctx, in)
}

func (m *mockConfig) ListVersions(ctx context.Context, in *pb.ListVersionsRequest, opts ...grpc.CallOption) (*pb.ListVersionsResponse, error) {
	return m.listVersionsFn(ctx, in)
}

func (m *mockConfig) GetVersion(ctx context.Context, in *pb.GetVersionRequest, opts ...grpc.CallOption) (*pb.GetVersionResponse, error) {
	return m.getVersionFn(ctx, in)
}

func (m *mockConfig) RollbackToVersion(ctx context.Context, in *pb.RollbackToVersionRequest, opts ...grpc.CallOption) (*pb.RollbackToVersionResponse, error) {
	return m.rollbackFn(ctx, in)
}

func (m *mockConfig) Subscribe(ctx context.Context, in *pb.SubscribeRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.SubscribeResponse], error) {
	return nil, nil
}

func (m *mockConfig) ExportConfig(ctx context.Context, in *pb.ExportConfigRequest, opts ...grpc.CallOption) (*pb.ExportConfigResponse, error) {
	return m.exportConfigFn(ctx, in)
}

func (m *mockConfig) ImportConfig(ctx context.Context, in *pb.ImportConfigRequest, opts ...grpc.CallOption) (*pb.ImportConfigResponse, error) {
	return m.importConfigFn(ctx, in)
}

// --- Mock AuditService ---

type mockAudit struct {
	pb.AuditServiceClient

	queryWriteLogFn   func(ctx context.Context, in *pb.QueryWriteLogRequest) (*pb.QueryWriteLogResponse, error)
	getFieldUsageFn   func(ctx context.Context, in *pb.GetFieldUsageRequest) (*pb.GetFieldUsageResponse, error)
	getTenantUsageFn  func(ctx context.Context, in *pb.GetTenantUsageRequest) (*pb.GetTenantUsageResponse, error)
	getUnusedFieldsFn func(ctx context.Context, in *pb.GetUnusedFieldsRequest) (*pb.GetUnusedFieldsResponse, error)
}

func (m *mockAudit) QueryWriteLog(ctx context.Context, in *pb.QueryWriteLogRequest, opts ...grpc.CallOption) (*pb.QueryWriteLogResponse, error) {
	return m.queryWriteLogFn(ctx, in)
}

func (m *mockAudit) GetFieldUsage(ctx context.Context, in *pb.GetFieldUsageRequest, opts ...grpc.CallOption) (*pb.GetFieldUsageResponse, error) {
	return m.getFieldUsageFn(ctx, in)
}

func (m *mockAudit) GetTenantUsage(ctx context.Context, in *pb.GetTenantUsageRequest, opts ...grpc.CallOption) (*pb.GetTenantUsageResponse, error) {
	return m.getTenantUsageFn(ctx, in)
}

func (m *mockAudit) GetUnusedFields(ctx context.Context, in *pb.GetUnusedFieldsRequest, opts ...grpc.CallOption) (*pb.GetUnusedFieldsResponse, error) {
	return m.getUnusedFieldsFn(ctx, in)
}

// --- Tests ---

func TestCreateSchema_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil, WithSubject("admin"))
	ctx := context.Background()

	ms.createSchemaFn = func(_ context.Context, _ *pb.CreateSchemaRequest) (*pb.CreateSchemaResponse, error) {
		return &pb.CreateSchemaResponse{
			Schema: &pb.Schema{Id: "s1", Name: "payments", Version: 1, CreatedAt: timestamppb.Now()},
		}, nil
	}

	s, err := client.CreateSchema(ctx, "payments", []Field{{Path: "a", Type: "STRING"}}, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := s.Name; got != "payments" {
		t.Errorf("got Name %v, want %v", got, "payments")
	}
	if got := s.Version; got != int32(1) {
		t.Errorf("got Version %v, want %v", got, int32(1))
	}
}

func TestGetSchema_NotFound(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)
	ctx := context.Background()

	ms.getSchemaFn = func(_ context.Context, _ *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
		return nil, status.Error(codes.NotFound, "not found")
	}

	_, err := client.GetSchema(ctx, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want %v", err, ErrNotFound)
	}
}

func TestCreateTenant_Success(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)
	ctx := context.Background()

	ms.createTenantFn = func(_ context.Context, _ *pb.CreateTenantRequest) (*pb.CreateTenantResponse, error) {
		return &pb.CreateTenantResponse{
			Tenant: &pb.Tenant{Id: "t1", Name: "acme", SchemaId: "s1", SchemaVersion: 1, CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now()},
		}, nil
	}

	tenant, err := client.CreateTenant(ctx, "acme", "s1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tenant.Name; got != "acme" {
		t.Errorf("got Name %v, want %v", got, "acme")
	}
}

func TestCreateTenant_UnpublishedSchema(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)
	ctx := context.Background()

	ms.createTenantFn = func(_ context.Context, _ *pb.CreateTenantRequest) (*pb.CreateTenantResponse, error) {
		return nil, status.Error(codes.FailedPrecondition, "not published")
	}

	_, err := client.CreateTenant(ctx, "acme", "s1", 1)
	if !errors.Is(err, ErrFailedPrecondition) {
		t.Errorf("got error %v, want %v", err, ErrFailedPrecondition)
	}
}

func TestLockUnlockField(t *testing.T) {
	ms := &mockSchema{}
	client := New(ms, nil, nil)
	ctx := context.Background()

	ms.lockFieldFn = func(_ context.Context, _ *pb.LockFieldRequest) (*pb.LockFieldResponse, error) {
		return &pb.LockFieldResponse{}, nil
	}
	ms.unlockFieldFn = func(_ context.Context, _ *pb.UnlockFieldRequest) (*pb.UnlockFieldResponse, error) {
		return &pb.UnlockFieldResponse{}, nil
	}

	if err := client.LockField(ctx, "t1", "fee"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := client.UnlockField(ctx, "t1", "fee"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListConfigVersions_AutoPaginate(t *testing.T) {
	mc := &mockConfig{}
	client := New(nil, mc, nil)
	ctx := context.Background()

	callCount := 0
	mc.listVersionsFn = func(_ context.Context, r *pb.ListVersionsRequest) (*pb.ListVersionsResponse, error) {
		callCount++
		if r.PageToken == "" {
			return &pb.ListVersionsResponse{
				Versions:      []*pb.ConfigVersion{{Version: 3, CreatedAt: timestamppb.Now()}, {Version: 2, CreatedAt: timestamppb.Now()}},
				NextPageToken: "page2",
			}, nil
		}
		return &pb.ListVersionsResponse{
			Versions: []*pb.ConfigVersion{{Version: 1, CreatedAt: timestamppb.Now()}},
		}, nil
	}

	versions, err := client.ListConfigVersions(ctx, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("got len %d, want %d", len(versions), 3)
	}
}

func TestRollbackConfig_Success(t *testing.T) {
	mc := &mockConfig{}
	client := New(nil, mc, nil)
	ctx := context.Background()

	mc.rollbackFn = func(_ context.Context, _ *pb.RollbackToVersionRequest) (*pb.RollbackToVersionResponse, error) {
		return &pb.RollbackToVersionResponse{
			ConfigVersion: &pb.ConfigVersion{Version: 5, CreatedAt: timestamppb.Now()},
		}, nil
	}

	v, err := client.RollbackConfig(ctx, "t1", 2, "rollback to v2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := v.Version; got != int32(5) {
		t.Errorf("got Version %v, want %v", got, int32(5))
	}
}

func TestExportImportConfig(t *testing.T) {
	mc := &mockConfig{}
	client := New(nil, mc, nil)
	ctx := context.Background()

	mc.exportConfigFn = func(_ context.Context, _ *pb.ExportConfigRequest) (*pb.ExportConfigResponse, error) {
		return &pb.ExportConfigResponse{
			YamlContent: []byte("syntax: v1\nvalues:\n  a:\n    value: x\n"),
		}, nil
	}

	data, err := client.ExportConfig(ctx, "t1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "syntax") {
		t.Errorf("expected %q to contain %q", string(data), "syntax")
	}

	mc.importConfigFn = func(_ context.Context, _ *pb.ImportConfigRequest) (*pb.ImportConfigResponse, error) {
		return &pb.ImportConfigResponse{
			ConfigVersion: &pb.ConfigVersion{Version: 3, CreatedAt: timestamppb.Now()},
		}, nil
	}

	v, err := client.ImportConfig(ctx, "t1", data, "imported")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := v.Version; got != int32(3) {
		t.Errorf("got Version %v, want %v", got, int32(3))
	}
}

func TestQueryWriteLog_Success(t *testing.T) {
	ma := &mockAudit{}
	client := New(nil, nil, ma)
	ctx := context.Background()

	ma.queryWriteLogFn = func(_ context.Context, _ *pb.QueryWriteLogRequest) (*pb.QueryWriteLogResponse, error) {
		return &pb.QueryWriteLogResponse{
			Entries: []*pb.AuditEntry{
				{Id: "e1", TenantId: "t1", Actor: "admin", Action: "set_field", CreatedAt: timestamppb.Now()},
			},
		}, nil
	}

	entries, err := client.QueryWriteLog(ctx, WithAuditTenant("t1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("got len %d, want %d", len(entries), 1)
	}
	if got := entries[0].Action; got != "set_field" {
		t.Errorf("got Action %v, want %v", got, "set_field")
	}
}

func TestFieldFromProto_AllMetadata(t *testing.T) {
	title := "Fee Rate"
	example := "0.025"
	format := "percentage"

	pf := &pb.SchemaField{
		Path:         "payments.fee",
		Type:         pb.FieldType_FIELD_TYPE_NUMBER,
		Nullable:     true,
		Deprecated:   true,
		RedirectTo:   ptrString("payments.new_fee"),
		DefaultValue: ptrString("0.01"),
		Description:  ptrString("Transaction fee"),
		Title:        &title,
		Example:      &example,
		Format:       &format,
		Tags:         []string{"billing", "critical"},
		ReadOnly:     true,
		WriteOnce:    true,
		Sensitive:    true,
		Examples: map[string]*pb.FieldExample{
			"low":  {Value: "0.01", Summary: "Low rate"},
			"high": {Value: "0.99", Summary: "High rate"},
		},
		ExternalDocs: &pb.ExternalDocs{
			Description: "Fee guide",
			Url:         "https://docs.example.com/fees",
		},
	}

	f := fieldFromProto(pf)
	if got := f.Path; got != "payments.fee" {
		t.Errorf("got Path %v, want %v", got, "payments.fee")
	}
	if got := f.Title; got != "Fee Rate" {
		t.Errorf("got Title %v, want %v", got, "Fee Rate")
	}
	if got := f.Example; got != "0.025" {
		t.Errorf("got Example %v, want %v", got, "0.025")
	}
	if got := f.Format; got != "percentage" {
		t.Errorf("got Format %v, want %v", got, "percentage")
	}
	if !reflect.DeepEqual(f.Tags, []string{"billing", "critical"}) {
		t.Errorf("got Tags %v, want %v", f.Tags, []string{"billing", "critical"})
	}
	if !f.ReadOnly {
		t.Error("expected ReadOnly to be true")
	}
	if !f.WriteOnce {
		t.Error("expected WriteOnce to be true")
	}
	if !f.Sensitive {
		t.Error("expected Sensitive to be true")
	}
	if !f.Nullable {
		t.Error("expected Nullable to be true")
	}
	if !f.Deprecated {
		t.Error("expected Deprecated to be true")
	}
	if got := f.RedirectTo; got != "payments.new_fee" {
		t.Errorf("got RedirectTo %v, want %v", got, "payments.new_fee")
	}
	if got := f.Default; got != "0.01" {
		t.Errorf("got Default %v, want %v", got, "0.01")
	}
	if got := f.Description; got != "Transaction fee" {
		t.Errorf("got Description %v, want %v", got, "Transaction fee")
	}

	if len(f.Examples) != 2 {
		t.Fatalf("got len %d, want %d", len(f.Examples), 2)
	}
	if got := f.Examples["low"].Value; got != "0.01" {
		t.Errorf("got Examples[low].Value %v, want %v", got, "0.01")
	}
	if got := f.Examples["low"].Summary; got != "Low rate" {
		t.Errorf("got Examples[low].Summary %v, want %v", got, "Low rate")
	}

	if f.ExternalDocs == nil {
		t.Fatal("expected non-nil ExternalDocs")
	}
	if got := f.ExternalDocs.Description; got != "Fee guide" {
		t.Errorf("got ExternalDocs.Description %v, want %v", got, "Fee guide")
	}
	if got := f.ExternalDocs.URL; got != "https://docs.example.com/fees" {
		t.Errorf("got ExternalDocs.URL %v, want %v", got, "https://docs.example.com/fees")
	}
}

func TestSchemaInfoFromProto(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if got := schemaInfoFromProto(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("full info", func(t *testing.T) {
		info := schemaInfoFromProto(&pb.SchemaInfo{
			Title:  "Payment Config",
			Author: "payments-team",
			Contact: &pb.SchemaContact{
				Name:  "Payments Team",
				Email: "pay@example.com",
				Url:   "https://wiki.example.com",
			},
			Labels: map[string]string{"team": "payments"},
		})
		if info == nil {
			t.Fatal("expected non-nil info")
		}
		if got := info.Title; got != "Payment Config" {
			t.Errorf("got Title %v, want %v", got, "Payment Config")
		}
		if got := info.Author; got != "payments-team" {
			t.Errorf("got Author %v, want %v", got, "payments-team")
		}
		if got := info.Contact.Email; got != "pay@example.com" {
			t.Errorf("got Contact.Email %v, want %v", got, "pay@example.com")
		}
		if got := info.Contact.URL; got != "https://wiki.example.com" {
			t.Errorf("got Contact.URL %v, want %v", got, "https://wiki.example.com")
		}
		if got := info.Labels["team"]; got != "payments" {
			t.Errorf("got Labels[team] %v, want %v", got, "payments")
		}
	})

	t.Run("without contact", func(t *testing.T) {
		info := schemaInfoFromProto(&pb.SchemaInfo{Author: "me"})
		if info == nil {
			t.Fatal("expected non-nil info")
		}
		if info.Contact != nil {
			t.Errorf("expected nil Contact, got %v", info.Contact)
		}
	})
}

func TestFieldsToProto_AllMetadata(t *testing.T) {
	fields := []Field{{
		Path:         "x",
		Type:         "STRING",
		Title:        "The X",
		Example:      "hello",
		Format:       "email",
		Tags:         []string{"core"},
		ReadOnly:     true,
		WriteOnce:    true,
		Sensitive:    true,
		Examples:     map[string]FieldExample{"ex1": {Value: "v1", Summary: "s1"}},
		ExternalDocs: &ExternalDocs{Description: "docs", URL: "https://x.com"},
	}}

	result := fieldsToProto(fields)
	if len(result) != 1 {
		t.Fatalf("got len %d, want %d", len(result), 1)
	}
	pf := result[0]
	if got := pf.GetTitle(); got != "The X" {
		t.Errorf("got Title %v, want %v", got, "The X")
	}
	if got := pf.GetExample(); got != "hello" {
		t.Errorf("got Example %v, want %v", got, "hello")
	}
	if got := pf.GetFormat(); got != "email" {
		t.Errorf("got Format %v, want %v", got, "email")
	}
	if !reflect.DeepEqual(pf.Tags, []string{"core"}) {
		t.Errorf("got Tags %v, want %v", pf.Tags, []string{"core"})
	}
	if !pf.ReadOnly {
		t.Error("expected ReadOnly to be true")
	}
	if !pf.WriteOnce {
		t.Error("expected WriteOnce to be true")
	}
	if !pf.Sensitive {
		t.Error("expected Sensitive to be true")
	}
	if len(pf.Examples) != 1 {
		t.Errorf("got len %d, want %d", len(pf.Examples), 1)
	}
	if got := pf.Examples["ex1"].Value; got != "v1" {
		t.Errorf("got Examples[ex1].Value %v, want %v", got, "v1")
	}
	if pf.ExternalDocs == nil {
		t.Fatal("expected non-nil ExternalDocs")
	}
	if got := pf.ExternalDocs.Url; got != "https://x.com" {
		t.Errorf("got ExternalDocs.Url %v, want %v", got, "https://x.com")
	}
}

func TestServiceNotConfigured(t *testing.T) {
	client := New(nil, nil, nil)
	ctx := context.Background()

	_, err := client.GetSchema(ctx, "s1")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ListConfigVersions(ctx, "t1")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.QueryWriteLog(ctx)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("got error %v, want %v", err, ErrServiceNotConfigured)
	}
}
