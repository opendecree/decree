package configclient

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// --- Typed setters ---

func TestSetTime_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.On("SetField", mock.Anything, mock.MatchedBy(func(r *pb.SetFieldRequest) bool {
		_, ok := r.Value.Kind.(*pb.TypedValue_TimeValue)
		return ok && r.FieldPath == "x"
	})).Return(&pb.SetFieldResponse{}, nil)

	require.NoError(t, client.SetTime(ctx, "t1", "x", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
}

func TestSetDuration_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.On("SetField", mock.Anything, mock.MatchedBy(func(r *pb.SetFieldRequest) bool {
		v, ok := r.Value.Kind.(*pb.TypedValue_DurationValue)
		return ok && v.DurationValue.AsDuration() == 30*time.Second
	})).Return(&pb.SetFieldResponse{}, nil)

	require.NoError(t, client.SetDuration(ctx, "t1", "x", 30*time.Second))
}

func TestSetTyped_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.On("SetField", mock.Anything, mock.Anything).Return(&pb.SetFieldResponse{}, nil)

	require.NoError(t, client.SetTyped(ctx, "t1", "x", StringValue("hello")))
}

// --- Typed getters ---

func TestGetTime_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	ts := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			FieldPath: "x",
			Value:     &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(ts)}},
		},
	}, nil)

	got, err := client.GetTime(ctx, "t1", "x")
	require.NoError(t, err)
	assert.Equal(t, ts, got)
}

func TestGetTime_TypeMismatch(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "not-a-time"}},
		},
	}, nil)

	_, err := client.GetTime(ctx, "t1", "x")
	assert.ErrorIs(t, err, ErrTypeMismatch)
}

func TestGetTime_Null(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{Value: nil},
	}, nil)

	got, err := client.GetTime(ctx, "t1", "x")
	require.NoError(t, err)
	assert.True(t, got.IsZero())
}

func TestGetDuration_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			Value: &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(5 * time.Minute)}},
		},
	}, nil)

	got, err := client.GetDuration(ctx, "t1", "x")
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, got)
}

func TestGetDuration_TypeMismatch(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			Value: &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}},
		},
	}, nil)

	_, err := client.GetDuration(ctx, "t1", "x")
	assert.ErrorIs(t, err, ErrTypeMismatch)
}

func TestGetFields_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.On("GetFields", mock.Anything, mock.Anything).Return(&pb.GetFieldsResponse{
		Values: []*pb.ConfigValue{
			{FieldPath: "a", Value: StringValue("1")},
			{FieldPath: "b", Value: StringValue("2")},
		},
	}, nil)

	vals, err := client.GetFields(ctx, "t1", []string{"a", "b"})
	require.NoError(t, err)
	assert.Equal(t, "1", vals["a"])
	assert.Equal(t, "2", vals["b"])
}

func TestGetBoolNullable_Present(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			Value: &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}},
		},
	}, nil)

	val, err := client.GetBoolNullable(ctx, "t1", "x")
	require.NoError(t, err)
	require.NotNil(t, val)
	assert.True(t, *val)
}

func TestGetStringNullable_CoercesAnyType(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.On("GetField", mock.Anything, mock.Anything).Return(&pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			Value: &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}},
		},
	}, nil)

	val, err := client.GetStringNullable(ctx, "t1", "x")
	require.NoError(t, err)
	require.NotNil(t, val)
	assert.Equal(t, "42", *val)
}

// --- Helper functions ---

func TestDerefString(t *testing.T) {
	s := "hello"
	assert.Equal(t, "hello", derefString(&s))
	assert.Equal(t, "", derefString(nil))
}

func TestTypedValueToString(t *testing.T) {
	tests := []struct {
		name     string
		input    *pb.TypedValue
		expected string
	}{
		{"nil", nil, ""},
		{"string", StringValue("hello"), "hello"},
		{"integer", &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}}, "42"},
		{"number", &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: 3.14}}, "3.14"},
		{"bool", &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}}, "true"},
		{"url", &pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: "https://x.com"}}, "https://x.com"},
		{"json", &pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: `{}`}}, "{}"},
		{"duration", &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(time.Hour)}}, "1h0m0s"},
		{"duration nil", &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{}}, ""},
		{"time nil", &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{}}, ""},
		{"empty kind", &pb.TypedValue{}, ""},
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
	assert.Contains(t, typedValueToString(tv), "2026-03-30")
}
