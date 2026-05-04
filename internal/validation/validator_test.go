package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func ptr[T any](v T) *T { return &v }

// --- Type checking ---

func TestValidate_TypeCheck_IntegerField(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, false, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hello"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}))
}

func TestValidate_TypeCheck_BoolField(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_BOOL, false, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 1}}))
}

func TestValidate_TypeCheck_StringField(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, false, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: ""}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 0}}))
}

func TestValidate_TypeCheck_TimeField(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_TIME, false, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.Now()}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "not-a-time"}}))
}

func TestValidate_TypeCheck_DurationField(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_DURATION, false, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(0)}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "24h"}}))
}

// --- Nullable ---

func TestValidate_NullOnNonNullable(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, false, false, nil)
	assert.Error(t, v.Validate(nil))
}

func TestValidate_NullOnNullable(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, true, false, nil)
	require.NoError(t, v.Validate(nil))
}

func TestValidate_NilKindOnNonNullable(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, false, false, nil)
	assert.Error(t, v.Validate(&pb.TypedValue{}))
}

// --- Integer constraints ---

func TestValidate_IntegerMinMax(t *testing.T) {
	v := NewFieldValidator("retries", pb.FieldType_FIELD_TYPE_INT, false, false, &pb.FieldConstraints{
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
	v := NewFieldValidator("rate", pb.FieldType_FIELD_TYPE_NUMBER, false, false, &pb.FieldConstraints{
		Min: ptr(float64(0)),
		Max: ptr(float64(1)),
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 0.5}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: -0.1}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 1.1}}))
}

// --- String constraints ---

func TestValidate_StringMinMaxLength(t *testing.T) {
	v := NewFieldValidator("name", pb.FieldType_FIELD_TYPE_STRING, false, false, &pb.FieldConstraints{
		MinLength: ptr(int32(2)),
		MaxLength: ptr(int32(10)),
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hello"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "x"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "this is too long"}}))
}

func TestValidate_StringPattern(t *testing.T) {
	v := NewFieldValidator("email", pb.FieldType_FIELD_TYPE_STRING, false, false, &pb.FieldConstraints{
		Regex: ptr(`^[^@]+@[^@]+$`),
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "user@example.com"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "not-an-email"}}))
}

// --- Enum constraints ---

func TestValidate_Enum(t *testing.T) {
	v := NewFieldValidator("currency", pb.FieldType_FIELD_TYPE_STRING, false, false, &pb.FieldConstraints{
		EnumValues: []string{"USD", "EUR", "GBP"},
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "USD"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "ILS"}}))
}

func TestValidate_EnumOnInteger(t *testing.T) {
	v := NewFieldValidator("level", pb.FieldType_FIELD_TYPE_INT, false, false, &pb.FieldConstraints{
		EnumValues: []string{"1", "2", "3"},
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 1}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 5}}))
}

// --- Duration constraints ---

func TestValidate_DurationMinMax(t *testing.T) {
	v := NewFieldValidator("timeout", pb.FieldType_FIELD_TYPE_DURATION, false, false, &pb.FieldConstraints{
		Min: ptr(float64(1)),    // 1 second
		Max: ptr(float64(3600)), // 1 hour
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(60_000_000_000)}})) // 60s
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(500_000_000)}}))       // 0.5s
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(7200_000_000_000)}}))  // 2h
}

// --- Exclusive min/max ---

func TestValidate_ExclusiveMinMax(t *testing.T) {
	v := NewFieldValidator("rate", pb.FieldType_FIELD_TYPE_NUMBER, false, false, &pb.FieldConstraints{
		ExclusiveMin: ptr(float64(0)),
		ExclusiveMax: ptr(float64(1)),
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 0.5}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 0}}))        // not > 0
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 1}}))        // not < 1
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 0.001}})) // just above 0
}

func TestValidate_ExclusiveMinMax_Integer(t *testing.T) {
	v := NewFieldValidator("level", pb.FieldType_FIELD_TYPE_INT, false, false, &pb.FieldConstraints{
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
	v := NewFieldValidator("webhook", pb.FieldType_FIELD_TYPE_URL, false, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://example.com/hook"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "not-a-url"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "/relative/path"}}))
}

// --- JSON Schema validation ---

func TestValidate_JSONSchema(t *testing.T) {
	schema := `{"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}`
	v := NewFieldValidator("metadata", pb.FieldType_FIELD_TYPE_JSON, false, false, &pb.FieldConstraints{
		JsonSchema: &schema,
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `{"name": "test"}`}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `{"foo": "bar"}`}})) // missing required "name"
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `not json`}}))
}

// --- No constraints (type check only) ---

func TestValidate_NoConstraints(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, false, false, nil)
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "anything"}}))
}

// --- Sensitive field error suppression ---

func TestValidate_Sensitive_URL_ErrorOmitsValue(t *testing.T) {
	v := NewFieldValidator("secret.webhook", pb.FieldType_FIELD_TYPE_URL, false, true, nil)

	err := v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "not-a-url"}})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "not-a-url")
	assert.Contains(t, err.Error(), "not a valid absolute URL")
}

func TestValidate_Sensitive_Regex_ErrorOmitsValue(t *testing.T) {
	v := NewFieldValidator("secret.token", pb.FieldType_FIELD_TYPE_STRING, false, true, &pb.FieldConstraints{
		Regex: ptr(`^\d{4}$`),
	})

	err := v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hunter2"}})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "hunter2")
	assert.Contains(t, err.Error(), "does not match pattern")
}

func TestValidate_Sensitive_Enum_ErrorOmitsValue(t *testing.T) {
	v := NewFieldValidator("secret.tier", pb.FieldType_FIELD_TYPE_STRING, false, true, &pb.FieldConstraints{
		EnumValues: []string{"gold", "silver"},
	})

	err := v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "bronze"}})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "bronze")
	assert.Contains(t, err.Error(), "not in allowed values")
}

func TestValidate_NonSensitive_ErrorIncludesValue(t *testing.T) {
	v := NewFieldValidator("public.tier", pb.FieldType_FIELD_TYPE_STRING, false, false, &pb.FieldConstraints{
		EnumValues: []string{"gold", "silver"},
	})

	err := v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "bronze"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bronze")
}

// --- Cache ---

func TestValidatorCache(t *testing.T) {
	c := NewValidatorCache(0)

	_, ok := c.Get("t1")
	assert.False(t, ok)

	validators := map[string]*FieldValidator{"x": NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, false, false, nil)}
	c.Set("t1", validators)

	got, ok := c.Get("t1")
	require.True(t, ok)
	assert.Len(t, got, 1)

	c.Invalidate("t1")
	_, ok = c.Get("t1")
	assert.False(t, ok)
}

func TestValidatorCache_EvictsOldestWhenFull(t *testing.T) {
	c := NewValidatorCache(2)

	v1 := map[string]*FieldValidator{"x": NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, false, false, nil)}
	v2 := map[string]*FieldValidator{"x": NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, false, false, nil)}
	v3 := map[string]*FieldValidator{"x": NewFieldValidator("x", pb.FieldType_FIELD_TYPE_BOOL, false, false, nil)}

	c.Set("t1", v1)
	c.Set("t2", v2)
	assert.Equal(t, 2, c.Len())

	// Adding t3 should evict t1 (oldest).
	c.Set("t3", v3)
	assert.Equal(t, 2, c.Len())

	_, ok := c.Get("t1")
	assert.False(t, ok, "oldest tenant should be evicted")

	_, ok = c.Get("t2")
	assert.True(t, ok)

	_, ok = c.Get("t3")
	assert.True(t, ok)
}

func TestValidatorCache_UpdateExistingDoesNotGrow(t *testing.T) {
	c := NewValidatorCache(2)

	v1 := map[string]*FieldValidator{"x": NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, false, false, nil)}
	v2 := map[string]*FieldValidator{"x": NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, false, false, nil)}

	c.Set("t1", v1)
	c.Set("t2", v2)

	// Update t1 — should not trigger eviction.
	v1Updated := map[string]*FieldValidator{"y": NewFieldValidator("y", pb.FieldType_FIELD_TYPE_BOOL, false, false, nil)}
	c.Set("t1", v1Updated)
	assert.Equal(t, 2, c.Len())

	got, ok := c.Get("t1")
	require.True(t, ok)
	assert.Contains(t, got, "y")

	_, ok = c.Get("t2")
	assert.True(t, ok, "t2 should not be evicted by update")
}
