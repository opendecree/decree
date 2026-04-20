package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/tools/docgen"
)

// --- typedValueDisplay ---

func TestTypedValueDisplay(t *testing.T) {
	tests := []struct {
		name     string
		input    *pb.TypedValue
		expected string
	}{
		{"nil", nil, "<null>"},
		{"string", &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hello"}}, "hello"},
		{"integer", &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}, "42"},
		{"number", &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 3.14}}, "3.14"},
		{"bool", &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}, "true"},
		{"url", &pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://example.com"}}, "https://example.com"},
		{"json", &pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `{"a":1}`}}, `{"a":1}`},
		{"duration", &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(5 * time.Minute)}}, "5m0s"},
		{"duration nil", &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{}}, ""},
		{"time nil", &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{}}, ""},
		{"empty kind", &pb.TypedValue{}, ""},
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
	tv := &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(ts)}}
	if !strings.Contains(typedValueDisplay(tv), "2026-03-30") {
		t.Errorf("expected %q to contain %q", typedValueDisplay(tv), "2026-03-30")
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
	m := parseConfigValues([]byte(yaml))
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
	m := parseConfigValues([]byte("not: [valid: yaml"))
	if m != nil {
		t.Errorf("expected nil, got %v", m)
	}
}

func TestParseConfigValues_Empty(t *testing.T) {
	m := parseConfigValues([]byte("spec_version: v1\n"))
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
	yaml := `name: payments
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

func TestSchemaFromYAML_Invalid(t *testing.T) {
	_, err := schemaFromYAML([]byte("not: [valid"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSchemaFromYAML_Empty(t *testing.T) {
	s, err := schemaFromYAML([]byte("name: test\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := s.Name; got != "test" {
		t.Errorf("got %v, want %v", got, "test")
	}
	if len(s.Fields) != 0 {
		t.Errorf("expected empty, got %v", s.Fields)
	}
}
