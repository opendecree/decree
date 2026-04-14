package validation

import (
	"testing"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

func ptr64(v float64) *float64 { return &v }

func BenchmarkValidate_Integer_NoConstraints(b *testing.B) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, false, nil)
	tv := &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}
	for b.Loop() {
		_ = v.Validate(tv)
	}
}

func BenchmarkValidate_Integer_MinMax(b *testing.B) {
	v := NewFieldValidator("x", pb.FieldType_FIELD_TYPE_INT, false, &pb.FieldConstraints{
		Min: ptr64(0),
		Max: ptr64(100),
	})
	tv := &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}
	for b.Loop() {
		_ = v.Validate(tv)
	}
}

func BenchmarkValidate_String_Pattern(b *testing.B) {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	v := NewFieldValidator("email", pb.FieldType_FIELD_TYPE_STRING, false, &pb.FieldConstraints{
		Regex: &pattern,
	})
	tv := &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "user@example.com"}}
	for b.Loop() {
		_ = v.Validate(tv)
	}
}

func BenchmarkValidate_String_Enum(b *testing.B) {
	v := NewFieldValidator("env", pb.FieldType_FIELD_TYPE_STRING, false, &pb.FieldConstraints{
		EnumValues: []string{"dev", "staging", "prod"},
	})
	tv := &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "prod"}}
	for b.Loop() {
		_ = v.Validate(tv)
	}
}

func BenchmarkValidate_URL(b *testing.B) {
	v := NewFieldValidator("hook", pb.FieldType_FIELD_TYPE_URL, false, nil)
	tv := &pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://example.com/webhook"}}
	for b.Loop() {
		_ = v.Validate(tv)
	}
}

func BenchmarkValidate_JSON_Schema(b *testing.B) {
	schema := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	v := NewFieldValidator("meta", pb.FieldType_FIELD_TYPE_JSON, false, &pb.FieldConstraints{
		JsonSchema: &schema,
	})
	tv := &pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `{"name":"test"}`}}
	for b.Loop() {
		_ = v.Validate(tv)
	}
}

func BenchmarkNewFieldValidator_WithConstraints(b *testing.B) {
	pattern := `^\d+$`
	constraints := &pb.FieldConstraints{
		Min:   ptr64(0),
		Max:   ptr64(100),
		Regex: &pattern,
	}
	for b.Loop() {
		NewFieldValidator("x", pb.FieldType_FIELD_TYPE_STRING, false, constraints)
	}
}

func BenchmarkValidatorCache_Get_Hit(b *testing.B) {
	c := NewValidatorCache()
	validators := map[string]*FieldValidator{
		"a": NewFieldValidator("a", pb.FieldType_FIELD_TYPE_STRING, false, nil),
		"b": NewFieldValidator("b", pb.FieldType_FIELD_TYPE_INT, false, nil),
	}
	c.Set("tenant-1", validators)

	for b.Loop() {
		c.Get("tenant-1")
	}
}

func BenchmarkValidatorCache_Get_Hit_Parallel(b *testing.B) {
	c := NewValidatorCache()
	validators := map[string]*FieldValidator{
		"a": NewFieldValidator("a", pb.FieldType_FIELD_TYPE_STRING, false, nil),
		"b": NewFieldValidator("b", pb.FieldType_FIELD_TYPE_INT, false, nil),
	}
	c.Set("tenant-1", validators)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Get("tenant-1")
		}
	})
}
