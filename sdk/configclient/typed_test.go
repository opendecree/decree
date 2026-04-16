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

// --- TypedValue accessors ---

func TestTypedValue_Accessors(t *testing.T) {
	if got := StringVal("hello").StringValue(); got != "hello" {
		t.Errorf("StringValue: got %v, want hello", got)
	}
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := TimeVal(ts).TimeValue(); !got.Equal(ts) {
		t.Errorf("TimeValue: got %v, want %v", got, ts)
	}
	if got := URLVal("https://x.com").URLValue(); got != "https://x.com" {
		t.Errorf("URLValue: got %v, want https://x.com", got)
	}
	if got := JSONVal(`{}`).JSONValue(); got != "{}" {
		t.Errorf("JSONValue: got %v, want {}", got)
	}
}

func TestTypedValue_AccessorPanics(t *testing.T) {
	assertPanics := func(t *testing.T, name string, fn func()) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: expected panic, got none", name)
			}
		}()
		fn()
	}
	t.Run("StringValue on int", func(t *testing.T) {
		assertPanics(t, "StringValue", func() { IntVal(1).StringValue() })
	})
	t.Run("TimeValue on string", func(t *testing.T) {
		assertPanics(t, "TimeValue", func() { StringVal("x").TimeValue() })
	})
	t.Run("URLValue on bool", func(t *testing.T) {
		assertPanics(t, "URLValue", func() { BoolVal(true).URLValue() })
	})
	t.Run("JSONValue on int", func(t *testing.T) {
		assertPanics(t, "JSONValue", func() { IntVal(1).JSONValue() })
	})
}

// --- Snapshot.GetFields ---

func TestSnapshot_GetFields(t *testing.T) {
	tr := &mockTransport{}
	client := New(tr)
	ctx := context.Background()

	snap := client.AtVersion("t1", 3)

	v := int32(3)
	tr.on("GetFields", func(args ...any) bool {
		r := args[0].(*GetFieldsRequest)
		return r.Version != nil && *r.Version == v
	}, &GetFieldsResponse{
		Values: []ConfigValue{
			{FieldPath: "a", Value: StringVal("1")},
			{FieldPath: "b", Value: StringVal("2")},
		},
	}, nil)

	vals, err := snap.GetFields(ctx, []string{"a", "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := vals["a"]; got != "1" {
		t.Errorf("got %v, want 1", got)
	}
	if got := vals["b"]; got != "2" {
		t.Errorf("got %v, want 2", got)
	}
}

// --- Error types ---

func TestInvalidArgumentError(t *testing.T) {
	err := InvalidArgumentError("bad value")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
	if !strings.Contains(err.Error(), "bad value") {
		t.Errorf("expected message to contain 'bad value', got %q", err.Error())
	}
}

func TestRetryableError(t *testing.T) {
	inner := errors.New("unavailable")
	re := &RetryableError{Err: inner}
	if re.Error() != "unavailable" {
		t.Errorf("Error(): got %v, want unavailable", re.Error())
	}
	if re.Unwrap() != inner {
		t.Errorf("Unwrap(): got %v, want %v", re.Unwrap(), inner)
	}
}
