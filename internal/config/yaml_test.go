package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

func TestConfigYAML_Roundtrip(t *testing.T) {
	fieldTypes := map[string]pb.FieldType{
		"payments.enabled":     pb.FieldType_FIELD_TYPE_BOOL,
		"payments.max_retries": pb.FieldType_FIELD_TYPE_INT,
		"payments.fee_rate":    pb.FieldType_FIELD_TYPE_NUMBER,
		"payments.currency":    pb.FieldType_FIELD_TYPE_STRING,
		"payments.window":      pb.FieldType_FIELD_TYPE_DURATION,
		"payments.metadata":    pb.FieldType_FIELD_TYPE_JSON,
	}

	rows := []configRow{
		{FieldPath: "payments.enabled", Value: "true"},
		{FieldPath: "payments.max_retries", Value: "3"},
		{FieldPath: "payments.fee_rate", Value: "0.025"},
		{FieldPath: "payments.currency", Value: "USD"},
		{FieldPath: "payments.window", Value: "24h"},
		{FieldPath: "payments.metadata", Value: `{"type":"object"}`},
	}

	// Export: rows → YAML doc → bytes
	doc := configToYAML(5, "test export", rows, fieldTypes)
	assert.Equal(t, yamlSyntaxV1, doc.Syntax)
	assert.Equal(t, int32(5), doc.Version)
	assert.Len(t, doc.Values, 6)

	// Check typed values
	assert.Equal(t, true, doc.Values["payments.enabled"].Value)
	assert.Equal(t, int64(3), doc.Values["payments.max_retries"].Value)
	assert.Equal(t, 0.025, doc.Values["payments.fee_rate"].Value)
	assert.Equal(t, "USD", doc.Values["payments.currency"].Value)
	assert.Equal(t, "24h", doc.Values["payments.window"].Value)
	// JSON becomes a map
	metadata := doc.Values["payments.metadata"].Value
	metaMap, ok := metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", metaMap["type"])

	// Marshal → unmarshal → import back
	data, err := marshalConfigYAML(doc)
	require.NoError(t, err)

	parsed, err := unmarshalConfigYAML(data)
	require.NoError(t, err)

	values, err := yamlToConfigValues(parsed, fieldTypes)
	require.NoError(t, err)
	assert.Len(t, values, 6)

	// Check string conversions
	valMap := make(map[string]string)
	for _, v := range values {
		valMap[v.FieldPath] = v.Value
	}
	assert.Equal(t, "true", valMap["payments.enabled"])
	assert.Equal(t, "3", valMap["payments.max_retries"])
	assert.Equal(t, "0.025", valMap["payments.fee_rate"])
	assert.Equal(t, "USD", valMap["payments.currency"])
	assert.Equal(t, "24h", valMap["payments.window"])
	assert.Equal(t, `{"type":"object"}`, valMap["payments.metadata"])
}

func TestConfigYAML_TypedValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		ft       pb.FieldType
		expected interface{}
	}{
		{"integer", "42", pb.FieldType_FIELD_TYPE_INT, int64(42)},
		{"negative integer", "-1", pb.FieldType_FIELD_TYPE_INT, int64(-1)},
		{"number float", "3.14", pb.FieldType_FIELD_TYPE_NUMBER, 3.14},
		{"number integer", "42", pb.FieldType_FIELD_TYPE_NUMBER, float64(42)},
		{"bool true", "true", pb.FieldType_FIELD_TYPE_BOOL, true},
		{"bool false", "false", pb.FieldType_FIELD_TYPE_BOOL, false},
		{"string", "hello", pb.FieldType_FIELD_TYPE_STRING, "hello"},
		{"time", "2025-01-15T09:30:00Z", pb.FieldType_FIELD_TYPE_TIME, "2025-01-15T09:30:00Z"},
		{"duration", "24h", pb.FieldType_FIELD_TYPE_DURATION, "24h"},
		{"url", "https://example.com", pb.FieldType_FIELD_TYPE_URL, "https://example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := typedValue(tc.value, tc.ft)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestConfigYAML_StringifyValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		ft       pb.FieldType
		expected string
	}{
		{"int from int", 3, pb.FieldType_FIELD_TYPE_INT, "3"},
		{"int from float64", float64(3), pb.FieldType_FIELD_TYPE_INT, "3"},
		{"number from float64", 3.14, pb.FieldType_FIELD_TYPE_NUMBER, "3.14"},
		{"number from int", 42, pb.FieldType_FIELD_TYPE_NUMBER, "42"},
		{"bool true", true, pb.FieldType_FIELD_TYPE_BOOL, "true"},
		{"bool false", false, pb.FieldType_FIELD_TYPE_BOOL, "false"},
		{"string", "hello", pb.FieldType_FIELD_TYPE_STRING, "hello"},
		{"json map", map[string]interface{}{"key": "val"}, pb.FieldType_FIELD_TYPE_JSON, `{"key":"val"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := stringifyValue(tc.value, tc.ft)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestConfigYAML_StringifyValue_Errors(t *testing.T) {
	// Non-integer float for integer type
	_, err := stringifyValue(3.14, pb.FieldType_FIELD_TYPE_INT)
	assert.ErrorContains(t, err, "expected integer")

	// Wrong type for bool
	_, err = stringifyValue(42, pb.FieldType_FIELD_TYPE_BOOL)
	assert.ErrorContains(t, err, "cannot convert")
}

func TestConfigYAML_Validation(t *testing.T) {
	t.Run("missing syntax", func(t *testing.T) {
		_, err := unmarshalConfigYAML([]byte(`
values:
  x:
    value: "hello"
`))
		assert.ErrorContains(t, err, "syntax is required")
	})

	t.Run("unsupported syntax", func(t *testing.T) {
		_, err := unmarshalConfigYAML([]byte(`
syntax: "v99"
values:
  x:
    value: "hello"
`))
		assert.ErrorContains(t, err, "unsupported syntax version")
	})

	t.Run("empty values", func(t *testing.T) {
		_, err := unmarshalConfigYAML([]byte(`
syntax: "v1"
values: {}
`))
		assert.ErrorContains(t, err, "at least one value is required")
	})

	t.Run("valid minimal", func(t *testing.T) {
		doc, err := unmarshalConfigYAML([]byte(`
syntax: "v1"
values:
  x:
    value: "hello"
`))
		require.NoError(t, err)
		assert.Equal(t, "hello", doc.Values["x"].Value)
	})
}

func TestConfigYAML_Description(t *testing.T) {
	desc := "custom fee"
	rows := []configRow{
		{FieldPath: "fee", Value: "0.5", Description: &desc},
	}
	fieldTypes := map[string]pb.FieldType{"fee": pb.FieldType_FIELD_TYPE_STRING}

	doc := configToYAML(1, "", rows, fieldTypes)
	assert.Equal(t, "custom fee", doc.Values["fee"].Description)

	data, err := marshalConfigYAML(doc)
	require.NoError(t, err)

	parsed, err := unmarshalConfigYAML(data)
	require.NoError(t, err)

	values, err := yamlToConfigValues(parsed, fieldTypes)
	require.NoError(t, err)
	require.Len(t, values, 1)
	require.NotNil(t, values[0].Description)
	assert.Equal(t, "custom fee", *values[0].Description)
}
