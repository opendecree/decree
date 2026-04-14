package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

func ptr[T any](v T) *T { return &v }

// --- Type checking ---

func TestValidate_TypeCheck_IntegerField(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hello"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}))
}

func TestValidate_TypeCheck_BoolField(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_BOOL, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 1}}))
}

func TestValidate_TypeCheck_StringField(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: ""}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 0}}))
}

func TestValidate_TypeCheck_TimeField(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_TIME, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.Now()}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "not-a-time"}}))
}

func TestValidate_TypeCheck_DurationField(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_DURATION, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(0)}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "24h"}}))
}

// --- Nullable ---

func TestValidate_NullOnNonNullable(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, false, nil)
	assert.Error(t, v.Validate(nil))
}

func TestValidate_NullOnNullable(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, true, nil)
	require.NoError(t, v.Validate(nil))
}

func TestValidate_NilKindOnNonNullable(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, false, nil)
	assert.Error(t, v.Validate(&pb.TypedValue{}))
}

// --- Integer constraints ---

func TestValidate_IntegerMinMax(t *testing.T) {
	v := NewFieldValidator("retries", pb.FieldType_FIELD_TYPE_INT, false, &pb.FieldConstraints{
		Min: ptr(float64(0)),
		Max: ptr(float64(10)),
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 5}}))
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 0}}))
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 10}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: -1}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 11}}))
}

// --- Number constraints ---

func TestValidate_NumberMinMax(t *testing.T) {
	v := NewFieldValidator("rate", pb.FieldType_FIELD_TYPE_NUMBER, false, &pb.FieldConstraints{
		Min: ptr(float64(0)),
		Max: ptr(float64(1)),
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 0.5}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: -0.1}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 1.1}}))
}

// --- String constraints ---

func TestValidate_StringMinMaxLength(t *testing.T) {
	v := NewFieldValidator("name", pb.FieldType_FIELD_TYPE_STRING, false, &pb.FieldConstraints{
		MinLength: ptr(int32(2)),
		MaxLength: ptr(int32(10)),
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hello"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "x"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "this is too long"}}))
}

func TestValidate_StringPattern(t *testing.T) {
	v := NewFieldValidator("email", pb.FieldType_FIELD_TYPE_STRING, false, &pb.FieldConstraints{
		Regex: ptr(`^[^@]+@[^@]+$`),
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "user@example.com"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "not-an-email"}}))
}

// --- Enum constraints ---

func TestValidate_Enum(t *testing.T) {
	v := NewFieldValidator("currency", pb.FieldType_FIELD_TYPE_STRING, false, &pb.FieldConstraints{
		EnumValues: []string{"USD", "EUR", "GBP"},
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "USD"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "ILS"}}))
}

func TestValidate_EnumOnInteger(t *testing.T) {
	v := NewFieldValidator("level", pb.FieldType_FIELD_TYPE_INT, false, &pb.FieldConstraints{
		EnumValues: []string{"1", "2", "3"},
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 1}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 5}}))
}

// --- Duration constraints ---

func TestValidate_DurationMinMax(t *testing.T) {
	v := NewFieldValidator("timeout", pb.FieldType_FIELD_TYPE_DURATION, false, &pb.FieldConstraints{
		Min: ptr(float64(1)),    // 1 second
		Max: ptr(float64(3600)), // 1 hour
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(60_000_000_000)}})) // 60s
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(500_000_000)}}))       // 0.5s
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(7200_000_000_000)}}))  // 2h
}

// --- Exclusive min/max ---

func TestValidate_ExclusiveMinMax(t *testing.T) {
	v := NewFieldValidator("rate", pb.FieldType_FIELD_TYPE_NUMBER, false, &pb.FieldConstraints{
		ExclusiveMin: ptr(float64(0)),
		ExclusiveMax: ptr(float64(1)),
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 0.5}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 0}}))        // not > 0
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 1}}))        // not < 1
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 0.001}})) // just above 0
}

func TestValidate_ExclusiveMinMax_Integer(t *testing.T) {
	v := NewFieldValidator("level", pb.FieldType_FIELD_TYPE_INT, false, &pb.FieldConstraints{
		ExclusiveMin: ptr(float64(0)),
		ExclusiveMax: ptr(float64(10)),
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 5}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 0}}))  // not > 0
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 10}})) // not < 10
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 1}}))
}

// --- URL validation ---

func TestValidate_URL(t *testing.T) {
	v := NewFieldValidator("webhook", pb.FieldType_FIELD_TYPE_URL, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://example.com/hook"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "not-a-url"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "/relative/path"}}))
}

// --- JSON Schema validation ---

func TestValidate_JSONSchema(t *testing.T) {
	schema := `{"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}`
	v := NewFieldValidator("metadata", pb.FieldType_FIELD_TYPE_JSON, false, &pb.FieldConstraints{
		JsonSchema: &schema,
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `{"name": "test"}`}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `{"foo": "bar"}`}})) // missing required "name"
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `not json`}}))
}

// --- No constraints (type check only) ---

func TestValidate_NoConstraints(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, false, nil)
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "anything"}}))
}

// --- Cache ---

func TestValidatorCache(t *testing.T) {
	c := NewValidatorCache()

	_, ok := c.Get("t1")
	assert.False(t, ok)

	validators := map[string]*FieldValidator{"x": NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, false, nil)}
	c.Set("t1", validators)

	got, ok := c.Get("t1")
	require.True(t, ok)
	assert.Len(t, got, 1)

	c.Invalidate("t1")
	_, ok = c.Get("t1")
	assert.False(t, ok)
}
