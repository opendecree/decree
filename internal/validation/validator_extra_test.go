package validation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

func TestFieldValidator_DomainFieldTypeAndSensitive(t *testing.T) {
	v := NewFieldValidator("secret.token", pb.FieldType_FIELD_TYPE_STRING, WithSensitive())
	assert.Equal(t, domain.FieldTypeString, v.DomainFieldType())
	assert.True(t, v.Sensitive())

	plain := NewFieldValidator("plain", pb.FieldType_FIELD_TYPE_INT)
	assert.Equal(t, domain.FieldTypeInteger, plain.DomainFieldType())
	assert.False(t, plain.Sensitive())
}

func TestTypedValueToString(t *testing.T) {
	tests := []struct {
		name string
		tv   *pb.TypedValue
		want string
	}{
		{"nil", nil, ""},
		{"nil kind hits default", &pb.TypedValue{}, ""},
		{"integer", &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 5}}, "5"},
		{"number", &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 2.5}}, "2.5"},
		{"string", &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hi"}}, "hi"},
		{"bool", &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}, "true"},
		{"url", &pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://x.io"}}, "https://x.io"},
		{"json", &pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: "[1]"}}, "[1]"},
		{"time nil inner", &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{}}, ""},
		{
			"time with value",
			&pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))}},
			"2024-01-02T03:04:05Z",
		},
		{"duration nil inner", &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{}}, ""},
		{
			"duration with value",
			&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(90 * time.Second)}},
			"1m30s",
		},
		{
			"large number uses f format not g",
			&pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 1_000_000}},
			"1000000",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, typedValueToString(tt.tv))
		})
	}
}

func TestCheckType(t *testing.T) {
	matching := map[pb.FieldType]*pb.TypedValue{
		pb.FieldType_FIELD_TYPE_INT:      {Kind: &pb.TypedValue_IntegerValue{}},
		pb.FieldType_FIELD_TYPE_NUMBER:   {Kind: &pb.TypedValue_NumberValue{}},
		pb.FieldType_FIELD_TYPE_STRING:   {Kind: &pb.TypedValue_StringValue{}},
		pb.FieldType_FIELD_TYPE_BOOL:     {Kind: &pb.TypedValue_BoolValue{}},
		pb.FieldType_FIELD_TYPE_TIME:     {Kind: &pb.TypedValue_TimeValue{}},
		pb.FieldType_FIELD_TYPE_DURATION: {Kind: &pb.TypedValue_DurationValue{}},
		pb.FieldType_FIELD_TYPE_URL:      {Kind: &pb.TypedValue_UrlValue{}},
		pb.FieldType_FIELD_TYPE_JSON:     {Kind: &pb.TypedValue_JsonValue{}},
	}
	for ft, tv := range matching {
		t.Run("match_"+ft.String(), func(t *testing.T) {
			assert.NoError(t, checkType(tv, ft))
		})
	}

	// A string value mismatches every non-string expected type.
	stringVal := &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "x"}}
	for ft := range matching {
		if ft == pb.FieldType_FIELD_TYPE_STRING {
			continue
		}
		t.Run("mismatch_"+ft.String(), func(t *testing.T) {
			assert.Error(t, checkType(stringVal, ft))
		})
	}
}
