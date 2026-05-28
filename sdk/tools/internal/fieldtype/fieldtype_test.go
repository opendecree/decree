package fieldtype

import "testing"

func TestName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"FIELD_TYPE_INT", "integer"},
		{"FIELD_TYPE_NUMBER", "number"},
		{"FIELD_TYPE_STRING", "string"},
		{"FIELD_TYPE_BOOL", "bool"},
		{"FIELD_TYPE_TIME", "time"},
		{"FIELD_TYPE_DURATION", "duration"},
		{"FIELD_TYPE_URL", "url"},
		{"FIELD_TYPE_JSON", "json"},
		{"UNKNOWN", "UNKNOWN"},
	}
	for _, c := range cases {
		if got := Name(c.input); got != c.want {
			t.Errorf("Name(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
