package html

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
	"github.com/opendecree/decree/contrib/decree-docs/loader"
)

// -update rewrites the golden files from current output:
//
//	go test . -run TestRender_Golden -update
var update = flag.Bool("update", false, "rewrite golden files")

// TestRender_Golden pins the rendered output for every theme against the
// full fixture, which exercises every documented schema and field property.
func TestRender_Golden(t *testing.T) {
	doc, err := loader.FromFile("../testdata/full.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	tests := []struct {
		name   string
		opts   Options
		golden string
	}{
		{name: "light", opts: Options{Theme: Light}, golden: "testdata/light.golden.html"},
		{name: "dark", opts: Options{Theme: Dark}, golden: "testdata/dark.golden.html"},
		{name: "auto", opts: Options{Theme: Auto}, golden: "testdata/auto.golden.html"},
		{name: "default theme", opts: Options{}, golden: "testdata/light.golden.html"},
		{
			name:   "css override",
			opts:   Options{Theme: Light, CSS: ":root {\n  --decree-accent: #7c3aed;\n}\n"},
			golden: "testdata/css-override.golden.html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(doc, tt.opts)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if *update {
				if err := os.WriteFile(tt.golden, []byte(got), 0o644); err != nil {
					t.Fatalf("update golden: %v", err)
				}
			}
			want, err := os.ReadFile(tt.golden)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if got != string(want) {
				t.Errorf("output does not match %s:\ngot:\n%s\nwant:\n%s", tt.golden, got, want)
			}
		})
	}
}

// TestRender_MinimalSchema pins behavior on a schema with no flags, no
// examples, and no constraints: no badges, no deprecation notice, no
// examples/constraints sections should be emitted.
func TestRender_MinimalSchema(t *testing.T) {
	doc, err := loader.FromFile("../testdata/minimal.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	got, err := Render(doc, Options{Theme: Light})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, unwanted := range []string{"Deprecated", "Sensitive", "Read-only", "Write-once", "class=\"examples\"", "class=\"constraints\""} {
		if strings.Contains(got, unwanted) {
			t.Errorf("unexpected %q in minimal output", unwanted)
		}
	}
}

// TestRender_NoExternalReferences asserts the output has no script tags,
// no external stylesheet links, and no remote font/CDN references, per
// #916's "single-file output renders offline with no network requests"
// acceptance criterion.
func TestRender_NoExternalReferences(t *testing.T) {
	doc, err := loader.FromFile("../testdata/full.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	got, err := Render(doc, Options{Theme: Auto})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, forbidden := range []string{"<script", "<link", "@import", "fonts.googleapis", "cdn.", "http://", "@font-face"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("output contains forbidden external reference %q", forbidden)
		}
	}
}

// TestRender_CSSInjectionPrecedence asserts the user --css content lands in
// a decree.user cascade layer declared after decree.reset/theme/components,
// so it overrides built-in styles without !important or extra specificity.
func TestRender_CSSInjectionPrecedence(t *testing.T) {
	doc, err := loader.FromFile("../testdata/minimal.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	userCSS := ":root { --decree-accent: #ff00ff; }"
	got, err := Render(doc, Options{Theme: Light, CSS: userCSS})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	layerDecl := "@layer decree.reset, decree.theme, decree.components, decree.user;"
	if !strings.Contains(got, layerDecl) {
		t.Fatalf("expected layer order declaration %q in output", layerDecl)
	}
	userBlock := "@layer decree.user {\n" + userCSS + "\n}"
	if !strings.Contains(got, userBlock) {
		t.Errorf("expected user CSS wrapped in decree.user layer, got:\n%s", got)
	}
	if strings.Contains(got, "!important") {
		t.Errorf("output should never need !important, got:\n%s", got)
	}

	// decree.user must be the last layer named in the order declaration,
	// and the user block must come after every built-in layer block.
	if i, j := strings.Index(got, layerDecl), strings.Index(got, userBlock); i < 0 || j < 0 || j < i {
		t.Errorf("expected layer order declaration before the user block")
	}
}

func TestRender_NoUserCSS(t *testing.T) {
	doc, err := loader.FromFile("../testdata/minimal.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	got, err := Render(doc, Options{Theme: Light})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(got, "decree.user {") {
		t.Errorf("expected no decree.user block when CSS is empty, got:\n%s", got)
	}
}

func TestRender_UnknownTheme(t *testing.T) {
	doc, err := loader.FromFile("../testdata/minimal.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	if _, err := Render(doc, Options{Theme: "neon"}); err == nil {
		t.Error("expected error for unknown theme, got nil")
	}
}

// TestRender_EdgeCases exercises branches the full/minimal fixtures don't
// reach: a field with a title (rendered next to its path instead of
// replacing it), a single inline example (not the named-examples map), and
// an externalDocs block with no description.
func TestRender_EdgeCases(t *testing.T) {
	half := 0.5
	doc := docmodel.New(docmodel.Schema{
		Name: "edge",
		Fields: []docmodel.Field{
			// Top-level field: path has no dot, so it equals its own group
			// prefix and fieldShortName must return it unchanged.
			{Path: "standalone", Type: "bool"},
			{
				Path:         "alpha.value",
				Type:         "string",
				Title:        "Alpha Value",
				Example:      "hello",
				ExternalDocs: &docmodel.ExternalDocs{URL: "https://example.com/alpha"},
			},
			{
				Path:        "alpha.fraction",
				Type:        "number",
				Constraints: &docmodel.Constraints{Minimum: &half},
			},
		},
	})
	got, err := Render(doc, Options{Theme: Light})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(got, "<h3>Alpha Value</h3>") {
		t.Errorf("expected title heading, got:\n%s", got)
	}
	if !strings.Contains(got, "<code class=\"value mono\">hello</code>") {
		t.Errorf("expected inline example value, got:\n%s", got)
	}
	if !strings.Contains(got, `href="https://example.com/alpha"`) {
		t.Errorf("expected externalDocs link, got:\n%s", got)
	}
	if !strings.Contains(got, ">standalone</a>") {
		t.Errorf("expected unchanged nav label for a top-level field, got:\n%s", got)
	}
	if !strings.Contains(got, "Minimum: 0.5") {
		t.Errorf("expected fractional minimum, got:\n%s", got)
	}
}

// TestRender_EscapesSchemaText ensures schema-sourced text that looks like
// markup is escaped, not interpreted as HTML, since descriptions/titles are
// untrusted relative to the renderer.
func TestRender_EscapesSchemaText(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name: "edge",
		Fields: []docmodel.Field{
			{
				Path:        "alpha.value",
				Type:        "string",
				Description: `<script>alert(1)</script>`,
			},
		},
	})
	got, err := Render(doc, Options{Theme: Light})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(got, "<script>alert(1)</script>") {
		t.Errorf("expected schema text to be escaped, got:\n%s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("expected escaped script tag, got:\n%s", got)
	}
}
