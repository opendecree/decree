package render

import (
	"reflect"
	"testing"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
)

func ptr[T any](v T) *T { return &v }

func TestGroupByPrefix(t *testing.T) {
	// Input is sorted by path, as docmodel guarantees. A top-level field
	// whose path has no dot forms its own single-field group; first
	// occurrence of each prefix fixes group order.
	fields := []docmodel.Field{
		{Path: "auth"},
		{Path: "auth.token"},
		{Path: "payments.fee"},
		{Path: "payments.limit"},
		{Path: "region"},
	}
	got := GroupByPrefix(fields)

	wantPrefixes := []string{"auth", "payments", "region"}
	if len(got) != len(wantPrefixes) {
		t.Fatalf("got %d groups, want %d: %+v", len(got), len(wantPrefixes), got)
	}
	for i, w := range wantPrefixes {
		if got[i].Prefix != w {
			t.Errorf("group %d prefix = %q, want %q", i, got[i].Prefix, w)
		}
	}
	if len(got[0].Fields) != 2 || len(got[1].Fields) != 2 || len(got[2].Fields) != 1 {
		t.Errorf("group field counts = %d/%d/%d, want 2/2/1",
			len(got[0].Fields), len(got[1].Fields), len(got[2].Fields))
	}
}

func TestGroupByPrefix_Empty(t *testing.T) {
	if got := GroupByPrefix(nil); got != nil {
		t.Errorf("GroupByPrefix(nil) = %+v, want nil", got)
	}
}

func TestTitle(t *testing.T) {
	tests := []struct {
		name string
		in   docmodel.Schema
		want string
	}{
		{"no info falls back to name", docmodel.Schema{Name: "payments"}, "payments"},
		{"info title wins", docmodel.Schema{Name: "payments", Info: &docmodel.Info{Title: "Payments API"}}, "Payments API"},
		{"empty info title falls back", docmodel.Schema{Name: "payments", Info: &docmodel.Info{Author: "x"}}, "payments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Title(tt.in); got != tt.want {
				t.Errorf("Title() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSeverity(t *testing.T) {
	tests := []struct {
		in        string
		wantKey   string
		wantLabel string
	}{
		{"error", "error", "Error"},
		{"warning", "warning", "Warning"},
		{"", "warning", "Warning"},
		{"bogus", "warning", "Warning"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			key, label := Severity(tt.in)
			if key != tt.wantKey || label != tt.wantLabel {
				t.Errorf("Severity(%q) = (%q, %q), want (%q, %q)", tt.in, key, label, tt.wantKey, tt.wantLabel)
			}
		})
	}
}

func TestSortedExampleNames(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if got := SortedExampleNames(docmodel.Field{}); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
	t.Run("sorted", func(t *testing.T) {
		f := docmodel.Field{Examples: map[string]docmodel.Example{
			"zulu": {}, "alpha": {}, "mike": {},
		}}
		got := SortedExampleNames(f)
		want := []string{"alpha", "mike", "zulu"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestConstraints_Nil(t *testing.T) {
	if got := Constraints(nil); got != nil {
		t.Errorf("Constraints(nil) = %+v, want nil", got)
	}
	if got := Constraints(&docmodel.Constraints{}); got != nil {
		t.Errorf("Constraints(empty) = %+v, want nil", got)
	}
}

func TestConstraints_OrderAndKinds(t *testing.T) {
	c := &docmodel.Constraints{
		Minimum:          ptr(1.0),
		Maximum:          ptr(10.5),
		ExclusiveMinimum: ptr(0.0),
		ExclusiveMaximum: ptr(11.0),
		MinLength:        ptr(int32(2)),
		MaxLength:        ptr(int32(8)),
		Pattern:          "^x$",
		Enum:             []string{"a", "b"},
		JSONSchema:       "{}",
		AllowedSchemes:   []string{"https"},
	}
	got := Constraints(c)

	want := []Constraint{
		{Label: "Minimum", Value: "1", Kind: ScalarConstraint},
		{Label: "Maximum", Value: "10.5", Kind: ScalarConstraint},
		{Label: "Exclusive minimum", Value: "0", Kind: ScalarConstraint},
		{Label: "Exclusive maximum", Value: "11", Kind: ScalarConstraint},
		{Label: "Min length", Value: "2", Kind: ScalarConstraint},
		{Label: "Max length", Value: "8", Kind: ScalarConstraint},
		{Label: "Pattern", Value: "^x$", Kind: CodeConstraint},
		{Label: "Enum", Values: []string{"a", "b"}, Kind: ListConstraint},
		{Label: "JSON Schema", Value: "(see schema definition)", Kind: ScalarConstraint},
		{Label: "Allowed schemes", Values: []string{"https"}, Kind: ListConstraint},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Constraints() mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{1.0, "1"},
		{1.5, "1.5"},
		{-2.0, "-2"},
		{0, "0"},
		{3.14, "3.14"},
		{1e19, "1e+19"}, // whole but beyond int64 range: takes the general path, no overflow
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := FormatFloat(tt.in); got != tt.want {
				t.Errorf("FormatFloat(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
