package adminclient

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// --- Schema operations ---

func TestGetSchemaVersion_Success(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.getSchemaFn = func(_ context.Context, id string, version *int32) (*Schema, error) {
		if id != "s1" || version == nil || *version != int32(2) {
			t.Fatalf("unexpected args: id=%v version=%v", id, version)
		}
		return &Schema{ID: "s1", Name: "test", Version: 2, CreatedAt: time.Now()}, nil
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
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.listSchemasFn = func(_ context.Context, _ int32, pageToken string) (*ListSchemasResponse, error) {
		if pageToken == "" {
			return &ListSchemasResponse{
				Schemas:       []*Schema{{ID: "s1", Name: "a", Version: 1, CreatedAt: time.Now()}},
				NextPageToken: "page2",
			}, nil
		}
		return &ListSchemasResponse{
			Schemas: []*Schema{{ID: "s2", Name: "b", Version: 1, CreatedAt: time.Now()}},
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
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.updateSchemaFn = func(_ context.Context, req *UpdateSchemaRequest) (*Schema, error) {
		if req.ID != "s1" {
			t.Errorf("got ID %v, want %v", req.ID, "s1")
		}
		if len(req.AddOrModify) != 1 || req.AddOrModify[0].Path != "new" {
			t.Errorf("unexpected AddOrModify: %v", req.AddOrModify)
		}
		if !reflect.DeepEqual(req.RemoveFields, []string{"old"}) {
			t.Errorf("got RemoveFields %v, want %v", req.RemoveFields, []string{"old"})
		}
		return &Schema{ID: "s1", Version: 2, CreatedAt: time.Now()}, nil
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
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.deleteSchemaFn = func(_ context.Context, id string) error {
		if id != "s1" {
			t.Errorf("got id %v, want %v", id, "s1")
		}
		return nil
	}
	if err := client.DeleteSchema(context.Background(), "s1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExportSchema_Success(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.exportSchemaFn = func(_ context.Context, _ string, _ *int32) ([]byte, error) {
		return []byte("spec_version: v1"), nil
	}

	data, err := client.ExportSchema(context.Background(), "s1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "spec_version") {
		t.Errorf("expected %q to contain %q", string(data), "spec_version")
	}
}

func TestExportSchema_SpecificVersion(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.exportSchemaFn = func(_ context.Context, id string, version *int32) ([]byte, error) {
		if id != "s1" || version == nil || *version != int32(2) {
			t.Fatalf("unexpected args: id=%v version=%v", id, version)
		}
		return []byte("spec_version: v1"), nil
	}

	v := int32(2)
	_, err := client.ExportSchema(context.Background(), "s1", &v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportSchema_Success(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.importSchemaFn = func(_ context.Context, yamlContent []byte, autoPublish bool) (*Schema, error) {
		if autoPublish {
			t.Fatal("expected autoPublish to be false")
		}
		return &Schema{ID: "s1", Name: "imported", Version: 1, CreatedAt: time.Now()}, nil
	}

	s, err := client.ImportSchema(context.Background(), []byte("spec_version: v1\nname: imported"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := s.Name; got != "imported" {
		t.Errorf("got Name %v, want %v", got, "imported")
	}
}

func TestImportSchema_AutoPublish(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.importSchemaFn = func(_ context.Context, _ []byte, autoPublish bool) (*Schema, error) {
		if !autoPublish {
			t.Fatal("expected autoPublish to be true")
		}
		return &Schema{ID: "s1", Version: 1, Published: true, CreatedAt: time.Now()}, nil
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
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.publishSchemaFn = func(_ context.Context, _ string, _ int32) (*Schema, error) {
		return nil, ErrFailedPrecondition
	}

	_, err := client.PublishSchema(context.Background(), "s1", 1)
	if !errors.Is(err, ErrFailedPrecondition) {
		t.Errorf("got error %v, want %v", err, ErrFailedPrecondition)
	}
}

func TestPublishSchema_Success(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.publishSchemaFn = func(_ context.Context, id string, version int32) (*Schema, error) {
		if id != "s1" || version != int32(1) {
			t.Fatalf("unexpected args: id=%v version=%v", id, version)
		}
		return &Schema{ID: "s1", Version: 1, Published: true, CreatedAt: time.Now()}, nil
	}

	s, err := client.PublishSchema(context.Background(), "s1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.Published {
		t.Error("expected Published to be true")
	}
}

// --- Tenant operations ---

func TestGetTenant_Success(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.getTenantFn = func(_ context.Context, id string) (*Tenant, error) {
		if id != "t1" {
			t.Errorf("got id %v, want %v", id, "t1")
		}
		return &Tenant{ID: "t1", Name: "acme", SchemaID: "s1", SchemaVersion: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
	}

	tenant, err := client.GetTenant(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := tenant.Name; got != "acme" {
		t.Errorf("got Name %v, want %v", got, "acme")
	}
}

func TestGetTenant_NotFound(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.getTenantFn = func(_ context.Context, _ string) (*Tenant, error) {
		return nil, ErrNotFound
	}

	_, err := client.GetTenant(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want %v", err, ErrNotFound)
	}
}

func TestListTenants_WithSchemaFilter(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.listTenantsFn = func(_ context.Context, schemaID *string, _ int32, _ string) (*ListTenantsResponse, error) {
		if schemaID == nil || *schemaID != "s1" {
			t.Fatalf("expected SchemaId filter s1, got %v", schemaID)
		}
		return &ListTenantsResponse{
			Tenants: []*Tenant{
				{ID: "t1", Name: "acme", SchemaID: "s1", SchemaVersion: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
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
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.listTenantsFn = func(_ context.Context, schemaID *string, _ int32, _ string) (*ListTenantsResponse, error) {
		if schemaID != nil {
			t.Fatalf("expected nil SchemaId, got %v", *schemaID)
		}
		return &ListTenantsResponse{}, nil
	}

	_, err := client.ListTenants(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTenants_AutoPaginate(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.listTenantsFn = func(_ context.Context, _ *string, _ int32, pageToken string) (*ListTenantsResponse, error) {
		if pageToken == "" {
			return &ListTenantsResponse{
				Tenants:       []*Tenant{{ID: "t1", Name: "a"}},
				NextPageToken: "page2",
			}, nil
		}
		return &ListTenantsResponse{
			Tenants: []*Tenant{{ID: "t2", Name: "b"}},
		}, nil
	}

	tenants, err := client.ListTenants(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tenants) != 2 {
		t.Errorf("got len %d, want %d", len(tenants), 2)
	}
}

func TestUpdateTenantName(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.updateTenantFn = func(_ context.Context, req *UpdateTenantRequest) (*Tenant, error) {
		if req.Name == nil || *req.Name != "new-name" {
			t.Fatalf("expected Name new-name, got %v", req.Name)
		}
		if req.SchemaVersion != nil {
			t.Fatalf("expected nil SchemaVersion, got %v", *req.SchemaVersion)
		}
		return &Tenant{ID: "t1", Name: "new-name", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
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
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.updateTenantFn = func(_ context.Context, req *UpdateTenantRequest) (*Tenant, error) {
		if req.SchemaVersion == nil || *req.SchemaVersion != int32(2) {
			t.Fatalf("expected SchemaVersion 2, got %v", req.SchemaVersion)
		}
		if req.Name != nil {
			t.Fatalf("expected nil Name, got %v", *req.Name)
		}
		return &Tenant{ID: "t1", SchemaVersion: 2, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
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
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.deleteTenantFn = func(_ context.Context, id string) error {
		if id != "t1" {
			t.Errorf("got id %v, want %v", id, "t1")
		}
		return nil
	}
	if err := client.DeleteTenant(context.Background(), "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Lock operations ---

func TestListFieldLocks_Success(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.listFieldLocksFn = func(_ context.Context, tenantID string) ([]FieldLock, error) {
		if tenantID != "t1" {
			t.Errorf("got tenantID %v, want %v", tenantID, "t1")
		}
		return []FieldLock{
			{TenantID: "t1", FieldPath: "app.fee", LockedValues: []string{"0.01"}},
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
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.lockFieldFn = func(_ context.Context, tenantID, fieldPath string, lockedValues []string) error {
		if tenantID != "t1" || fieldPath != "app.env" {
			t.Fatalf("unexpected args: tenantID=%v fieldPath=%v", tenantID, fieldPath)
		}
		if len(lockedValues) != 2 {
			t.Fatalf("expected 2 locked values, got %d", len(lockedValues))
		}
		return nil
	}

	if err := client.LockField(context.Background(), "t1", "app.env", "prod", "staging"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLockField_NoValues(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(ms, nil, nil, nil)

	ms.lockFieldFn = func(_ context.Context, _, _ string, lockedValues []string) error {
		if len(lockedValues) != 0 {
			t.Fatalf("expected 0 locked values, got %d", len(lockedValues))
		}
		return nil
	}

	if err := client.LockField(context.Background(), "t1", "app.fee"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Audit operations ---

func TestGetFieldUsage_Success(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(nil, nil, ma, nil)

	ma.getFieldUsageFn = func(_ context.Context, tenantID, fieldPath string, start, end *time.Time) (*UsageStats, error) {
		if tenantID != "t1" || fieldPath != "app.fee" {
			t.Fatalf("unexpected args: tenantID=%v fieldPath=%v", tenantID, fieldPath)
		}
		return &UsageStats{TenantID: "t1", FieldPath: "app.fee", ReadCount: 42, LastReadBy: "reader"}, nil
	}

	stats, err := client.GetFieldUsage(context.Background(), "t1", "app.fee", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := stats.ReadCount; got != int64(42) {
		t.Errorf("got ReadCount %v, want %v", got, int64(42))
	}
	if got := stats.LastReadBy; got != "reader" {
		t.Errorf("got LastReadBy %v, want %v", got, "reader")
	}
}

func TestGetFieldUsage_WithTimeRange(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(nil, nil, ma, nil)

	start := time.Now().Add(-time.Hour)
	end := time.Now()

	ma.getFieldUsageFn = func(_ context.Context, _, _ string, s, e *time.Time) (*UsageStats, error) {
		if s == nil || e == nil {
			t.Fatal("expected non-nil start and end times")
		}
		return &UsageStats{ReadCount: 10}, nil
	}

	_, err := client.GetFieldUsage(context.Background(), "t1", "app.fee", &start, &end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTenantUsage_Success(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(nil, nil, ma, nil)

	ma.getTenantUsageFn = func(_ context.Context, tenantID string, _, _ *time.Time) ([]*UsageStats, error) {
		if tenantID != "t1" {
			t.Errorf("got tenantID %v, want %v", tenantID, "t1")
		}
		return []*UsageStats{
			{FieldPath: "a", ReadCount: 10},
			{FieldPath: "b", ReadCount: 5},
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
	ma := &mockAuditTransport{}
	client := New(nil, nil, ma, nil)

	ma.getUnusedFieldsFn = func(_ context.Context, tenantID string, _ time.Time) ([]string, error) {
		if tenantID != "t1" {
			t.Errorf("got tenantID %v, want %v", tenantID, "t1")
		}
		return []string{"old.field"}, nil
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
	req := &QueryWriteLogRequest{}

	WithAuditTenant("t1")(req)
	if req.TenantID == nil || *req.TenantID != "t1" {
		t.Errorf("got TenantId %v, want %v", req.TenantID, "t1")
	}

	WithAuditActor("admin")(req)
	if req.Actor == nil || *req.Actor != "admin" {
		t.Errorf("got Actor %v, want %v", req.Actor, "admin")
	}

	WithAuditField("app.fee")(req)
	if req.FieldPath == nil || *req.FieldPath != "app.fee" {
		t.Errorf("got FieldPath %v, want %v", req.FieldPath, "app.fee")
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

func TestQueryWriteLog_AutoPaginate(t *testing.T) {
	ma := &mockAuditTransport{}
	client := New(nil, nil, ma, nil)

	ma.queryWriteLogFn = func(_ context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		if req.PageToken == "" {
			return &QueryWriteLogResponse{
				Entries:       []*AuditEntry{{ID: "e1", Action: "set_field"}, {ID: "e2", Action: "set_field"}},
				NextPageToken: "page2",
			}, nil
		}
		return &QueryWriteLogResponse{
			Entries: []*AuditEntry{{ID: "e3", Action: "set_field"}},
		}, nil
	}

	entries, err := client.QueryWriteLog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("got len %d, want %d", len(entries), 3)
	}
}

// --- Config version operations ---

func TestGetConfigVersion_Success(t *testing.T) {
	mc := &mockConfigTransport{}
	client := New(nil, mc, nil, nil)

	mc.getVersionFn = func(_ context.Context, tenantID string, version int32) (*Version, error) {
		if tenantID != "t1" || version != int32(3) {
			t.Fatalf("unexpected args: tenantID=%v version=%v", tenantID, version)
		}
		return &Version{ID: "v1", TenantID: "t1", Version: 3, Description: "test", CreatedBy: "admin", CreatedAt: time.Now()}, nil
	}

	v, err := client.GetConfigVersion(context.Background(), "t1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := v.Version; got != int32(3) {
		t.Errorf("got Version %v, want %v", got, int32(3))
	}
	if got := v.CreatedBy; got != "admin" {
		t.Errorf("got CreatedBy %v, want %v", got, "admin")
	}
}

func TestImportConfig_WithMode(t *testing.T) {
	mc := &mockConfigTransport{}
	client := New(nil, mc, nil, nil)

	mc.importConfigFn = func(_ context.Context, req *ImportConfigRequest) (*Version, error) {
		if req.TenantID != "t1" {
			t.Errorf("got TenantID %v, want %v", req.TenantID, "t1")
		}
		if req.Mode != ImportModeReplace {
			t.Errorf("got Mode %v, want %v", req.Mode, ImportModeReplace)
		}
		if req.Description != "full replace" {
			t.Errorf("got Description %v, want %v", req.Description, "full replace")
		}
		return &Version{Version: 4, CreatedAt: time.Now()}, nil
	}

	v, err := client.ImportConfig(context.Background(), "t1", []byte("yaml"), "full replace", ImportModeReplace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := v.Version; got != int32(4) {
		t.Errorf("got Version %v, want %v", got, int32(4))
	}
}

func TestImportConfig_DefaultMode(t *testing.T) {
	mc := &mockConfigTransport{}
	client := New(nil, mc, nil, nil)

	mc.importConfigFn = func(_ context.Context, req *ImportConfigRequest) (*Version, error) {
		if req.Mode != ImportModeMerge {
			t.Errorf("got Mode %v, want ImportModeMerge (%v)", req.Mode, ImportModeMerge)
		}
		return &Version{Version: 2, CreatedAt: time.Now()}, nil
	}

	_, err := client.ImportConfig(context.Background(), "t1", []byte("yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Error propagation ---

func TestTransportError_Propagated(t *testing.T) {
	sentinel := errors.New("transport failure")

	ms := &mockSchemaTransport{}
	ms.getSchemaFn = func(_ context.Context, _ string, _ *int32) (*Schema, error) {
		return nil, sentinel
	}
	client := New(ms, nil, nil, nil)

	_, err := client.GetSchema(context.Background(), "s1")
	if !errors.Is(err, sentinel) {
		t.Errorf("got error %v, want %v", err, sentinel)
	}
}

// --- Service not configured (all methods) ---

func TestServiceNotConfigured_AllMethods(t *testing.T) {
	client := New(nil, nil, nil, nil)
	ctx := context.Background()

	_, err := client.GetSchemaVersion(ctx, "s1", 1)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("GetSchemaVersion: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ListSchemas(ctx)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("ListSchemas: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.UpdateSchema(ctx, "s1", nil, nil, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("UpdateSchema: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	err = client.DeleteSchema(ctx, "s1")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("DeleteSchema: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.PublishSchema(ctx, "s1", 1)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("PublishSchema: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ExportSchema(ctx, "s1", nil)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("ExportSchema: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ImportSchema(ctx, nil)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("ImportSchema: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.GetTenant(ctx, "t1")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("GetTenant: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ListTenants(ctx, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("ListTenants: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.UpdateTenantName(ctx, "t1", "new")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("UpdateTenantName: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.UpdateTenantSchema(ctx, "t1", 2)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("UpdateTenantSchema: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	err = client.DeleteTenant(ctx, "t1")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("DeleteTenant: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	err = client.LockField(ctx, "t1", "x")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("LockField: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	err = client.UnlockField(ctx, "t1", "x")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("UnlockField: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ListFieldLocks(ctx, "t1")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("ListFieldLocks: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.GetFieldUsage(ctx, "t1", "x", nil, nil)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("GetFieldUsage: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.GetTenantUsage(ctx, "t1", nil, nil)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("GetTenantUsage: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.GetUnusedFields(ctx, "t1", time.Now())
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("GetUnusedFields: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.GetConfigVersion(ctx, "t1", 1)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("GetConfigVersion: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.RollbackConfig(ctx, "t1", 1, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("RollbackConfig: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ExportConfig(ctx, "t1", nil)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("ExportConfig: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ImportConfig(ctx, "t1", nil, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("ImportConfig: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.ListConfigVersions(ctx, "t1")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("ListConfigVersions: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.CreateSchema(ctx, "x", nil, "")
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("CreateSchema: got error %v, want %v", err, ErrServiceNotConfigured)
	}

	_, err = client.CreateTenant(ctx, "x", "s1", 1)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("CreateTenant: got error %v, want %v", err, ErrServiceNotConfigured)
	}
}
