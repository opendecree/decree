package main

import (
	"testing"
	"time"

	"github.com/opendecree/decree/sdk/configclient"
)

func TestParseTypedValue_Success(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		raw       string
		wantKind  configclient.ValueKind
		check     func(*configclient.TypedValue) bool
	}{
		{"string", "string", "hello", configclient.KindString, func(v *configclient.TypedValue) bool { return v.StringValue() == "hello" }},
		{"empty field type defaults to string", "", "hello", configclient.KindString, func(v *configclient.TypedValue) bool { return v.StringValue() == "hello" }},
		{"integer", "integer", "42", configclient.KindInteger, func(v *configclient.TypedValue) bool { return v.IntValue() == 42 }},
		{"integer negative", "integer", "-7", configclient.KindInteger, func(v *configclient.TypedValue) bool { return v.IntValue() == -7 }},
		{"number", "number", "3.14", configclient.KindNumber, func(v *configclient.TypedValue) bool { return v.FloatValue() == 3.14 }},
		{"bool true", "bool", "true", configclient.KindBool, func(v *configclient.TypedValue) bool { return v.BoolValue() }},
		{"bool false", "bool", "false", configclient.KindBool, func(v *configclient.TypedValue) bool { return !v.BoolValue() }},
		{"time RFC3339", "time", "2026-03-30T12:00:00Z", configclient.KindTime, func(v *configclient.TypedValue) bool {
			return v.TimeValue().Equal(time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC))
		}},
		{"duration", "duration", "15s", configclient.KindDuration, func(v *configclient.TypedValue) bool { return v.DurationValue() == 15*time.Second }},
		{"url", "url", "https://example.com", configclient.KindURL, func(v *configclient.TypedValue) bool { return v.URLValue() == "https://example.com" }},
		{"json object", "json", `{"a":1}`, configclient.KindJSON, func(v *configclient.TypedValue) bool { return v.JSONValue() == `{"a":1}` }},
		{"json array", "json", `[1,2,3]`, configclient.KindJSON, func(v *configclient.TypedValue) bool { return v.JSONValue() == `[1,2,3]` }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv, err := parseTypedValue(tt.fieldType, tt.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tv.Kind() != tt.wantKind {
				t.Errorf("kind: got %v, want %v", tv.Kind(), tt.wantKind)
			}
			if !tt.check(tv) {
				t.Errorf("value check failed for %q -> %q", tt.fieldType, tt.raw)
			}
		})
	}
}

func TestParseTypedValue_Errors(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		raw       string
	}{
		{"integer garbage", "integer", "abc"},
		{"number garbage", "number", "abc"},
		{"bool garbage", "bool", "yes-please"},
		{"time wrong format", "time", "2026-03-30 12:00"},
		{"duration garbage", "duration", "2 hours"},
		{"json malformed", "json", `{"a":`},
		{"unsupported type", "complex", "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseTypedValue(tt.fieldType, tt.raw); err == nil {
				t.Errorf("expected error for %q -> %q", tt.fieldType, tt.raw)
			}
		})
	}
}

func TestLookupFieldType(t *testing.T) {
	types := map[string]string{"a": "integer", "b": "bool"}

	if got, err := lookupFieldType(types, "a"); err != nil || got != "integer" {
		t.Errorf("a: got (%q, %v), want (integer, nil)", got, err)
	}
	if _, err := lookupFieldType(types, "missing"); err == nil {
		t.Error("expected error for missing field")
	}
}
