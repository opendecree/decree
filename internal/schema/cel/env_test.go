package cel

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func TestBuildEnv_CompilesShowcaseRules(t *testing.T) {
	env, err := BuildEnv(showcaseFields())
	require.NoError(t, err)

	cases := []struct {
		name string
		rule string
	}{
		{"cross-field comparison", "self.payments.min_amount < self.payments.max_amount"},
		{"hyphenated segment via index", `self["app-name"].foo == "bar"`},
		{"unknown path tolerated at compile", "self.does.not.exist > 0"},
		{"tenant binding", `tenant.id != ""`},
		{"ternary on null check", "has(self.payments.refund_window) ? self.payments.refund_window > duration('0s') : true"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ast, issues := env.Compile(tc.rule)
			require.Nil(t, issues.Err(), "compile must succeed: %v", issues.Err())
			require.NotNil(t, ast)
		})
	}
}

func TestBuildEnv_RejectsSyntaxErrors(t *testing.T) {
	env, err := BuildEnv(nil)
	require.NoError(t, err)

	_, issues := env.Compile("self.payments.min_amount <")
	require.NotNil(t, issues)
	require.Error(t, issues.Err())
}

func TestCelTypeFor(t *testing.T) {
	cases := []struct {
		ft   pb.FieldType
		want *cel.Type
	}{
		{pb.FieldType_FIELD_TYPE_INT, cel.IntType},
		{pb.FieldType_FIELD_TYPE_NUMBER, cel.DoubleType},
		{pb.FieldType_FIELD_TYPE_STRING, cel.StringType},
		{pb.FieldType_FIELD_TYPE_URL, cel.StringType},
		{pb.FieldType_FIELD_TYPE_BOOL, cel.BoolType},
		{pb.FieldType_FIELD_TYPE_TIME, cel.TimestampType},
		{pb.FieldType_FIELD_TYPE_DURATION, cel.DurationType},
		{pb.FieldType_FIELD_TYPE_JSON, cel.DynType},
		{pb.FieldType_FIELD_TYPE_UNSPECIFIED, cel.DynType},
	}

	for _, tc := range cases {
		t.Run(tc.ft.String(), func(t *testing.T) {
			assert.Equal(t, tc.want, celTypeFor(tc.ft))
		})
	}
}

func TestSelfDescriptor_KeysEveryFieldPath(t *testing.T) {
	descriptor := selfDescriptor(showcaseFields())

	require.Len(t, descriptor, 4)
	assert.Equal(t, cel.DoubleType, descriptor["payments.min_amount"])
	assert.Equal(t, cel.DoubleType, descriptor["payments.max_amount"])
	assert.Equal(t, cel.BoolType, descriptor["payments.refunds_enabled"])
	assert.Equal(t, cel.DurationType, descriptor["payments.refund_window"])
}

func showcaseFields() []*pb.SchemaField {
	return []*pb.SchemaField{
		{Path: "payments.min_amount", Type: pb.FieldType_FIELD_TYPE_NUMBER},
		{Path: "payments.max_amount", Type: pb.FieldType_FIELD_TYPE_NUMBER},
		{Path: "payments.refunds_enabled", Type: pb.FieldType_FIELD_TYPE_BOOL},
		{Path: "payments.refund_window", Type: pb.FieldType_FIELD_TYPE_DURATION, Nullable: true},
	}
}
