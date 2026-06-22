package markdown

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

// TestRender_Golden pins the rendered output for every flavor x page mode
// combination against the full fixture, which exercises every documented
// schema and field property.
func TestRender_Golden(t *testing.T) {
	doc, err := loader.FromFile("../testdata/full.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	tests := []struct {
		name   string
		opts   Options
		golden map[string]string // page name -> golden file path
	}{
		{
			name: "plain single",
			opts: Options{Flavor: Plain, Pages: SinglePage},
			golden: map[string]string{
				"index": "testdata/plain-single.golden.md",
			},
		},
		{
			name: "material single",
			opts: Options{Flavor: Material, Pages: SinglePage},
			golden: map[string]string{
				"index": "testdata/material-single.golden.md",
			},
		},
		{
			name: "plain multi",
			opts: Options{Flavor: Plain, Pages: MultiPage},
			golden: map[string]string{
				"index":    "testdata/plain-multi-index.golden.md",
				"payments": "testdata/plain-multi-payments.golden.md",
			},
		},
		{
			name: "material multi",
			opts: Options{Flavor: Material, Pages: MultiPage},
			golden: map[string]string{
				"index":    "testdata/material-multi-index.golden.md",
				"payments": "testdata/material-multi-payments.golden.md",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages, err := Render(doc, tt.opts)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if len(pages) != len(tt.golden) {
				t.Fatalf("got %d pages, want %d", len(pages), len(tt.golden))
			}
			for _, p := range pages {
				golden, ok := tt.golden[p.Name]
				if !ok {
					t.Fatalf("unexpected page %q", p.Name)
				}
				if *update {
					if err := os.WriteFile(golden, []byte(p.Content), 0o644); err != nil {
						t.Fatalf("update golden: %v", err)
					}
				}
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatalf("read golden: %v", err)
				}
				if p.Content != string(want) {
					t.Errorf("page %q does not match %s:\ngot:\n%s\nwant:\n%s", p.Name, golden, p.Content, want)
				}
			}
		})
	}
}

// TestRender_MinimalSchema pins behavior on a schema with no flags, no
// examples, and no constraints: no badge line, no deprecation notice, no
// examples/constraints sections should be emitted.
func TestRender_MinimalSchema(t *testing.T) {
	doc, err := loader.FromFile("../testdata/minimal.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	for _, flavor := range []Flavor{Plain, Material} {
		t.Run(string(flavor), func(t *testing.T) {
			pages, err := Render(doc, Options{Flavor: flavor, Pages: SinglePage})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			content := pages[0].Content
			for _, unwanted := range []string{"Deprecated", "Sensitive", "Read-only", "Write-once", "Example", "Constraints"} {
				if strings.Contains(content, unwanted) {
					t.Errorf("unexpected %q in minimal output:\n%s", unwanted, content)
				}
			}
		})
	}
}

// TestRender_EdgeCases exercises branches the full/minimal fixtures don't
// reach: a bare version (no versionDescription), a contact with only a URL
// or only a name, an externalDocs block with no description, a non-nil but
// empty constraints block, a fractional constraint value, and a multi-page
// group with exactly one field (singular "field" in the index).
func TestRender_EdgeCases(t *testing.T) {
	half := 0.5
	doc := docmodel.New(docmodel.Schema{
		Name:    "edge",
		Version: 2,
		Info: &docmodel.Info{
			Contact: &docmodel.Contact{Name: "Sam", URL: "https://example.com/sam"},
		},
		Fields: []docmodel.Field{
			{
				Path:         "alpha.value",
				Type:         "string",
				ExternalDocs: &docmodel.ExternalDocs{URL: "https://example.com/alpha"},
				Constraints:  &docmodel.Constraints{},
			},
			{
				Path:        "beta.value",
				Type:        "number",
				Constraints: &docmodel.Constraints{Minimum: &half},
			},
		},
	})

	pages, err := Render(doc, Options{Flavor: Plain, Pages: MultiPage})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	index := pages[0]
	if !strings.Contains(index.Content, "**Version:** 2\n") {
		t.Errorf("expected bare version line, got:\n%s", index.Content)
	}
	if !strings.Contains(index.Content, "**Contact:** [Sam](https://example.com/sam)") {
		t.Errorf("expected URL-only contact line, got:\n%s", index.Content)
	}
	if !strings.Contains(index.Content, "- [alpha](alpha.md) — 1 field\n") {
		t.Errorf("expected singular 'field' for a one-field group, got:\n%s", index.Content)
	}

	var alpha, beta string
	for _, p := range pages {
		switch p.Name {
		case "alpha":
			alpha = p.Content
		case "beta":
			beta = p.Content
		}
	}
	if !strings.Contains(alpha, "**See also:** https://example.com/alpha\n") {
		t.Errorf("expected description-less externalDocs line, got:\n%s", alpha)
	}
	if strings.Contains(alpha, "Constraints") {
		t.Errorf("expected empty constraints block to render nothing, got:\n%s", alpha)
	}
	if !strings.Contains(beta, "Minimum: 0.5\n") {
		t.Errorf("expected fractional minimum, got:\n%s", beta)
	}
}

func TestRender_NameOnlyContact(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name: "edge",
		Info: &docmodel.Info{Contact: &docmodel.Contact{Name: "Sam"}},
		Fields: []docmodel.Field{
			{Path: "alpha.value", Type: "string"},
		},
	})
	pages, err := Render(doc, Options{Flavor: Plain, Pages: SinglePage})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(pages[0].Content, "**Contact:** Sam\n") {
		t.Errorf("expected name-only contact line, got:\n%s", pages[0].Content)
	}
}

func TestRender_UnknownFlavor(t *testing.T) {
	doc, err := loader.FromFile("../testdata/minimal.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	if _, err := Render(doc, Options{Flavor: "fancy", Pages: SinglePage}); err == nil {
		t.Error("expected error for unknown flavor, got nil")
	}
}

func TestRender_UnknownPageMode(t *testing.T) {
	doc, err := loader.FromFile("../testdata/minimal.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	if _, err := Render(doc, Options{Flavor: Plain, Pages: "weekly"}); err == nil {
		t.Error("expected error for unknown page mode, got nil")
	}
}

func TestRender_MultiPage_GroupsByPrefix(t *testing.T) {
	doc, err := loader.FromFile("../testdata/full.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	pages, err := Render(doc, Options{Flavor: Plain, Pages: MultiPage})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	names := make([]string, 0, len(pages))
	for _, p := range pages {
		names = append(names, p.Name)
	}
	want := []string{"index", "payments"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("got pages %v, want %v", names, want)
	}
}

func TestRender_MultiPage_IndexLinksToGroupPages(t *testing.T) {
	doc, err := loader.FromFile("../testdata/full.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	pages, err := Render(doc, Options{Flavor: Plain, Pages: MultiPage})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	index := pages[0]
	if index.Name != "index" {
		t.Fatalf("got first page %q, want index", index.Name)
	}
	if !strings.Contains(index.Content, "(payments.md)") {
		t.Errorf("expected index to link to payments.md, got:\n%s", index.Content)
	}
}
