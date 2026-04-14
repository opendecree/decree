package configwatcher

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func TestTypedValueToString(t *testing.T) {
	tests := []struct {
		name     string
		input    *pb.TypedValue
		expected string
	}{
		{"nil", nil, ""},
		{"string", &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hello"}}, "hello"},
		{"integer", &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}, "42"},
		{"number", &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 3.14}}, "3.14"},
		{"bool true", &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}, "true"},
		{"bool false", &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: false}}, "false"},
		{"url", &pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://example.com"}}, "https://example.com"},
		{"json", &pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `{"key":"val"}`}}, `{"key":"val"}`},
		{"duration", &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(30 * time.Second)}}, "30s"},
		{"duration nil", &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{}}, ""},
		{"time nil", &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{}}, ""},
		{"nil kind", &pb.TypedValue{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, typedValueToString(tt.input))
		})
	}
}

func TestTypedValueToString_Time(t *testing.T) {
	ts := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	tv := &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(ts)}}
	result := typedValueToString(tv)
	assert.Contains(t, result, "2026-03-30")
}

func TestParseFunctions(t *testing.T) {
	t.Run("parseString", func(t *testing.T) {
		v, err := parseString("hello")
		assert.NoError(t, err)
		assert.Equal(t, "hello", v)
	})

	t.Run("parseInt valid", func(t *testing.T) {
		v, err := parseInt("42")
		assert.NoError(t, err)
		assert.Equal(t, int64(42), v)
	})

	t.Run("parseInt invalid", func(t *testing.T) {
		_, err := parseInt("abc")
		assert.Error(t, err)
	})

	t.Run("parseFloat valid", func(t *testing.T) {
		v, err := parseFloat("3.14")
		assert.NoError(t, err)
		assert.Equal(t, 3.14, v)
	})

	t.Run("parseFloat invalid", func(t *testing.T) {
		_, err := parseFloat("abc")
		assert.Error(t, err)
	})

	t.Run("parseBool valid", func(t *testing.T) {
		v, err := parseBool("true")
		assert.NoError(t, err)
		assert.True(t, v)
	})

	t.Run("parseBool invalid", func(t *testing.T) {
		_, err := parseBool("maybe")
		assert.Error(t, err)
	})

	t.Run("parseDuration valid", func(t *testing.T) {
		v, err := parseDuration("5m30s")
		assert.NoError(t, err)
		assert.Equal(t, 5*time.Minute+30*time.Second, v)
	})

	t.Run("parseDuration invalid", func(t *testing.T) {
		_, err := parseDuration("nope")
		assert.Error(t, err)
	})
}
