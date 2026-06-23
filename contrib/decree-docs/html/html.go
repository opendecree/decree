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
	"strings"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
	"github.com/opendecree/decree/contrib/decree-docs/internal/render"
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

	groups := render.GroupByPrefix(doc.Schema.Fields)
	page := pageView{
		Title:       render.Title(doc.Schema),
		Sub:         schemaSubtitle(doc.Schema),
		Groups:      make([]groupView, 0, len(groups)),
		CSS:         template.CSS(buildCSS(theme, opts.CSS)),
		NumField:    len(doc.Schema.Fields),
		NumType:     countTypes(doc.Schema.Fields),
		Validations: newValidationViews(doc.Schema.Validations),
	}
	for _, g := range groups {
		gv := groupView{Prefix: g.Prefix}
		for _, f := range g.Fields {
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

// --- View model ---

type pageView struct {
	Title       string
	Sub         string
	Groups      []groupView
	CSS         template.CSS
	NumField    int
	NumType     int
	Validations []validationView
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

// validationView is the rendered form of a schema-level CEL validation
// rule. Severity is normalized to "error" or "warning" (anything other
// than "error" is treated as a warning) so the template can select the
// matching CSS class without re-deriving the default.
type validationView struct {
	Rule     string
	Message  string
	Severity string // "error" or "warning"
	Label    string // "Error" or "Warning"
}

func newValidationViews(validations []docmodel.Validation) []validationView {
	if len(validations) == 0 {
		return nil
	}
	out := make([]validationView, 0, len(validations))
	for _, v := range validations {
		severity, label := render.Severity(v.Severity)
		out = append(out, validationView{
			Rule:     v.Rule,
			Message:  v.Message,
			Severity: severity,
			Label:    label,
		})
	}
	return out
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
	for _, name := range render.SortedExampleNames(f) {
		ex := f.Examples[name]
		fv.Examples = append(fv.Examples, exampleView{Name: name, Value: ex.Value, Summary: ex.Summary})
	}

	fv.Constraints = constraintLines(f.Constraints)
	return fv
}

func constraintLines(c *docmodel.Constraints) []string {
	cs := render.Constraints(c)
	if len(cs) == 0 {
		return nil
	}
	lines := make([]string, len(cs))
	for i, k := range cs {
		if k.Kind == render.ListConstraint {
			lines[i] = fmt.Sprintf("%s: %s", k.Label, strings.Join(k.Values, ", "))
		} else {
			lines[i] = fmt.Sprintf("%s: %s", k.Label, k.Value)
		}
	}
	return lines
}
