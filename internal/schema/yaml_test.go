package schema

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func ptr[T any](v T) *T { return &v }

func TestUnmarshalSchemaYAML_DependentRequired_Valid(t *testing.T) {
	doc, err := unmarshalSchemaYAML([]byte(`
spec_version: v1
name: payments
fields:
  payments.refunds_enabled:
    type: bool
  payments.refund_window:
    type: duration
    nullable: true
dependentRequired:
  payments.refunds_enabled: [payments.refund_window]
`))
	require.NoError(t, err)
	require.NotNil(t, doc)
	require.Len(t, doc.DependentRequired, 1)
	assert.Equal(t, []string{"payments.refund_window"}, doc.DependentRequired["payments.refunds_enabled"])
}

func TestUnmarshalSchemaYAML_DependentRequired_RejectsUnknownTrigger(t *testing.T) {
	_, err := unmarshalSchemaYAML([]byte(`
spec_version: v1
name: payments
fields:
  payments.a:
    type: string
dependentRequired:
  payments.ghost: [payments.a]
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
	assert.Contains(t, err.Error(), "not a defined field")
}

func TestUnmarshalSchemaYAML_DependentRequired_RejectsUnknownDependent(t *testing.T) {
	_, err := unmarshalSchemaYAML([]byte(`
spec_version: v1
name: payments
fields:
  payments.a:
    type: string
dependentRequired:
  payments.a: [payments.ghost]
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost")
}

func TestUnmarshalSchemaYAML_DependentRequired_RejectsSelfReference(t *testing.T) {
	_, err := unmarshalSchemaYAML([]byte(`
spec_version: v1
name: payments
fields:
  payments.a:
    type: string
dependentRequired:
  payments.a: [payments.a]
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot list itself")
}

func TestYAMLRoundtrip(t *testing.T) {
	original := &pb.Schema{
		Id:                 "test-id",
		Name:               "payments",
		Description:        "Payment config",
		Version:            3,
		VersionDescription: "Add retries",
		Fields: []*pb.SchemaField{
			{
				Path:         "payments.fee",
				Type:         pb.FieldType_FIELD_TYPE_STRING,
				Description:  ptr("Fee percentage"),
				DefaultValue: ptr("0.5%"),
				Constraints: &pb.FieldConstraints{
					Regex: ptr(`^\d+(\.\d+)?%$`),
				},
			},
			{
				Path:     "payments.max_retries",
				Type:     pb.FieldType_FIELD_TYPE_INT,
				Nullable: true,
				Constraints: &pb.FieldConstraints{
					Min: ptr(float64(0)),
					Max: ptr(float64(10)),
				},
			},
			{
				Path: "payments.currency",
				Type: pb.FieldType_FIELD_TYPE_STRING,
				Constraints: &pb.FieldConstraints{
					EnumValues: []string{"USD", "EUR", "GBP"},
				},
			},
			{
				Path:       "payments.old_fee",
				Type:       pb.FieldType_FIELD_TYPE_STRING,
				Deprecated: true,
				RedirectTo: ptr("payments.fee"),
			},
		},
	}

	// Proto → YAML
	doc := schemaToYAML(original)
	assert.Equal(t, yamlSpecVersionV1, doc.SpecVersion)
	assert.Equal(t, metaSchemaURL, doc.Schema)
	assert.Equal(t, "urn:decree:schema:payments:v3", doc.ID)
	assert.Equal(t, "payments", doc.Name)
	assert.Equal(t, "Payment config", doc.Description)
	assert.Equal(t, int32(3), doc.Version)
	assert.Len(t, doc.Fields, 4)

	// Check OAS constraint naming
	feeField := doc.Fields["payments.fee"]
	assert.Equal(t, "string", feeField.Type)
	assert.Equal(t, `^\d+(\.\d+)?%$`, feeField.Constraints.Pattern)
	assert.Equal(t, "0.5%", feeField.Default)

	retriesField := doc.Fields["payments.max_retries"]
	assert.Equal(t, "integer", retriesField.Type)
	assert.True(t, retriesField.Nullable)
	assert.Equal(t, float64(0), *retriesField.Constraints.Minimum)
	assert.Equal(t, float64(10), *retriesField.Constraints.Maximum)

	currencyField := doc.Fields["payments.currency"]
	assert.Equal(t, []string{"USD", "EUR", "GBP"}, currencyField.Constraints.Enum)

	oldFeeField := doc.Fields["payments.old_fee"]
	assert.True(t, oldFeeField.Deprecated)
	assert.Equal(t, "payments.fee", oldFeeField.RedirectTo)

	// Marshal → Unmarshal → convert back
	data, err := marshalSchemaYAML(doc)
	require.NoError(t, err)

	parsed, err := unmarshalSchemaYAML(data)
	require.NoError(t, err)

	fields := yamlToProtoFields(parsed)
	assert.Len(t, fields, 4)

	// Verify roundtrip: find each field and check
	fieldMap := make(map[string]*pb.SchemaField)
	for _, f := range fields {
		fieldMap[f.Path] = f
	}

	fee := fieldMap["payments.fee"]
	require.NotNil(t, fee)
	assert.Equal(t, pb.FieldType_FIELD_TYPE_STRING, fee.Type)
	assert.Equal(t, "0.5%", *fee.DefaultValue)
	assert.Equal(t, `^\d+(\.\d+)?%$`, *fee.Constraints.Regex)

	retries := fieldMap["payments.max_retries"]
	require.NotNil(t, retries)
	assert.Equal(t, pb.FieldType_FIELD_TYPE_INT, retries.Type)
	assert.True(t, retries.Nullable)
	assert.Equal(t, float64(0), *retries.Constraints.Min)
	assert.Equal(t, float64(10), *retries.Constraints.Max)

	currency := fieldMap["payments.currency"]
	require.NotNil(t, currency)
	assert.Equal(t, []string{"USD", "EUR", "GBP"}, currency.Constraints.EnumValues)

	oldFee := fieldMap["payments.old_fee"]
	require.NotNil(t, oldFee)
	assert.True(t, oldFee.Deprecated)
	assert.Equal(t, "payments.fee", *oldFee.RedirectTo)
}

func TestYAMLTypeMapping(t *testing.T) {
	cases := []struct {
		yaml  string
		proto pb.FieldType
	}{
		{"integer", pb.FieldType_FIELD_TYPE_INT},
		{"number", pb.FieldType_FIELD_TYPE_NUMBER},
		{"string", pb.FieldType_FIELD_TYPE_STRING},
		{"bool", pb.FieldType_FIELD_TYPE_BOOL},
		{"time", pb.FieldType_FIELD_TYPE_TIME},
		{"duration", pb.FieldType_FIELD_TYPE_DURATION},
		{"url", pb.FieldType_FIELD_TYPE_URL},
		{"json", pb.FieldType_FIELD_TYPE_JSON},
	}

	for _, tc := range cases {
		t.Run(tc.yaml, func(t *testing.T) {
			got, ok := yamlTypeToProto(tc.yaml)
			assert.True(t, ok)
			assert.Equal(t, tc.proto, got)
			assert.Equal(t, tc.yaml, protoTypeToYAML(tc.proto))
		})
	}

	// Unknown type
	_, ok := yamlTypeToProto("unknown")
	assert.False(t, ok)
}

func TestYAMLValidation(t *testing.T) {
	t.Run("missing spec_version", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte(`
name: test
fields:
  x:
    type: string
`))
		assert.ErrorContains(t, err, "spec_version is required")
	})

	t.Run("unsupported spec_version", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte(`
spec_version: "v99"
name: test
fields:
  x:
    type: string
`))
		assert.ErrorContains(t, err, "unsupported spec_version")
	})

	t.Run("missing name", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte(`
spec_version: "v1"
fields:
  x:
    type: string
`))
		assert.ErrorContains(t, err, "name is required")
	})

	t.Run("no fields", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte(`
spec_version: "v1"
name: test
fields: {}
`))
		assert.ErrorContains(t, err, "at least one field is required")
	})

	t.Run("unknown field type", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte(`
spec_version: "v1"
name: test
fields:
  x:
    type: foobar
`))
		assert.ErrorContains(t, err, "unknown type")
	})

	t.Run("valid minimal", func(t *testing.T) {
		doc, err := unmarshalSchemaYAML([]byte(`
spec_version: "v1"
name: test
fields:
  x:
    type: string
`))
		require.NoError(t, err)
		assert.Equal(t, "test", doc.Name)
	})
}

func TestYAMLValidation_InvalidSlug(t *testing.T) {
	cases := []struct {
		name string
		slug string
	}{
		{"uppercase", "Payment-Config"},
		{"spaces", "payment config"},
		{"starts with hyphen", "-payments"},
		{"ends with hyphen", "payments-"},
		{"special chars", "pay@ments"},
		{"underscore", "payment_config"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := unmarshalSchemaYAML([]byte("spec_version: \"v1\"\nname: " + tc.slug + "\nfields:\n  x:\n    type: string\n"))
			assert.ErrorContains(t, err, "slug")
		})
	}
}

func TestYAMLValidation_SchemaAndID(t *testing.T) {
	validBody := "\nname: test\nfields:\n  x:\n    type: string\n"

	t.Run("both absent (optional)", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte("spec_version: \"v1\"" + validBody))
		require.NoError(t, err)
	})

	t.Run("both accepted when well-formed", func(t *testing.T) {
		doc, err := unmarshalSchemaYAML([]byte("spec_version: \"v1\"\n$schema: https://schemas.opendecree.dev/schema/v0.1.0/decree.json\n$id: urn:decree:schema:test:v1" + validBody))
		require.NoError(t, err)
		assert.Equal(t, "https://schemas.opendecree.dev/schema/v0.1.0/decree.json", doc.Schema)
		assert.Equal(t, "urn:decree:schema:test:v1", doc.ID)
	})

	t.Run("$schema must be HTTPS", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte("spec_version: \"v1\"\n$schema: http://example.com/decree.json" + validBody))
		assert.ErrorContains(t, err, "$schema")
		assert.ErrorContains(t, err, "HTTPS")
	})

	t.Run("$schema must have host", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte("spec_version: \"v1\"\n$schema: not-a-url" + validBody))
		assert.ErrorContains(t, err, "$schema")
	})

	t.Run("$id must be decree schema URN", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte("spec_version: \"v1\"\n$id: not-a-urn" + validBody))
		assert.ErrorContains(t, err, "$id")
	})

	t.Run("$id rejects wrong namespace", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte("spec_version: \"v1\"\n$id: urn:other:schema:test:v1" + validBody))
		assert.ErrorContains(t, err, "$id")
	})
}

func TestYAMLValidation_FieldPath(t *testing.T) {
	mkYAML := func(path string) []byte {
		return []byte(fmt.Sprintf("spec_version: \"v1\"\nname: test\nfields:\n  %q:\n    type: string\n", path))
	}

	t.Run("valid paths accepted", func(t *testing.T) {
		for _, path := range []string{"x", "app.name", "app_name", "app-name", "a.b.c", "_leading_underscore", "x1", "x.y-z_2"} {
			_, err := unmarshalSchemaYAML(mkYAML(path))
			assert.NoError(t, err, "expected %q to be accepted", path)
		}
	})

	t.Run("invalid paths rejected", func(t *testing.T) {
		cases := []struct {
			name string
			path string
		}{
			{"empty", ""},
			{"leading digit", "1app"},
			{"leading dot", ".app"},
			{"leading hyphen", "-app"},
			{"whitespace", "app name"},
			{"tab", "app\tname"},
			{"special chars", "app@name"},
			{"colon", "app:name"},
			{"slash", "app/name"},
			{"dollar", "$app"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := unmarshalSchemaYAML(mkYAML(tc.path))
				assert.ErrorContains(t, err, "invalid field path")
			})
		}
	})
}

func TestYAMLValidation_RejectsUnknownKeys(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		locHint string // substring expected in error
	}{
		{
			name: "unknown top-level key",
			yaml: `spec_version: "v1"
typo_top: 1
name: test
fields:
  x:
    type: string
`,
			locHint: "top level",
		},
		{
			name: "unknown key under info",
			yaml: `spec_version: "v1"
name: test
info:
  typo_info: hi
fields:
  x:
    type: string
`,
			locHint: "info",
		},
		{
			name: "unknown key under info.contact",
			yaml: `spec_version: "v1"
name: test
info:
  contact:
    typo_contact: hi
fields:
  x:
    type: string
`,
			locHint: "info.contact",
		},
		{
			name: "unknown key under a field",
			yaml: `spec_version: "v1"
name: test
fields:
  x:
    type: string
    typ: string
`,
			locHint: "fields.x",
		},
		{
			name: "unknown key under constraints",
			yaml: `spec_version: "v1"
name: test
fields:
  x:
    type: string
    constraints:
      minimun: 1
`,
			locHint: "fields.x.constraints",
		},
		{
			name: "unknown key under externalDocs",
			yaml: `spec_version: "v1"
name: test
fields:
  x:
    type: string
    externalDocs:
      url: https://example.com
      typo_docs: hi
`,
			locHint: "fields.x.externalDocs",
		},
		{
			name: "unknown key under an example",
			yaml: `spec_version: "v1"
name: test
fields:
  x:
    type: string
    examples:
      ok:
        value: hi
        typo_ex: 1
`,
			locHint: "fields.x.examples.ok",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := unmarshalSchemaYAML([]byte(tc.yaml))
			require.Error(t, err)
			assert.ErrorContains(t, err, "unknown key")
			assert.ErrorContains(t, err, tc.locHint)
		})
	}
}

func TestYAMLValidation_AcceptsXExtensions(t *testing.T) {
	input := []byte(`spec_version: "v1"
x-owner: platform
name: test
info:
  title: Test
  x-info-ext: info-val
  contact:
    name: a
    x-contact-ext: c-val
fields:
  x:
    type: string
    x-field-ext: field-val
    constraints:
      minLength: 1
      x-constraint-ext: cn-val
    externalDocs:
      url: https://example.com
      x-docs-ext: d-val
    examples:
      ok:
        value: hi
        x-example-ext: e-val
`)
	doc, err := unmarshalSchemaYAML(input)
	require.NoError(t, err)

	assert.Equal(t, "platform", doc.Extensions["x-owner"])
	assert.Equal(t, "info-val", doc.Info.Extensions["x-info-ext"])
	assert.Equal(t, "c-val", doc.Info.Contact.Extensions["x-contact-ext"])
	f := doc.Fields["x"]
	assert.Equal(t, "field-val", f.Extensions["x-field-ext"])
	assert.Equal(t, "cn-val", f.Constraints.Extensions["x-constraint-ext"])
	assert.Equal(t, "d-val", f.ExternalDocs.Extensions["x-docs-ext"])
	assert.Equal(t, "e-val", f.Examples["ok"].Extensions["x-example-ext"])
}

func TestYAML_XExtensionsRoundTrip(t *testing.T) {
	input := []byte(`spec_version: "v1"
x-owner: platform
name: test
fields:
  a:
    type: string
    x-field-ext: field-val
`)
	doc, err := unmarshalSchemaYAML(input)
	require.NoError(t, err)

	out, err := marshalSchemaYAML(doc)
	require.NoError(t, err)
	outStr := string(out)
	assert.Contains(t, outStr, "x-owner: platform")
	assert.Contains(t, outStr, "x-field-ext: field-val")

	// Re-parse — round-trip must succeed.
	_, err = unmarshalSchemaYAML(out)
	require.NoError(t, err)
}

func TestSchemaToYAML_EmitsSchemaAndID(t *testing.T) {
	doc := schemaToYAML(&pb.Schema{
		Name:    "billing",
		Version: 7,
		Fields:  []*pb.SchemaField{{Path: "x", Type: pb.FieldType_FIELD_TYPE_STRING}},
	})
	assert.Equal(t, metaSchemaURL, doc.Schema)
	assert.Equal(t, "urn:decree:schema:billing:v7", doc.ID)

	// Synthesized $id must satisfy the same pattern the parser enforces.
	assert.Regexp(t, schemaURNPattern, doc.ID)
}

func TestConstraintsNilWhenEmpty(t *testing.T) {
	field := &pb.SchemaField{
		Path: "x",
		Type: pb.FieldType_FIELD_TYPE_STRING,
	}
	doc := schemaToYAML(&pb.Schema{
		Name:   "test",
		Fields: []*pb.SchemaField{field},
	})
	assert.Nil(t, doc.Fields["x"].Constraints)
}
