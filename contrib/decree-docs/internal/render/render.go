// Package render holds the format-agnostic derivations the html, markdown,
// and mdx backends share. Each backend keeps its own syntax (HTML templates,
// CommonMark, MDX/JSX); what lives here is the logic that has no format in it
// at all: how fields group into sections, how a schema's display title is
// chosen, how a validation severity normalizes, the canonical order and
// labels of constraint lines, and numeric formatting. Centralizing these
// keeps the three backends from drifting — e.g. adding a constraint kind is a
// single edit here, not three parallel ones.
package render

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
)

// Group is a set of fields sharing a top-level path prefix.
type Group struct {
	Prefix string
	Fields []docmodel.Field
}

// GroupByPrefix groups fields by their top-level path prefix. fields is
// assumed sorted by path (docmodel guarantees this), so the first occurrence
// of each prefix establishes deterministic group order.
func GroupByPrefix(fields []docmodel.Field) []Group {
	var groups []Group
	index := make(map[string]int)
	for _, f := range fields {
		prefix := f.Path
		if i := strings.IndexByte(f.Path, '.'); i > 0 {
			prefix = f.Path[:i]
		}
		if i, ok := index[prefix]; ok {
			groups[i].Fields = append(groups[i].Fields, f)
		} else {
			index[prefix] = len(groups)
			groups = append(groups, Group{Prefix: prefix, Fields: []docmodel.Field{f}})
		}
	}
	return groups
}

// Title is the schema's display title: the info title when set, otherwise the
// schema slug.
func Title(s docmodel.Schema) string {
	if s.Info != nil && s.Info.Title != "" {
		return s.Info.Title
	}
	return s.Name
}

// Severity normalizes a validation rule's severity to its canonical key
// ("error" or "warning") and human label ("Error" or "Warning"). Any value
// other than "error" is treated as a warning, so the default is well-defined
// when Severity is unset or unrecognized. Backends map the canonical key onto
// their own admonition vocabulary (e.g. mdx renders "error" as "danger").
func Severity(s string) (key, label string) {
	if s == "error" {
		return "error", "Error"
	}
	return "warning", "Warning"
}

// SortedExampleNames returns the keys of f.Examples in sorted order, the
// deterministic iteration order every backend renders named examples in.
func SortedExampleNames(f docmodel.Field) []string {
	if len(f.Examples) == 0 {
		return nil
	}
	names := make([]string, 0, len(f.Examples))
	for name := range f.Examples {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// ConstraintKind tells a backend how to format a constraint line's value.
type ConstraintKind int

const (
	// ScalarConstraint carries a plain Value (a number, length, or note).
	ScalarConstraint ConstraintKind = iota
	// CodeConstraint carries a single code-like Value (a regex pattern) that
	// backends may wrap in a code span.
	CodeConstraint
	// ListConstraint carries Values (enum members, URI schemes) that backends
	// join, optionally wrapping each member.
	ListConstraint
)

// Constraint is one rendered constraint line: a fixed Label plus a value
// carried in Value (scalar/code) or Values (list), tagged by Kind so each
// backend can format it in its own syntax.
type Constraint struct {
	Label  string
	Value  string
	Values []string
	Kind   ConstraintKind
}

// Constraints flattens c into the canonical ordered list of constraint lines
// shared by every backend. Numeric bounds run through FormatFloat; the order
// and labels here are the single source of truth, so a new constraint is
// added once. Returns nil when c is nil or carries no rules.
func Constraints(c *docmodel.Constraints) []Constraint {
	if c == nil {
		return nil
	}
	var out []Constraint
	scalar := func(label, value string) {
		out = append(out, Constraint{Label: label, Value: value})
	}
	if c.Minimum != nil {
		scalar("Minimum", FormatFloat(*c.Minimum))
	}
	if c.Maximum != nil {
		scalar("Maximum", FormatFloat(*c.Maximum))
	}
	if c.ExclusiveMinimum != nil {
		scalar("Exclusive minimum", FormatFloat(*c.ExclusiveMinimum))
	}
	if c.ExclusiveMaximum != nil {
		scalar("Exclusive maximum", FormatFloat(*c.ExclusiveMaximum))
	}
	if c.MinLength != nil {
		scalar("Min length", strconv.FormatInt(int64(*c.MinLength), 10))
	}
	if c.MaxLength != nil {
		scalar("Max length", strconv.FormatInt(int64(*c.MaxLength), 10))
	}
	if c.Pattern != "" {
		out = append(out, Constraint{Label: "Pattern", Value: c.Pattern, Kind: CodeConstraint})
	}
	if len(c.Enum) > 0 {
		out = append(out, Constraint{Label: "Enum", Values: c.Enum, Kind: ListConstraint})
	}
	if c.JSONSchema != "" {
		scalar("JSON Schema", "(see schema definition)")
	}
	if len(c.AllowedSchemes) > 0 {
		out = append(out, Constraint{Label: "Allowed schemes", Values: c.AllowedSchemes, Kind: ListConstraint})
	}
	return out
}

// FormatFloat formats f for display, omitting the decimal point when the
// value is a whole number (e.g. 1.0 -> "1", 1.5 -> "1.5"). The range guard
// keeps the int64 conversion in bounds; whole numbers too large for int64
// fall through to the general format rather than overflowing the cast.
func FormatFloat(f float64) string {
	if f == math.Trunc(f) && f >= math.MinInt64 && f <= math.MaxInt64 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}
