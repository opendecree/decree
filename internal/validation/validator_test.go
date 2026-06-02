package validation

import (
	"math"
	"strings"
	"testing"
	"time"

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

func TestValidate_InvalidRegexInDB_FailsClosed(t *testing.T) {
	// Simulates a bad pattern written directly to the DB bypassing schema validation.
	// The validator must fail closed: every value is rejected, not silently accepted.
	v := NewFieldValidator("field", pb.FieldType_FIELD_TYPE_STRING, false, false, &pb.FieldConstraints{
		Regex: ptr(`[invalid`),
	})

	// Fail closed: broken constraint rejects all values, not accepts everything.
	err := v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "any value"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "constraint cannot be enforced")
}

func TestValidate_InvalidRegex_IncrementsCounter(t *testing.T) {
	ctr := &countingCounter{}
	NewFieldValidator("field", pb.FieldType_FIELD_TYPE_STRING, false, false, &pb.FieldConstraints{
		Regex: ptr(`[invalid`),
	}, WithRegexErrorCounter(ctr))
	assert.Equal(t, int64(1), ctr.n.Load())
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

func TestValidate_EnumOnLargeNumber(t *testing.T) {
	// Storage writes 1000000 as "1000000" (FormatFloat 'f'), not "1e+06" (%g).
	// Enum comparison must use the same format or valid stored values are falsely rejected.
	v := NewFieldValidator("quota", pb.FieldType_FIELD_TYPE_NUMBER, false, false, &pb.FieldConstraints{
		EnumValues: []string{"1000000", "2000000"},
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 1_000_000}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 3_000_000}}))
}

func TestValidate_EnumOnDuration(t *testing.T) {
	// Enum on a duration field: stored values use Duration.String() format.
	v := NewFieldValidator("ttl", pb.FieldType_FIELD_TYPE_DURATION, false, false, &pb.FieldConstraints{
		EnumValues: []string{"1m30s", "5m0s"},
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(90 * time.Second)}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(10 * time.Second)}}))
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

// --- URL scheme allowlist ---

func TestValidate_URL_DefaultSchemeAllowlist(t *testing.T) {
	v := NewFieldValidator("webhook", pb.FieldType_FIELD_TYPE_URL, false, false, nil)

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://example.com"}}))
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "http://example.com"}}))

	for _, blocked := range []string{
		"gopher://example.com",
		"file:///etc/passwd",
		"ftp://files.example.com",
		"javascript:alert(1)",
		"data:text/html,<h1>x</h1>",
	} {
		err := v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: blocked}})
		assert.Errorf(t, err, "expected error for scheme in %q", blocked)
		assert.Contains(t, err.Error(), "not in the allowed list")
	}
}

func TestValidate_URL_CustomSchemeAllowlist(t *testing.T) {
	v := NewFieldValidator("s3path", pb.FieldType_FIELD_TYPE_URL, false, false, &pb.FieldConstraints{
		AllowedSchemes: []string{"s3", "gs"},
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "s3://my-bucket/key"}}))
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "gs://my-bucket/key"}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://example.com"}}))
}

func TestValidate_URL_SensitiveSchemeError(t *testing.T) {
	v := NewFieldValidator("secret.url", pb.FieldType_FIELD_TYPE_URL, false, true, nil)

	err := v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "gopher://example.com"}})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "gopher://example.com")
	assert.Contains(t, err.Error(), "not in the allowed list")
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

func TestValidate_InvalidJSONSchemaInDB_FailsClosed(t *testing.T) {
	// Simulates a malformed json_schema written directly to the DB bypassing schema validation.
	// The validator must fail closed: every value is rejected, not silently accepted.
	bad := `{not valid json`
	v := NewFieldValidator("meta", pb.FieldType_FIELD_TYPE_JSON, false, false, &pb.FieldConstraints{
		JsonSchema: &bad,
	})

	// Fail closed: broken constraint rejects all values, not accepts everything.
	err := v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `{"any": "value"}`}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "constraint cannot be enforced")
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

// --- Unicode / rune-based string length ---

func TestValidate_StringLength_Unicode(t *testing.T) {
	v := NewFieldValidator("label", pb.FieldType_FIELD_TYPE_STRING, false, false, &pb.FieldConstraints{
		MinLength: ptr(int32(2)),
		MaxLength: ptr(int32(3)),
	})

	// Each emoji is 1 rune but 4 bytes — byte count would produce wrong results.
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "🎉🎉"}}))  // 2 runes, 8 bytes
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "🎉🎉🎉"}})) // 3 runes, 12 bytes
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "🎉"}}))      // 1 rune  — below min
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "🎉🎉🎉🎉"}}))   // 4 runes — above max

	// Multi-byte Unicode: "日本語" = 3 runes, 9 bytes.
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "日本語"}}))

	// Accented ASCII: "héllo" = 5 runes, 6 bytes — above max.
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "héllo"}}))
}

// --- String exactly at MaxLength boundary ---

func TestValidate_StringLength_ExactBoundary(t *testing.T) {
	v := NewFieldValidator("desc", pb.FieldType_FIELD_TYPE_STRING, false, false, &pb.FieldConstraints{
		MaxLength: ptr(int32(5)),
	})

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hello"}})) // exactly 5
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "helloo"}}))   // 6 — one over
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hell"}}))  // 4 — one under
}

// --- math.MaxInt64 / MaxFloat64 / very small floats ---

func TestValidate_Integer_MaxInt64(t *testing.T) {
	v := NewFieldValidator("count", pb.FieldType_FIELD_TYPE_INT, false, false, nil)
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: math.MaxInt64}}))
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: math.MinInt64}}))
}

func TestValidate_Integer_MaxInt64_WithMax(t *testing.T) {
	v := NewFieldValidator("count", pb.FieldType_FIELD_TYPE_INT, false, false, &pb.FieldConstraints{
		Max: ptr(float64(math.MaxInt64)),
	})
	// MaxInt64 as float64 rounds up slightly, so the integer MaxInt64 satisfies the constraint.
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: math.MaxInt64}}))
}

func TestValidate_Number_MaxFloat64(t *testing.T) {
	v := NewFieldValidator("big", pb.FieldType_FIELD_TYPE_NUMBER, false, false, nil)
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: math.MaxFloat64}}))
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: -math.MaxFloat64}}))
}

func TestValidate_Number_SmallFloat(t *testing.T) {
	v := NewFieldValidator("tiny", pb.FieldType_FIELD_TYPE_NUMBER, false, false, nil)
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: math.SmallestNonzeroFloat64}}))
	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: -math.SmallestNonzeroFloat64}}))
}

// --- NaN / Inf ---

func TestValidate_Number_NaN_Rejected(t *testing.T) {
	v := NewFieldValidator("rate", pb.FieldType_FIELD_TYPE_NUMBER, false, false, nil)
	err := v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: math.NaN()}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finite")
}

func TestValidate_Number_PosInf_Rejected(t *testing.T) {
	v := NewFieldValidator("rate", pb.FieldType_FIELD_TYPE_NUMBER, false, false, nil)
	err := v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: math.Inf(1)}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finite")
}

func TestValidate_Number_NegInf_Rejected(t *testing.T) {
	v := NewFieldValidator("rate", pb.FieldType_FIELD_TYPE_NUMBER, false, false, nil)
	err := v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: math.Inf(-1)}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "finite")
}

func TestValidate_Number_NaN_WithConstraints_Rejected(t *testing.T) {
	// NaN must be rejected even when min/max constraints are present, because
	// NaN comparisons are always false and would otherwise silently pass.
	v := NewFieldValidator("rate", pb.FieldType_FIELD_TYPE_NUMBER, false, false, &pb.FieldConstraints{
		Min: ptr(float64(0)),
		Max: ptr(float64(1)),
	})
	require.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: math.NaN()}}))
}

// --- Nil TypedValue oneof ---

func TestValidate_NilKindOnNullable(t *testing.T) {
	// A TypedValue with no oneof set (Kind==nil) is treated as null for nullable fields.
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, true, false, nil)
	require.NoError(t, v.Validate(&pb.TypedValue{}))
}

func TestValidate_NilKindOnNullable_Number(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_NUMBER, true, false, nil)
	require.NoError(t, v.Validate(&pb.TypedValue{}))
}

func TestValidate_NilKindOnNullable_String(t *testing.T) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, true, false, nil)
	require.NoError(t, v.Validate(&pb.TypedValue{}))
}

// --- String at large boundary (doc-size scale) ---

func TestValidate_StringLength_LargeBoundary(t *testing.T) {
	// Verify boundary behaviour at a doc-size-scale length (5 MiB = 5242880 bytes).
	// All ASCII, so rune count == byte count.
	const limit = 5 * 1024 * 1024
	v := NewFieldValidator("blob", pb.FieldType_FIELD_TYPE_STRING, false, false, &pb.FieldConstraints{
		MaxLength: ptr(int32(limit)),
	})

	atLimit := strings.Repeat("a", limit)
	overLimit := atLimit + "a"

	require.NoError(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: atLimit}}))
	assert.Error(t, v.Validate(&pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: overLimit}}))
}
