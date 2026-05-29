package cel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func TestStringToCelNative(t *testing.T) {
	sp := func(s string) *string { return &s }
	wantTime, _ := time.Parse(time.RFC3339Nano, "2026-03-30T12:00:00Z")

	tests := []struct {
		name string
		s    *string
		ft   pb.FieldType
		want any
	}{
		{"nil string is CEL null", nil, pb.FieldType_FIELD_TYPE_INT, nil},
		{"int parses", sp("42"), pb.FieldType_FIELD_TYPE_INT, int64(42)},
		{"int parse error falls back to string", sp("nope"), pb.FieldType_FIELD_TYPE_INT, "nope"},
		{"number parses", sp("3.5"), pb.FieldType_FIELD_TYPE_NUMBER, 3.5},
		{"number parse error falls back", sp("nope"), pb.FieldType_FIELD_TYPE_NUMBER, "nope"},
		{"bool parses", sp("true"), pb.FieldType_FIELD_TYPE_BOOL, true},
		{"bool parse error falls back", sp("nope"), pb.FieldType_FIELD_TYPE_BOOL, "nope"},
		{"time parses", sp("2026-03-30T12:00:00Z"), pb.FieldType_FIELD_TYPE_TIME, wantTime},
		{"time parse error falls back", sp("nope"), pb.FieldType_FIELD_TYPE_TIME, "nope"},
		{"duration parses", sp("5m"), pb.FieldType_FIELD_TYPE_DURATION, 5 * time.Minute},
		{"duration parse error falls back", sp("nope"), pb.FieldType_FIELD_TYPE_DURATION, "nope"},
		{"json parses", sp(`{"k":1}`), pb.FieldType_FIELD_TYPE_JSON, map[string]any{"k": float64(1)}},
		{"json parse error falls back", sp("{bad"), pb.FieldType_FIELD_TYPE_JSON, "{bad"},
		{"string passthrough", sp("hello"), pb.FieldType_FIELD_TYPE_STRING, "hello"},
		{"url passthrough", sp("https://x.io"), pb.FieldType_FIELD_TYPE_URL, "https://x.io"},
		{"unspecified passthrough", sp("raw"), pb.FieldType_FIELD_TYPE_UNSPECIFIED, "raw"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stringToCelNative(tt.s, tt.ft))
		})
	}
}

func TestSetNested_EmptySegmentsIsNoOp(t *testing.T) {
	m := map[string]any{"keep": 1}
	setNested(m, nil, "ignored")
	assert.Equal(t, map[string]any{"keep": 1}, m)
}

func TestSetNested_CreatesIntermediateMaps(t *testing.T) {
	m := map[string]any{}
	setNested(m, []string{"a", "b", "c"}, 42)
	ab := m["a"].(map[string]any)["b"].(map[string]any)
	assert.Equal(t, 42, ab["c"])
}

func TestSetNested_ReusesExistingIntermediateMap(t *testing.T) {
	m := map[string]any{"a": map[string]any{"existing": 1}}
	setNested(m, []string{"a", "b"}, 2)
	a := m["a"].(map[string]any)
	assert.Equal(t, 1, a["existing"])
	assert.Equal(t, 2, a["b"])
}
