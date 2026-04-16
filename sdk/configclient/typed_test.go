package configclient

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// --- Typed setters ---

func TestSetTime_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.Value != nil && r.Value.Kind() == KindTime && r.FieldPath == "x"
	}, &SetFieldResponse{}, nil)

	if err := client.SetTime(ctx, "t1", "x", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetDuration_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", func(args ...any) bool {
		r := args[0].(*SetFieldRequest)
		return r.Value != nil && r.Value.Kind() == KindDuration && r.Value.DurationValue() == 30*time.Second
	}, &SetFieldResponse{}, nil)

	if err := client.SetDuration(ctx, "t1", "x", 30*time.Second); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetTyped_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("SetField", nil, &SetFieldResponse{}, nil)

	if err := client.SetTyped(ctx, "t1", "x", StringVal("hello")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Typed getters ---

func TestGetTime_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	ts := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "x", Value: TimeVal(ts),
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
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "x", Value: StringVal("not-a-time"),
	}, nil)

	_, err := client.GetTime(ctx, "t1", "x")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("got error %v, want %v", err, ErrTypeMismatch)
	}
}

func TestGetTime_Null(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{FieldPath: "x", Value: nil}, nil)

	got, err := client.GetTime(ctx, "t1", "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("expected zero time, got %v", got)
	}
}

func TestGetDuration_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "x", Value: DurationVal(5 * time.Minute),
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
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "x", Value: IntVal(42),
	}, nil)

	_, err := client.GetDuration(ctx, "t1", "x")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("got error %v, want %v", err, ErrTypeMismatch)
	}
}

func TestGetFields_Success(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetFields", nil, &GetFieldsResponse{
		Values: []ConfigValue{
			{FieldPath: "a", Value: StringVal("1")},
			{FieldPath: "b", Value: StringVal("2")},
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
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "x", Value: BoolVal(true),
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
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	tr.on("GetField", nil, &GetFieldResponse{
		FieldPath: "x", Value: IntVal(42),
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

// --- TypedValue ---

func TestTypedValue_String(t *testing.T) {
	tests := []struct {
		name     string
		input    *TypedValue
		expected string
	}{
		{"nil", nil, ""},
		{"string", StringVal("hello"), "hello"},
		{"integer", IntVal(42), "42"},
		{"number", FloatVal(3.14), "3.14"},
		{"bool", BoolVal(true), "true"},
		{"url", URLVal("https://x.com"), "https://x.com"},
		{"json", JSONVal("{}"), "{}"},
		{"duration", DurationVal(time.Hour), "1h0m0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.input.String(); got != tt.expected {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTypedValue_StringTime(t *testing.T) {
	ts := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	tv := TimeVal(ts)
	if got := tv.String(); !strings.Contains(got, "2026-03-30") {
		t.Errorf("expected %q to contain %q", got, "2026-03-30")
	}
}
