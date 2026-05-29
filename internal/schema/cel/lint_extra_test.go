package cel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func TestLintValidations_EnumChain_LiteralOnLeft(t *testing.T) {
	// Each leaf has the literal on the left, exercising the right-hand self
	// branch of equalityComponents.
	fields := []*pb.SchemaField{{Path: "mode", Type: pb.FieldType_FIELD_TYPE_STRING}}
	err := LintValidations([]*pb.ValidationRule{
		{Rule: `"a" == self.mode || "b" == self.mode`, Message: "x"},
	}, fields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use enum:")
}

func TestLintValidations_BoundedRange_LiteralOnLeft(t *testing.T) {
	// Both bounds carry the literal on the left, exercising the right-hand
	// self branch of boundComponents.
	fields := []*pb.SchemaField{{Path: "x", Type: pb.FieldType_FIELD_TYPE_NUMBER}}
	err := LintValidations([]*pb.ValidationRule{
		{Rule: `0.0 < self.x && 100.0 > self.x`, Message: "x"},
	}, fields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use minimum:")
	assert.Contains(t, err.Error(), "and maximum:")
}

func TestLintValidations_OrRulesNotDependentRequiredSubstitutable(t *testing.T) {
	// Valid cross-field OR rules that are NOT dependentRequired-substitutable,
	// exercising the negative branches of negatedHasSelf and hasSelf.
	fields := []*pb.SchemaField{
		{Path: "a", Type: pb.FieldType_FIELD_TYPE_STRING, Nullable: true},
		{Path: "b", Type: pb.FieldType_FIELD_TYPE_STRING, Nullable: true},
	}
	cases := []struct {
		name string
		rule string
	}{
		{"first operand not negated", `has(self.a) || has(self.b)`},
		{"second operand not has()", `!has(self.a) || self.b == "x"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, LintValidations([]*pb.ValidationRule{{Rule: tc.rule, Message: "x"}}, fields))
		})
	}
}

func TestLintValidations_MapLiteralTraversed(t *testing.T) {
	// A map literal in the AST exercises the MapKind branch of visit, which
	// collectSelfChains walks to find self.* references nested under it.
	fields := []*pb.SchemaField{{Path: "x", Type: pb.FieldType_FIELD_TYPE_NUMBER}}
	rules := []*pb.ValidationRule{
		{Rule: `{"lo": 0.0}["lo"] < self.x`, Message: "x positive"},
	}
	require.NoError(t, LintValidations(rules, fields))
}

func TestLintSuggestionHelpers_ArityGuards(t *testing.T) {
	// CEL operators are always binary, so these defensive arity guards are
	// unreachable through LintValidations; call the helpers directly.
	if _, ok := suggestComparisonAgainstLiteral("_<_", nil); ok {
		t.Error("suggestComparisonAgainstLiteral should reject wrong arity")
	}
	if _, ok := suggestEqualityLiteral(nil); ok {
		t.Error("suggestEqualityLiteral should reject wrong arity")
	}
	if _, ok := suggestEnumChain(nil); ok {
		t.Error("suggestEnumChain should reject wrong arity")
	}
	if _, ok := suggestDependentRequiredImplication(nil); ok {
		t.Error("suggestDependentRequiredImplication should reject wrong arity")
	}
	if _, ok := suggestBoundedRange(nil); ok {
		t.Error("suggestBoundedRange should reject wrong arity")
	}
}
