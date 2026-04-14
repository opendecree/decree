package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			assert.Equal(t, tt.expected, typedValueDisplay(tt.input))
		})
	}
}

func TestTypedValueDisplay_Time(t *testing.T) {
	ts := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	tv := &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(ts)}}
	assert.Contains(t, typedValueDisplay(tv), "2026-03-30")
}

// --- printTable edge cases ---

func TestPrintTable_NonTableData(t *testing.T) {
	var buf bytes.Buffer
	err := printTable(&buf, map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "key")
}

func TestPrintTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := printTable(&buf, [][]string{})
	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

// --- versionOrEmpty ---

func TestVersionOrEmpty(t *testing.T) {
	assert.Equal(t, "", versionOrEmpty(0))
	assert.Equal(t, "v1", versionOrEmpty(1))
	assert.Equal(t, "v42", versionOrEmpty(42))
}

// --- parseConfigValues ---

func TestParseConfigValues(t *testing.T) {
	yaml := `syntax: v1
values:
  app.name:
    value: MyApp
  app.retries:
    value: 3
  app.enabled:
    value: true
`
	m := parseConfigValues([]byte(yaml))
	require.NotNil(t, m)
	assert.Equal(t, "MyApp", m["app.name"])
	assert.Equal(t, "3", m["app.retries"])
	assert.Equal(t, "true", m["app.enabled"])
}

func TestParseConfigValues_Invalid(t *testing.T) {
	m := parseConfigValues([]byte("not: [valid: yaml"))
	assert.Nil(t, m)
}

func TestParseConfigValues_Empty(t *testing.T) {
	m := parseConfigValues([]byte("syntax: v1\n"))
	assert.NotNil(t, m)
	assert.Empty(t, m)
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
	assert.Equal(t, "payments", ds.Name)
	assert.Equal(t, "test", ds.Description)
	assert.Equal(t, int32(2), ds.Version)
	assert.Len(t, ds.Fields, 2)

	f := ds.Fields[0]
	assert.Equal(t, "app.retries", f.Path)
	assert.Equal(t, "retry count", f.Description)
	assert.Equal(t, "3", f.Default)
	assert.True(t, f.Nullable)
	assert.True(t, f.Deprecated)
	assert.Equal(t, "app.max_retries", f.RedirectTo)
	require.NotNil(t, f.Constraints)
	assert.Equal(t, &min, f.Constraints.Min)
	assert.Equal(t, []string{"1", "2", "3"}, f.Constraints.Enum)
}

func TestAdminSchemaToDocgen_NoConstraints(t *testing.T) {
	s := &adminclient.Schema{
		Name:   "test",
		Fields: []adminclient.Field{{Path: "x", Type: "STRING"}},
	}
	ds := adminSchemaToDocgen(s)
	assert.Nil(t, ds.Fields[0].Constraints)
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
	require.NoError(t, err)
	assert.Equal(t, "payments", s.Name)
	assert.Equal(t, "Payment config", s.Description)
	assert.Len(t, s.Fields, 2)

	// Find the fee field.
	var fee *docgen.Field
	for i := range s.Fields {
		if s.Fields[i].Path == "app.fee" {
			fee = &s.Fields[i]
			break
		}
	}
	require.NotNil(t, fee)
	assert.Equal(t, "number", fee.Type)
	assert.True(t, fee.Nullable)
	require.NotNil(t, fee.Constraints)
	assert.Equal(t, 0.0, *fee.Constraints.Min)
	assert.Equal(t, 1.0, *fee.Constraints.Max)
}

func TestSchemaFromYAML_Invalid(t *testing.T) {
	_, err := schemaFromYAML([]byte("not: [valid"))
	assert.Error(t, err)
}

func TestSchemaFromYAML_Empty(t *testing.T) {
	s, err := schemaFromYAML([]byte("name: test\n"))
	require.NoError(t, err)
	assert.Equal(t, "test", s.Name)
	assert.Empty(t, s.Fields)
}
