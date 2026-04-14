// Package docgen generates human-readable Markdown documentation from schema
// definitions. It is a pure function with no external dependencies, suitable
// for embedding in CLIs, documentation servers, or CI pipelines.
package docgen

import (
	"fmt"
	"sort"
	"strings"
)

// Schema is the input to documentation generation.
type Schema struct {
	Name        string
	Description string
	Version     int32
	Fields      []Field
}

// Field describes a single schema field for documentation purposes.
type Field struct {
	Path        string
	Type        string // short name: "string", "integer", "number", "bool", "time", "duration", "url", "json"
	Description string
	Default     string
	Nullable    bool
	Deprecated  bool
	RedirectTo  string
	Constraints *Constraints
}

// Constraints defines validation rules for a field.
type Constraints struct {
	Min          *float64
	Max          *float64
	ExclusiveMin *float64
	ExclusiveMax *float64
	MinLength    *int32
	MaxLength    *int32
	Pattern      string
	Enum         []string
	JSONSchema   string
}

// Option configures documentation generation.
type Option func(*config)

type config struct {
	includeDeprecated  bool
	includeConstraints bool
	groupByPrefix      bool
}

func defaults() config {
	return config{
		includeDeprecated:  true,
		includeConstraints: true,
		groupByPrefix:      true,
	}
}

// WithoutDeprecated excludes deprecated fields from the output.
func WithoutDeprecated() Option {
	return func(c *config) { c.includeDeprecated = false }
}

// WithoutConstraints omits constraint details from the output.
func WithoutConstraints() Option {
	return func(c *config) { c.includeConstraints = false }
}

// WithoutGrouping renders all fields in a flat list instead of grouping by prefix.
func WithoutGrouping() Option {
	return func(c *config) { c.groupByPrefix = false }
}

// Generate produces Markdown documentation for the given schema.
func Generate(schema Schema, opts ...Option) string {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}

	fields := schema.Fields
	if !cfg.includeDeprecated {
		var filtered []Field
		for _, f := range fields {
			if !f.Deprecated {
				filtered = append(filtered, f)
			}
		}
		fields = filtered
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Path < fields[j].Path
	})

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", schema.Name)
	if schema.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", schema.Description)
	}
	if schema.Version > 0 {
		fmt.Fprintf(&b, "**Version:** %d\n\n", schema.Version)
	}

	if cfg.groupByPrefix {
		writeGrouped(&b, fields, &cfg)
	} else {
		for _, f := range fields {
			writeField(&b, f, &cfg)
		}
	}

	return b.String()
}

func writeGrouped(b *strings.Builder, fields []Field, cfg *config) {
	groups := groupByPrefix(fields)
	for _, g := range groups {
		fmt.Fprintf(b, "## %s\n\n", g.prefix)
		for _, f := range g.fields {
			writeField(b, f, cfg)
		}
	}
}

type fieldGroup struct {
	prefix string
	fields []Field
}

func groupByPrefix(fields []Field) []fieldGroup {
	var groups []fieldGroup
	groupMap := make(map[string]int) // prefix → index in groups

	for _, f := range fields {
		prefix := f.Path
		if idx := strings.IndexByte(f.Path, '.'); idx > 0 {
			prefix = f.Path[:idx]
		}

		if i, ok := groupMap[prefix]; ok {
			groups[i].fields = append(groups[i].fields, f)
		} else {
			groupMap[prefix] = len(groups)
			groups = append(groups, fieldGroup{prefix: prefix, fields: []Field{f}})
		}
	}
	return groups
}

func writeField(b *strings.Builder, f Field, cfg *config) {
	fmt.Fprintf(b, "### `%s`\n\n", f.Path)

	// Property table.
	fmt.Fprintln(b, "| Property | Value |")
	fmt.Fprintln(b, "|----------|-------|")
	fmt.Fprintf(b, "| Type | %s |\n", f.Type)
	fmt.Fprintf(b, "| Nullable | %s |\n", yesNo(f.Nullable))
	if f.Default != "" {
		fmt.Fprintf(b, "| Default | `%s` |\n", f.Default)
	}
	if f.Deprecated {
		fmt.Fprint(b, "| Deprecated | yes |\n")
		if f.RedirectTo != "" {
			fmt.Fprintf(b, "| Redirect | `%s` |\n", f.RedirectTo)
		}
	}
	fmt.Fprintln(b)

	if f.Description != "" {
		fmt.Fprintf(b, "%s\n\n", f.Description)
	}

	if cfg.includeConstraints && f.Constraints != nil {
		writeConstraints(b, f.Constraints)
	}
}

func writeConstraints(b *strings.Builder, c *Constraints) {
	var lines []string

	if c.Min != nil {
		lines = append(lines, fmt.Sprintf("Minimum: %s", formatFloat(*c.Min)))
	}
	if c.Max != nil {
		lines = append(lines, fmt.Sprintf("Maximum: %s", formatFloat(*c.Max)))
	}
	if c.ExclusiveMin != nil {
		lines = append(lines, fmt.Sprintf("Exclusive minimum: %s", formatFloat(*c.ExclusiveMin)))
	}
	if c.ExclusiveMax != nil {
		lines = append(lines, fmt.Sprintf("Exclusive maximum: %s", formatFloat(*c.ExclusiveMax)))
	}
	if c.MinLength != nil {
		lines = append(lines, fmt.Sprintf("Min length: %d", *c.MinLength))
	}
	if c.MaxLength != nil {
		lines = append(lines, fmt.Sprintf("Max length: %d", *c.MaxLength))
	}
	if c.Pattern != "" {
		lines = append(lines, fmt.Sprintf("Pattern: `%s`", c.Pattern))
	}
	if len(c.Enum) > 0 {
		lines = append(lines, fmt.Sprintf("Enum: %s", strings.Join(c.Enum, ", ")))
	}
	if c.JSONSchema != "" {
		lines = append(lines, "JSON Schema: (see schema definition)")
	}

	if len(lines) > 0 {
		fmt.Fprintln(b, "**Constraints:**")
		for _, l := range lines {
			fmt.Fprintf(b, "- %s\n", l)
		}
		fmt.Fprintln(b)
	}
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%g", f)
}

// protoTypeToShort maps proto enum names to YAML short names.
var protoTypeToShort = map[string]string{
	"FIELD_TYPE_INT":      "integer",
	"FIELD_TYPE_NUMBER":   "number",
	"FIELD_TYPE_STRING":   "string",
	"FIELD_TYPE_BOOL":     "bool",
	"FIELD_TYPE_TIME":     "time",
	"FIELD_TYPE_DURATION": "duration",
	"FIELD_TYPE_URL":      "url",
	"FIELD_TYPE_JSON":     "json",
}

// FieldTypeName returns the human-readable short name for a proto field type
// enum name (e.g., "FIELD_TYPE_STRING" → "string"). Returns the input
// unchanged if not recognized.
func FieldTypeName(protoName string) string {
	if short, ok := protoTypeToShort[protoName]; ok {
		return short
	}
	return protoName
}
