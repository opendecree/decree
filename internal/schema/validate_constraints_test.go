package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func pf(v float64) *float64 { return &v }
func pi(v int32) *int32     { return &v }
func ps(v string) *string   { return &v }

// --- Valid constraint/type combinations ---

func TestValidateConstraints_IntegerMinMax(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_INT,
		Constraints: &pb.FieldConstraints{Min: pf(0), Max: pf(10)},
	})
	require.NoError(t, err)
}

func TestValidateConstraints_NumberExclusive(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_NUMBER,
		Constraints: &pb.FieldConstraints{ExclusiveMin: pf(0), ExclusiveMax: pf(1)},
	})
	require.NoError(t, err)
}

func TestValidateConstraints_StringLength(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_STRING,
		Constraints: &pb.FieldConstraints{MinLength: pi(2), MaxLength: pi(50)},
	})
	require.NoError(t, err)
}

func TestValidateConstraints_StringPattern(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_STRING,
		Constraints: &pb.FieldConstraints{Regex: ps("^[A-Z]+$")},
	})
	require.NoError(t, err)
}

func TestValidateConstraints_DurationMinMax(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_DURATION,
		Constraints: &pb.FieldConstraints{Min: pf(1), Max: pf(3600)},
	})
	require.NoError(t, err)
}

func TestValidateConstraints_JSONSchema(t *testing.T) {
	schema := `{"type":"object"}`
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_JSON,
		Constraints: &pb.FieldConstraints{JsonSchema: &schema},
	})
	require.NoError(t, err)
}

func TestValidateConstraints_EnumOnAnyType(t *testing.T) {
	// Enum is valid on any type.
	for _, ft := range []pb.FieldType{
		pb.FieldType_FIELD_TYPE_INT,
		pb.FieldType_FIELD_TYPE_STRING,
		pb.FieldType_FIELD_TYPE_BOOL,
	} {
		err := validateFieldConstraints(&pb.SchemaField{
			Path: "x", Type: ft,
			Constraints: &pb.FieldConstraints{EnumValues: []string{"a", "b"}},
		})
		require.NoError(t, err, "enum should be valid on %s", ft)
	}
}

func TestValidateConstraints_NoConstraints(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_BOOL,
	})
	require.NoError(t, err)
}

// --- Invalid constraint/type combinations ---

func TestValidateConstraints_MinOnString_Rejected(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_STRING,
		Constraints: &pb.FieldConstraints{Min: pf(0)},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "minimum")
	assert.Contains(t, err.Error(), "not valid")
}

func TestValidateConstraints_MinLengthOnInteger_Rejected(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_INT,
		Constraints: &pb.FieldConstraints{MinLength: pi(2)},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "minLength")
}

func TestValidateConstraints_PatternOnBool_Rejected(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_BOOL,
		Constraints: &pb.FieldConstraints{Regex: ps("^true$")},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pattern")
}

func TestValidateConstraints_JsonSchemaOnString_Rejected(t *testing.T) {
	schema := `{"type":"object"}`
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_STRING,
		Constraints: &pb.FieldConstraints{JsonSchema: &schema},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "json_schema")
}

func TestValidateConstraints_ExclusiveMinOnBool_Rejected(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_BOOL,
		Constraints: &pb.FieldConstraints{ExclusiveMin: pf(0)},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exclusiveMinimum")
}

func TestValidateConstraints_MaxOnURL_Rejected(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_URL,
		Constraints: &pb.FieldConstraints{Max: pf(100)},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum")
}

// --- Range sanity checks ---

func TestValidateConstraints_MinGreaterThanMax_Rejected(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_INT,
		Constraints: &pb.FieldConstraints{Min: pf(10), Max: pf(5)},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "greater than maximum")
}

func TestValidateConstraints_ExclusiveMinEqualToMax_Rejected(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_NUMBER,
		Constraints: &pb.FieldConstraints{ExclusiveMin: pf(5), ExclusiveMax: pf(5)},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be less than")
}

func TestValidateConstraints_MinLengthGreaterThanMaxLength_Rejected(t *testing.T) {
	err := validateFieldConstraints(&pb.SchemaField{
		Path: "x", Type: pb.FieldType_FIELD_TYPE_STRING,
		Constraints: &pb.FieldConstraints{MinLength: pi(10), MaxLength: pi(5)},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "minLength")
}

// --- Prefix-overlap (cross-field) ---

func mkField(path string) *pb.SchemaField {
	return &pb.SchemaField{Path: path, Type: pb.FieldType_FIELD_TYPE_STRING}
}

func TestValidateNoPrefixOverlap_Empty(t *testing.T) {
	require.NoError(t, validateNoPrefixOverlap(nil))
	require.NoError(t, validateNoPrefixOverlap([]*pb.SchemaField{}))
}

func TestValidateNoPrefixOverlap_Single(t *testing.T) {
	require.NoError(t, validateNoPrefixOverlap([]*pb.SchemaField{mkField("payments")}))
}

func TestValidateNoPrefixOverlap_NoOverlap(t *testing.T) {
	require.NoError(t, validateNoPrefixOverlap([]*pb.SchemaField{
		mkField("payments.fee"),
		mkField("payments.window"),
		mkField("billing.invoice"),
	}))
}

func TestValidateNoPrefixOverlap_Direct(t *testing.T) {
	err := validateNoPrefixOverlap([]*pb.SchemaField{
		mkField("payments"),
		mkField("payments.fee"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"payments"`)
	assert.Contains(t, err.Error(), `"payments.fee"`)
}

func TestValidateNoPrefixOverlap_DeepGap(t *testing.T) {
	// "payments" is a strict prefix of "payments.refunds.window" even though
	// the intermediate path "payments.refunds" is not declared.
	err := validateNoPrefixOverlap([]*pb.SchemaField{
		mkField("payments"),
		mkField("payments.refunds.window"),
	})
	require.Error(t, err)
}

func TestValidateNoPrefixOverlap_TransitiveSiblings(t *testing.T) {
	// Sibling siblings are fine; only ancestor-descendant overlap fails.
	err := validateNoPrefixOverlap([]*pb.SchemaField{
		mkField("a.b"),
		mkField("a.b.c"),
		mkField("a.b.d"),
	})
	require.Error(t, err)
	// First sorted pair (a.b, a.b.c) trips the check.
	assert.Contains(t, err.Error(), `"a.b"`)
	assert.Contains(t, err.Error(), `"a.b.c"`)
}

func TestValidateNoPrefixOverlap_SubstringNotPrefix(t *testing.T) {
	// "payment" and "payments" share a string prefix but no path-segment
	// boundary — must NOT be flagged.
	require.NoError(t, validateNoPrefixOverlap([]*pb.SchemaField{
		mkField("payment"),
		mkField("payments"),
	}))
}

func TestValidateNoPrefixOverlap_SiblingsWithCommonPrefix(t *testing.T) {
	// "payments.fee" and "payments.fees" — neither is an ancestor of the other.
	require.NoError(t, validateNoPrefixOverlap([]*pb.SchemaField{
		mkField("payments.fee"),
		mkField("payments.fees"),
	}))
}

func TestValidateNoPrefixOverlap_UnsortedInput(t *testing.T) {
	// Conflict must be caught regardless of input order.
	err := validateNoPrefixOverlap([]*pb.SchemaField{
		mkField("payments.refunds.window"),
		mkField("billing"),
		mkField("payments"),
	})
	require.Error(t, err)
}
