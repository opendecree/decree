package seed

import (
	"context"
	"fmt"
	"testing"

	"github.com/opendecree/decree/sdk/adminclient"
)

// --- Mock client ---

type mockClient struct {
	importSchemaFn func(ctx context.Context, yamlContent []byte, autoPublish ...bool) (*adminclient.Schema, error)
	listSchemasFn  func(ctx context.Context) ([]*adminclient.Schema, error)
	listTenantsFn  func(ctx context.Context, schemaID string) ([]*adminclient.Tenant, error)
	createTenantFn func(ctx context.Context, name, schemaID string, schemaVersion int32) (*adminclient.Tenant, error)
	importConfigFn func(ctx context.Context, tenantID string, yamlContent []byte, description string, mode ...adminclient.ImportMode) (*adminclient.Version, error)
	lockFieldFn    func(ctx context.Context, tenantID, fieldPath string, lockedValues ...string) error
}

func (m *mockClient) ImportSchema(ctx context.Context, yamlContent []byte, autoPublish ...bool) (*adminclient.Schema, error) {
	return m.importSchemaFn(ctx, yamlContent, autoPublish...)
}

func (m *mockClient) ListSchemas(ctx context.Context) ([]*adminclient.Schema, error) {
	return m.listSchemasFn(ctx)
}

func (m *mockClient) ListTenants(ctx context.Context, schemaID string) ([]*adminclient.Tenant, error) {
	return m.listTenantsFn(ctx, schemaID)
}

func (m *mockClient) CreateTenant(ctx context.Context, name, schemaID string, schemaVersion int32) (*adminclient.Tenant, error) {
	return m.createTenantFn(ctx, name, schemaID, schemaVersion)
}

func (m *mockClient) ImportConfig(ctx context.Context, tenantID string, yamlContent []byte, description string, mode ...adminclient.ImportMode) (*adminclient.Version, error) {
	return m.importConfigFn(ctx, tenantID, yamlContent, description, mode...)
}

func (m *mockClient) LockField(ctx context.Context, tenantID, fieldPath string, lockedValues ...string) error {
	return m.lockFieldFn(ctx, tenantID, fieldPath, lockedValues...)
}

// --- Seed file for tests ---

func testFile() *File {
	return &File{
		Syntax: "v1",
		Schema: SchemaDef{
			Name: "test-schema",
			Fields: map[string]FieldDef{
				"rate": {Type: "number"},
			},
		},
		Tenant: TenantDef{Name: "test-tenant"},
		Config: ConfigDef{
			Description: "initial",
			Values: map[string]ConfigValueDef{
				"rate": {Value: 42},
			},
		},
		Locks: []LockDef{
			{FieldPath: "rate"},
		},
	}
}

// --- Parse tests ---

const validSeedYAML = `syntax: "v1"
schema:
  name: payment-config
  description: "Payment settings"
  fields:
    payments.fee:
      type: string
      description: "Fee percentage"
    payments.enabled:
      type: bool
      default: "true"
tenant:
  name: acme-corp
config:
  description: "Initial values"
  values:
    payments.fee:
      value: "0.5%"
    payments.enabled:
      value: true
locks:
  - field_path: payments.fee
  - field_path: payments.enabled
    locked_values: ["true"]
`

func TestParseFile_Valid(t *testing.T) {
	f, err := ParseFile([]byte(validSeedYAML))
	if err != nil {
		t.Fatal(err)
	}

	if f.Syntax != "v1" {
		t.Errorf("syntax = %q, want v1", f.Syntax)
	}
	if f.Schema.Name != "payment-config" {
		t.Errorf("schema.name = %q, want payment-config", f.Schema.Name)
	}
	if len(f.Schema.Fields) != 2 {
		t.Errorf("schema.fields count = %d, want 2", len(f.Schema.Fields))
	}
	if f.Tenant.Name != "acme-corp" {
		t.Errorf("tenant.name = %q, want acme-corp", f.Tenant.Name)
	}
	if len(f.Config.Values) != 2 {
		t.Errorf("config.values count = %d, want 2", len(f.Config.Values))
	}
	if len(f.Locks) != 2 {
		t.Errorf("locks count = %d, want 2", len(f.Locks))
	}
	if f.Locks[1].FieldPath != "payments.enabled" {
		t.Errorf("locks[1].field_path = %q", f.Locks[1].FieldPath)
	}
	if len(f.Locks[1].LockedValues) != 1 || f.Locks[1].LockedValues[0] != "true" {
		t.Errorf("locks[1].locked_values = %v", f.Locks[1].LockedValues)
	}
}

func TestParseFile_Errors(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"invalid yaml", "{{bad"},
		{"wrong syntax", `syntax: "v2"
schema:
  name: test
  fields:
    a:
      type: string
tenant:
  name: t`},
		{"no schema name", `syntax: "v1"
schema:
  fields:
    a:
      type: string
tenant:
  name: t`},
		{"no fields", `syntax: "v1"
schema:
  name: test
  fields: {}
tenant:
  name: t`},
		{"no tenant name", `syntax: "v1"
schema:
  name: test
  fields:
    a:
      type: string
tenant:
  name: ""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFile([]byte(tt.data))
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestParseFile_MinimalValid(t *testing.T) {
	data := `syntax: "v1"
schema:
  name: minimal
  fields:
    x:
      type: string
tenant:
  name: test-tenant
`
	f, err := ParseFile([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Config.Values) != 0 {
		t.Errorf("expected empty config values, got %d", len(f.Config.Values))
	}
	if len(f.Locks) != 0 {
		t.Errorf("expected empty locks, got %d", len(f.Locks))
	}
}

func TestMarshal_RoundTrip(t *testing.T) {
	f, err := ParseFile([]byte(validSeedYAML))
	if err != nil {
		t.Fatal(err)
	}

	data, err := Marshal(f)
	if err != nil {
		t.Fatal(err)
	}

	f2, err := ParseFile(data)
	if err != nil {
		t.Fatalf("re-parse failed: %v\nYAML:\n%s", err, data)
	}

	if f2.Schema.Name != f.Schema.Name {
		t.Errorf("round-trip schema name: %q != %q", f2.Schema.Name, f.Schema.Name)
	}
	if f2.Tenant.Name != f.Tenant.Name {
		t.Errorf("round-trip tenant name: %q != %q", f2.Tenant.Name, f.Tenant.Name)
	}
	if len(f2.Schema.Fields) != len(f.Schema.Fields) {
		t.Errorf("round-trip fields count: %d != %d", len(f2.Schema.Fields), len(f.Schema.Fields))
	}
	if len(f2.Config.Values) != len(f.Config.Values) {
		t.Errorf("round-trip values count: %d != %d", len(f2.Config.Values), len(f.Config.Values))
	}
	if len(f2.Locks) != len(f.Locks) {
		t.Errorf("round-trip locks count: %d != %d", len(f2.Locks), len(f.Locks))
	}
}

func TestParseFile_FieldConstraints(t *testing.T) {
	data := `syntax: "v1"
schema:
  name: constrained
  fields:
    rate:
      type: number
      constraints:
        minimum: 0
        maximum: 100
        exclusiveMinimum: 0.1
        exclusiveMaximum: 99.9
    code:
      type: string
      constraints:
        minLength: 2
        maxLength: 10
        pattern: "^[A-Z]+$"
        enum: [USD, EUR]
tenant:
  name: t
`
	f, err := ParseFile([]byte(data))
	if err != nil {
		t.Fatal(err)
	}

	rate := f.Schema.Fields["rate"]
	if rate.Constraints == nil {
		t.Fatal("rate constraints is nil")
	}
	if rate.Constraints.Minimum == nil || *rate.Constraints.Minimum != 0 {
		t.Errorf("rate minimum: %v", rate.Constraints.Minimum)
	}
	if rate.Constraints.Maximum == nil || *rate.Constraints.Maximum != 100 {
		t.Errorf("rate maximum: %v", rate.Constraints.Maximum)
	}

	code := f.Schema.Fields["code"]
	if code.Constraints == nil {
		t.Fatal("code constraints is nil")
	}
	if code.Constraints.MinLength == nil || *code.Constraints.MinLength != 2 {
		t.Errorf("code minLength: %v", code.Constraints.MinLength)
	}
	if code.Constraints.Pattern != "^[A-Z]+$" {
		t.Errorf("code pattern: %q", code.Constraints.Pattern)
	}
	if len(code.Constraints.Enum) != 2 {
		t.Errorf("code enum: %v", code.Constraints.Enum)
	}
}

// --- Run tests with mocks ---

func TestRun_NewSchemaNewTenantWithConfig(t *testing.T) {
	file := testFile()
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return &adminclient.Schema{ID: "s1", Version: 1}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return nil, nil // no existing tenants
		},
		createTenantFn: func(_ context.Context, name, schemaID string, _ int32) (*adminclient.Tenant, error) {
			return &adminclient.Tenant{ID: "t1", Name: name, SchemaID: schemaID}, nil
		},
		importConfigFn: func(_ context.Context, _ string, _ []byte, _ string, _ ...adminclient.ImportMode) (*adminclient.Version, error) {
			return &adminclient.Version{Version: 1}, nil
		},
		lockFieldFn: func(_ context.Context, _, _ string, _ ...string) error {
			return nil
		},
	}

	result, err := Run(context.Background(), mock, file)
	if err != nil {
		t.Fatal(err)
	}
	if !result.SchemaCreated {
		t.Error("expected schema to be created")
	}
	if result.SchemaID != "s1" {
		t.Errorf("schema ID = %q", result.SchemaID)
	}
	if !result.TenantCreated {
		t.Error("expected tenant to be created")
	}
	if result.TenantID != "t1" {
		t.Errorf("tenant ID = %q", result.TenantID)
	}
	if !result.ConfigImported {
		t.Error("expected config to be imported")
	}
	if result.ConfigVersion != 1 {
		t.Errorf("config version = %d", result.ConfigVersion)
	}
	if result.LocksApplied != 1 {
		t.Errorf("locks applied = %d", result.LocksApplied)
	}
}

func TestRun_ExistingSchemaExistingTenant(t *testing.T) {
	file := testFile()
	file.Config.Values = nil // no config to import
	file.Locks = nil         // no locks

	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return nil, adminclient.ErrAlreadyExists
		},
		listSchemasFn: func(_ context.Context) ([]*adminclient.Schema, error) {
			return []*adminclient.Schema{
				{ID: "s1", Name: "test-schema", Version: 2},
			}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return []*adminclient.Tenant{
				{ID: "t1", Name: "test-tenant"},
			}, nil
		},
	}

	result, err := Run(context.Background(), mock, file)
	if err != nil {
		t.Fatal(err)
	}
	if result.SchemaCreated {
		t.Error("expected schema to be skipped")
	}
	if result.SchemaID != "s1" {
		t.Errorf("schema ID = %q", result.SchemaID)
	}
	if result.SchemaVersion != 2 {
		t.Errorf("schema version = %d", result.SchemaVersion)
	}
	if result.TenantCreated {
		t.Error("expected tenant to be reused")
	}
	if result.TenantID != "t1" {
		t.Errorf("tenant ID = %q", result.TenantID)
	}
	if result.ConfigImported {
		t.Error("expected no config import")
	}
}

func TestRun_AutoPublish(t *testing.T) {
	file := testFile()
	file.Config.Values = nil
	file.Locks = nil

	var gotAutoPublish bool
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, autoPublish ...bool) (*adminclient.Schema, error) {
			if len(autoPublish) > 0 {
				gotAutoPublish = autoPublish[0]
			}
			return &adminclient.Schema{ID: "s1", Version: 1}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return nil, nil
		},
		createTenantFn: func(_ context.Context, _, _ string, _ int32) (*adminclient.Tenant, error) {
			return &adminclient.Tenant{ID: "t1"}, nil
		},
	}

	_, err := Run(context.Background(), mock, file, AutoPublish())
	if err != nil {
		t.Fatal(err)
	}
	if !gotAutoPublish {
		t.Error("expected AutoPublish to be passed to ImportSchema")
	}
}

func TestRun_SchemaImportError(t *testing.T) {
	file := testFile()
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	_, err := Run(context.Background(), mock, file)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_SchemaExistsButNotFound(t *testing.T) {
	file := testFile()
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return nil, adminclient.ErrAlreadyExists
		},
		listSchemasFn: func(_ context.Context) ([]*adminclient.Schema, error) {
			return []*adminclient.Schema{
				{ID: "s1", Name: "other-schema"},
			}, nil
		},
	}

	_, err := Run(context.Background(), mock, file)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_ListSchemasError(t *testing.T) {
	file := testFile()
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return nil, adminclient.ErrAlreadyExists
		},
		listSchemasFn: func(_ context.Context) ([]*adminclient.Schema, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	_, err := Run(context.Background(), mock, file)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_ListTenantsError(t *testing.T) {
	file := testFile()
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return &adminclient.Schema{ID: "s1", Version: 1}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	_, err := Run(context.Background(), mock, file)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_CreateTenantError(t *testing.T) {
	file := testFile()
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return &adminclient.Schema{ID: "s1", Version: 1}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return nil, nil
		},
		createTenantFn: func(_ context.Context, _, _ string, _ int32) (*adminclient.Tenant, error) {
			return nil, fmt.Errorf("already exists")
		},
	}

	_, err := Run(context.Background(), mock, file)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_ImportConfigError(t *testing.T) {
	file := testFile()
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return &adminclient.Schema{ID: "s1", Version: 1}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return nil, nil
		},
		createTenantFn: func(_ context.Context, _, _ string, _ int32) (*adminclient.Tenant, error) {
			return &adminclient.Tenant{ID: "t1"}, nil
		},
		importConfigFn: func(_ context.Context, _ string, _ []byte, _ string, _ ...adminclient.ImportMode) (*adminclient.Version, error) {
			return nil, fmt.Errorf("validation error")
		},
	}

	_, err := Run(context.Background(), mock, file)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_LockFieldError(t *testing.T) {
	file := testFile()
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return &adminclient.Schema{ID: "s1", Version: 1}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return nil, nil
		},
		createTenantFn: func(_ context.Context, _, _ string, _ int32) (*adminclient.Tenant, error) {
			return &adminclient.Tenant{ID: "t1"}, nil
		},
		importConfigFn: func(_ context.Context, _ string, _ []byte, _ string, _ ...adminclient.ImportMode) (*adminclient.Version, error) {
			return &adminclient.Version{Version: 1}, nil
		},
		lockFieldFn: func(_ context.Context, _, _ string, _ ...string) error {
			return fmt.Errorf("permission denied")
		},
	}

	_, err := Run(context.Background(), mock, file)
	if err == nil {
		t.Fatal("expected error")
	}
}
