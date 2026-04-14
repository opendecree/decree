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
	Info        *SchemaInfo
	Fields      []Field
}

// SchemaInfo contains optional schema-level metadata.
type SchemaInfo struct {
	Title   string
	Author  string
	Contact *SchemaContact
	Labels  map[string]string
}

// SchemaContact contains contact information for a schema owner.
type SchemaContact struct {
	Name  string
	Email string
	URL   string
}

// Field describes a single schema field for documentation purposes.
type Field struct {
	Path         string
	Type         string // short name: "string", "integer", "number", "bool", "time", "duration", "url", "json"
	Description  string
	Default      string
	Nullable     bool
	Deprecated   bool
	RedirectTo   string
	Constraints  *Constraints
	Title        string
	Example      string
	Examples     map[string]FieldExample
	ExternalDocs *ExternalDocs
	Tags         []string
	Format       string
	ReadOnly     bool
	WriteOnce    bool
	Sensitive    bool
}

// FieldExample represents a named example value.
type FieldExample struct {
	Value   string
	Summary string
}

// ExternalDocs links to external documentation.
type ExternalDocs struct {
	Description string
	URL         string
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
	// Use info.title if available, otherwise schema name.
	title := schema.Name
	if schema.Info != nil && schema.Info.Title != "" {
		title = schema.Info.Title
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	if schema.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", schema.Description)
	}
	if schema.Version > 0 {
		fmt.Fprintf(&b, "**Version:** %d\n\n", schema.Version)
	}
	if schema.Info != nil {
		writeSchemaInfo(&b, schema.Info)
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

func writeSchemaInfo(b *strings.Builder, info *SchemaInfo) {
	if info.Author != "" {
		fmt.Fprintf(b, "**Author:** %s\n\n", info.Author)
	}
	if info.Contact != nil {
		if info.Contact.Email != "" {
			fmt.Fprintf(b, "**Contact:** %s <%s>\n\n", info.Contact.Name, info.Contact.Email)
		} else if info.Contact.URL != "" {
			fmt.Fprintf(b, "**Contact:** [%s](%s)\n\n", info.Contact.Name, info.Contact.URL)
		} else if info.Contact.Name != "" {
			fmt.Fprintf(b, "**Contact:** %s\n\n", info.Contact.Name)
		}
	}
	if len(info.Labels) > 0 {
		var labels []string
		for k, v := range info.Labels {
			labels = append(labels, fmt.Sprintf("`%s: %s`", k, v))
		}
		sort.Strings(labels)
		fmt.Fprintf(b, "**Labels:** %s\n\n", strings.Join(labels, ", "))
	}
}

func writeField(b *strings.Builder, f Field, cfg *config) {
	if f.Title != "" {
		fmt.Fprintf(b, "### %s (`%s`)\n\n", f.Title, f.Path)
	} else {
		fmt.Fprintf(b, "### `%s`\n\n", f.Path)
	}

	// Property table.
	fmt.Fprintln(b, "| Property | Value |")
	fmt.Fprintln(b, "|----------|-------|")
	fmt.Fprintf(b, "| Type | %s |\n", f.Type)
	if f.Format != "" {
		fmt.Fprintf(b, "| Format | %s |\n", f.Format)
	}
	fmt.Fprintf(b, "| Nullable | %s |\n", yesNo(f.Nullable))
	if f.Default != "" {
		fmt.Fprintf(b, "| Default | `%s` |\n", f.Default)
	}
	if f.ReadOnly {
		fmt.Fprint(b, "| Read-only | yes |\n")
	}
	if f.WriteOnce {
		fmt.Fprint(b, "| Write-once | yes |\n")
	}
	if f.Sensitive {
		fmt.Fprint(b, "| Sensitive | yes |\n")
	}
	if f.Deprecated {
		fmt.Fprint(b, "| Deprecated | yes |\n")
		if f.RedirectTo != "" {
			fmt.Fprintf(b, "| Redirect | `%s` |\n", f.RedirectTo)
		}
	}
	if len(f.Tags) > 0 {
		fmt.Fprintf(b, "| Tags | %s |\n", strings.Join(f.Tags, ", "))
	}
	fmt.Fprintln(b)

	if f.Description != "" {
		fmt.Fprintf(b, "%s\n\n", f.Description)
	}

	// Example(s).
	if f.Example != "" {
		fmt.Fprintf(b, "**Example:** `%s`\n\n", f.Example)
	}
	if len(f.Examples) > 0 {
		fmt.Fprintln(b, "**Examples:**")
		for name, ex := range f.Examples {
			if ex.Summary != "" {
				fmt.Fprintf(b, "- **%s:** `%s` — %s\n", name, ex.Value, ex.Summary)
			} else {
				fmt.Fprintf(b, "- **%s:** `%s`\n", name, ex.Value)
			}
		}
		fmt.Fprintln(b)
	}

	// External docs link.
	if f.ExternalDocs != nil && f.ExternalDocs.URL != "" {
		if f.ExternalDocs.Description != "" {
			fmt.Fprintf(b, "**See also:** [%s](%s)\n\n", f.ExternalDocs.Description, f.ExternalDocs.URL)
		} else {
			fmt.Fprintf(b, "**See also:** %s\n\n", f.ExternalDocs.URL)
		}
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
