package loader_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
	"github.com/opendecree/decree/contrib/decree-docs/loader"
	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/tools/validate"
)

// The server loader depends on the admin client through this seam.
var _ loader.SchemaClient = (*adminclient.Client)(nil)

func ptrTo[T any](v T) *T { return &v }

// --- Shared fixture ---
//
// equivalenceYAML and equivalenceAdminSchema describe the same schema
// content through the two sources. The webhook field carries every field
// property and every constraint so the drift and mapping tests can verify
// the complete surface; the fee field stays minimal. The admin schema lists
// fields in reverse path order and sets every server-side bookkeeping
// property to prove neither leaks into the model.

const equivalenceYAML = `spec_version: v1
name: payments
description: Payment configuration
version: 3
version_description: Added refund_window field
info:
  title: Payments Configuration
  author: platform-team
  contact:
    name: Pat
    email: pat@example.com
    url: https://example.com/payments-team
  labels:
    team: platform
fields:
  payments.webhook:
    type: url
    title: Webhook URL
    description: Webhook endpoint
    default: "https://example.com/hook"
    nullable: true
    deprecated: true
    redirect_to: payments.callback_url
    example: "https://example.com/hook-example"
    examples:
      primary:
        value: "https://hooks.example.com/a"
        summary: Primary endpoint
    externalDocs:
      description: Webhook guide
      url: https://docs.example.com/webhooks
    tags: [billing, integration]
    format: uri
    readOnly: true
    writeOnce: true
    sensitive: true
    constraints:
      minimum: 1
      maximum: 9
      exclusiveMinimum: 0.5
      exclusiveMaximum: 9.5
      minLength: 2
      maxLength: 64
      pattern: "^https://"
      enum: ["https://example.com/hook", "https://b.example.com"]
      json_schema: '{"type":"string"}'
      allowed_schemes: [https, sftp]
  payments.fee:
    type: number
`

func equivalenceAdminSchema() *adminclient.Schema {
	return &adminclient.Schema{
		// Server bookkeeping the doc model excludes.
		ID:            "schema-uuid",
		ParentVersion: ptrTo(int32(2)),
		Checksum:      "abc123",
		Published:     true,
		CreatedAt:     time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),

		Name:               "payments",
		Description:        "Payment configuration",
		Version:            3,
		VersionDescription: "Added refund_window field",
		Info: &adminclient.SchemaInfo{
			Title:  "Payments Configuration",
			Author: "platform-team",
			Contact: &adminclient.SchemaContact{
				Name:  "Pat",
				Email: "pat@example.com",
				URL:   "https://example.com/payments-team",
			},
			Labels: map[string]string{"team": "platform"},
		},
		// Reverse path order: the loader must sort.
		Fields: []adminclient.Field{
			{
				Path:        "payments.webhook",
				Type:        adminclient.FieldTypeURL,
				Title:       "Webhook URL",
				Description: "Webhook endpoint",
				Default:     "https://example.com/hook",
				Nullable:    true,
				Deprecated:  true,
				RedirectTo:  "payments.callback_url",
				Example:     "https://example.com/hook-example",
				Examples: map[string]adminclient.FieldExample{
					"primary": {Value: "https://hooks.example.com/a", Summary: "Primary endpoint"},
				},
				ExternalDocs: &adminclient.ExternalDocs{
					Description: "Webhook guide",
					URL:         "https://docs.example.com/webhooks",
				},
				Tags:      []string{"billing", "integration"},
				Format:    "uri",
				ReadOnly:  true,
				WriteOnce: true,
				Sensitive: true,
				Constraints: &adminclient.FieldConstraints{
					Min:            ptrTo(1.0),
					Max:            ptrTo(9.0),
					ExclusiveMin:   ptrTo(0.5),
					ExclusiveMax:   ptrTo(9.5),
					MinLength:      ptrTo(int32(2)),
					MaxLength:      ptrTo(int32(64)),
					Pattern:        "^https://",
					Enum:           []string{"https://example.com/hook", "https://b.example.com"},
					JSONSchema:     `{"type":"string"}`,
					AllowedSchemes: []string{"https", "sftp"},
				},
			},
			{Path: "payments.fee", Type: adminclient.FieldTypeNumber},
		},
	}
}

// expectedDocument is the doc model both loaders must produce from the
// fixture above.
func expectedDocument() *docmodel.Document {
	return &docmodel.Document{
		DocModelVersion: docmodel.Version,
		Schema: docmodel.Schema{
			Name:               "payments",
			Description:        "Payment configuration",
			Version:            3,
			VersionDescription: "Added refund_window field",
			Info: &docmodel.Info{
				Title:  "Payments Configuration",
				Author: "platform-team",
				Contact: &docmodel.Contact{
					Name:  "Pat",
					Email: "pat@example.com",
					URL:   "https://example.com/payments-team",
				},
				Labels: map[string]string{"team": "platform"},
			},
			Fields: []docmodel.Field{
				{Path: "payments.fee", Type: "number"},
				{
					Path:        "payments.webhook",
					Type:        "url",
					Title:       "Webhook URL",
					Description: "Webhook endpoint",
					Default:     "https://example.com/hook",
					Nullable:    true,
					Deprecated:  true,
					RedirectTo:  "payments.callback_url",
					Example:     "https://example.com/hook-example",
					Examples: map[string]docmodel.Example{
						"primary": {Value: "https://hooks.example.com/a", Summary: "Primary endpoint"},
					},
					ExternalDocs: &docmodel.ExternalDocs{
						Description: "Webhook guide",
						URL:         "https://docs.example.com/webhooks",
					},
					Tags:      []string{"billing", "integration"},
					Format:    "uri",
					ReadOnly:  true,
					WriteOnce: true,
					Sensitive: true,
					Constraints: &docmodel.Constraints{
						Minimum:          ptrTo(1.0),
						Maximum:          ptrTo(9.0),
						ExclusiveMinimum: ptrTo(0.5),
						ExclusiveMaximum: ptrTo(9.5),
						MinLength:        ptrTo(int32(2)),
						MaxLength:        ptrTo(int32(64)),
						Pattern:          "^https://",
						Enum:             []string{"https://example.com/hook", "https://b.example.com"},
						JSONSchema:       `{"type":"string"}`,
						AllowedSchemes:   []string{"https", "sftp"},
					},
				},
			},
		},
	}
}

// --- File loader ---

func TestFromYAML_Full(t *testing.T) {
	got, err := loader.FromYAML([]byte(equivalenceYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := expectedDocument(); !reflect.DeepEqual(got, want) {
		t.Errorf("FromYAML mismatch:\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestFromYAML_Invalid(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{"malformed", "not: [valid"},
		{"missing spec_version", "name: test\nfields:\n  x:\n    type: string\n"},
		{"no fields", "spec_version: v1\nname: test\n"},
		{"unknown type", "spec_version: v1\nname: test\nfields:\n  x:\n    type: blob\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := loader.FromYAML([]byte(tt.yaml)); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.yaml")
	if err := os.WriteFile(path, []byte(equivalenceYAML), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	got, err := loader.FromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := expectedDocument(); !reflect.DeepEqual(got, want) {
		t.Errorf("FromFile mismatch:\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestFromFile_NotFound(t *testing.T) {
	_, err := loader.FromFile(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFromFile_InvalidContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.yaml")
	if err := os.WriteFile(path, []byte("not: [valid"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := loader.FromFile(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("expected error to name the file, got %q", err)
	}
}

// --- Server loader ---

// fakeSchemaTransport implements GetSchema and records the requested
// version. The embedded nil interface panics on any other method, keeping
// the fake honest about what the loader uses.
type fakeSchemaTransport struct {
	adminclient.SchemaTransport
	schema *adminclient.Schema
	err    error

	gotID      string
	gotVersion *int32
}

func (f *fakeSchemaTransport) GetSchema(_ context.Context, id string, version *int32) (*adminclient.Schema, error) {
	f.gotID = id
	f.gotVersion = version
	return f.schema, f.err
}

func TestFromServer(t *testing.T) {
	fake := &fakeSchemaTransport{schema: equivalenceAdminSchema()}
	client := adminclient.New(adminclient.WithSchemaTransport(fake))

	got, err := loader.FromServer(context.Background(), client, "schema-uuid", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := expectedDocument(); !reflect.DeepEqual(got, want) {
		t.Errorf("FromServer mismatch:\ngot:  %+v\nwant: %+v", got, want)
	}
	if fake.gotID != "schema-uuid" {
		t.Errorf("got schema id %q, want %q", fake.gotID, "schema-uuid")
	}
	if fake.gotVersion != nil {
		t.Errorf("expected latest version (nil), got %v", *fake.gotVersion)
	}
}

func TestFromServer_SpecificVersion(t *testing.T) {
	fake := &fakeSchemaTransport{schema: equivalenceAdminSchema()}
	client := adminclient.New(adminclient.WithSchemaTransport(fake))

	if _, err := loader.FromServer(context.Background(), client, "schema-uuid", 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.gotVersion == nil || *fake.gotVersion != 3 {
		t.Errorf("got version %v, want 3", fake.gotVersion)
	}
}

func TestFromServer_Error(t *testing.T) {
	fake := &fakeSchemaTransport{err: errors.New("boom")}
	client := adminclient.New(adminclient.WithSchemaTransport(fake))

	_, err := loader.FromServer(context.Background(), client, "schema-uuid", 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "schema-uuid") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected error to name the schema and cause, got %q", err)
	}
}

// --- Loader equivalence ---

// TestLoaderEquivalence asserts the acceptance criterion of #914: the same
// schema content produces a deep-equal doc model whether it is read from a
// YAML file or fetched from a server via the admin client (with the admin
// side listing fields in a different order and carrying server bookkeeping).
func TestLoaderEquivalence(t *testing.T) {
	fromFile, err := loader.FromYAML([]byte(equivalenceYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fake := &fakeSchemaTransport{schema: equivalenceAdminSchema()}
	client := adminclient.New(adminclient.WithSchemaTransport(fake))
	fromServer, err := loader.FromServer(context.Background(), client, "schema-uuid", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(fromFile, fromServer) {
		t.Errorf("file and server loaders disagree on the same schema content:\nfile:   %+v\nserver: %+v", fromFile, fromServer)
	}
}

// --- Property drift ---

// TestFromAdminSchema_PropertyDrift reflects over every exported property of
// the adminclient schema types and asserts each one is either mapped into
// the doc model or explicitly recorded as non-documentation metadata. A
// property added to adminclient fails this test until it is populated in
// equivalenceAdminSchema and either mapped or recorded.
func TestFromAdminSchema_PropertyDrift(t *testing.T) {
	in := equivalenceAdminSchema()
	out := mapDenseAdminSchema(t, in)

	// The dense webhook field sorts after fee.
	inField := in.Fields[0]
	outField := out.Schema.Fields[1]

	tests := []driftCase{
		{
			name:  "Schema",
			src:   *in,
			model: out.Schema,
			// Server bookkeeping that schema YAML cannot express; documenting
			// it would break file/server model equivalence.
			notDocumented: []string{"ID", "ParentVersion", "Checksum", "Published", "CreatedAt"},
		},
		{name: "Field", src: inField, model: outField},
		{
			name:  "FieldConstraints",
			src:   *inField.Constraints,
			model: *outField.Constraints,
			// adminclient abbreviates; the doc model uses the OAS-style names
			// of the schema YAML format.
			renames: map[string]string{
				"Min":          "Minimum",
				"Max":          "Maximum",
				"ExclusiveMin": "ExclusiveMinimum",
				"ExclusiveMax": "ExclusiveMaximum",
			},
		},
		{name: "SchemaInfo", src: *in.Info, model: *out.Schema.Info},
		{name: "SchemaContact", src: *in.Info.Contact, model: *out.Schema.Info.Contact},
		{name: "FieldExample", src: inField.Examples["primary"], model: outField.Examples["primary"]},
		{name: "ExternalDocs", src: *inField.ExternalDocs, model: *outField.ExternalDocs},
	}
	runDriftCases(t, "adminclient", tests)
}

// mapDenseAdminSchema maps the dense fixture and sanity-checks the shape
// the drift cases index into.
func mapDenseAdminSchema(t *testing.T, in *adminclient.Schema) *docmodel.Document {
	t.Helper()
	out := loader.FromAdminSchema(in)
	if len(out.Schema.Fields) != 2 || out.Schema.Fields[1].Path != "payments.webhook" {
		t.Fatalf("unexpected mapped fields: %+v", out.Schema.Fields)
	}
	return out
}

// TestFromYAML_PropertyDrift is the file-side twin: every property the
// schema YAML format can express (per sdk/tools/validate) must be mapped
// into the doc model or recorded as a file-format marker.
func TestFromYAML_PropertyDrift(t *testing.T) {
	sf, err := validate.ParseSchema([]byte(equivalenceYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := loader.FromYAML([]byte(equivalenceYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	inField := sf.Fields["payments.webhook"]
	outField := out.Schema.Fields[1]
	if outField.Path != "payments.webhook" {
		t.Fatalf("unexpected mapped fields: %+v", out.Schema.Fields)
	}

	tests := []driftCase{
		{
			name:  "SchemaFile",
			src:   *sf,
			model: out.Schema,
			// File-format markers, not documentation content: SpecVersion is
			// the format version; Schema ($schema) and ID ($id) are JSON
			// Schema editor hints the server does not store.
			notDocumented: []string{"SpecVersion", "Schema", "ID"},
		},
		{name: "FieldDef", src: inField, model: outField},
		{name: "ConstraintsDef", src: *inField.Constraints, model: *outField.Constraints},
		{name: "SchemaInfoDef", src: *sf.Info, model: *out.Schema.Info},
		{name: "SchemaContactDef", src: *sf.Info.Contact, model: *out.Schema.Info.Contact},
		{name: "ExampleDef", src: inField.Examples["primary"], model: outField.Examples["primary"]},
		{name: "ExternalDocsDef", src: *inField.ExternalDocs, model: *outField.ExternalDocs},
	}
	runDriftCases(t, "validate", tests)
}

type driftCase struct {
	name  string
	src   any
	model any
	// notDocumented lists source properties that deliberately have no doc
	// model counterpart.
	notDocumented []string
	// renames maps source property names to doc model property names where
	// they differ.
	renames map[string]string
}

func runDriftCases(t *testing.T, srcPkg string, tests []driftCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := reflect.ValueOf(tt.src)
			mv := reflect.ValueOf(tt.model)
			skip := make(map[string]bool, len(tt.notDocumented))
			for _, name := range tt.notDocumented {
				if _, ok := sv.Type().FieldByName(name); !ok {
					t.Errorf("stale notDocumented entry %q: %s.%s has no such property", name, srcPkg, sv.Type().Name())
				}
				skip[name] = true
			}
			for i := 0; i < sv.NumField(); i++ {
				name := sv.Type().Field(i).Name
				if skip[name] {
					continue
				}
				if sv.Field(i).IsZero() {
					t.Errorf("%s.%s.%s is zero in the dense fixture — populate it so this test can verify the mapping", srcPkg, sv.Type().Name(), name)
					continue
				}
				modelName := name
				if renamed, ok := tt.renames[name]; ok {
					modelName = renamed
				}
				mf, ok := mv.Type().FieldByName(modelName)
				if !ok {
					t.Errorf("%s.%s.%s has no doc model counterpart — map it in the loader or record it in notDocumented", srcPkg, sv.Type().Name(), name)
					continue
				}
				if mv.FieldByIndex(mf.Index).IsZero() {
					t.Errorf("%s.%s.%s is dropped by the loader", srcPkg, sv.Type().Name(), name)
				}
			}
		})
	}
}

// --- Canonicalization ---

// TestFromYAML_CanonicalEmpty asserts that explicitly empty optional blocks
// in the YAML load identically to absent ones.
func TestFromYAML_CanonicalEmpty(t *testing.T) {
	const yamlWithEmpties = `spec_version: v1
name: empties
info:
  contact: {}
  labels: {}
fields:
  app.name:
    type: string
    tags: []
    examples: {}
    constraints: {}
`
	got, err := loader.FromYAML([]byte(yamlWithEmpties))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := &docmodel.Document{
		DocModelVersion: docmodel.Version,
		Schema: docmodel.Schema{
			Name:   "empties",
			Fields: []docmodel.Field{{Path: "app.name", Type: "string"}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("empty blocks were not canonicalized:\ngot:  %+v\nwant: %+v", got, want)
	}
}

// TestFromAdminSchema_CanonicalEmpty asserts that empty-but-non-nil
// collections and blocks from the server load identically to absent ones.
func TestFromAdminSchema_CanonicalEmpty(t *testing.T) {
	got := loader.FromAdminSchema(&adminclient.Schema{
		Name: "empties",
		Info: &adminclient.SchemaInfo{
			Contact: &adminclient.SchemaContact{},
			Labels:  map[string]string{},
		},
		Fields: []adminclient.Field{
			{
				Path:         "app.name",
				Type:         adminclient.FieldTypeString,
				Tags:         []string{},
				Examples:     map[string]adminclient.FieldExample{},
				ExternalDocs: &adminclient.ExternalDocs{},
				Constraints:  &adminclient.FieldConstraints{Enum: []string{}, AllowedSchemes: []string{}},
			},
		},
	})
	want := &docmodel.Document{
		DocModelVersion: docmodel.Version,
		Schema: docmodel.Schema{
			Name:   "empties",
			Fields: []docmodel.Field{{Path: "app.name", Type: "string"}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("empty blocks were not canonicalized:\ngot:  %+v\nwant: %+v", got, want)
	}
}

// TestLoaders_NoInfo asserts a schema without info metadata maps to a nil
// Info on both paths.
func TestLoaders_NoInfo(t *testing.T) {
	fromFile, err := loader.FromYAML([]byte("spec_version: v1\nname: bare\nfields:\n  x:\n    type: string\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fromServer := loader.FromAdminSchema(&adminclient.Schema{
		Name:   "bare",
		Fields: []adminclient.Field{{Path: "x", Type: adminclient.FieldTypeString}},
	})
	want := &docmodel.Document{
		DocModelVersion: docmodel.Version,
		Schema: docmodel.Schema{
			Name:   "bare",
			Fields: []docmodel.Field{{Path: "x", Type: "string"}},
		},
	}
	if !reflect.DeepEqual(fromFile, want) {
		t.Errorf("FromYAML mismatch:\ngot:  %+v\nwant: %+v", fromFile, want)
	}
	if !reflect.DeepEqual(fromServer, want) {
		t.Errorf("FromAdminSchema mismatch:\ngot:  %+v\nwant: %+v", fromServer, want)
	}
}

// TestLoaders_DoNotAliasSource asserts the model owns its collections: the
// admin schema can be mutated after loading without changing the document.
func TestLoaders_DoNotAliasSource(t *testing.T) {
	in := equivalenceAdminSchema()
	got := loader.FromAdminSchema(in)

	in.Info.Labels["team"] = "mutated"
	in.Fields[0].Tags[0] = "mutated"
	in.Fields[0].Constraints.Enum[0] = "mutated"
	in.Fields[0].Constraints.AllowedSchemes[0] = "mutated"

	if want := expectedDocument(); !reflect.DeepEqual(got, want) {
		t.Errorf("mutating the source changed the model:\ngot:  %+v\nwant: %+v", got, want)
	}
}
