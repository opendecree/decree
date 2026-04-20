package seed

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/opendecree/decree/sdk/adminclient"
)

// --- Mock client ---

type mockClient struct {
	importSchemaFn                    func(ctx context.Context, yamlContent []byte, autoPublish ...bool) (*adminclient.Schema, error)
	listSchemasFn                     func(ctx context.Context) ([]*adminclient.Schema, error)
	getLatestPublishedSchemaVersionFn func(ctx context.Context, name string) (string, int32, error)
	listTenantsFn                     func(ctx context.Context, schemaID string) ([]*adminclient.Tenant, error)
	createTenantFn                    func(ctx context.Context, name, schemaID string, schemaVersion int32) (*adminclient.Tenant, error)
	importConfigFn                    func(ctx context.Context, tenantID string, yamlContent []byte, description string, mode ...adminclient.ImportMode) (*adminclient.Version, error)
	listConfigVersionsFn              func(ctx context.Context, tenantID string) ([]*adminclient.Version, error)
	lockFieldFn                       func(ctx context.Context, tenantID, fieldPath string, lockedValues ...string) error
}

func (m *mockClient) ImportSchema(ctx context.Context, yamlContent []byte, autoPublish ...bool) (*adminclient.Schema, error) {
	return m.importSchemaFn(ctx, yamlContent, autoPublish...)
}

func (m *mockClient) ListSchemas(ctx context.Context) ([]*adminclient.Schema, error) {
	return m.listSchemasFn(ctx)
}

func (m *mockClient) GetLatestPublishedSchemaVersion(ctx context.Context, name string) (string, int32, error) {
	return m.getLatestPublishedSchemaVersionFn(ctx, name)
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

func (m *mockClient) ListConfigVersions(ctx context.Context, tenantID string) ([]*adminclient.Version, error) {
	return m.listConfigVersionsFn(ctx, tenantID)
}

func (m *mockClient) LockField(ctx context.Context, tenantID, fieldPath string, lockedValues ...string) error {
	return m.lockFieldFn(ctx, tenantID, fieldPath, lockedValues...)
}

// --- Seed file for tests ---

func testFile() *File {
	return &File{
		SpecVersion: "v1",
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

const validSeedYAML = `spec_version: "v1"
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

	if f.SpecVersion != "v1" {
		t.Errorf("spec_version = %q, want v1", f.SpecVersion)
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
		{"wrong spec_version", `spec_version: "v2"
schema:
  name: test
  fields:
    a:
      type: string
tenant:
  name: t`},
		{"no schema name", `spec_version: "v1"
schema:
  fields:
    a:
      type: string
tenant:
  name: t`},
		{"no fields", `spec_version: "v1"
schema:
  name: test
  fields: {}
tenant:
  name: t`},
		{"empty file", `spec_version: "v1"`},
		{"config without tenant", `spec_version: "v1"
config:
  values:
    x:
      value: 1`},
		{"config-only missing schema ref", `spec_version: "v1"
tenant:
  name: t
config:
  values:
    x:
      value: 1`},
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
	data := `spec_version: "v1"
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
	data := `spec_version: "v1"
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

// --- ParseFile: decoupled modes ---

func TestParseFile_SchemaOnly(t *testing.T) {
	data := `spec_version: "v1"
schema:
  name: payments
  fields:
    rate:
      type: number
`
	f, err := ParseFile([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if f.Schema.Name != "payments" || f.Tenant.Name != "" || len(f.Config.Values) != 0 {
		t.Errorf("schema-only parsed incorrectly: %+v", f)
	}
}

func TestParseFile_ConfigOnly_SchemaVersionOmitted(t *testing.T) {
	data := `spec_version: "v1"
tenant:
  name: org1
  schema: payments
config:
  values:
    rate:
      value: 42
`
	f, err := ParseFile([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if f.Tenant.Schema != "payments" {
		t.Errorf("tenant.schema = %q", f.Tenant.Schema)
	}
	if f.Tenant.SchemaVersion != nil {
		t.Errorf("expected schema_version nil (latest), got %d", *f.Tenant.SchemaVersion)
	}
}

func TestParseFile_ConfigOnly_SchemaVersionExplicit(t *testing.T) {
	data := `spec_version: "v1"
tenant:
  name: org1
  schema: payments
  schema_version: 3
config:
  values:
    rate:
      value: 42
`
	f, err := ParseFile([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if f.Tenant.SchemaVersion == nil || *f.Tenant.SchemaVersion != 3 {
		t.Errorf("schema_version = %v, want 3", f.Tenant.SchemaVersion)
	}
}

// --- Run: schema-only mode ---

func TestRun_SchemaOnly(t *testing.T) {
	file := &File{
		SpecVersion: "v1",
		Schema: SchemaDef{
			Name:   "payments",
			Fields: map[string]FieldDef{"rate": {Type: "number"}},
		},
	}
	var importConfigCalled, createTenantCalled bool
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return &adminclient.Schema{ID: "s1", Version: 1}, nil
		},
		createTenantFn: func(_ context.Context, _, _ string, _ int32) (*adminclient.Tenant, error) {
			createTenantCalled = true
			return nil, nil
		},
		importConfigFn: func(_ context.Context, _ string, _ []byte, _ string, _ ...adminclient.ImportMode) (*adminclient.Version, error) {
			importConfigCalled = true
			return nil, nil
		},
	}
	result, err := Run(context.Background(), mock, file)
	if err != nil {
		t.Fatal(err)
	}
	if !result.SchemaCreated || result.SchemaID != "s1" {
		t.Errorf("expected schema created, got %+v", result)
	}
	if result.TenantID != "" || createTenantCalled {
		t.Error("tenant should not be touched in schema-only mode")
	}
	if result.ConfigImported || importConfigCalled {
		t.Error("config should not be touched in schema-only mode")
	}
}

// --- Run: config-only mode ---

func configOnlyFile(schemaVersion *int32) *File {
	return &File{
		SpecVersion: "v1",
		Tenant: TenantDef{
			Name:          "org1",
			Schema:        "payments",
			SchemaVersion: schemaVersion,
		},
		Config: ConfigDef{
			Values: map[string]ConfigValueDef{"rate": {Value: 42}},
		},
	}
}

func TestRun_ConfigOnly_LatestPublished(t *testing.T) {
	file := configOnlyFile(nil)
	var resolvedName string
	var importSchemaCalled bool
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			importSchemaCalled = true
			return nil, nil
		},
		getLatestPublishedSchemaVersionFn: func(_ context.Context, name string) (string, int32, error) {
			resolvedName = name
			return "s1", 5, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return []*adminclient.Tenant{{ID: "t1", Name: "org1"}}, nil
		},
		importConfigFn: func(_ context.Context, _ string, _ []byte, _ string, _ ...adminclient.ImportMode) (*adminclient.Version, error) {
			return &adminclient.Version{Version: 2}, nil
		},
	}
	result, err := Run(context.Background(), mock, file)
	if err != nil {
		t.Fatal(err)
	}
	if importSchemaCalled {
		t.Error("ImportSchema should not be called in config-only mode")
	}
	if resolvedName != "payments" {
		t.Errorf("resolved schema name = %q", resolvedName)
	}
	if result.SchemaID != "s1" || result.SchemaVersion != 5 {
		t.Errorf("expected s1@5, got %s@%d", result.SchemaID, result.SchemaVersion)
	}
	if result.TenantID != "t1" || result.TenantCreated {
		t.Errorf("expected reused tenant t1, got %+v", result)
	}
	if !result.ConfigImported || result.ConfigVersion != 2 {
		t.Errorf("expected config v2, got %+v", result)
	}
}

func TestRun_ConfigOnly_ExplicitVersion(t *testing.T) {
	v := int32(3)
	file := configOnlyFile(&v)
	var getLatestCalled bool
	mock := &mockClient{
		listSchemasFn: func(_ context.Context) ([]*adminclient.Schema, error) {
			return []*adminclient.Schema{{ID: "s1", Name: "payments", Version: 5}}, nil
		},
		getLatestPublishedSchemaVersionFn: func(_ context.Context, _ string) (string, int32, error) {
			getLatestCalled = true
			return "", 0, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return nil, nil
		},
		createTenantFn: func(_ context.Context, _, _ string, schemaVersion int32) (*adminclient.Tenant, error) {
			if schemaVersion != 3 {
				t.Errorf("CreateTenant called with schema_version=%d, want 3", schemaVersion)
			}
			return &adminclient.Tenant{ID: "t1"}, nil
		},
		importConfigFn: func(_ context.Context, _ string, _ []byte, _ string, _ ...adminclient.ImportMode) (*adminclient.Version, error) {
			return &adminclient.Version{Version: 1}, nil
		},
	}
	_, err := Run(context.Background(), mock, file)
	if err != nil {
		t.Fatal(err)
	}
	if getLatestCalled {
		t.Error("GetLatestPublishedSchemaVersion should not be called when schema_version is explicit")
	}
}

func TestRun_ConfigOnly_NoPublishedVersion(t *testing.T) {
	file := configOnlyFile(nil)
	mock := &mockClient{
		getLatestPublishedSchemaVersionFn: func(_ context.Context, _ string) (string, int32, error) {
			return "", 0, adminclient.ErrNotFound
		},
	}
	_, err := Run(context.Background(), mock, file)
	if err == nil {
		t.Fatal("expected error when no published schema version")
	}
}

func TestRun_ConfigOnly_SchemaMismatch(t *testing.T) {
	file := configOnlyFile(nil)
	mock := &mockClient{
		getLatestPublishedSchemaVersionFn: func(_ context.Context, _ string) (string, int32, error) {
			return "s_payments", 1, nil
		},
		listSchemasFn: func(_ context.Context) ([]*adminclient.Schema, error) {
			return []*adminclient.Schema{
				{ID: "s_payments", Name: "payments"},
				{ID: "s_billing", Name: "billing"},
			}, nil
		},
		listTenantsFn: func(_ context.Context, schemaID string) ([]*adminclient.Tenant, error) {
			if schemaID == "s_billing" {
				return []*adminclient.Tenant{{ID: "t_existing", Name: "org1"}}, nil
			}
			return nil, nil
		},
	}
	_, err := Run(context.Background(), mock, file)
	if err == nil {
		t.Fatal("expected schema-mismatch error")
	}
}

// --- Run: idempotency ---

func TestRun_SchemaReseed_NoNewVersion(t *testing.T) {
	file := testFile()
	file.Config.Values = nil
	file.Locks = nil
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return nil, adminclient.ErrAlreadyExists
		},
		listSchemasFn: func(_ context.Context) ([]*adminclient.Schema, error) {
			return []*adminclient.Schema{{ID: "s1", Name: "test-schema", Version: 4}}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return []*adminclient.Tenant{{ID: "t1", Name: "test-tenant"}}, nil
		},
	}
	result, err := Run(context.Background(), mock, file)
	if err != nil {
		t.Fatal(err)
	}
	if result.SchemaCreated {
		t.Error("schema was re-created — expected no new version")
	}
	if result.SchemaVersion != 4 {
		t.Errorf("expected existing version 4, got %d", result.SchemaVersion)
	}
}

func TestRun_ConfigReseed_NoNewVersion(t *testing.T) {
	file := testFile()
	file.Locks = nil
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return &adminclient.Schema{ID: "s1", Version: 1}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return []*adminclient.Tenant{{ID: "t1", Name: "test-tenant"}}, nil
		},
		importConfigFn: func(_ context.Context, _ string, _ []byte, _ string, _ ...adminclient.ImportMode) (*adminclient.Version, error) {
			return nil, adminclient.ErrAlreadyExists
		},
		listConfigVersionsFn: func(_ context.Context, _ string) ([]*adminclient.Version, error) {
			return []*adminclient.Version{{Version: 7}, {Version: 6}}, nil
		},
	}
	result, err := Run(context.Background(), mock, file)
	if err != nil {
		t.Fatalf("re-seed with identical config should not error, got: %v", err)
	}
	if result.ConfigImported {
		t.Error("config was imported — expected no new version")
	}
	if result.ConfigVersion != 7 {
		t.Errorf("expected existing version 7, got %d", result.ConfigVersion)
	}
}

func TestRun_FullNoOpReseed(t *testing.T) {
	file := testFile()
	file.Locks = nil
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return nil, adminclient.ErrAlreadyExists
		},
		listSchemasFn: func(_ context.Context) ([]*adminclient.Schema, error) {
			return []*adminclient.Schema{{ID: "s1", Name: "test-schema", Version: 2}}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return []*adminclient.Tenant{{ID: "t1", Name: "test-tenant"}}, nil
		},
		importConfigFn: func(_ context.Context, _ string, _ []byte, _ string, _ ...adminclient.ImportMode) (*adminclient.Version, error) {
			return nil, adminclient.ErrAlreadyExists
		},
		listConfigVersionsFn: func(_ context.Context, _ string) ([]*adminclient.Version, error) {
			return []*adminclient.Version{{Version: 3}}, nil
		},
	}
	result, err := Run(context.Background(), mock, file)
	if err != nil {
		t.Fatal(err)
	}
	if result.SchemaCreated || result.TenantCreated || result.ConfigImported {
		t.Errorf("expected fully no-op re-seed, got %+v", result)
	}
}

// --- ParseFile / Run: remaining modes ---

func TestParseFile_SchemaAndTenant(t *testing.T) {
	data := `spec_version: "v1"
schema:
  name: payments
  fields:
    rate:
      type: number
tenant:
  name: org1
`
	f, err := ParseFile([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if f.Schema.Name != "payments" || f.Tenant.Name != "org1" || len(f.Config.Values) != 0 {
		t.Errorf("schema+tenant parsed incorrectly: %+v", f)
	}
}

func TestParseFile_TenantOnly(t *testing.T) {
	data := `spec_version: "v1"
tenant:
  name: org1
  schema: payments
`
	f, err := ParseFile([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if f.Tenant.Name != "org1" || f.Tenant.Schema != "payments" {
		t.Errorf("tenant-only parsed incorrectly: %+v", f)
	}
}

func TestParseFile_LocksWithoutTenant(t *testing.T) {
	data := `spec_version: "v1"
locks:
  - field_path: a
`
	if _, err := ParseFile([]byte(data)); err == nil {
		t.Fatal("expected error for locks without tenant")
	}
}

func TestRun_SchemaAndTenant(t *testing.T) {
	file := &File{
		SpecVersion: "v1",
		Schema: SchemaDef{
			Name:   "payments",
			Fields: map[string]FieldDef{"rate": {Type: "number"}},
		},
		Tenant: TenantDef{Name: "org1"},
	}
	var importConfigCalled bool
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
			importConfigCalled = true
			return nil, nil
		},
	}
	result, err := Run(context.Background(), mock, file)
	if err != nil {
		t.Fatal(err)
	}
	if !result.SchemaCreated || !result.TenantCreated {
		t.Errorf("expected schema + tenant created, got %+v", result)
	}
	if importConfigCalled || result.ConfigImported {
		t.Error("ImportConfig should not be called when no config section")
	}
}

func TestRun_TenantOnly(t *testing.T) {
	file := &File{
		SpecVersion: "v1",
		Tenant:      TenantDef{Name: "org1", Schema: "payments"},
	}
	var importSchemaCalled, importConfigCalled bool
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			importSchemaCalled = true
			return nil, nil
		},
		getLatestPublishedSchemaVersionFn: func(_ context.Context, _ string) (string, int32, error) {
			return "s1", 2, nil
		},
		listSchemasFn: func(_ context.Context) ([]*adminclient.Schema, error) {
			return []*adminclient.Schema{{ID: "s1", Name: "payments"}}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return nil, nil
		},
		createTenantFn: func(_ context.Context, _, _ string, _ int32) (*adminclient.Tenant, error) {
			return &adminclient.Tenant{ID: "t1"}, nil
		},
		importConfigFn: func(_ context.Context, _ string, _ []byte, _ string, _ ...adminclient.ImportMode) (*adminclient.Version, error) {
			importConfigCalled = true
			return nil, nil
		},
	}
	result, err := Run(context.Background(), mock, file)
	if err != nil {
		t.Fatal(err)
	}
	if importSchemaCalled {
		t.Error("ImportSchema should not be called in tenant-only mode")
	}
	if !result.TenantCreated {
		t.Error("expected tenant created")
	}
	if importConfigCalled {
		t.Error("ImportConfig should not be called in tenant-only mode")
	}
}

// --- Mismatch error-message content + config-only + locks ---

func TestRun_ConfigOnly_SchemaMismatch_ErrorMessage(t *testing.T) {
	file := configOnlyFile(nil)
	mock := &mockClient{
		getLatestPublishedSchemaVersionFn: func(_ context.Context, _ string) (string, int32, error) {
			return "s_payments", 1, nil
		},
		listSchemasFn: func(_ context.Context) ([]*adminclient.Schema, error) {
			return []*adminclient.Schema{
				{ID: "s_payments", Name: "payments"},
				{ID: "s_billing", Name: "billing"},
			}, nil
		},
		listTenantsFn: func(_ context.Context, schemaID string) ([]*adminclient.Tenant, error) {
			if schemaID == "s_billing" {
				return []*adminclient.Tenant{{ID: "t_existing", Name: "org1"}}, nil
			}
			return nil, nil
		},
	}
	_, err := Run(context.Background(), mock, file)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"org1", "billing", "payments"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q missing %q", msg, want)
		}
	}
}

func TestRun_ConfigOnly_WithLocks(t *testing.T) {
	file := configOnlyFile(nil)
	file.Locks = []LockDef{
		{FieldPath: "rate", LockedValues: []string{"42"}},
	}
	var lockedTenantID, lockedPath string
	mock := &mockClient{
		getLatestPublishedSchemaVersionFn: func(_ context.Context, _ string) (string, int32, error) {
			return "s1", 1, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return []*adminclient.Tenant{{ID: "t1", Name: "org1"}}, nil
		},
		importConfigFn: func(_ context.Context, _ string, _ []byte, _ string, _ ...adminclient.ImportMode) (*adminclient.Version, error) {
			return &adminclient.Version{Version: 1}, nil
		},
		lockFieldFn: func(_ context.Context, tenantID, fieldPath string, _ ...string) error {
			lockedTenantID, lockedPath = tenantID, fieldPath
			return nil
		},
	}
	result, err := Run(context.Background(), mock, file)
	if err != nil {
		t.Fatal(err)
	}
	if lockedTenantID != "t1" || lockedPath != "rate" {
		t.Errorf("LockField called with (%q, %q); want (t1, rate)", lockedTenantID, lockedPath)
	}
	if result.LocksApplied != 1 {
		t.Errorf("LocksApplied = %d, want 1", result.LocksApplied)
	}
}

// --- Round-trip: TenantDef.SchemaVersion pointer ---

func TestMarshal_TenantSchemaVersionOmitEmpty(t *testing.T) {
	f := &File{
		SpecVersion: "v1",
		Tenant:      TenantDef{Name: "org1", Schema: "payments"},
		Config:      ConfigDef{Values: map[string]ConfigValueDef{"rate": {Value: 1}}},
	}
	data, err := Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "schema_version") {
		t.Errorf("nil SchemaVersion should not appear in YAML; got:\n%s", data)
	}
	f2, err := ParseFile(data)
	if err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
	if f2.Tenant.SchemaVersion != nil {
		t.Errorf("expected SchemaVersion nil after round-trip, got %d", *f2.Tenant.SchemaVersion)
	}
}

func TestMarshal_TenantSchemaVersionRoundTrip(t *testing.T) {
	v := int32(3)
	f := &File{
		SpecVersion: "v1",
		Tenant:      TenantDef{Name: "org1", Schema: "payments", SchemaVersion: &v},
		Config:      ConfigDef{Values: map[string]ConfigValueDef{"rate": {Value: 1}}},
	}
	data, err := Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := ParseFile(data)
	if err != nil {
		t.Fatal(err)
	}
	if f2.Tenant.SchemaVersion == nil || *f2.Tenant.SchemaVersion != 3 {
		t.Errorf("SchemaVersion round-trip lost value: got %v", f2.Tenant.SchemaVersion)
	}
}

func TestRun_ConfigOnly_ExplicitVersion_SchemaNotFound(t *testing.T) {
	v := int32(3)
	file := configOnlyFile(&v)
	mock := &mockClient{
		listSchemasFn: func(_ context.Context) ([]*adminclient.Schema, error) {
			return []*adminclient.Schema{{ID: "s1", Name: "other"}}, nil
		},
	}
	_, err := Run(context.Background(), mock, file)
	if err == nil {
		t.Fatal("expected error when schema name not found")
	}
}

func TestRun_ConfigOnly_ExplicitVersion_ListSchemasError(t *testing.T) {
	v := int32(3)
	file := configOnlyFile(&v)
	mock := &mockClient{
		listSchemasFn: func(_ context.Context) ([]*adminclient.Schema, error) {
			return nil, fmt.Errorf("rpc down")
		},
	}
	_, err := Run(context.Background(), mock, file)
	if err == nil || !strings.Contains(err.Error(), "rpc down") {
		t.Fatalf("expected rpc down error, got %v", err)
	}
}

func TestRun_ConfigOnly_LatestPublished_GenericError(t *testing.T) {
	file := configOnlyFile(nil)
	mock := &mockClient{
		getLatestPublishedSchemaVersionFn: func(_ context.Context, _ string) (string, int32, error) {
			return "", 0, fmt.Errorf("transport error")
		},
	}
	_, err := Run(context.Background(), mock, file)
	if err == nil || !strings.Contains(err.Error(), "transport error") {
		t.Fatalf("expected transport error, got %v", err)
	}
}

func TestRun_ConfigReseed_ListConfigVersionsError(t *testing.T) {
	file := testFile()
	file.Locks = nil
	mock := &mockClient{
		importSchemaFn: func(_ context.Context, _ []byte, _ ...bool) (*adminclient.Schema, error) {
			return &adminclient.Schema{ID: "s1", Version: 1}, nil
		},
		listTenantsFn: func(_ context.Context, _ string) ([]*adminclient.Tenant, error) {
			return []*adminclient.Tenant{{ID: "t1", Name: "test-tenant"}}, nil
		},
		importConfigFn: func(_ context.Context, _ string, _ []byte, _ string, _ ...adminclient.ImportMode) (*adminclient.Version, error) {
			return nil, adminclient.ErrAlreadyExists
		},
		listConfigVersionsFn: func(_ context.Context, _ string) ([]*adminclient.Version, error) {
			return nil, fmt.Errorf("db down")
		},
	}
	_, err := Run(context.Background(), mock, file)
	if err == nil || !strings.Contains(err.Error(), "db down") {
		t.Fatalf("expected db down error, got %v", err)
	}
}
