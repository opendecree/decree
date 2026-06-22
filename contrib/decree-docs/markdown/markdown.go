// Package markdown renders a decree-docs documentation model
// ([docmodel.Document]) as Markdown.
//
// Two flavors are supported. "plain" emits portable CommonMark. "material"
// emits Markdown that additionally relies on the MkDocs Material extensions
// this repo's mkdocs.yml already enables: admonition blocks for deprecation
// notices, and pymdownx.tabbed content tabs for fields with two or more
// named examples. In both flavors, field flags (deprecated, sensitive,
// read-only, write-once) render as a bold badge line distinct from the
// field's type/format meta line; material adds an icon per badge.
//
// Two page modes are supported. [SinglePage] renders the whole schema as one
// page. [MultiPage] renders an index page plus one page per top-level
// path-prefix group (the same grouping sdk/tools/docgen uses for "##"
// sections), each holding the fields under that group.
package markdown

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
)

// Flavor selects the Markdown dialect Render emits.
type Flavor string

const (
	// Plain emits portable CommonMark.
	Plain Flavor = "plain"
	// Material emits Markdown using MkDocs Material's admonition and
	// content-tab extensions.
	Material Flavor = "material"
)

// PageMode selects how Render splits the document into pages.
type PageMode string

const (
	// SinglePage renders the whole schema as one page.
	SinglePage PageMode = "single"
	// MultiPage renders an index page plus one page per path-prefix group.
	MultiPage PageMode = "multi"
)

// Options configures Render.
type Options struct {
	Flavor Flavor
	Pages  PageMode
}

// Page is one rendered Markdown page.
type Page struct {
	// Name is the page's logical name: "index" for the single page or the
	// multi-page index, and the path-prefix group name for each multi-page
	// group page. Callers writing to disk join it with ".md".
	Name    string
	Content string
}

// Render renders doc as one or more Markdown pages per opts.
func Render(doc *docmodel.Document, opts Options) ([]Page, error) {
	if opts.Flavor != Plain && opts.Flavor != Material {
		return nil, fmt.Errorf("unknown flavor %q (valid flavors: %s, %s)", opts.Flavor, Plain, Material)
	}

	groups := groupByPrefix(doc.Schema.Fields)

	switch opts.Pages {
	case SinglePage, "":
		var b strings.Builder
		writeHeader(&b, doc.Schema)
		for _, g := range groups {
			fmt.Fprintf(&b, "## %s\n\n", g.prefix)
			for _, f := range g.fields {
				writeField(&b, f, opts.Flavor)
			}
		}
		return []Page{{Name: "index", Content: b.String()}}, nil
	case MultiPage:
		pages := []Page{indexPage(doc.Schema, groups)}
		for _, g := range groups {
			var b strings.Builder
			fmt.Fprintf(&b, "# %s\n\n", g.prefix)
			for _, f := range g.fields {
				writeField(&b, f, opts.Flavor)
			}
			pages = append(pages, Page{Name: g.prefix, Content: b.String()})
		}
		return pages, nil
	default:
		return nil, fmt.Errorf("unknown pages mode %q (valid modes: %s, %s)", opts.Pages, SinglePage, MultiPage)
	}
}

func indexPage(s docmodel.Schema, groups []fieldGroup) Page {
	var b strings.Builder
	writeHeader(&b, s)
	fmt.Fprintln(&b, "## Groups")
	fmt.Fprintln(&b)
	for _, g := range groups {
		noun := "fields"
		if len(g.fields) == 1 {
			noun = "field"
		}
		fmt.Fprintf(&b, "- [%s](%s.md) — %d %s\n", g.prefix, g.prefix, len(g.fields), noun)
	}
	fmt.Fprintln(&b)
	return Page{Name: "index", Content: b.String()}
}

func writeHeader(b *strings.Builder, s docmodel.Schema) {
	title := s.Name
	if s.Info != nil && s.Info.Title != "" {
		title = s.Info.Title
	}
	fmt.Fprintf(b, "# %s\n\n", title)
	if s.Description != "" {
		fmt.Fprintf(b, "%s\n\n", s.Description)
	}
	if s.Version > 0 {
		if s.VersionDescription != "" {
			fmt.Fprintf(b, "**Version:** %d — %s\n\n", s.Version, s.VersionDescription)
		} else {
			fmt.Fprintf(b, "**Version:** %d\n\n", s.Version)
		}
	}
	if s.Info != nil {
		writeInfo(b, s.Info)
	}
}

func writeInfo(b *strings.Builder, info *docmodel.Info) {
	if info.Author != "" {
		fmt.Fprintf(b, "**Author:** %s\n\n", info.Author)
	}
	if c := info.Contact; c != nil {
		switch {
		case c.Email != "":
			fmt.Fprintf(b, "**Contact:** %s <%s>\n\n", c.Name, c.Email)
		case c.URL != "":
			fmt.Fprintf(b, "**Contact:** [%s](%s)\n\n", c.Name, c.URL)
		case c.Name != "":
			fmt.Fprintf(b, "**Contact:** %s\n\n", c.Name)
		}
	}
	if len(info.Labels) > 0 {
		labels := make([]string, 0, len(info.Labels))
		for k, v := range info.Labels {
			labels = append(labels, fmt.Sprintf("`%s: %s`", k, v))
		}
		sort.Strings(labels)
		fmt.Fprintf(b, "**Labels:** %s\n\n", strings.Join(labels, ", "))
	}
}

// --- Grouping ---

type fieldGroup struct {
	prefix string
	fields []docmodel.Field
}

// groupByPrefix groups fields by their top-level path prefix. fields is
// assumed sorted by path (docmodel guarantees this), so the first occurrence
// of each prefix establishes deterministic group order.
func groupByPrefix(fields []docmodel.Field) []fieldGroup {
	var groups []fieldGroup
	index := make(map[string]int)
	for _, f := range fields {
		prefix := f.Path
		if i := strings.IndexByte(f.Path, '.'); i > 0 {
			prefix = f.Path[:i]
		}
		if i, ok := index[prefix]; ok {
			groups[i].fields = append(groups[i].fields, f)
		} else {
			index[prefix] = len(groups)
			groups = append(groups, fieldGroup{prefix: prefix, fields: []docmodel.Field{f}})
		}
	}
	return groups
}

// --- Fields ---

func writeField(b *strings.Builder, f docmodel.Field, flavor Flavor) {
	if f.Title != "" {
		fmt.Fprintf(b, "### %s (`%s`)\n\n", f.Title, f.Path)
	} else {
		fmt.Fprintf(b, "### `%s`\n\n", f.Path)
	}

	fmt.Fprintf(b, "%s\n\n", fieldMeta(f))
	if line := badgeLine(f, flavor); line != "" {
		fmt.Fprintf(b, "%s\n\n", line)
	}

	if f.Deprecated {
		writeDeprecationNotice(b, f, flavor)
	}

	if f.Description != "" {
		fmt.Fprintf(b, "%s\n\n", f.Description)
	}

	writeExamples(b, f, flavor)

	if f.ExternalDocs != nil && f.ExternalDocs.URL != "" {
		if f.ExternalDocs.Description != "" {
			fmt.Fprintf(b, "**See also:** [%s](%s)\n\n", f.ExternalDocs.Description, f.ExternalDocs.URL)
		} else {
			fmt.Fprintf(b, "**See also:** %s\n\n", f.ExternalDocs.URL)
		}
	}

	if f.Constraints != nil {
		writeConstraints(b, f.Constraints)
	}
}

// fieldMeta renders a field's type and non-flag metadata as a single italic
// line, e.g. "*type: `string` · format: email · default: `prod`*". Flags
// (deprecated, sensitive, read-only, write-once) render separately as
// badges; see badgeLine.
func fieldMeta(f docmodel.Field) string {
	parts := []string{fmt.Sprintf("type: `%s`", f.Type)}
	if f.Format != "" {
		parts = append(parts, fmt.Sprintf("format: %s", f.Format))
	}
	if f.Nullable {
		parts = append(parts, "nullable")
	}
	if f.Default != "" {
		parts = append(parts, fmt.Sprintf("default: `%s`", f.Default))
	}
	if len(f.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags: %s", strings.Join(f.Tags, ", ")))
	}
	return "*" + strings.Join(parts, " · ") + "*"
}

type badge struct {
	icon, label string
}

// badgeLine renders deprecated/sensitive/read-only/write-once as a bold
// badge line, visually distinct from the italic meta line. Material adds an
// icon per badge.
func badgeLine(f docmodel.Field, flavor Flavor) string {
	var badges []badge
	if f.Deprecated {
		badges = append(badges, badge{"⚠️", "Deprecated"})
	}
	if f.Sensitive {
		badges = append(badges, badge{"🔒", "Sensitive"})
	}
	if f.ReadOnly {
		badges = append(badges, badge{"👁", "Read-only"})
	}
	if f.WriteOnce {
		badges = append(badges, badge{"🔏", "Write-once"})
	}
	if len(badges) == 0 {
		return ""
	}
	parts := make([]string, len(badges))
	for i, bd := range badges {
		if flavor == Material {
			parts[i] = fmt.Sprintf("**%s %s**", bd.icon, bd.label)
		} else {
			parts[i] = fmt.Sprintf("**%s**", bd.label)
		}
	}
	return strings.Join(parts, " · ")
}

func writeDeprecationNotice(b *strings.Builder, f docmodel.Field, flavor Flavor) {
	body := "This field should no longer be used."
	if f.RedirectTo != "" {
		body = fmt.Sprintf("Use `%s` instead.", f.RedirectTo)
	}
	if flavor == Material {
		fmt.Fprintln(b, `!!! warning "Deprecated"`)
		fmt.Fprintf(b, "    %s\n\n", body)
		return
	}
	fmt.Fprintf(b, "> **Deprecated** — %s\n\n", body)
}

func writeExamples(b *strings.Builder, f docmodel.Field, flavor Flavor) {
	if f.Example != "" {
		fmt.Fprintf(b, "**Example:** `%s`\n\n", f.Example)
	}
	if len(f.Examples) == 0 {
		return
	}

	names := make([]string, 0, len(f.Examples))
	for name := range f.Examples {
		names = append(names, name)
	}
	sort.Strings(names)

	if flavor == Material && len(names) >= 2 {
		fmt.Fprintln(b, "**Examples:**")
		fmt.Fprintln(b)
		for _, name := range names {
			ex := f.Examples[name]
			fmt.Fprintf(b, "=== %q\n\n", name)
			fmt.Fprintf(b, "    **Value:** `%s`\n", ex.Value)
			if ex.Summary != "" {
				fmt.Fprintf(b, "\n    %s\n", ex.Summary)
			}
			fmt.Fprintln(b)
		}
		return
	}

	fmt.Fprintln(b, "**Examples:**")
	for _, name := range names {
		ex := f.Examples[name]
		if ex.Summary != "" {
			fmt.Fprintf(b, "- **%s:** `%s` — %s\n", name, ex.Value, ex.Summary)
		} else {
			fmt.Fprintf(b, "- **%s:** `%s`\n", name, ex.Value)
		}
	}
	fmt.Fprintln(b)
}

func writeConstraints(b *strings.Builder, c *docmodel.Constraints) {
	var lines []string
	if c.Minimum != nil {
		lines = append(lines, fmt.Sprintf("Minimum: %s", formatFloat(*c.Minimum)))
	}
	if c.Maximum != nil {
		lines = append(lines, fmt.Sprintf("Maximum: %s", formatFloat(*c.Maximum)))
	}
	if c.ExclusiveMinimum != nil {
		lines = append(lines, fmt.Sprintf("Exclusive minimum: %s", formatFloat(*c.ExclusiveMinimum)))
	}
	if c.ExclusiveMaximum != nil {
		lines = append(lines, fmt.Sprintf("Exclusive maximum: %s", formatFloat(*c.ExclusiveMaximum)))
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
	if len(c.AllowedSchemes) > 0 {
		lines = append(lines, fmt.Sprintf("Allowed schemes: %s", strings.Join(c.AllowedSchemes, ", ")))
	}
	if len(lines) == 0 {
		return
	}
	fmt.Fprintln(b, "**Constraints:**")
	for _, l := range lines {
		fmt.Fprintf(b, "- %s\n", l)
	}
	fmt.Fprintln(b)
}

// formatFloat formats f for display, omitting the decimal point when the
// value is a whole number (e.g. 1.0 -> "1", 1.5 -> "1.5").
func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}
