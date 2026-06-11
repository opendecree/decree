package main

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/tools/docgen"
)

// --- typedValueDisplay ---

func TestTypedValueDisplay(t *testing.T) {
	tests := []struct {
		name     string
		input    *configclient.TypedValue
		expected string
	}{
		{"nil", nil, "<null>"},
		{"string", configclient.StringVal("hello"), "hello"},
		{"integer", configclient.IntVal(42), "42"},
		{"number", configclient.FloatVal(3.14), "3.14"},
		{"bool", configclient.BoolVal(true), "true"},
		{"url", configclient.URLVal("https://example.com"), "https://example.com"},
		{"json", configclient.JSONVal(`{"a":1}`), `{"a":1}`},
		{"duration", configclient.DurationVal(5 * time.Minute), "5m0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := typedValueDisplay(tt.input); got != tt.expected {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTypedValueDisplay_Time(t *testing.T) {
	ts := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	tv := configclient.TimeVal(ts)
	if !strings.Contains(typedValueDisplay(tv), "2026-03-30") {
		t.Errorf("expected %q to contain %q", typedValueDisplay(tv), "2026-03-30")
	}
}

// --- printOutput (exercises the flagOutput switch) ---

func TestPrintOutput_JSON(t *testing.T) {
	orig := flagOutput
	t.Cleanup(func() { flagOutput = orig })
	flagOutput = "json"

	// Redirect stdout.
	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	err := printOutput(map[string]string{"k": "v"})
	w.Close()

	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), `"k"`) {
		t.Errorf("expected JSON output, got: %q", buf.String())
	}
}

func TestPrintOutput_YAML(t *testing.T) {
	orig := flagOutput
	t.Cleanup(func() { flagOutput = orig })
	flagOutput = "yaml"

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	err := printOutput(map[string]string{"k": "v"})
	w.Close()

	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "k:") {
		t.Errorf("expected YAML output, got: %q", buf.String())
	}
}

func TestPrintOutput_Table(t *testing.T) {
	orig := flagOutput
	t.Cleanup(func() { flagOutput = orig })
	flagOutput = "table"

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	err := printOutput([][]string{{"HEADER"}, {"row"}})
	w.Close()

	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "HEADER") {
		t.Errorf("expected table output, got: %q", buf.String())
	}
}

// --- printTable edge cases ---

func TestPrintTable_NonTableData(t *testing.T) {
	var buf bytes.Buffer
	err := printTable(&buf, map[string]string{"key": "val"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "key") {
		t.Errorf("expected %q to contain %q", buf.String(), "key")
	}
}

func TestPrintTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := printTable(&buf, [][]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buf.String()) != 0 {
		t.Errorf("expected empty, got %v", buf.String())
	}
}

// --- versionOrEmpty ---

func TestVersionOrEmpty(t *testing.T) {
	if got := versionOrEmpty(0); got != "" {
		t.Errorf("got %v, want %v", got, "")
	}
	if got := versionOrEmpty(1); got != "v1" {
		t.Errorf("got %v, want %v", got, "v1")
	}
	if got := versionOrEmpty(42); got != "v42" {
		t.Errorf("got %v, want %v", got, "v42")
	}
}

// --- parseConfigValues ---

func TestParseConfigValues(t *testing.T) {
	yaml := `spec_version: v1
values:
  app.name:
    value: MyApp
  app.retries:
    value: 3
  app.enabled:
    value: true
`
	m, err := parseConfigValues([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil")
	}
	if got := m["app.name"]; got != "MyApp" {
		t.Errorf("got %v, want %v", got, "MyApp")
	}
	if got := m["app.retries"]; got != "3" {
		t.Errorf("got %v, want %v", got, "3")
	}
	if got := m["app.enabled"]; got != "true" {
		t.Errorf("got %v, want %v", got, "true")
	}
}

func TestParseConfigValues_Invalid(t *testing.T) {
	_, err := parseConfigValues([]byte("not: [valid: yaml"))
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestParseConfigValues_Empty(t *testing.T) {
	m, err := parseConfigValues([]byte("spec_version: v1\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil")
	}
	if len(m) != 0 {
		t.Errorf("expected empty, got %v", m)
	}
}

// --- adminSchemaToDocgen ---

func TestAdminSchemaToDocgen(t *testing.T) {
	min := 0.0
	max := 10.0
	s := &adminclient.Schema{
		Name:        "payments",
		Description: "test",
		Version:     2,
		Fields: []adminclient.Field{
			{
				Path:        "app.retries",
				Type:        "FIELD_TYPE_INT",
				Description: "retry count",
				Default:     "3",
				Nullable:    true,
				Deprecated:  true,
				RedirectTo:  "app.max_retries",
				Constraints: &adminclient.FieldConstraints{
					Min:  &min,
					Max:  &max,
					Enum: []string{"1", "2", "3"},
				},
			},
			{Path: "app.name", Type: "FIELD_TYPE_STRING"},
		},
	}

	ds := adminSchemaToDocgen(s)
	if got := ds.Name; got != "payments" {
		t.Errorf("got %v, want %v", got, "payments")
	}
	if got := ds.Description; got != "test" {
		t.Errorf("got %v, want %v", got, "test")
	}
	if got := ds.Version; got != int32(2) {
		t.Errorf("got %v, want %v", got, int32(2))
	}
	if len(ds.Fields) != 2 {
		t.Fatalf("got len %d, want %d", len(ds.Fields), 2)
	}

	f := ds.Fields[0]
	if got := f.Path; got != "app.retries" {
		t.Errorf("got %v, want %v", got, "app.retries")
	}
	if got := f.Description; got != "retry count" {
		t.Errorf("got %v, want %v", got, "retry count")
	}
	if got := f.Default; got != "3" {
		t.Errorf("got %v, want %v", got, "3")
	}
	if !f.Nullable {
		t.Error("expected Nullable to be true")
	}
	if !f.Deprecated {
		t.Error("expected Deprecated to be true")
	}
	if got := f.RedirectTo; got != "app.max_retries" {
		t.Errorf("got %v, want %v", got, "app.max_retries")
	}
	if f.Constraints == nil {
		t.Fatal("expected non-nil Constraints")
	}
	if got := f.Constraints.Min; !reflect.DeepEqual(got, &min) {
		t.Errorf("got %v, want %v", got, &min)
	}
	if got := f.Constraints.Enum; !reflect.DeepEqual(got, []string{"1", "2", "3"}) {
		t.Errorf("got %v, want %v", got, []string{"1", "2", "3"})
	}
}

func TestAdminSchemaToDocgen_NoConstraints(t *testing.T) {
	s := &adminclient.Schema{
		Name:   "test",
		Fields: []adminclient.Field{{Path: "x", Type: "STRING"}},
	}
	ds := adminSchemaToDocgen(s)
	if ds.Fields[0].Constraints != nil {
		t.Errorf("expected nil, got %v", ds.Fields[0].Constraints)
	}
}

// --- schemaFromYAML ---

func TestSchemaFromYAML(t *testing.T) {
	yaml := `spec_version: v1
name: payments
description: Payment config
version: 1
fields:
  app.fee:
    type: number
    description: Fee rate
    default: "0.01"
    nullable: true
    constraints:
      minimum: 0
      maximum: 1
      enum: ["0.01", "0.05"]
  app.name:
    type: string
`
	s, err := schemaFromYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := s.Name; got != "payments" {
		t.Errorf("got %v, want %v", got, "payments")
	}
	if got := s.Description; got != "Payment config" {
		t.Errorf("got %v, want %v", got, "Payment config")
	}
	if len(s.Fields) != 2 {
		t.Fatalf("got len %d, want %d", len(s.Fields), 2)
	}

	// Find the fee field.
	var fee *docgen.Field
	for i := range s.Fields {
		if s.Fields[i].Path == "app.fee" {
			fee = &s.Fields[i]
			break
		}
	}
	if fee == nil {
		t.Fatal("expected non-nil fee field")
	}
	if got := fee.Type; got != "number" {
		t.Errorf("got %v, want %v", got, "number")
	}
	if !fee.Nullable {
		t.Error("expected Nullable to be true")
	}
	if fee.Constraints == nil {
		t.Fatal("expected non-nil Constraints")
	}
	if got := *fee.Constraints.Min; got != 0.0 {
		t.Errorf("got %v, want %v", got, 0.0)
	}
	if got := *fee.Constraints.Max; got != 1.0 {
		t.Errorf("got %v, want %v", got, 1.0)
	}
}

const metadataSchemaYAML = `spec_version: v1
name: payments
version: 3
version_description: Added refund_window field
info:
  title: Payments Configuration
  author: platform-team
  contact:
    name: Pat
    email: pat@example.com
  labels:
    team: platform
fields:
  payments.fee:
    type: number
    examples:
      low:
        value: "0.01"
        summary: Low rate
      high:
        value: "0.99"
    externalDocs:
      description: Fee guide
      url: https://docs.example.com/fees
  payments.webhook:
    type: url
    constraints:
      allowed_schemes: [https, sftp]
`

func TestSchemaFromYAML_Metadata(t *testing.T) {
	s, err := schemaFromYAML([]byte(metadataSchemaYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := s.VersionDescription; got != "Added refund_window field" {
		t.Errorf("got %v, want %v", got, "Added refund_window field")
	}
	if s.Info == nil {
		t.Fatal("expected non-nil Info")
	}
	want := &docgen.SchemaInfo{
		Title:   "Payments Configuration",
		Author:  "platform-team",
		Contact: &docgen.SchemaContact{Name: "Pat", Email: "pat@example.com"},
		Labels:  map[string]string{"team": "platform"},
	}
	if !reflect.DeepEqual(s.Info, want) {
		t.Errorf("got %+v, want %+v", s.Info, want)
	}

	var fee, webhook *docgen.Field
	for i := range s.Fields {
		switch s.Fields[i].Path {
		case "payments.fee":
			fee = &s.Fields[i]
		case "payments.webhook":
			webhook = &s.Fields[i]
		}
	}
	if fee == nil || webhook == nil {
		t.Fatal("expected both fields to be mapped")
	}
	wantExamples := map[string]docgen.FieldExample{
		"low":  {Value: "0.01", Summary: "Low rate"},
		"high": {Value: "0.99"},
	}
	if !reflect.DeepEqual(fee.Examples, wantExamples) {
		t.Errorf("got %+v, want %+v", fee.Examples, wantExamples)
	}
	wantDocs := &docgen.ExternalDocs{Description: "Fee guide", URL: "https://docs.example.com/fees"}
	if !reflect.DeepEqual(fee.ExternalDocs, wantDocs) {
		t.Errorf("got %+v, want %+v", fee.ExternalDocs, wantDocs)
	}
	if webhook.Constraints == nil || !reflect.DeepEqual(webhook.Constraints.AllowedSchemes, []string{"https", "sftp"}) {
		t.Errorf("unexpected constraints: %+v", webhook.Constraints)
	}
}

func TestSchemaFromYAML_InfoWithoutContact(t *testing.T) {
	yaml := `spec_version: v1
name: test
info:
  title: Test
fields:
  x:
    type: string
`
	s, err := schemaFromYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Info == nil || s.Info.Title != "Test" {
		t.Fatalf("unexpected Info: %+v", s.Info)
	}
	if s.Info.Contact != nil {
		t.Errorf("expected nil Contact, got %+v", s.Info.Contact)
	}
}

func TestSchemaFromYAML_MetadataRenders(t *testing.T) {
	s, err := schemaFromYAML([]byte(metadataSchemaYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	md := docgen.Generate(*s)
	for _, substr := range []string{
		"# Payments Configuration",
		"**Version:** 3 — Added refund_window field",
		"**Author:** platform-team",
		"**Contact:** Pat <pat@example.com>",
		"`team: platform`",
		"- **low:** `0.01` — Low rate",
		"- **high:** `0.99`",
		"**See also:** [Fee guide](https://docs.example.com/fees)",
		"- Allowed schemes: https, sftp",
	} {
		if !strings.Contains(md, substr) {
			t.Errorf("expected output to contain %q, got:\n%s", substr, md)
		}
	}
}

func TestSchemaFromYAML_Invalid(t *testing.T) {
	_, err := schemaFromYAML([]byte("not: [valid"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSchemaFromYAML_MissingSpecVersion(t *testing.T) {
	_, err := schemaFromYAML([]byte("name: test\nfields:\n  x:\n    type: string\n"))
	if err == nil {
		t.Fatal("expected error for missing spec_version, got nil")
	}
}

func TestSchemaFromYAML_NoFields(t *testing.T) {
	_, err := schemaFromYAML([]byte("spec_version: v1\nname: test\n"))
	if err == nil {
		t.Fatal("expected error for missing fields, got nil")
	}
}
