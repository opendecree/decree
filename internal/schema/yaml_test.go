package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

func ptr[T any](v T) *T { return &v }

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
	assert.Equal(t, yamlSyntaxV1, doc.Syntax)
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
	t.Run("missing syntax", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte(`
name: test
fields:
  x:
    type: string
`))
		assert.ErrorContains(t, err, "syntax is required")
	})

	t.Run("unsupported syntax", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte(`
syntax: "v99"
name: test
fields:
  x:
    type: string
`))
		assert.ErrorContains(t, err, "unsupported syntax version")
	})

	t.Run("missing name", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte(`
syntax: "v1"
fields:
  x:
    type: string
`))
		assert.ErrorContains(t, err, "name is required")
	})

	t.Run("no fields", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte(`
syntax: "v1"
name: test
fields: {}
`))
		assert.ErrorContains(t, err, "at least one field is required")
	})

	t.Run("unknown field type", func(t *testing.T) {
		_, err := unmarshalSchemaYAML([]byte(`
syntax: "v1"
name: test
fields:
  x:
    type: foobar
`))
		assert.ErrorContains(t, err, "unknown type")
	})

	t.Run("valid minimal", func(t *testing.T) {
		doc, err := unmarshalSchemaYAML([]byte(`
syntax: "v1"
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
			_, err := unmarshalSchemaYAML([]byte("syntax: \"v1\"\nname: " + tc.slug + "\nfields:\n  x:\n    type: string\n"))
			assert.ErrorContains(t, err, "slug")
		})
	}
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
