// Package mdx renders a decree-docs documentation model
// ([docmodel.Document]) as Docusaurus-compatible MDX.
//
// Render emits a doc tree, not a single file: an index.mdx overview page,
// plus one category folder per top-level path-prefix group, each holding a
// _category_.json (sidebar label + position) and an index.mdx with that
// group's fields. The tree drops directly into a Docusaurus docs/ folder.
//
// Schema-level cross-field validation rules ([docmodel.Schema.Validations])
// render as a "## Validations" section on the index page (a rule can
// reference fields across groups, so there is no single group page to put it
// on). Each rule's CEL expression renders as an untagged fenced code block —
// CEL isn't a Prism language, so tagging the fence would invite a wrong
// language guess and mismatched-keyword highlighting — and its severity
// (error/warning) renders via the same bracket-label admonition syntax as
// the deprecation notice below.
//
// MDX v3 parses '{' and '<' as the start of a JSX expression or tag, so
// every piece of schema-sourced text (descriptions, examples, enum values,
// defaults, patterns, tags) must be neutralized before it reaches the
// output: prose runs through [escapeText], which backslash-escapes '{',
// '<', '`', and '\' (escaping '<' also defangs "<!--" HTML-comment
// sequences); values that are naturally code-like run through [codeSpan]
// instead, which wraps them in backticks — a code span's content is read as
// literal text and not reparsed as MDX, the standard escaping technique for
// this class of generator.
package mdx

import (
	"fmt"
	"sort"
	"strings"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
	"github.com/opendecree/decree/contrib/decree-docs/internal/render"
)

// Page is one file in the rendered MDX tree.
type Page struct {
	// Path is the file's path relative to the output root, e.g.
	// "index.mdx" or "auth/_category_.json".
	Path string
	// Content is the file's contents.
	Content string
}

// Render renders doc as a Docusaurus MDX doc tree.
func Render(doc *docmodel.Document) ([]Page, error) {
	groups := render.GroupByPrefix(doc.Schema.Fields)

	pages := make([]Page, 0, 1+2*len(groups))
	pages = append(pages, indexPage(doc.Schema, groups))
	for i, g := range groups {
		position := i + 1
		pages = append(pages, categoryFile(g.Prefix, position))
		pages = append(pages, groupPage(g))
	}
	return pages, nil
}

func indexPage(s docmodel.Schema, groups []render.Group) Page {
	var b strings.Builder
	writeFrontmatter(&b, "index", render.Title(s), "Overview", 0)

	if s.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", escapeText(s.Description))
	}
	if s.Version > 0 {
		if s.VersionDescription != "" {
			fmt.Fprintf(&b, "**Version:** %d — %s\n\n", s.Version, escapeText(s.VersionDescription))
		} else {
			fmt.Fprintf(&b, "**Version:** %d\n\n", s.Version)
		}
	}
	if s.Info != nil {
		writeInfo(&b, s.Info)
	}

	writeValidations(&b, s.Validations)

	fmt.Fprintln(&b, "## Groups")
	fmt.Fprintln(&b)
	for _, g := range groups {
		noun := "fields"
		if len(g.Fields) == 1 {
			noun = "field"
		}
		fmt.Fprintf(&b, "- [%s](./%s/index) — %d %s\n", escapeText(g.Prefix), g.Prefix, len(g.Fields), noun)
	}
	fmt.Fprintln(&b)

	return Page{Path: "index.mdx", Content: b.String()}
}

func writeInfo(b *strings.Builder, info *docmodel.Info) {
	if info.Author != "" {
		fmt.Fprintf(b, "**Author:** %s\n\n", escapeText(info.Author))
	}
	if c := info.Contact; c != nil {
		switch {
		case c.Email != "":
			fmt.Fprintf(b, "**Contact:** [%s](mailto:%s)\n\n", escapeText(c.Name), c.Email)
		case c.URL != "":
			fmt.Fprintf(b, "**Contact:** [%s](%s)\n\n", escapeText(c.Name), c.URL)
		case c.Name != "":
			fmt.Fprintf(b, "**Contact:** %s\n\n", escapeText(c.Name))
		}
	}
	if len(info.Labels) > 0 {
		labels := make([]string, 0, len(info.Labels))
		for k, v := range info.Labels {
			labels = append(labels, codeSpan(fmt.Sprintf("%s: %s", k, v)))
		}
		sort.Strings(labels)
		fmt.Fprintf(b, "**Labels:** %s\n\n", strings.Join(labels, ", "))
	}
}

func categoryFile(prefix string, position int) Page {
	label := jsonString(prefix)
	content := fmt.Sprintf("{\n  \"label\": %s,\n  \"position\": %d\n}\n", label, position)
	return Page{Path: prefix + "/_category_.json", Content: content}
}

func groupPage(g render.Group) Page {
	var b strings.Builder
	writeFrontmatter(&b, "index", g.Prefix, g.Prefix, 1)
	for i, f := range g.Fields {
		if i > 0 {
			fmt.Fprintln(&b, "---")
			fmt.Fprintln(&b)
		}
		writeField(&b, f)
	}
	return Page{Path: g.Prefix + "/index.mdx", Content: b.String()}
}

func writeFrontmatter(b *strings.Builder, id, title, sidebarLabel string, position int) {
	fmt.Fprintln(b, "---")
	fmt.Fprintf(b, "id: %s\n", yamlQuote(id))
	fmt.Fprintf(b, "title: %s\n", yamlQuote(title))
	fmt.Fprintf(b, "sidebar_label: %s\n", yamlQuote(sidebarLabel))
	fmt.Fprintf(b, "sidebar_position: %d\n", position)
	fmt.Fprintln(b, "---")
	fmt.Fprintln(b)
}

// --- Fields ---

func writeField(b *strings.Builder, f docmodel.Field) {
	// The heading is always the bare path, never "Title (path)" — a mixed
	// format makes the right-hand TOC wrap unevenly and gives the page two
	// different scanning rhythms. Title (when set) becomes a line under the
	// heading instead, so the data isn't lost.
	fmt.Fprintf(b, "### %s\n", monoHeading(f.Path))
	if f.Title != "" {
		fmt.Fprintf(b, "%s\n", escapeText(f.Title))
	}

	// nameBadges render on their own line directly under the heading, not
	// inside it — Docusaurus's right-hand TOC pulls its labels straight from
	// the heading's rendered content, so any HTML embedded in the "###" line
	// itself leaks into the sidebar too.
	var nameBadges []string
	if f.Nullable {
		nameBadges = append(nameBadges, pill("Nullable", "outline"))
	}
	if f.ReadOnly {
		nameBadges = append(nameBadges, pill("Read-only", "outline"))
	}
	if f.WriteOnce {
		nameBadges = append(nameBadges, pill("Write-once", "info"))
	}
	if f.Sensitive {
		nameBadges = append(nameBadges, pill("Sensitive", "danger"))
	}
	if len(nameBadges) > 0 {
		fmt.Fprintf(b, "%s\n", strings.Join(nameBadges, " "))
	}
	fmt.Fprintln(b)

	if f.Description != "" {
		fmt.Fprintf(b, "%s\n\n", escapeText(f.Description))
	}

	fmt.Fprintf(b, "%s\n\n", fieldMeta(f))

	if f.Deprecated {
		writeDeprecationNotice(b, f)
	}

	writeExamples(b, f)

	if f.ExternalDocs != nil && f.ExternalDocs.URL != "" {
		if f.ExternalDocs.Description != "" {
			fmt.Fprintf(b, "**See also:** [%s](%s)\n\n", escapeText(f.ExternalDocs.Description), f.ExternalDocs.URL)
		} else {
			fmt.Fprintf(b, "**See also:** %s\n\n", f.ExternalDocs.URL)
		}
	}

	if f.Constraints != nil {
		writeConstraints(b, f.Constraints)
	}
}

// fieldMeta renders a field's type and remaining metadata as a single
// upright (non-italic) line — italic monospace fights itself legibly, and
// upright with monospace value chips is the convention API references like
// Stripe and Prisma use. type is a plain code chip, not a pill — it
// identifies the field rather than flagging a state, so it doesn't need a
// color. The nullable/read-only/write-once/sensitive flags sit next to the
// field name instead (see writeField) and deprecated gets its own
// admonition (see writeDeprecationNotice).
func fieldMeta(f docmodel.Field) string {
	parts := []string{fmt.Sprintf("type %s", codeSpan(f.Type))}
	if f.Format != "" {
		parts = append(parts, fmt.Sprintf("format %s", codeSpan(f.Format)))
	}
	if f.Default != "" {
		parts = append(parts, fmt.Sprintf("default %s", codeSpan(f.Default)))
	}
	if len(f.Tags) > 0 {
		tags := make([]string, len(f.Tags))
		for i, t := range f.Tags {
			tags[i] = codeSpan(t)
		}
		parts = append(parts, fmt.Sprintf("tags %s", strings.Join(tags, " ")))
	}
	return strings.Join(parts, " · ")
}

// pillTitles holds the tooltip text (HTML title attribute) shown on hover
// for each fixed pill label.
var pillTitles = map[string]string{
	"Nullable":   "Accepts a null value",
	"Read-only":  "Clients cannot write this field",
	"Write-once": "Can only be set once; immutable after that",
	"Sensitive":  "Value is not displayed in clear text",
}

// pill renders a colored badge. label is always one of this package's own
// fixed strings, never schema-sourced text, so it needs no escaping. The
// HTML title attribute gives a native hover tooltip with no extra JS or
// component.
//
// variant "outline" is a muted low-emphasis style for flags that are just
// FYI properties (nullable, read-only), not states worth alarm — it's a
// custom inline style rather than Infima's badge--secondary class, because
// badge--secondary renders as a near-white solid fill in dark mode, making
// the least urgent flag the loudest thing on the card. The inline style
// uses Infima's emphasis CSS variables so it still adapts to the host
// site's light/dark theme. Other variants use Infima's built-in
// badge--<variant> classes — the same CSS Docusaurus already ships
// globally — since those colors (danger/info/...) are semantically fixed
// regardless of the site's brand color, unlike badge--primary/secondary.
func pill(label, variant string) string {
	if variant == "outline" {
		style := "background:transparent;border:1px solid var(--ifm-color-emphasis-400);" +
			"color:var(--ifm-color-emphasis-700);border-radius:var(--ifm-badge-border-radius,0.4rem);" +
			"padding:0.2em 0.6em;font-size:0.75em;font-weight:700"
		return fmt.Sprintf(`<span style={{%s}} title="%s">%s</span>`, jsxStyle(style), pillTitles[label], label)
	}
	return fmt.Sprintf(`<span className="badge badge--%s" title="%s">%s</span>`, variant, pillTitles[label], label)
}

// jsxStyle converts a CSS declaration string ("a:b;c:d") into a JSX object
// literal body ('"a": "b", "c": "d"') for use in a style={{...}} attribute —
// MDX/JSX requires style as an object, not a CSS string like plain HTML.
func jsxStyle(css string) string {
	decls := strings.Split(strings.TrimSuffix(css, ";"), ";")
	pairs := make([]string, 0, len(decls))
	for _, d := range decls {
		k, v, _ := strings.Cut(d, ":")
		pairs = append(pairs, fmt.Sprintf(`"%s": "%s"`, strings.TrimSpace(k), strings.TrimSpace(v)))
	}
	return strings.Join(pairs, ", ")
}

// monoHeading renders a field path as bold monospace text without the
// <code> chip background a markdown backtick span would give it — a
// heading shouldn't look like a code block, it should read as a heading
// first. path is always schema-controlled (never free-form prose), but
// still passed through escapeText defensively since it ends up inside an
// MDX/JSX-adjacent span rather than a code span.
func monoHeading(path string) string {
	return fmt.Sprintf(`<span style={{fontFamily: "var(--ifm-font-family-monospace)"}}>%s</span>`, escapeText(path))
}

// writeValidations renders the schema's cross-field CEL validation rules on
// the index page (not per-group), since a rule may reference fields across
// groups. Each rule renders as a list entry: the CEL expression as an
// untagged fenced code block (CEL isn't a Prism language, so an untagged
// fence avoids a wrong language guess and mismatched-keyword highlighting),
// followed by the severity-distinguished message. Mirrors the md backend's
// writeValidations/writeValidationMessage.
func writeValidations(b *strings.Builder, validations []docmodel.Validation) {
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
		writeValidationMessage(b, v)
	}
}

// writeValidationMessage renders a validation's message with its severity
// distinguished via the same bracket-label admonition mechanism
// writeDeprecationNotice uses: ":::danger[Error]" for error severity,
// ":::caution[Warning]" for warning (and the default, in case Severity is
// unset or holds an unrecognized value). The canonical key from
// [render.Severity] selects the admonition; mdx maps error onto Docusaurus's
// "danger" and warning onto "caution".
func writeValidationMessage(b *strings.Builder, v docmodel.Validation) {
	key, label := render.Severity(v.Severity)
	admonition := "caution"
	if key == "error" {
		admonition = "danger"
	}
	fmt.Fprintf(b, ":::%s[%s]\n", admonition, label)
	fmt.Fprintf(b, "%s\n", escapeText(v.Message))
	fmt.Fprintln(b, ":::")
	fmt.Fprintln(b)
}

func writeDeprecationNotice(b *strings.Builder, f docmodel.Field) {
	body := "This field should no longer be used."
	if f.RedirectTo != "" {
		body = fmt.Sprintf("Use %s instead.", codeSpan(f.RedirectTo))
	}
	fmt.Fprintln(b, ":::caution[Deprecated]")
	fmt.Fprintln(b, body)
	fmt.Fprintln(b, ":::")
	fmt.Fprintln(b)
}

// writeExamples renders f.Example and f.Examples as a single "Examples:"
// list — keeping them as two separate blocks (a singular "Example:" right
// above a plural "Examples:") reads like a duplication bug rather than two
// distinct pieces of data.
func writeExamples(b *strings.Builder, f docmodel.Field) {
	if f.Example == "" && len(f.Examples) == 0 {
		return
	}

	names := render.SortedExampleNames(f)

	fmt.Fprintln(b, "**Examples:**")
	if f.Example != "" {
		fmt.Fprintf(b, "- %s\n", codeSpan(f.Example))
	}
	for _, name := range names {
		ex := f.Examples[name]
		if ex.Summary != "" {
			fmt.Fprintf(b, "- **%s:** %s — %s\n", escapeText(name), codeSpan(ex.Value), escapeText(ex.Summary))
		} else {
			fmt.Fprintf(b, "- **%s:** %s\n", escapeText(name), codeSpan(ex.Value))
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

// constraintValue formats a constraint value for MDX: regex patterns and
// every list member run through codeSpan, since enum members and patterns
// are schema-sourced and may contain MDX-special characters. Scalars are
// numeric or fixed notes and render verbatim.
func constraintValue(k render.Constraint) string {
	switch k.Kind {
	case render.CodeConstraint:
		return codeSpan(k.Value)
	case render.ListConstraint:
		parts := make([]string, len(k.Values))
		for i, v := range k.Values {
			parts[i] = codeSpan(v)
		}
		return strings.Join(parts, ", ")
	default:
		return k.Value
	}
}

// --- Escaping ---

// escapeText backslash-escapes the characters MDX v3 treats specially in
// prose: '<' and '{' open JSX tags/expressions, '`' opens a code span, and
// '\' is the escape character itself. Escaping '<' also defangs "<!--"
// HTML-comment sequences, since a comment can't open without an unescaped
// '<'.
func escapeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\', '<', '{', '`':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// codeSpan wraps s in backticks, widening the delimiter when s itself
// contains a run of backticks (CommonMark code span rules). Code span
// content is read as literal text, not reparsed as MDX, so this is the
// preferred way to render schema-sourced values that are naturally
// code-like (examples, defaults, enum members, patterns).
func codeSpan(s string) string {
	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	delim := strings.Repeat("`", longest+1)
	if s == "" || strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		return delim + " " + s + " " + delim
	}
	return delim + s + delim
}

// yamlQuote double-quotes s for use as a YAML frontmatter scalar value,
// escaping backslashes, double quotes, and newlines.
func yamlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return `"` + s + `"`
}

// jsonString renders s as a JSON string literal for _category_.json.
func jsonString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
