package config

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

func TestComputeChecksum_Deterministic(t *testing.T) {
	c1 := computeChecksum("hello world")
	c2 := computeChecksum("hello world")
	assert.Equal(t, c1, c2)
}

func TestComputeChecksum_DifferentValues(t *testing.T) {
	c1 := computeChecksum("hello")
	c2 := computeChecksum("world")
	assert.NotEqual(t, c1, c2)
}

func TestPtrString_Empty(t *testing.T) {
	assert.Nil(t, ptrString(""))
}

func TestPtrString_NonEmpty(t *testing.T) {
	s := ptrString("test")
	require.NotNil(t, s)
	assert.Equal(t, "test", *s)
}

func TestStringToTypedValue_Nil(t *testing.T) {
	assert.Nil(t, stringToTypedValue(nil, domain.FieldTypeString))
}

func TestStringToTypedValue_AllTypes(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		ft     domain.FieldType
		assert func(t *testing.T, tv *pb.TypedValue)
	}{
		{"integer", "42", domain.FieldTypeInteger, func(t *testing.T, tv *pb.TypedValue) {
			assert.Equal(t, int64(42), tv.GetIntegerValue())
		}},
		{"number", "3.14", domain.FieldTypeNumber, func(t *testing.T, tv *pb.TypedValue) {
			assert.InEpsilon(t, 3.14, tv.GetNumberValue(), 1e-9)
		}},
		{"string", "hello", domain.FieldTypeString, func(t *testing.T, tv *pb.TypedValue) {
			assert.Equal(t, "hello", tv.GetStringValue())
		}},
		{"bool", "true", domain.FieldTypeBool, func(t *testing.T, tv *pb.TypedValue) {
			assert.True(t, tv.GetBoolValue())
		}},
		{"time", "2026-03-30T12:00:00Z", domain.FieldTypeTime, func(t *testing.T, tv *pb.TypedValue) {
			assert.Equal(t, "2026-03-30T12:00:00Z", tv.GetTimeValue().AsTime().UTC().Format(time.RFC3339))
		}},
		{"duration", "5m", domain.FieldTypeDuration, func(t *testing.T, tv *pb.TypedValue) {
			assert.Equal(t, 5*time.Minute, tv.GetDurationValue().AsDuration())
		}},
		{"url", "https://example.com", domain.FieldTypeURL, func(t *testing.T, tv *pb.TypedValue) {
			assert.Equal(t, "https://example.com", tv.GetUrlValue())
		}},
		{"json", `{"k":"v"}`, domain.FieldTypeJSON, func(t *testing.T, tv *pb.TypedValue) {
			assert.Equal(t, `{"k":"v"}`, tv.GetJsonValue())
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			tv := stringToTypedValue(&input, tt.ft)
			require.NotNil(t, tv)
			tt.assert(t, tv)
		})
	}
}

func TestStringToTypedValue_UnknownTypeFallsBackToString(t *testing.T) {
	tv := stringToTypedValue(strPtr("x"), domain.FieldType("mystery"))
	require.NotNil(t, tv)
	assert.Equal(t, "x", tv.GetStringValue())
}

func TestStringToTypedValue_ParseErrorsYieldZeroValues(t *testing.T) {
	// Malformed numeric/bool/time/duration strings: parse errors are logged and
	// the zero value is returned (never a nil TypedValue for a non-nil input).
	assert.Equal(t, int64(0), stringToTypedValue(strPtr("notint"), domain.FieldTypeInteger).GetIntegerValue())
	assert.Equal(t, float64(0), stringToTypedValue(strPtr("notfloat"), domain.FieldTypeNumber).GetNumberValue())
	assert.False(t, stringToTypedValue(strPtr("notbool"), domain.FieldTypeBool).GetBoolValue())
	assert.True(t, stringToTypedValue(strPtr("nottime"), domain.FieldTypeTime).GetTimeValue().AsTime().IsZero())
	assert.Equal(t, time.Duration(0), stringToTypedValue(strPtr("notdur"), domain.FieldTypeDuration).GetDurationValue().AsDuration())
}

func TestStringToTypedValue_ParseErrorsAreLogged(t *testing.T) {
	// Corrupt DB values must produce a log warning instead of being silently swallowed.
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	prev := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cases := []struct {
		raw string
		ft  domain.FieldType
	}{
		{"not-an-int", domain.FieldTypeInteger},
		{"not-a-float", domain.FieldTypeNumber},
		{"not-a-bool", domain.FieldTypeBool},
		{"not-a-time", domain.FieldTypeTime},
		{"not-a-duration", domain.FieldTypeDuration},
	}
	for _, c := range cases {
		buf.Reset()
		tv := stringToTypedValue(strPtr(c.raw), c.ft)
		require.NotNil(t, tv, "expected non-nil TypedValue for field type %s", c.ft)
		logged := buf.String()
		assert.Contains(t, logged, "stringToTypedValue", "expected warning log for field type %s", c.ft)
		assert.Contains(t, logged, c.raw, "expected raw value in log for field type %s", c.ft)
	}
}

func TestTypedValueToString_Nil(t *testing.T) {
	assert.Nil(t, typedValueToString(nil))
}

func TestTypedValueToString_AllKinds(t *testing.T) {
	ts := timestamppb.New(time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC))
	tests := []struct {
		name string
		tv   *pb.TypedValue
		want string
	}{
		{"integer", &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}, "42"},
		{"number", &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 3.5}}, "3.5"},
		{"string", &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "hi"}}, "hi"},
		{"bool", &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}, "true"},
		{"time", &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: ts}}, "2026-03-30T12:00:00Z"},
		{"duration", &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(90 * time.Second)}}, "1m30s"},
		{"url", &pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://x.io"}}, "https://x.io"},
		{"json", &pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: "[1,2]"}}, "[1,2]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typedValueToString(tt.tv)
			require.NotNil(t, got)
			assert.Equal(t, tt.want, *got)
		})
	}
}

func TestTypedValueToString_NilInnerTimeAndDuration(t *testing.T) {
	// Time/Duration kinds whose inner message is nil leave the string empty.
	gotT := typedValueToString(&pb.TypedValue{Kind: &pb.TypedValue_TimeValue{}})
	require.NotNil(t, gotT)
	assert.Equal(t, "", *gotT)
	gotD := typedValueToString(&pb.TypedValue{Kind: &pb.TypedValue_DurationValue{}})
	require.NotNil(t, gotD)
	assert.Equal(t, "", *gotD)
}

func TestTypedValueToString_UnknownKindReturnsNil(t *testing.T) {
	// A TypedValue with no Kind set hits the default branch.
	assert.Nil(t, typedValueToString(&pb.TypedValue{}))
}

func TestConfigVersionToProto_WithDescription(t *testing.T) {
	desc := "release notes"
	v := domain.ConfigVersion{
		ID:          "v-1",
		TenantID:    "t-1",
		Version:     3,
		Description: &desc,
		CreatedBy:   "user-1",
		CreatedAt:   time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC),
	}
	got := configVersionToProto(v)
	assert.Equal(t, "v-1", got.GetId())
	assert.Equal(t, "t-1", got.GetTenantId())
	assert.Equal(t, int32(3), got.GetVersion())
	assert.Equal(t, "user-1", got.GetCreatedBy())
	assert.Equal(t, "release notes", got.GetDescription())
	assert.Equal(t, v.CreatedAt.UTC(), got.GetCreatedAt().AsTime().UTC())
}

func TestConfigVersionToProto_NilDescription(t *testing.T) {
	got := configVersionToProto(domain.ConfigVersion{ID: "v-2", Description: nil})
	assert.Equal(t, "", got.GetDescription())
}
