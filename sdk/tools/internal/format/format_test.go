package format

import "testing"

func TestFloat(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{42, "42"},
		{3.14, "3.14"},
		{0, "0"},
		{-1, "-1"},
		{1.0, "1"},
		{-2.5, "-2.5"},
		{1000000, "1000000"},
	}
	for _, tt := range tests {
		if got := Float(tt.in); got != tt.want {
			t.Errorf("Float(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
