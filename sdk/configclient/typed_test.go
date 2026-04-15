package configclient

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// --- Typed setters ---

func TestSetTime_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.on("SetField", func(args ...any) bool {
		r := args[0].(*pb.SetFieldRequest)
		_, ok := r.Value.Kind.(*pb.TypedValue_TimeValue)
		return ok && r.FieldPath == "x"
	}, &pb.SetFieldResponse{}, nil)

	if err := client.SetTime(ctx, "t1", "x", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetDuration_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.on("SetField", func(args ...any) bool {
		r := args[0].(*pb.SetFieldRequest)
		v, ok := r.Value.Kind.(*pb.TypedValue_DurationValue)
		return ok && v.DurationValue.AsDuration() == 30*time.Second
	}, &pb.SetFieldResponse{}, nil)

	if err := client.SetDuration(ctx, "t1", "x", 30*time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetTyped_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.on("SetField", nil, &pb.SetFieldResponse{}, nil)

	if err := client.SetTyped(ctx, "t1", "x", StringValue("hello")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Typed getters ---

func TestGetTime_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	ts := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			FieldPath: "x",
			Value:     &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(ts)}},
		},
	}, nil)

	got, err := client.GetTime(ctx, "t1", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(ts) {
		t.Errorf("got %v, want %v", got, ts)
	}
}

func TestGetTime_TypeMismatch(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			Value: &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "not-a-time"}},
		},
	}, nil)

	_, err := client.GetTime(ctx, "t1", "x")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("got error %v, want %v", err, ErrTypeMismatch)
	}
}

func TestGetTime_Null(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{Value: nil},
	}, nil)

	got, err := client.GetTime(ctx, "t1", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time, got %v", got)
	}
}

func TestGetDuration_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			Value: &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(5 * time.Minute)}},
		},
	}, nil)

	got, err := client.GetDuration(ctx, "t1", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 5*time.Minute {
		t.Errorf("got %v, want %v", got, 5*time.Minute)
	}
}

func TestGetDuration_TypeMismatch(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			Value: &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}},
		},
	}, nil)

	_, err := client.GetDuration(ctx, "t1", "x")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("got error %v, want %v", err, ErrTypeMismatch)
	}
}

func TestGetFields_Success(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.on("GetFields", nil, &pb.GetFieldsResponse{
		Values: []*pb.ConfigValue{
			{FieldPath: "a", Value: StringValue("1")},
			{FieldPath: "b", Value: StringValue("2")},
		},
	}, nil)

	vals, err := client.GetFields(ctx, "t1", []string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := vals["a"]; got != "1" {
		t.Errorf("got %v, want %v", got, "1")
	}
	if got := vals["b"]; got != "2" {
		t.Errorf("got %v, want %v", got, "2")
	}
}

func TestGetBoolNullable_Present(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			Value: &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: true}},
		},
	}, nil)

	val, err := client.GetBoolNullable(ctx, "t1", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil {
		t.Fatal("expected non-nil")
	}
	if !*val {
		t.Error("expected true, got false")
	}
}

func TestGetStringNullable_CoercesAnyType(t *testing.T) {
	rpc := &mockRPC{}
	client := New(rpc, WithSubject("test"))
	ctx := context.Background()

	rpc.on("GetField", nil, &pb.GetFieldResponse{
		Value: &pb.ConfigValue{
			Value: &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: 42}},
		},
	}, nil)

	val, err := client.GetStringNullable(ctx, "t1", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil {
		t.Fatal("expected non-nil")
	}
	if *val != "42" {
		t.Errorf("got %v, want %v", *val, "42")
	}
}

// --- Helper functions ---

func TestDerefString(t *testing.T) {
	s := "hello"
	if got := derefString(&s); got != "hello" {
		t.Errorf("got %v, want %v", got, "hello")
	}
	if got := derefString(nil); got != "" {
		t.Errorf("got %v, want %v", got, "")
	}
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
			if got := typedValueToString(tt.input); got != tt.expected {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTypedValueToString_Time(t *testing.T) {
	ts := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	tv := &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(ts)}}
	if got := typedValueToString(tv); !strings.Contains(got, "2026-03-30") {
		t.Errorf("expected %q to contain %q", got, "2026-03-30")
	}
}
