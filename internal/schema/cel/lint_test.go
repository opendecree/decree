package cel

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func TestLintValidations_AcceptsValidRules(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: "self.payments.min_amount < self.payments.max_amount", Message: "min < max"},
		{Rule: "self.payments.refunds_enabled ? has(self.payments.refund_window) : true", Message: "refund window required"},
	}
	require.NoError(t, LintValidations(rules, showcaseFields()))
}

func TestLintValidations_Rule1_SyntaxError(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: "self.payments.min_amount <", Message: "broken"},
	}
	err := LintValidations(rules, showcaseFields())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validations[0]:")
}

func TestLintValidations_Rule2_NoSelfReference(t *testing.T) {
	cases := []struct {
		name string
		rule string
	}{
		{"constant", "1 == 1"},
		{"tenant only", `tenant.id == "x"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := LintValidations([]*pb.ValidationRule{{Rule: tc.rule, Message: "x"}}, showcaseFields())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "at least one self.* field")
		})
	}
}

func TestLintValidations_Rule3_UnknownPath(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: "self.unknown.path > 0", Message: "x"},
	}
	err := LintValidations(rules, showcaseFields())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "self.unknown.path does not resolve")
}

func TestLintValidations_Rule3_HyphenatedSegmentResolves(t *testing.T) {
	fields := []*pb.SchemaField{
		{Path: "app-name.title", Type: pb.FieldType_FIELD_TYPE_STRING},
	}
	rules := []*pb.ValidationRule{
		{Rule: `self["app-name"].title != ""`, Message: "title required"},
	}
	require.NoError(t, LintValidations(rules, fields))
}

func TestLintValidations_Rule3_ParentPrefixTolerated(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: "has(self.payments) && self.payments.min_amount < self.payments.max_amount", Message: "x"},
	}
	require.NoError(t, LintValidations(rules, showcaseFields()),
		"self.payments alone is a parent prefix of declared leaves, must be tolerated")
}

func TestLintValidations_Rule4_NumericComparison(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: "self.payments.min_amount > 0.0", Message: "x"},
	}
	err := LintValidations(rules, showcaseFields())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use exclusiveMinimum")
}

func TestLintValidations_Rule4_GTE(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: "self.payments.max_amount >= 100.0", Message: "x"},
	}
	err := LintValidations(rules, showcaseFields())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use minimum: 100")
}

func TestLintValidations_Rule4_Equality(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: `self.payments.refunds_enabled == true`, Message: "x"},
	}
	err := LintValidations(rules, showcaseFields())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use enum:")
}

func TestLintValidations_Rule4_EnumOrChain(t *testing.T) {
	fields := []*pb.SchemaField{
		{Path: "mode", Type: pb.FieldType_FIELD_TYPE_STRING},
	}
	rules := []*pb.ValidationRule{
		{Rule: `self.mode == "a" || self.mode == "b" || self.mode == "c"`, Message: "x"},
	}
	err := LintValidations(rules, fields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use enum:")
	assert.Contains(t, err.Error(), `"a"`)
	assert.Contains(t, err.Error(), `"c"`)
}

func TestLintValidations_Rule4_DependentRequired(t *testing.T) {
	fields := []*pb.SchemaField{
		{Path: "a", Type: pb.FieldType_FIELD_TYPE_STRING, Nullable: true},
		{Path: "b", Type: pb.FieldType_FIELD_TYPE_STRING, Nullable: true},
	}
	rules := []*pb.ValidationRule{
		{Rule: "!has(self.a) || has(self.b)", Message: "x"},
	}
	err := LintValidations(rules, fields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use dependentRequired")
	assert.Contains(t, err.Error(), "a: [b]")
}

func TestLintValidations_Rule4_BoundedRange(t *testing.T) {
	fields := []*pb.SchemaField{
		{Path: "x", Type: pb.FieldType_FIELD_TYPE_NUMBER},
	}
	rules := []*pb.ValidationRule{
		{Rule: "self.x > 0.0 && self.x < 100.0", Message: "x"},
	}
	err := LintValidations(rules, fields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use minimum:")
	assert.Contains(t, err.Error(), "and maximum:")
}

func TestLintValidations_Rule4_CrossFieldNotSubstitutable(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: "self.payments.min_amount < self.payments.max_amount", Message: "x"},
	}
	require.NoError(t, LintValidations(rules, showcaseFields()))
}

func TestLintValidations_AggregatesAcrossRules(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: "1 == 1", Message: "no self ref"},
		{Rule: "self.unknown > 0.0", Message: "bad path"},
	}
	err := LintValidations(rules, showcaseFields())
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "validations[0]:")
	assert.Contains(t, msg, "validations[1]:")
	assert.Equal(t, 2, strings.Count(msg, "validations["))
}

func TestLintValidations_NoRulesIsNoOp(t *testing.T) {
	require.NoError(t, LintValidations(nil, showcaseFields()))
	require.NoError(t, LintValidations([]*pb.ValidationRule{}, showcaseFields()))
}

func TestLintValidations_Rule4_ComparisonLiteralOnLeft(t *testing.T) {
	// Tests the flipped-operand path: `literal <op> self.x`.
	rules := []*pb.ValidationRule{
		{Rule: "0.0 < self.payments.min_amount", Message: "x"},
	}
	err := LintValidations(rules, showcaseFields())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use exclusiveMinimum: 0 on the field")
}

func TestLintValidations_Rule4_ComparisonLiteralOnLeft_LE(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: "0.0 <= self.payments.min_amount", Message: "x"},
	}
	err := LintValidations(rules, showcaseFields())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use minimum: 0 on the field")
}

func TestLintValidations_Rule4_LessThanComparisonLiteralOnRight(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: "self.payments.max_amount < 1000.0", Message: "x"},
	}
	err := LintValidations(rules, showcaseFields())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use exclusiveMaximum: 1000 on the field")
}

func TestLintValidations_Rule4_LessThanEqualComparisonLiteralOnRight(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: "self.payments.max_amount <= 1000.0", Message: "x"},
	}
	err := LintValidations(rules, showcaseFields())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use maximum: 1000 on the field")
}

func TestLintValidations_Rule4_EqualityLiteralOnLeft(t *testing.T) {
	rules := []*pb.ValidationRule{
		{Rule: `true == self.payments.refunds_enabled`, Message: "x"},
	}
	err := LintValidations(rules, showcaseFields())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "use enum: [true]")
}
