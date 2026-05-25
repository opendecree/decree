package adminclient_test

import (
	"context"
	"fmt"
	"time"

	"github.com/opendecree/decree/sdk/adminclient"
)

// fakeSchemaTransport implements SchemaTransport for documentation examples.
type fakeSchemaTransport struct{}

func (f *fakeSchemaTransport) CreateSchema(_ context.Context, req *adminclient.CreateSchemaRequest) (*adminclient.Schema, error) {
	return &adminclient.Schema{ID: "schema-1", Name: req.Name, Version: 1}, nil
}

func (f *fakeSchemaTransport) GetSchema(_ context.Context, id string, _ *int32) (*adminclient.Schema, error) {
	return &adminclient.Schema{ID: id, Name: "app-config", Version: 1}, nil
}

func (f *fakeSchemaTransport) ListSchemas(_ context.Context, _ int32, _ string) (*adminclient.ListSchemasResponse, error) {
	return &adminclient.ListSchemasResponse{}, nil
}

func (f *fakeSchemaTransport) UpdateSchema(_ context.Context, _ *adminclient.UpdateSchemaRequest) (*adminclient.Schema, error) {
	return &adminclient.Schema{ID: "schema-1", Version: 2}, nil
}

func (f *fakeSchemaTransport) PublishSchema(_ context.Context, id string, version int32) (*adminclient.Schema, error) {
	return &adminclient.Schema{ID: id, Version: version, Published: true}, nil
}

func (f *fakeSchemaTransport) DeleteSchema(_ context.Context, _ string) error { return nil }

func (f *fakeSchemaTransport) ExportSchema(_ context.Context, _ string, _ *int32) ([]byte, error) {
	return []byte("fields: []"), nil
}

func (f *fakeSchemaTransport) ImportSchema(_ context.Context, _ []byte, _ bool) (*adminclient.Schema, error) {
	return &adminclient.Schema{ID: "schema-1", Name: "app-config", Version: 1, Published: true}, nil
}

func (f *fakeSchemaTransport) CreateTenant(_ context.Context, req *adminclient.CreateTenantRequest) (*adminclient.Tenant, error) {
	return &adminclient.Tenant{ID: "tenant-1", Name: req.Name, SchemaID: req.SchemaID}, nil
}

func (f *fakeSchemaTransport) GetTenant(_ context.Context, id string) (*adminclient.Tenant, error) {
	return &adminclient.Tenant{ID: id, Name: "acme-corp"}, nil
}

func (f *fakeSchemaTransport) ListTenants(_ context.Context, _ *string, _ int32, _ string) (*adminclient.ListTenantsResponse, error) {
	return &adminclient.ListTenantsResponse{}, nil
}

func (f *fakeSchemaTransport) UpdateTenant(_ context.Context, req *adminclient.UpdateTenantRequest) (*adminclient.Tenant, error) {
	return &adminclient.Tenant{ID: req.ID}, nil
}

func (f *fakeSchemaTransport) DeleteTenant(_ context.Context, _ string) error { return nil }

func (f *fakeSchemaTransport) LockField(_ context.Context, _ string, fieldPath string, _ []string) error {
	_ = fieldPath
	return nil
}

func (f *fakeSchemaTransport) UnlockField(_ context.Context, _ string, _ string) error { return nil }

func (f *fakeSchemaTransport) ListFieldLocks(_ context.Context, _ string) ([]adminclient.FieldLock, error) {
	return []adminclient.FieldLock{
		{TenantID: "tenant-1", FieldPath: "app.tier", LockedValues: []string{"free"}},
	}, nil
}

// fakeAuditTransport implements AuditTransport for documentation examples.
type fakeAuditTransport struct{}

func (f *fakeAuditTransport) QueryWriteLog(_ context.Context, _ *adminclient.QueryWriteLogRequest) (*adminclient.QueryWriteLogResponse, error) {
	return &adminclient.QueryWriteLogResponse{}, nil
}

func (f *fakeAuditTransport) GetFieldUsage(_ context.Context, _, _ string, _, _ *time.Time) (*adminclient.UsageStats, error) {
	return &adminclient.UsageStats{}, nil
}

func (f *fakeAuditTransport) GetTenantUsage(_ context.Context, _ string, _, _ *time.Time) ([]*adminclient.UsageStats, error) {
	return nil, nil
}

func (f *fakeAuditTransport) GetUnusedFields(_ context.Context, _ string, _ time.Time) ([]string, error) {
	return nil, nil
}

func newClient() *adminclient.Client {
	return adminclient.New(
		adminclient.WithSchemaTransport(&fakeSchemaTransport{}),
		adminclient.WithAuditTransport(&fakeAuditTransport{}),
	)
}

func ExampleClient_ImportSchema() {
	client := newClient()
	ctx := context.Background()

	yaml := []byte(`
name: app-config
fields:
  - path: app.environment
    type: string
`)
	schema, err := client.ImportSchema(ctx, yaml, adminclient.WithAutoPublish())
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(schema.Name)
	// Output: app-config
}

func ExampleClient_LockField() {
	client := newClient()
	ctx := context.Background()

	// Lock app.tier to "free" for tenant-1 — prevents runtime overrides.
	err := client.LockField(ctx, "tenant-1", "app.tier", "free")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	locks, err := client.ListFieldLocks(ctx, "tenant-1")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(len(locks), "lock(s)")
	// Output: 1 lock(s)
}

// fakeConfigTransport implements ConfigTransport for documentation examples.
type fakeConfigTransport struct{}

func (f *fakeConfigTransport) ListVersions(_ context.Context, _ string, _ int32, _ string) (*adminclient.ListVersionsResponse, error) {
	return &adminclient.ListVersionsResponse{
		Versions: []*adminclient.Version{{Version: 2}, {Version: 1}},
	}, nil
}

func (f *fakeConfigTransport) GetVersion(_ context.Context, _ string, version int32) (*adminclient.Version, error) {
	return &adminclient.Version{Version: version}, nil
}

func (f *fakeConfigTransport) RollbackToVersion(_ context.Context, _ string, targetVersion int32, _ string) (*adminclient.Version, error) {
	return &adminclient.Version{Version: targetVersion + 1}, nil
}

func (f *fakeConfigTransport) ExportConfig(_ context.Context, _ string, _ *int32) ([]byte, error) {
	return []byte("fields: []"), nil
}

func (f *fakeConfigTransport) ImportConfig(_ context.Context, _ *adminclient.ImportConfigRequest) (*adminclient.Version, error) {
	return &adminclient.Version{Version: 3}, nil
}

// fakeServerTransport implements ServerTransport for documentation examples.
type fakeServerTransport struct{}

func (f *fakeServerTransport) GetServerInfo(_ context.Context) (*adminclient.ServerInfo, error) {
	return &adminclient.ServerInfo{Version: "v0.3.0"}, nil
}

func newFullClient() *adminclient.Client {
	return adminclient.New(
		adminclient.WithSchemaTransport(&fakeSchemaTransport{}),
		adminclient.WithConfigTransport(&fakeConfigTransport{}),
		adminclient.WithAuditTransport(&fakeAuditTransport{}),
		adminclient.WithServerTransport(&fakeServerTransport{}),
	)
}

func ExampleClient_CreateSchema() {
	client := newClient()
	ctx := context.Background()

	schema, err := client.CreateSchema(ctx, "app-config", []adminclient.Field{
		{Path: "app.env", Type: "FIELD_TYPE_STRING", Description: "Deployment environment"},
	}, "initial schema")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(schema.Name)
	// Output: app-config
}

func ExampleClient_CreateTenant() {
	client := newClient()
	ctx := context.Background()

	tenant, err := client.CreateTenant(ctx, "acme-corp", "schema-1", 1)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(tenant.Name)
	// Output: acme-corp
}

func ExampleWithAuditTenant() {
	client := newClient()
	ctx := context.Background()

	// Filter the audit log to a single tenant.
	entries, err := client.QueryWriteLog(ctx, adminclient.WithAuditTenant("tenant-1"))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(len(entries) >= 0)
	// Output: true
}

func ExampleClient_QueryWriteLog() {
	client := newClient()
	ctx := context.Background()

	entries, err := client.QueryWriteLog(ctx,
		adminclient.WithAuditTenant("tenant-1"),
		adminclient.WithAuditField("app.env"),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(len(entries) >= 0)
	// Output: true
}

func ExampleClient_VerifyChain() {
	client := newClient()
	ctx := context.Background()

	result, err := client.VerifyChain(ctx, "tenant-1")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(result.OK)
	// Output: true
}

func ExampleClient_RollbackConfig() {
	client := newFullClient()
	ctx := context.Background()

	// Roll back to version 2; a new version is created with those values.
	v, err := client.RollbackConfig(ctx, "tenant-1", 2, "revert bad deploy")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(v.Version > 0)
	// Output: true
}
