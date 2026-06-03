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
		return r.Value != nil && r.Value.Kind() == KindDuration && r.Value.MustDurationValue() == 30*time.Second
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
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// (T, bool) — matching kind returns value + true
	if s, ok := StringVal("hello").StringValue(); !ok || s != "hello" {
		t.Errorf("StringValue: got (%v, %v), want (hello, true)", s, ok)
	}
	if n, ok := IntVal(42).IntValue(); !ok || n != 42 {
		t.Errorf("IntValue: got (%v, %v), want (42, true)", n, ok)
	}
	if f, ok := FloatVal(3.14).FloatValue(); !ok || f != 3.14 {
		t.Errorf("FloatValue: got (%v, %v), want (3.14, true)", f, ok)
	}
	if b, ok := BoolVal(true).BoolValue(); !ok || !b {
		t.Errorf("BoolValue: got (%v, %v), want (true, true)", b, ok)
	}
	if tv, ok := TimeVal(ts).TimeValue(); !ok || !tv.Equal(ts) {
		t.Errorf("TimeValue: got (%v, %v), want (%v, true)", tv, ok, ts)
	}
	if d, ok := DurationVal(time.Hour).DurationValue(); !ok || d != time.Hour {
		t.Errorf("DurationValue: got (%v, %v), want (1h0m0s, true)", d, ok)
	}
	if u, ok := URLVal("https://x.com").URLValue(); !ok || u != "https://x.com" {
		t.Errorf("URLValue: got (%v, %v), want (https://x.com, true)", u, ok)
	}
	if j, ok := JSONVal(`{}`).JSONValue(); !ok || j != "{}" {
		t.Errorf("JSONValue: got (%v, %v), want ({}, true)", j, ok)
	}

	// (T, bool) — mismatched kind returns zero + false
	if s, ok := IntVal(1).StringValue(); ok || s != "" {
		t.Errorf("StringValue mismatch: got (%v, %v), want ('', false)", s, ok)
	}
	if n, ok := StringVal("x").IntValue(); ok || n != 0 {
		t.Errorf("IntValue mismatch: got (%v, %v), want (0, false)", n, ok)
	}
	if f, ok := BoolVal(true).FloatValue(); ok || f != 0 {
		t.Errorf("FloatValue mismatch: got (%v, %v), want (0, false)", f, ok)
	}
	if b, ok := IntVal(1).BoolValue(); ok || b {
		t.Errorf("BoolValue mismatch: got (%v, %v), want (false, false)", b, ok)
	}
	if tv, ok := StringVal("x").TimeValue(); ok || !tv.IsZero() {
		t.Errorf("TimeValue mismatch: got (%v, %v), want (zero, false)", tv, ok)
	}
	if d, ok := StringVal("x").DurationValue(); ok || d != 0 {
		t.Errorf("DurationValue mismatch: got (%v, %v), want (0, false)", d, ok)
	}
	if u, ok := BoolVal(true).URLValue(); ok || u != "" {
		t.Errorf("URLValue mismatch: got (%v, %v), want ('', false)", u, ok)
	}
	if j, ok := IntVal(1).JSONValue(); ok || j != "" {
		t.Errorf("JSONValue mismatch: got (%v, %v), want ('', false)", j, ok)
	}

	// Must variants — correct kind
	if got := StringVal("hello").MustStringValue(); got != "hello" {
		t.Errorf("MustStringValue: got %v, want hello", got)
	}
	if got := TimeVal(ts).MustTimeValue(); !got.Equal(ts) {
		t.Errorf("MustTimeValue: got %v, want %v", got, ts)
	}
	if got := URLVal("https://x.com").MustURLValue(); got != "https://x.com" {
		t.Errorf("MustURLValue: got %v, want https://x.com", got)
	}
	if got := JSONVal(`{}`).MustJSONValue(); got != "{}" {
		t.Errorf("MustJSONValue: got %v, want {}", got)
	}
}

func TestTypedValue_MustAccessorPanics(t *testing.T) {
	assertPanics := func(t *testing.T, name string, fn func()) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: expected panic, got none", name)
			}
		}()
		fn()
	}
	t.Run("MustStringValue on int", func(t *testing.T) {
		assertPanics(t, "MustStringValue", func() { IntVal(1).MustStringValue() })
	})
	t.Run("MustTimeValue on string", func(t *testing.T) {
		assertPanics(t, "MustTimeValue", func() { StringVal("x").MustTimeValue() })
	})
	t.Run("MustURLValue on bool", func(t *testing.T) {
		assertPanics(t, "MustURLValue", func() { BoolVal(true).MustURLValue() })
	})
	t.Run("MustJSONValue on int", func(t *testing.T) {
		assertPanics(t, "MustJSONValue", func() { IntVal(1).MustJSONValue() })
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
	err := NewInvalidArgumentError("bad value")
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
