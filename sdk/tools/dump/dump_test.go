package dump

import (
	"context"
	"fmt"
	"testing"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/tools/seed"
)

// --- Mock client ---

type mockClient struct {
	getTenantFn        func(ctx context.Context, id string) (*adminclient.Tenant, error)
	getSchemaVersionFn func(ctx context.Context, id string, version int32) (*adminclient.Schema, error)
	exportConfigFn     func(ctx context.Context, tenantID string, version *int32) ([]byte, error)
	listFieldLocksFn   func(ctx context.Context, tenantID string) ([]adminclient.FieldLock, error)
}

func (m *mockClient) GetTenant(ctx context.Context, id string) (*adminclient.Tenant, error) {
	return m.getTenantFn(ctx, id)
}
func (m *mockClient) GetSchemaVersion(ctx context.Context, id string, version int32) (*adminclient.Schema, error) {
	return m.getSchemaVersionFn(ctx, id, version)
}
func (m *mockClient) ExportConfig(ctx context.Context, tenantID string, version *int32) ([]byte, error) {
	return m.exportConfigFn(ctx, tenantID, version)
}
func (m *mockClient) ListFieldLocks(ctx context.Context, tenantID string) ([]adminclient.FieldLock, error) {
	return m.listFieldLocksFn(ctx, tenantID)
}

// --- Test helpers ---

func baseMock() *mockClient {
	min := float64(0)
	return &mockClient{
		getTenantFn: func(_ context.Context, _ string) (*adminclient.Tenant, error) {
			return &adminclient.Tenant{
				ID: "t1", Name: "acme", SchemaID: "s1", SchemaVersion: 2,
			}, nil
		},
		getSchemaVersionFn: func(_ context.Context, _ string, _ int32) (*adminclient.Schema, error) {
			return &adminclient.Schema{
				ID: "s1", Name: "payments", Description: "Payment config", Version: 2,
				Fields: []adminclient.Field{
					{
						Path: "rate", Type: "FIELD_TYPE_NUMBER", Description: "Rate",
						Nullable: true,
						Constraints: &adminclient.FieldConstraints{Min: &min},
					},
					{Path: "name", Type: "FIELD_TYPE_STRING", Default: "default"},
				},
			}, nil
		},
		exportConfigFn: func(_ context.Context, _ string, _ *int32) ([]byte, error) {
			return []byte(`syntax: "v1"
values:
  rate:
    value: 42
  name:
    value: hello
`), nil
		},
		listFieldLocksFn: func(_ context.Context, _ string) ([]adminclient.FieldLock, error) {
			return []adminclient.FieldLock{
				{TenantID: "t1", FieldPath: "rate", LockedValues: []string{"0"}},
			}, nil
		},
	}
}

// --- Run tests ---

func TestRun_FullDump(t *testing.T) {
	mock := baseMock()

	file, err := Run(context.Background(), mock, "t1")
	if err != nil {
		t.Fatal(err)
	}

	if file.Syntax != "v1" {
		t.Errorf("syntax = %q", file.Syntax)
	}
	if file.Schema.Name != "payments" {
		t.Errorf("schema.name = %q", file.Schema.Name)
	}
	if file.Schema.Description != "Payment config" {
		t.Errorf("schema.description = %q", file.Schema.Description)
	}
	if len(file.Schema.Fields) != 2 {
		t.Errorf("schema fields = %d", len(file.Schema.Fields))
	}
	if file.Tenant.Name != "acme" {
		t.Errorf("tenant.name = %q", file.Tenant.Name)
	}
	if len(file.Config.Values) != 2 {
		t.Errorf("config values = %d", len(file.Config.Values))
	}
	if len(file.Locks) != 1 {
		t.Errorf("locks = %d", len(file.Locks))
	}
	if file.Locks[0].FieldPath != "rate" {
		t.Errorf("lock field = %q", file.Locks[0].FieldPath)
	}
	if len(file.Locks[0].LockedValues) != 1 || file.Locks[0].LockedValues[0] != "0" {
		t.Errorf("lock values = %v", file.Locks[0].LockedValues)
	}

	// Verify schema field details.
	rate := file.Schema.Fields["rate"]
	if rate.Type != "number" {
		t.Errorf("rate.type = %q", rate.Type)
	}
	if !rate.Nullable {
		t.Error("rate should be nullable")
	}
	if rate.Constraints == nil || rate.Constraints.Minimum == nil {
		t.Error("rate constraints missing")
	}

	name := file.Schema.Fields["name"]
	if name.Type != "string" {
		t.Errorf("name.type = %q", name.Type)
	}
	if name.Default != "default" {
		t.Errorf("name.default = %q", name.Default)
	}
}

func TestRun_WithoutLocks(t *testing.T) {
	mock := baseMock()

	file, err := Run(context.Background(), mock, "t1", WithoutLocks())
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Locks) != 0 {
		t.Errorf("expected no locks, got %d", len(file.Locks))
	}
}

func TestRun_WithConfigVersion(t *testing.T) {
	mock := baseMock()
	var gotVersion *int32
	mock.exportConfigFn = func(_ context.Context, _ string, version *int32) ([]byte, error) {
		gotVersion = version
		return []byte(`syntax: "v1"
values:
  rate:
    value: 1
`), nil
	}

	_, err := Run(context.Background(), mock, "t1", WithConfigVersion(5))
	if err != nil {
		t.Fatal(err)
	}
	if gotVersion == nil || *gotVersion != 5 {
		t.Errorf("expected config version 5, got %v", gotVersion)
	}
}

func TestRun_GetTenantError(t *testing.T) {
	mock := baseMock()
	mock.getTenantFn = func(_ context.Context, _ string) (*adminclient.Tenant, error) {
		return nil, fmt.Errorf("not found")
	}

	_, err := Run(context.Background(), mock, "t1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_GetSchemaError(t *testing.T) {
	mock := baseMock()
	mock.getSchemaVersionFn = func(_ context.Context, _ string, _ int32) (*adminclient.Schema, error) {
		return nil, fmt.Errorf("not found")
	}

	_, err := Run(context.Background(), mock, "t1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_ExportConfigError(t *testing.T) {
	mock := baseMock()
	mock.exportConfigFn = func(_ context.Context, _ string, _ *int32) ([]byte, error) {
		return nil, fmt.Errorf("db error")
	}

	_, err := Run(context.Background(), mock, "t1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_InvalidConfigYAML(t *testing.T) {
	mock := baseMock()
	mock.exportConfigFn = func(_ context.Context, _ string, _ *int32) ([]byte, error) {
		return []byte("{{invalid"), nil
	}

	_, err := Run(context.Background(), mock, "t1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_ListLocksError(t *testing.T) {
	mock := baseMock()
	mock.listFieldLocksFn = func(_ context.Context, _ string) ([]adminclient.FieldLock, error) {
		return nil, fmt.Errorf("permission denied")
	}

	_, err := Run(context.Background(), mock, "t1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_RoundTrip(t *testing.T) {
	mock := baseMock()

	file, err := Run(context.Background(), mock, "t1")
	if err != nil {
		t.Fatal(err)
	}

	data, err := Marshal(file)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := seed.ParseFile(data)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v\nYAML:\n%s", err, data)
	}
	if parsed.Schema.Name != "payments" {
		t.Errorf("round-trip schema name = %q", parsed.Schema.Name)
	}
	if parsed.Tenant.Name != "acme" {
		t.Errorf("round-trip tenant name = %q", parsed.Tenant.Name)
	}
}

// --- Helper tests ---

func TestBuildSchemaDef(t *testing.T) {
	min := float64(0)
	max := float64(100)
	s := &adminclient.Schema{
		Name:        "test-schema",
		Description: "Test description",
		Version:     2,
		Fields: []adminclient.Field{
			{
				Path:        "rate",
				Type:        "FIELD_TYPE_NUMBER",
				Description: "A rate",
				Nullable:    true,
				Constraints: &adminclient.FieldConstraints{Min: &min, Max: &max},
			},
			{
				Path:       "name",
				Type:       "FIELD_TYPE_STRING",
				Default:    "default",
				Deprecated: true,
				RedirectTo: "new_name",
			},
		},
	}

	def := buildSchemaDef(s)

	if def.Name != "test-schema" {
		t.Errorf("name = %q", def.Name)
	}
	if len(def.Fields) != 2 {
		t.Fatalf("fields count = %d", len(def.Fields))
	}

	rate := def.Fields["rate"]
	if rate.Type != "number" {
		t.Errorf("rate.type = %q", rate.Type)
	}
	if rate.Constraints == nil || rate.Constraints.Minimum == nil || *rate.Constraints.Minimum != 0 {
		t.Errorf("rate.constraints.minimum = %v", rate.Constraints)
	}

	name := def.Fields["name"]
	if name.Type != "string" || !name.Deprecated || name.RedirectTo != "new_name" {
		t.Errorf("name = %+v", name)
	}
}

func TestFieldTypeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"FIELD_TYPE_STRING", "string"},
		{"FIELD_TYPE_INT", "integer"},
		{"FIELD_TYPE_NUMBER", "number"},
		{"FIELD_TYPE_BOOL", "bool"},
		{"FIELD_TYPE_TIME", "time"},
		{"FIELD_TYPE_DURATION", "duration"},
		{"FIELD_TYPE_URL", "url"},
		{"FIELD_TYPE_JSON", "json"},
		{"UNKNOWN", "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := fieldTypeName(tt.input); got != tt.want {
			t.Errorf("fieldTypeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestConvertConstraints(t *testing.T) {
	min := float64(1)
	max := float64(10)
	exMin := float64(0)
	exMax := float64(11)
	minLen := int32(2)
	maxLen := int32(50)

	c := &adminclient.FieldConstraints{
		Min:          &min,
		Max:          &max,
		ExclusiveMin: &exMin,
		ExclusiveMax: &exMax,
		MinLength:    &minLen,
		MaxLength:    &maxLen,
		Pattern:      "^[a-z]+$",
		Enum:         []string{"a", "b"},
		JSONSchema:   `{"type":"object"}`,
	}

	cd := convertConstraints(c)
	if *cd.Minimum != 1 || *cd.Maximum != 10 {
		t.Errorf("min/max = %v/%v", cd.Minimum, cd.Maximum)
	}
	if *cd.ExclusiveMinimum != 0 || *cd.ExclusiveMaximum != 11 {
		t.Errorf("exMin/exMax = %v/%v", cd.ExclusiveMinimum, cd.ExclusiveMaximum)
	}
	if *cd.MinLength != 2 || *cd.MaxLength != 50 {
		t.Errorf("minLen/maxLen = %v/%v", cd.MinLength, cd.MaxLength)
	}
	if cd.Pattern != "^[a-z]+$" {
		t.Errorf("pattern = %q", cd.Pattern)
	}
	if len(cd.Enum) != 2 {
		t.Errorf("enum = %v", cd.Enum)
	}
	if cd.JSONSchema != `{"type":"object"}` {
		t.Errorf("jsonSchema = %q", cd.JSONSchema)
	}
}

func TestMarshal(t *testing.T) {
	f := &seed.File{
		Syntax: "v1",
		Schema: seed.SchemaDef{
			Name:   "test",
			Fields: map[string]seed.FieldDef{"x": {Type: "string"}},
		},
		Tenant: seed.TenantDef{Name: "t"},
	}

	data, err := Marshal(f)
	if err != nil {
		t.Fatal(err)
	}

	f2, err := seed.ParseFile(data)
	if err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}
	if f2.Schema.Name != "test" || f2.Tenant.Name != "t" {
		t.Error("round-trip mismatch")
	}
}
