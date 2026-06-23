// Package markdown renders a decree-docs documentation model
// ([docmodel.Document]) as Markdown.
//
// Two flavors are supported. "plain" emits portable CommonMark. "material"
// emits Markdown that additionally relies on the MkDocs Material extensions
// this repo's mkdocs.yml already enables: admonition blocks for deprecation
// notices and validation severity, and pymdownx.tabbed content tabs for
// fields with two or more named examples. In both flavors, field flags
// (deprecated, sensitive, read-only, write-once) render as a bold badge line
// distinct from the field's type/format meta line; material adds an icon
// per badge.
//
// Two page modes are supported. [SinglePage] renders the whole schema as one
// page. [MultiPage] renders an index page plus one page per top-level
// path-prefix group (the same grouping sdk/tools/docgen uses for "##"
// sections), each holding the fields under that group.
//
// Schema-level cross-field validation rules ([docmodel.Schema.Validations])
// render as a "## Validations" section on the single page (right after the
// header) or on the multi-page index (since a rule can span multiple
// groups). Each rule's CEL expression renders as an untagged fenced code
// block — CEL isn't a Prism/standard-highlighter language, so tagging the
// fence would invite a wrong language guess and mismatched-keyword
// highlighting.
package markdown

import (
	"fmt"
	"sort"
	"strings"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
	"github.com/opendecree/decree/contrib/decree-docs/internal/render"
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

	groups := render.GroupByPrefix(doc.Schema.Fields)

	switch opts.Pages {
	case SinglePage, "":
		var b strings.Builder
		writeHeader(&b, doc.Schema)
		writeValidations(&b, doc.Schema.Validations, opts.Flavor)
		for _, g := range groups {
			fmt.Fprintf(&b, "## %s\n\n", g.Prefix)
			for _, f := range g.Fields {
				writeField(&b, f, opts.Flavor)
			}
		}
		return []Page{{Name: "index", Content: b.String()}}, nil
	case MultiPage:
		pages := []Page{indexPage(doc.Schema, groups, opts.Flavor)}
		for _, g := range groups {
			var b strings.Builder
			fmt.Fprintf(&b, "# %s\n\n", g.Prefix)
			for _, f := range g.Fields {
				writeField(&b, f, opts.Flavor)
			}
			pages = append(pages, Page{Name: g.Prefix, Content: b.String()})
		}
		return pages, nil
	default:
		return nil, fmt.Errorf("unknown pages mode %q (valid modes: %s, %s)", opts.Pages, SinglePage, MultiPage)
	}
}

func indexPage(s docmodel.Schema, groups []render.Group, flavor Flavor) Page {
	var b strings.Builder
	writeHeader(&b, s)
	writeValidations(&b, s.Validations, flavor)
	fmt.Fprintln(&b, "## Groups")
	fmt.Fprintln(&b)
	for _, g := range groups {
		noun := "fields"
		if len(g.Fields) == 1 {
			noun = "field"
		}
		fmt.Fprintf(&b, "- [%s](%s.md) — %d %s\n", g.Prefix, g.Prefix, len(g.Fields), noun)
	}
	fmt.Fprintln(&b)
	return Page{Name: "index", Content: b.String()}
}

func writeHeader(b *strings.Builder, s docmodel.Schema) {
	fmt.Fprintf(b, "# %s\n\n", render.Title(s))
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

// writeValidations renders the schema's cross-field CEL validation rules as
// a list, one entry per rule: the rule expression as a fenced code block
// (deliberately untagged — CEL isn't a Prism/standard-highlighter language,
// so an untagged fence avoids mismatched-keyword highlighting from a wrong
// language guess) followed by the human-readable message. Severity is
// distinguished using the same admonition/blockquote treatment as
// writeDeprecationNotice: material gets a `!!!` admonition (error or
// warning), plain gets a bold-prefixed blockquote.
func writeValidations(b *strings.Builder, validations []docmodel.Validation, flavor Flavor) {
	if len(validations) == 0 {
		return
	}
	fmt.Fprintln(b, "## Validations")
	fmt.Fprintln(b)
	for _, v := range validations {
		fmt.Fprintln(b, "```")
		fmt.Fprintln(b, v.Rule)
		fmt.Fprintln(b, "```")
		fmt.Fprintln(b)
		writeValidationMessage(b, v, flavor)
	}
}

// writeValidationMessage renders a validation's message with its severity
// visually distinguished, mirroring writeDeprecationNotice's admonition
// (material) / blockquote (plain) pattern. The canonical severity key from
// [render.Severity] doubles as the material admonition type.
func writeValidationMessage(b *strings.Builder, v docmodel.Validation, flavor Flavor) {
	admonition, label := render.Severity(v.Severity)
	if flavor == Material {
		fmt.Fprintf(b, "!!! %s %q\n", admonition, label)
		fmt.Fprintf(b, "    %s\n\n", v.Message)
		return
	}
	fmt.Fprintf(b, "> **%s** — %s\n\n", label, v.Message)
}

func writeExamples(b *strings.Builder, f docmodel.Field, flavor Flavor) {
	if f.Example != "" {
		fmt.Fprintf(b, "**Example:** `%s`\n\n", f.Example)
	}
	if len(f.Examples) == 0 {
		return
	}

	names := render.SortedExampleNames(f)

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
	lines := render.Constraints(c)
	if len(lines) == 0 {
		return
	}
	fmt.Fprintln(b, "**Constraints:**")
	for _, k := range lines {
		fmt.Fprintf(b, "- %s: %s\n", k.Label, constraintValue(k))
	}
	fmt.Fprintln(b)
}

// constraintValue formats a constraint value for Markdown: a regex pattern
// renders as a code span, list values join with commas, and scalars render
// verbatim.
func constraintValue(k render.Constraint) string {
	switch k.Kind {
	case render.CodeConstraint:
		return fmt.Sprintf("`%s`", k.Value)
	case render.ListConstraint:
		return strings.Join(k.Values, ", ")
	default:
		return k.Value
	}
}
