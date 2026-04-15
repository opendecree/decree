package configwatcher

import (
	"strings"
	"testing"
	"time"

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
			if got := typedValueToString(tt.input); got != tt.expected {
				t.Errorf("got %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTypedValueToString_Time(t *testing.T) {
	ts := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	tv := &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(ts)}}
	result := typedValueToString(tv)
	if !strings.Contains(result, "2026-03-30") {
		t.Errorf("expected %q to contain %q", result, "2026-03-30")
	}
}

func TestParseFunctions(t *testing.T) {
	t.Run("parseString", func(t *testing.T) {
		v, err := parseString("hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := v; got != "hello" {
			t.Errorf("got %v, want %v", got, "hello")
		}
	})

	t.Run("parseInt valid", func(t *testing.T) {
		v, err := parseInt("42")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := v; got != int64(42) {
			t.Errorf("got %v, want %v", got, int64(42))
		}
	})

	t.Run("parseInt invalid", func(t *testing.T) {
		_, err := parseInt("abc")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("parseFloat valid", func(t *testing.T) {
		v, err := parseFloat("3.14")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := v; got != 3.14 {
			t.Errorf("got %v, want %v", got, 3.14)
		}
	})

	t.Run("parseFloat invalid", func(t *testing.T) {
		_, err := parseFloat("abc")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("parseBool valid", func(t *testing.T) {
		v, err := parseBool("true")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !v {
			t.Error("expected true, got false")
		}
	})

	t.Run("parseBool invalid", func(t *testing.T) {
		_, err := parseBool("maybe")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("parseDuration valid", func(t *testing.T) {
		v, err := parseDuration("5m30s")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := v; got != 5*time.Minute+30*time.Second {
			t.Errorf("got %v, want %v", got, 5*time.Minute+30*time.Second)
		}
	})

	t.Run("parseDuration invalid", func(t *testing.T) {
		_, err := parseDuration("nope")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
