package adminclient

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// --- Mock SchemaTransport ---

type mockSchemaTransport struct {
	createSchemaFn   func(ctx context.Context, req *CreateSchemaRequest) (*Schema, error)
	getSchemaFn      func(ctx context.Context, id string, version *int32) (*Schema, error)
	listSchemasFn    func(ctx context.Context, pageSize int32, pageToken string) (*ListSchemasResponse, error)
	updateSchemaFn   func(ctx context.Context, req *UpdateSchemaRequest) (*Schema, error)
	publishSchemaFn  func(ctx context.Context, id string, version int32) (*Schema, error)
	deleteSchemaFn   func(ctx context.Context, id string) error
	exportSchemaFn   func(ctx context.Context, id string, version *int32) ([]byte, error)
	importSchemaFn   func(ctx context.Context, yamlContent []byte, autoPublish bool) (*Schema, error)
	createTenantFn   func(ctx context.Context, req *CreateTenantRequest) (*Tenant, error)
	getTenantFn      func(ctx context.Context, id string) (*Tenant, error)
	listTenantsFn    func(ctx context.Context, schemaID *string, pageSize int32, pageToken string) (*ListTenantsResponse, error)
	updateTenantFn   func(ctx context.Context, req *UpdateTenantRequest) (*Tenant, error)
	deleteTenantFn   func(ctx context.Context, id string) error
	lockFieldFn      func(ctx context.Context, tenantID, fieldPath string, lockedValues []string) error
	unlockFieldFn    func(ctx context.Context, tenantID, fieldPath string) error
	listFieldLocksFn func(ctx context.Context, tenantID string) ([]FieldLock, error)
}

func (m *mockSchemaTransport) CreateSchema(ctx context.Context, req *CreateSchemaRequest) (*Schema, error) {
	return m.createSchemaFn(ctx, req)
}

func (m *mockSchemaTransport) GetSchema(ctx context.Context, id string, version *int32) (*Schema, error) {
	return m.getSchemaFn(ctx, id, version)
}

func (m *mockSchemaTransport) ListSchemas(ctx context.Context, pageSize int32, pageToken string) (*ListSchemasResponse, error) {
	return m.listSchemasFn(ctx, pageSize, pageToken)
}

func (m *mockSchemaTransport) UpdateSchema(ctx context.Context, req *UpdateSchemaRequest) (*Schema, error) {
	return m.updateSchemaFn(ctx, req)
}

func (m *mockSchemaTransport) PublishSchema(ctx context.Context, id string, version int32) (*Schema, error) {
	return m.publishSchemaFn(ctx, id, version)
}

func (m *mockSchemaTransport) DeleteSchema(ctx context.Context, id string) error {
	return m.deleteSchemaFn(ctx, id)
}

func (m *mockSchemaTransport) ExportSchema(ctx context.Context, id string, version *int32) ([]byte, error) {
	return m.exportSchemaFn(ctx, id, version)
}

func (m *mockSchemaTransport) ImportSchema(ctx context.Context, yamlContent []byte, autoPublish bool) (*Schema, error) {
	return m.importSchemaFn(ctx, yamlContent, autoPublish)
}

func (m *mockSchemaTransport) CreateTenant(ctx context.Context, req *CreateTenantRequest) (*Tenant, error) {
	return m.createTenantFn(ctx, req)
}

func (m *mockSchemaTransport) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	return m.getTenantFn(ctx, id)
}

func (m *mockSchemaTransport) ListTenants(ctx context.Context, schemaID *string, pageSize int32, pageToken string) (*ListTenantsResponse, error) {
	return m.listTenantsFn(ctx, schemaID, pageSize, pageToken)
}

func (m *mockSchemaTransport) UpdateTenant(ctx context.Context, req *UpdateTenantRequest) (*Tenant, error) {
	return m.updateTenantFn(ctx, req)
}

func (m *mockSchemaTransport) DeleteTenant(ctx context.Context, id string) error {
	return m.deleteTenantFn(ctx, id)
}

func (m *mockSchemaTransport) LockField(ctx context.Context, tenantID, fieldPath string, lockedValues []string) error {
	return m.lockFieldFn(ctx, tenantID, fieldPath, lockedValues)
}

func (m *mockSchemaTransport) UnlockField(ctx context.Context, tenantID, fieldPath string) error {
	return m.unlockFieldFn(ctx, tenantID, fieldPath)
}

func (m *mockSchemaTransport) ListFieldLocks(ctx context.Context, tenantID string) ([]FieldLock, error) {
	return m.listFieldLocksFn(ctx, tenantID)
}

// --- Mock ConfigTransport ---

type mockConfigTransport struct {
	listVersionsFn func(ctx context.Context, tenantID string, pageSize int32, pageToken string) (*ListVersionsResponse, error)
	getVersionFn   func(ctx context.Context, tenantID string, version int32) (*Version, error)
	rollbackFn     func(ctx context.Context, tenantID string, version int32, description string) (*Version, error)
	exportConfigFn func(ctx context.Context, tenantID string, version *int32) ([]byte, error)
	importConfigFn func(ctx context.Context, req *ImportConfigRequest) (*Version, error)
}

func (m *mockConfigTransport) ListVersions(ctx context.Context, tenantID string, pageSize int32, pageToken string) (*ListVersionsResponse, error) {
	return m.listVersionsFn(ctx, tenantID, pageSize, pageToken)
}

func (m *mockConfigTransport) GetVersion(ctx context.Context, tenantID string, version int32) (*Version, error) {
	return m.getVersionFn(ctx, tenantID, version)
}

func (m *mockConfigTransport) RollbackToVersion(ctx context.Context, tenantID string, version int32, description string) (*Version, error) {
	return m.rollbackFn(ctx, tenantID, version, description)
}

func (m *mockConfigTransport) ExportConfig(ctx context.Context, tenantID string, version *int32) ([]byte, error) {
	return m.exportConfigFn(ctx, tenantID, version)
}

func (m *mockConfigTransport) ImportConfig(ctx context.Context, req *ImportConfigRequest) (*Version, error) {
	return m.importConfigFn(ctx, req)
}

// --- Mock AuditTransport ---

type mockAuditTransport struct {
	queryWriteLogFn   func(ctx context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error)
	getFieldUsageFn   func(ctx context.Context, tenantID, fieldPath string, start, end *time.Time) (*UsageStats, error)
	getTenantUsageFn  func(ctx context.Context, tenantID string, start, end *time.Time) ([]*UsageStats, error)
	getUnusedFieldsFn func(ctx context.Context, tenantID string, since time.Time) ([]string, error)
}

func (m *mockAuditTransport) QueryWriteLog(ctx context.Context, req *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
	return m.queryWriteLogFn(ctx, req)
}

func (m *mockAuditTransport) GetFieldUsage(ctx context.Context, tenantID, fieldPath string, start, end *time.Time) (*UsageStats, error) {
	return m.getFieldUsageFn(ctx, tenantID, fieldPath, start, end)
}

func (m *mockAuditTransport) GetTenantUsage(ctx context.Context, tenantID string, start, end *time.Time) ([]*UsageStats, error) {
	return m.getTenantUsageFn(ctx, tenantID, start, end)
}

func (m *mockAuditTransport) GetUnusedFields(ctx context.Context, tenantID string, since time.Time) ([]string, error) {
	return m.getUnusedFieldsFn(ctx, tenantID, since)
}

// --- Mock ServerTransport ---

type mockServerTransport struct {
	getServerInfoFn func(ctx context.Context) (*ServerInfo, error)
}

func (m *mockServerTransport) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	return m.getServerInfoFn(ctx)
}

// --- Basic tests ---

func TestNew(t *testing.T) {
	ms := &mockSchemaTransport{}
	mc := &mockConfigTransport{}
	ma := &mockAuditTransport{}
	client := New(WithSchemaTransport(ms), WithConfigTransport(mc), WithAuditTransport(ma))
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNew_NilTransports(t *testing.T) {
	client := New()
	if client == nil {
		t.Fatal("expected non-nil client even with nil transports")
	}
}

func TestCreateSchema_Success(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))
	ctx := context.Background()

	ms.createSchemaFn = func(_ context.Context, req *CreateSchemaRequest) (*Schema, error) {
		if req.Name != "payments" {
			t.Errorf("got Name %v, want %v", req.Name, "payments")
		}
		return &Schema{ID: "s1", Name: "payments", Version: 1, CreatedAt: time.Now()}, nil
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
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))
	ctx := context.Background()

	ms.getSchemaFn = func(_ context.Context, _ string, _ *int32) (*Schema, error) {
		return nil, ErrNotFound
	}

	_, err := client.GetSchema(ctx, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want %v", err, ErrNotFound)
	}
}

func TestCreateTenant_Success(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))
	ctx := context.Background()

	ms.createTenantFn = func(_ context.Context, req *CreateTenantRequest) (*Tenant, error) {
		return &Tenant{ID: "t1", Name: "acme", SchemaID: "s1", SchemaVersion: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
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
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))
	ctx := context.Background()

	ms.createTenantFn = func(_ context.Context, _ *CreateTenantRequest) (*Tenant, error) {
		return nil, ErrFailedPrecondition
	}

	_, err := client.CreateTenant(ctx, "acme", "s1", 1)
	if !errors.Is(err, ErrFailedPrecondition) {
		t.Errorf("got error %v, want %v", err, ErrFailedPrecondition)
	}
}

func TestLockUnlockField(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))
	ctx := context.Background()

	ms.lockFieldFn = func(_ context.Context, _, _ string, _ []string) error { return nil }
	ms.unlockFieldFn = func(_ context.Context, _, _ string) error { return nil }

	if err := client.LockField(ctx, "t1", "fee"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := client.UnlockField(ctx, "t1", "fee"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListConfigVersions_AutoPaginate(t *testing.T) {
	mc := &mockConfigTransport{}
	client := New(WithConfigTransport(mc))
	ctx := context.Background()

	mc.listVersionsFn = func(_ context.Context, _ string, _ int32, pageToken string) (*ListVersionsResponse, error) {
		if pageToken == "" {
			return &ListVersionsResponse{
				Versions:      []*Version{{Version: 3, CreatedAt: time.Now()}, {Version: 2, CreatedAt: time.Now()}},
				NextPageToken: "page2",
			}, nil
		}
		return &ListVersionsResponse{
			Versions: []*Version{{Version: 1, CreatedAt: time.Now()}},
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
	mc := &mockConfigTransport{}
	client := New(WithConfigTransport(mc))
	ctx := context.Background()

	mc.rollbackFn = func(_ context.Context, _ string, _ int32, _ string) (*Version, error) {
		return &Version{Version: 5, CreatedAt: time.Now()}, nil
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
	mc := &mockConfigTransport{}
	client := New(WithConfigTransport(mc))
	ctx := context.Background()

	mc.exportConfigFn = func(_ context.Context, _ string, _ *int32) ([]byte, error) {
		return []byte("spec_version: v1\nvalues:\n  a:\n    value: x\n"), nil
	}

	data, err := client.ExportConfig(ctx, "t1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(data), "spec_version") {
		t.Errorf("expected %q to contain %q", string(data), "spec_version")
	}

	mc.importConfigFn = func(_ context.Context, req *ImportConfigRequest) (*Version, error) {
		return &Version{Version: 3, CreatedAt: time.Now()}, nil
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
	ma := &mockAuditTransport{}
	client := New(WithAuditTransport(ma))
	ctx := context.Background()

	ma.queryWriteLogFn = func(_ context.Context, _ *QueryWriteLogRequest) (*QueryWriteLogResponse, error) {
		return &QueryWriteLogResponse{
			Entries: []*AuditEntry{
				{ID: "e1", TenantID: "t1", Actor: "admin", Action: "set_field", CreatedAt: time.Now()},
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

func TestGetServerInfo_Success(t *testing.T) {
	ms := &mockServerTransport{
		getServerInfoFn: func(_ context.Context) (*ServerInfo, error) {
			return &ServerInfo{
				Version:  "1.0.0",
				Commit:   "abc123",
				Features: map[string]bool{"schema": true, "config": true, "audit": false},
			}, nil
		},
	}
	client := New(WithServerTransport(ms))
	info, err := client.GetServerInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Version != "1.0.0" {
		t.Errorf("got Version %q, want %q", info.Version, "1.0.0")
	}
	if info.Commit != "abc123" {
		t.Errorf("got Commit %q, want %q", info.Commit, "abc123")
	}
	if !info.Features["schema"] {
		t.Error("expected schema=true")
	}
	if info.Features["audit"] {
		t.Error("expected audit=false")
	}
}

func TestServiceNotConfigured(t *testing.T) {
	client := New()
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

	_, err = client.GetServerInfo(ctx)
	if !errors.Is(err, ErrServiceNotConfigured) {
		t.Errorf("GetServerInfo: got error %v, want %v", err, ErrServiceNotConfigured)
	}
}

func TestGetLatestPublishedSchemaVersion_LatestIsPublished(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))

	ms.listSchemasFn = func(_ context.Context, _ int32, _ string) (*ListSchemasResponse, error) {
		return &ListSchemasResponse{
			Schemas: []*Schema{
				{ID: "s_other", Name: "other", Version: 1, Published: true},
				{ID: "s_payments", Name: "payments", Version: 3, Published: true},
			},
		}, nil
	}

	id, version, err := client.GetLatestPublishedSchemaVersion(context.Background(), "payments")
	if err != nil {
		t.Fatal(err)
	}
	if id != "s_payments" || version != 3 {
		t.Errorf("got %s@%d, want s_payments@3", id, version)
	}
}

func TestGetLatestPublishedSchemaVersion_WalksBackFromDraft(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))

	ms.listSchemasFn = func(_ context.Context, _ int32, _ string) (*ListSchemasResponse, error) {
		return &ListSchemasResponse{
			Schemas: []*Schema{{ID: "s1", Name: "payments", Version: 5, Published: false}},
		}, nil
	}
	// v5 draft, v4 draft, v3 published.
	ms.getSchemaFn = func(_ context.Context, id string, version *int32) (*Schema, error) {
		if id != "s1" {
			t.Errorf("unexpected id %q", id)
		}
		switch *version {
		case 4:
			return &Schema{ID: "s1", Version: 4, Published: false}, nil
		case 3:
			return &Schema{ID: "s1", Version: 3, Published: true}, nil
		}
		return nil, ErrNotFound
	}

	id, version, err := client.GetLatestPublishedSchemaVersion(context.Background(), "payments")
	if err != nil {
		t.Fatal(err)
	}
	if id != "s1" || version != 3 {
		t.Errorf("got %s@%d, want s1@3", id, version)
	}
}

func TestGetLatestPublishedSchemaVersion_NoPublishedVersion(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))

	ms.listSchemasFn = func(_ context.Context, _ int32, _ string) (*ListSchemasResponse, error) {
		return &ListSchemasResponse{
			Schemas: []*Schema{{ID: "s1", Name: "payments", Version: 2, Published: false}},
		}, nil
	}
	ms.getSchemaFn = func(_ context.Context, _ string, _ *int32) (*Schema, error) {
		return &Schema{Published: false}, nil
	}

	_, _, err := client.GetLatestPublishedSchemaVersion(context.Background(), "payments")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetLatestPublishedSchemaVersion_SchemaNameNotFound(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))

	ms.listSchemasFn = func(_ context.Context, _ int32, _ string) (*ListSchemasResponse, error) {
		return &ListSchemasResponse{Schemas: []*Schema{{ID: "s_other", Name: "other", Published: true}}}, nil
	}

	_, _, err := client.GetLatestPublishedSchemaVersion(context.Background(), "payments")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetLatestPublishedSchemaVersion_ListError(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))

	ms.listSchemasFn = func(_ context.Context, _ int32, _ string) (*ListSchemasResponse, error) {
		return nil, errors.New("rpc failed")
	}

	_, _, err := client.GetLatestPublishedSchemaVersion(context.Background(), "payments")
	if err == nil || !strings.Contains(err.Error(), "rpc failed") {
		t.Errorf("expected rpc failed error, got %v", err)
	}
}

func TestGetLatestPublishedSchemaVersion_GetSchemaVersionError(t *testing.T) {
	ms := &mockSchemaTransport{}
	client := New(WithSchemaTransport(ms))

	ms.listSchemasFn = func(_ context.Context, _ int32, _ string) (*ListSchemasResponse, error) {
		return &ListSchemasResponse{Schemas: []*Schema{{ID: "s1", Name: "payments", Version: 3, Published: false}}}, nil
	}
	ms.getSchemaFn = func(_ context.Context, _ string, _ *int32) (*Schema, error) {
		return nil, errors.New("transport error")
	}

	_, _, err := client.GetLatestPublishedSchemaVersion(context.Background(), "payments")
	if err == nil || !strings.Contains(err.Error(), "transport error") {
		t.Errorf("expected transport error, got %v", err)
	}
}
