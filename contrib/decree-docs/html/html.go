// Package html renders a decree-docs documentation model
// ([docmodel.Document]) as a single self-contained HTML document.
//
// The output has no external dependencies: CSS is inlined in a <style>
// block, there is no JavaScript, and no fonts/icons/scripts are loaded from
// a CDN (system font stack + Unicode glyphs only). It renders correctly
// when opened directly from disk with no server.
//
// Theming follows Scalar's cascade-layer pattern: built-in theme tokens
// live in the decree.theme layer as --decree-* custom properties, and a
// user-supplied [Options.CSS] string is appended in a later decree.user
// layer so it overrides built-in styles without needing higher selector
// specificity or !important. Three named themes are supported: light,
// dark, and auto (light at :root, dark via prefers-color-scheme).
package html

import (
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
)

// Theme selects the built-in color scheme baked into the document.
type Theme string

const (
	// Light is the default theme.
	Light Theme = "light"
	// Dark uses the dark token values at :root.
	Dark Theme = "dark"
	// Auto emits light token values at :root and overrides them with dark
	// values inside an @media (prefers-color-scheme: dark) block, so the
	// page follows the reader's OS/browser preference.
	Auto Theme = "auto"
)

// Options configures Render.
type Options struct {
	// Theme selects the built-in color scheme. Defaults to [Light] when empty.
	Theme Theme
	// CSS is raw CSS injected after the built-in styles, in the decree.user
	// cascade layer, so it takes precedence over every built-in rule. Empty
	// means no user override.
	CSS string
}

// Render renders doc as a single self-contained HTML document.
func Render(doc *docmodel.Document, opts Options) (string, error) {
	theme := opts.Theme
	if theme == "" {
		theme = Light
	}
	if theme != Light && theme != Dark && theme != Auto {
		return "", fmt.Errorf("unknown theme %q (valid themes: %s, %s, %s)", theme, Light, Dark, Auto)
	}

	groups := groupByPrefix(doc.Schema.Fields)
	page := pageView{
		Title:    schemaTitle(doc.Schema),
		Sub:      schemaSubtitle(doc.Schema),
		Groups:   make([]groupView, 0, len(groups)),
		CSS:      template.CSS(buildCSS(theme, opts.CSS)),
		NumField: len(doc.Schema.Fields),
		NumType:  countTypes(doc.Schema.Fields),
	}
	for _, g := range groups {
		gv := groupView{Prefix: g.prefix}
		for _, f := range g.fields {
			gv.Fields = append(gv.Fields, newFieldView(f))
		}
		page.Groups = append(page.Groups, gv)
	}

	var b strings.Builder
	if err := pageTemplate.Execute(&b, page); err != nil {
		return "", fmt.Errorf("render html: %w", err)
	}
	return b.String(), nil
}

func schemaTitle(s docmodel.Schema) string {
	if s.Info != nil && s.Info.Title != "" {
		return s.Info.Title
	}
	return s.Name
}

func schemaSubtitle(s docmodel.Schema) string {
	var parts []string
	if s.Version > 0 {
		parts = append(parts, fmt.Sprintf("v%d", s.Version))
	}
	if s.Info != nil && s.Info.Author != "" {
		parts = append(parts, s.Info.Author)
	}
	return strings.Join(parts, " · ")
}

func countTypes(fields []docmodel.Field) int {
	seen := make(map[string]struct{})
	for _, f := range fields {
		seen[f.Type] = struct{}{}
	}
	return len(seen)
}

// --- Grouping ---

type fieldGroup struct {
	prefix string
	fields []docmodel.Field
}

// groupByPrefix groups fields by their top-level path prefix. fields is
// assumed sorted by path (docmodel guarantees this), so the first
// occurrence of each prefix establishes deterministic group order.
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

// --- View model ---

type pageView struct {
	Title    string
	Sub      string
	Groups   []groupView
	CSS      template.CSS
	NumField int
	NumType  int
}

type groupView struct {
	Prefix string
	Fields []fieldView
}

type badgeView struct {
	Label string
	Class string // CSS class selecting the badge color
	Icon  string
}

type exampleView struct {
	Name    string
	Value   string
	Summary string
}

type fieldView struct {
	docmodel.Field
	Badges      []badgeView
	Examples    []exampleView
	Constraints []string
}

func newFieldView(f docmodel.Field) fieldView {
	fv := fieldView{Field: f}

	if f.Nullable {
		fv.Badges = append(fv.Badges, badgeView{Label: "Nullable", Class: "neutral"})
	}
	if f.ReadOnly {
		fv.Badges = append(fv.Badges, badgeView{Label: "Read-only", Class: "neutral", Icon: "⊙"})
	}
	if f.WriteOnce {
		fv.Badges = append(fv.Badges, badgeView{Label: "Write-once", Class: "info", Icon: "⟳"})
	}
	if f.Sensitive {
		fv.Badges = append(fv.Badges, badgeView{Label: "Sensitive", Class: "danger", Icon: "⊘"})
	}
	if f.Deprecated {
		fv.Badges = append(fv.Badges, badgeView{Label: "Deprecated", Class: "warn", Icon: "⚠"})
	}

	if f.Example != "" {
		fv.Examples = append(fv.Examples, exampleView{Value: f.Example})
	}
	if len(f.Examples) > 0 {
		names := make([]string, 0, len(f.Examples))
		for name := range f.Examples {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			ex := f.Examples[name]
			fv.Examples = append(fv.Examples, exampleView{Name: name, Value: ex.Value, Summary: ex.Summary})
		}
	}

	fv.Constraints = constraintLines(f.Constraints)
	return fv
}

func constraintLines(c *docmodel.Constraints) []string {
	if c == nil {
		return nil
	}
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
		lines = append(lines, fmt.Sprintf("Pattern: %s", c.Pattern))
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
	return lines
}

// formatFloat formats f for display, omitting the decimal point when the
// value is a whole number (e.g. 1.0 -> "1", 1.5 -> "1.5").
func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}
