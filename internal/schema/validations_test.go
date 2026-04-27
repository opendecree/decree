package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// --- validateValidationsYAML (structural lint) ---

func TestValidateValidationsYAML_Empty(t *testing.T) {
	require.NoError(t, validateValidationsYAML(&SchemaYAML{}))
}

func TestValidateValidationsYAML_ValidMinimal(t *testing.T) {
	doc := &SchemaYAML{
		Validations: []ValidationYAML{
			{Rule: "self.a > 0", Message: "a must be positive"},
		},
	}
	require.NoError(t, validateValidationsYAML(doc))
}

func TestValidateValidationsYAML_ValidFull(t *testing.T) {
	doc := &SchemaYAML{
		Validations: []ValidationYAML{
			{Path: "payments", Rule: "self.a > 0", Message: "msg", Severity: "error", Reason: "POSITIVE_REQUIRED"},
			{Path: "billing", Rule: "self.b < 100", Message: "msg2", Severity: "warning"},
		},
	}
	require.NoError(t, validateValidationsYAML(doc))
}

func TestValidateValidationsYAML_RejectsEmptyRule(t *testing.T) {
	doc := &SchemaYAML{
		Validations: []ValidationYAML{
			{Rule: "", Message: "x"},
		},
	}
	err := validateValidationsYAML(doc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rule is required")
}

func TestValidateValidationsYAML_RejectsEmptyMessage(t *testing.T) {
	doc := &SchemaYAML{
		Validations: []ValidationYAML{
			{Rule: "self.a > 0", Message: ""},
		},
	}
	err := validateValidationsYAML(doc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "message is required")
}

func TestValidateValidationsYAML_RejectsBadSeverity(t *testing.T) {
	doc := &SchemaYAML{
		Validations: []ValidationYAML{
			{Rule: "self.a > 0", Message: "msg", Severity: "panic"},
		},
	}
	err := validateValidationsYAML(doc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"panic"`)
	assert.Contains(t, err.Error(), `"error" or "warning"`)
}

func TestValidateValidationsYAML_RejectsUnknownExtension(t *testing.T) {
	doc := &SchemaYAML{
		Validations: []ValidationYAML{
			{
				Rule:       "self.a > 0",
				Message:    "msg",
				Extensions: map[string]any{"unknown": "x"},
			},
		},
	}
	err := validateValidationsYAML(doc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestValidateValidationsYAML_AcceptsXExtension(t *testing.T) {
	doc := &SchemaYAML{
		Validations: []ValidationYAML{
			{
				Rule:       "self.a > 0",
				Message:    "msg",
				Extensions: map[string]any{"x-vendor-id": "abc"},
			},
		},
	}
	require.NoError(t, validateValidationsYAML(doc))
}

func TestValidateValidationsYAML_ErrorIncludesIndex(t *testing.T) {
	doc := &SchemaYAML{
		Validations: []ValidationYAML{
			{Rule: "self.a > 0", Message: "ok"},
			{Rule: "", Message: "msg"}, // index 1 is bad
		},
	}
	err := validateValidationsYAML(doc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validations[1]")
}

// --- Marshal / Unmarshal round-trip ---

func TestMarshalValidations_EmptyReturnsBracketArray(t *testing.T) {
	raw, err := MarshalValidations(nil)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(raw))
}

func TestMarshalUnmarshalValidations_RoundTrip(t *testing.T) {
	in := []*pb.ValidationRule{
		{Path: "p", Rule: "self.a > 0", Message: "m1", Severity: "error", Reason: "R1"},
		{Rule: "self.b < 100", Message: "m2"},
	}
	raw, err := MarshalValidations(in)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	out := UnmarshalValidations(raw)
	require.Len(t, out, 2)
	assert.Equal(t, "p", out[0].Path)
	assert.Equal(t, "self.a > 0", out[0].Rule)
	assert.Equal(t, "m1", out[0].Message)
	assert.Equal(t, "error", out[0].Severity)
	assert.Equal(t, "R1", out[0].Reason)
	assert.Equal(t, "", out[1].Path)
	assert.Equal(t, "self.b < 100", out[1].Rule)
}

func TestUnmarshalValidations_EmptyInputReturnsNil(t *testing.T) {
	assert.Nil(t, UnmarshalValidations(nil))
	assert.Nil(t, UnmarshalValidations([]byte("[]")))
}

func TestUnmarshalValidations_MalformedReturnsNil(t *testing.T) {
	assert.Nil(t, UnmarshalValidations([]byte("not json")))
}

// --- proto <-> YAML conversion helpers ---

func TestYamlToProtoValidations_EmptyReturnsNil(t *testing.T) {
	assert.Nil(t, yamlToProtoValidations(nil))
	assert.Nil(t, yamlToProtoValidations([]ValidationYAML{}))
}

func TestProtoValidationsToYAML_EmptyReturnsNil(t *testing.T) {
	assert.Nil(t, protoValidationsToYAML(nil))
	assert.Nil(t, protoValidationsToYAML([]*pb.ValidationRule{}))
}

func TestProtoValidationsToYAML_PreservesOrder(t *testing.T) {
	in := []*pb.ValidationRule{
		{Path: "z", Rule: "rZ", Message: "mZ"},
		{Path: "a", Rule: "rA", Message: "mA"},
	}
	out := protoValidationsToYAML(in)
	require.Len(t, out, 2)
	// Order preserved — schema authors expect this.
	assert.Equal(t, "z", out[0].Path)
	assert.Equal(t, "a", out[1].Path)
}

func TestValidationsRoundTrip_YAMLToProtoToYAML(t *testing.T) {
	yamlIn := []ValidationYAML{
		{Path: "p", Rule: "self.a > 0", Message: "m1", Severity: "warning", Reason: "R"},
	}
	proto := yamlToProtoValidations(yamlIn)
	yamlOut := protoValidationsToYAML(proto)
	require.Len(t, yamlOut, 1)
	assert.Equal(t, yamlIn[0], yamlOut[0])
}

// --- Top-level YAML parser integration ---

func TestUnmarshalSchemaYAML_Validations_Valid(t *testing.T) {
	doc, err := unmarshalSchemaYAML([]byte(`
spec_version: v1
name: payments
fields:
  payments.min: { type: integer }
  payments.max: { type: integer }
validations:
  - path: payments
    rule: "self.payments.min < self.payments.max"
    message: "min must be less than max"
`))
	require.NoError(t, err)
	require.Len(t, doc.Validations, 1)
	assert.Equal(t, "payments", doc.Validations[0].Path)
	assert.Equal(t, "self.payments.min < self.payments.max", doc.Validations[0].Rule)
	assert.Equal(t, "min must be less than max", doc.Validations[0].Message)
}

func TestUnmarshalSchemaYAML_Validations_RejectsEmptyRule(t *testing.T) {
	_, err := unmarshalSchemaYAML([]byte(`
spec_version: v1
name: payments
fields:
  payments.x: { type: string }
validations:
  - rule: ""
    message: "msg"
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rule is required")
}

func TestUnmarshalSchemaYAML_Validations_RejectsBadSeverity(t *testing.T) {
	_, err := unmarshalSchemaYAML([]byte(`
spec_version: v1
name: payments
fields:
  payments.x: { type: string }
validations:
  - rule: "self.payments.x != ''"
    message: "msg"
    severity: critical
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"critical"`)
}
