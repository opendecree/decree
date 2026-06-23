package mdx

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

// TestRender_Golden pins the rendered output for the full fixture, which
// exercises every documented schema and field property, across every page
// the renderer emits: the index plus one category folder per top-level
// field-path-prefix group.
func TestRender_Golden(t *testing.T) {
	doc, err := loader.FromFile("../testdata/full.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	golden := map[string]string{
		"index.mdx":                "testdata/full-index.golden.mdx",
		"payments/_category_.json": "testdata/full-payments-category.golden.json",
		"payments/index.mdx":       "testdata/full-payments-index.golden.mdx",
	}
	if len(pages) != len(golden) {
		t.Fatalf("got %d pages, want %d", len(pages), len(golden))
	}
	for _, p := range pages {
		path, ok := golden[p.Path]
		if !ok {
			t.Fatalf("unexpected page %q", p.Path)
		}
		if *update {
			if err := os.WriteFile(path, []byte(p.Content), 0o644); err != nil {
				t.Fatalf("update golden: %v", err)
			}
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read golden: %v", err)
		}
		if p.Content != string(want) {
			t.Errorf("page %q does not match %s:\ngot:\n%s\nwant:\n%s", p.Path, path, p.Content, want)
		}
	}
}

// TestRender_MinimalSchema pins behavior on a schema with no flags, no
// examples, and no constraints: no badges, no deprecation admonition, no
// examples/constraints sections should be emitted.
func TestRender_MinimalSchema(t *testing.T) {
	doc, err := loader.FromFile("../testdata/minimal.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var content string
	for _, p := range pages {
		content += p.Content
	}
	for _, unwanted := range []string{"Deprecated", "Sensitive", "Read-only", "Write-once", "Nullable", "Examples", "Constraints", ":::caution"} {
		if strings.Contains(content, unwanted) {
			t.Errorf("unexpected %q in minimal output:\n%s", unwanted, content)
		}
	}
}

// TestRender_CategoryFiles ensures every top-level field-path-prefix group
// gets a _category_.json with a label and a 1-based sidebar position, and
// that the index page is not itself treated as a group.
func TestRender_CategoryFiles(t *testing.T) {
	doc, err := loader.FromFile("../testdata/full.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var found bool
	for _, p := range pages {
		if p.Path == "payments/_category_.json" {
			found = true
			if !strings.Contains(p.Content, `"label": "payments"`) {
				t.Errorf("expected label in category file, got:\n%s", p.Content)
			}
			if !strings.Contains(p.Content, `"position": 1`) {
				t.Errorf("expected position 1 for the only group, got:\n%s", p.Content)
			}
		}
	}
	if !found {
		t.Fatal("expected a payments/_category_.json page")
	}
}

// TestRender_EdgeCases exercises branches the full/minimal fixtures don't
// reach: a bare version (no versionDescription), a contact with only a URL
// or only a name, an externalDocs block with no description, a non-nil but
// empty constraints block, a fractional constraint value, a multi-group
// schema, and a single-field group (singular "field" in the index).
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

	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var index, alpha, beta string
	for _, p := range pages {
		switch p.Path {
		case "index.mdx":
			index = p.Content
		case "alpha/index.mdx":
			alpha = p.Content
		case "beta/index.mdx":
			beta = p.Content
		}
	}

	if !strings.Contains(index, "**Version:** 2\n") {
		t.Errorf("expected bare version line, got:\n%s", index)
	}
	if !strings.Contains(index, "**Contact:** [Sam](https://example.com/sam)") {
		t.Errorf("expected URL-only contact line, got:\n%s", index)
	}
	if !strings.Contains(index, "- [alpha](./alpha/index) — 1 field\n") {
		t.Errorf("expected singular 'field' for a one-field group, got:\n%s", index)
	}
	if !strings.Contains(index, "- [beta](./beta/index) — 1 field\n") {
		t.Errorf("expected beta group link, got:\n%s", index)
	}

	if alpha == "" {
		t.Fatal("expected an alpha/index.mdx page")
	}
	if !strings.Contains(alpha, "**See also:** https://example.com/alpha\n") {
		t.Errorf("expected description-less externalDocs line, got:\n%s", alpha)
	}
	if strings.Contains(alpha, "Constraints") {
		t.Errorf("expected empty constraints block to render nothing, got:\n%s", alpha)
	}

	if beta == "" {
		t.Fatal("expected a beta/index.mdx page")
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
	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(pages[0].Content, "**Contact:** Sam\n") {
		t.Errorf("expected name-only contact line, got:\n%s", pages[0].Content)
	}
}

func TestRender_EmailContact(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name: "edge",
		Info: &docmodel.Info{Contact: &docmodel.Contact{Name: "Sam", Email: "sam@example.com"}},
		Fields: []docmodel.Field{
			{Path: "alpha.value", Type: "string"},
		},
	})
	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(pages[0].Content, "**Contact:** [Sam](mailto:sam@example.com)\n") {
		t.Errorf("expected email contact line, got:\n%s", pages[0].Content)
	}
}

func TestRender_InfoLabels(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name: "edge",
		Info: &docmodel.Info{
			Author: "platform-team",
			Labels: map[string]string{"tier": "critical", "team": "platform"},
		},
		Fields: []docmodel.Field{
			{Path: "alpha.value", Type: "string"},
		},
	})
	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	content := pages[0].Content
	if !strings.Contains(content, "**Author:** platform-team\n") {
		t.Errorf("expected author line, got:\n%s", content)
	}
	if !strings.Contains(content, "**Labels:** `team: platform`, `tier: critical`\n") {
		t.Errorf("expected sorted label chips, got:\n%s", content)
	}
}

// TestRender_DeprecatedWithoutRedirect ensures the deprecation admonition
// falls back to generic text when RedirectTo is empty.
func TestRender_DeprecatedWithoutRedirect(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name: "edge",
		Fields: []docmodel.Field{
			{Path: "alpha.value", Type: "string", Deprecated: true},
		},
	})
	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var all string
	for _, p := range pages {
		all += p.Content
	}
	if !strings.Contains(all, ":::caution[Deprecated]\nThis field should no longer be used.\n:::\n") {
		t.Errorf("expected generic deprecation admonition, got:\n%s", all)
	}
}

// TestRender_MultipleFieldsInGroup ensures fields within the same group are
// separated by a horizontal rule.
func TestRender_MultipleFieldsInGroup(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name: "edge",
		Fields: []docmodel.Field{
			{Path: "alpha.one", Type: "string"},
			{Path: "alpha.two", Type: "string"},
		},
	})
	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var alpha string
	for _, p := range pages {
		if p.Path == "alpha/index.mdx" {
			alpha = p.Content
		}
	}
	if !strings.Contains(alpha, "---\n") {
		t.Errorf("expected a horizontal rule between fields, got:\n%s", alpha)
	}
}

// TestRender_Validations_Golden pins rendering of schema-level CEL
// validation rules (added in #960): one error-severity and one
// warning-severity rule, to exercise the severity distinction.
func TestRender_Validations_Golden(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name: "payments",
		Fields: []docmodel.Field{
			{Path: "payments.fee", Type: "number"},
			{Path: "payments.retries", Type: "integer"},
		},
		Validations: []docmodel.Validation{
			{
				Rule:     "self.payments.fee < self.payments.retries",
				Message:  "Fee rate must be less than the retry count.",
				Severity: "error",
			},
			{
				Rule:     `self.payments.webhook.startsWith("https://")`,
				Message:  "Webhook URLs should use https.",
				Severity: "warning",
			},
		},
	})

	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var index string
	for _, p := range pages {
		if p.Path == "index.mdx" {
			index = p.Content
		}
	}
	if index == "" {
		t.Fatal("expected an index.mdx page")
	}

	const golden = "testdata/validations-index.golden.mdx"
	if *update {
		if err := os.WriteFile(golden, []byte(index), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if index != string(want) {
		t.Errorf("index page does not match %s:\ngot:\n%s\nwant:\n%s", golden, index, want)
	}
}

// TestRender_NoValidations ensures schemas with no validations render no
// "## Validations" section.
func TestRender_NoValidations(t *testing.T) {
	doc, err := loader.FromFile("../testdata/minimal.schema.yaml")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, p := range pages {
		if strings.Contains(p.Content, "Validations") {
			t.Errorf("unexpected Validations section in page %q:\n%s", p.Path, p.Content)
		}
	}
}

// TestRender_Validations_OnIndexNotGroup ensures validations render on the
// top-level index page, not on any group page, since a rule can reference
// fields in more than one group.
func TestRender_Validations_OnIndexNotGroup(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name: "payments",
		Fields: []docmodel.Field{
			{Path: "alpha.value", Type: "string"},
			{Path: "beta.value", Type: "string"},
		},
		Validations: []docmodel.Validation{
			{Rule: "self.alpha.value != self.beta.value", Message: "alpha and beta must differ.", Severity: "error"},
		},
	})

	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var index, alpha string
	for _, p := range pages {
		switch p.Path {
		case "index.mdx":
			index = p.Content
		case "alpha/index.mdx":
			alpha = p.Content
		}
	}
	if !strings.Contains(index, "## Validations") {
		t.Errorf("expected Validations section on index page, got:\n%s", index)
	}
	if strings.Contains(alpha, "Validations") {
		t.Errorf("unexpected Validations section on group page, got:\n%s", alpha)
	}
}

// TestRender_Validations_DefaultSeverity ensures an unset/unrecognized
// Severity falls back to the warning admonition, mirroring how
// writeDeprecationNotice falls back to generic text when optional data is
// missing.
func TestRender_Validations_DefaultSeverity(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name: "edge",
		Fields: []docmodel.Field{
			{Path: "alpha.value", Type: "string"},
		},
		Validations: []docmodel.Validation{
			{Rule: "self.alpha.value != \"\"", Message: "alpha.value must not be empty."},
		},
	})
	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(pages[0].Content, ":::caution[Warning]\n") {
		t.Errorf("expected default-severity validation to render as a warning admonition, got:\n%s", pages[0].Content)
	}
}

// --- Escaping edge cases ---
//
// MDX v3 parses '{' and '<' as JSX, so these tests pin escapeText and
// codeSpan's behavior directly, at the unit level, rather than only through
// golden-file output — that way the exact character-by-character escaping
// is locked down independent of surrounding prose.

func TestEscapeText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain text", "hello world", "hello world"},
		{"literal brace", "a {b} c", `a \{b} c`},
		{"literal angle bracket", "a < b > c", `a \< b > c`},
		{"backtick", "use `code`", "use \\`code\\`"},
		{"backslash", `a\b`, `a\\b`},
		{"html comment", "a <!-- comment --> b", `a \<!-- comment --> b`},
		{"jsx-like tag", "<Foo bar={baz} />", `\<Foo bar=\{baz} />`},
		{"multiple braces", "{{nested}}", `\{\{nested}}`},
		{"mixed all specials", "<{`" + `\` + "}>", "\\<\\{\\`\\\\}>"},
		{"angle then bang dash dash", "<!--", `\<!--`},
		{"only backslash", `\`, `\\`},
		{"unicode passthrough", "café résumé", "café résumé"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeText(tt.in)
			if got != tt.want {
				t.Errorf("escapeText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCodeSpan(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "`  `"},
		{"plain", "hello", "`hello`"},
		{"contains jsx-like text", "<Foo/>", "`<Foo/>`"},
		{"contains braces", "{value}", "`{value}`"},
		{"single backtick", "a`b", "``a`b``"},
		{"leading backtick", "`abc", "`` `abc ``"},
		{"trailing backtick", "abc`", "`` abc` ``"},
		{"double backtick run", "a``b", "```a``b```"},
		{"only backticks", "```", "```` ``` ````"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := codeSpan(tt.in)
			if got != tt.want {
				t.Errorf("codeSpan(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestJSONString pins jsonString's JSON-literal escaping: double quotes and
// backslashes are backslash-escaped, embedded newlines become the two-char
// "\n" escape (category labels come from field path prefixes, but the
// function itself makes no such assumption), and plain text passes through.
func TestJSONString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "payments", `"payments"`},
		{"double quote", `say "hi"`, `"say \"hi\""`},
		{"backslash", `a\b`, `"a\\b"`},
		{"newline", "a\nb", `"a\nb"`},
		{"empty", "", `""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonString(tt.in)
			if got != tt.want {
				t.Errorf("jsonString(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestRender_EscapesSchemaText ensures schema-sourced text that looks like
// JSX/markup survives a full Render call escaped, not interpreted, since
// descriptions/titles/examples are untrusted relative to the renderer. This
// is the integration-level companion to TestEscapeText/TestCodeSpan.
func TestRender_EscapesSchemaText(t *testing.T) {
	doc := docmodel.New(docmodel.Schema{
		Name:        "edge",
		Description: "Uses {braces} and <tags> and `backticks` and <!-- comments -->.",
		Fields: []docmodel.Field{
			{
				Path:        "alpha.value",
				Type:        "string",
				Title:       "<Title/>",
				Description: "A {curly} <b>bold</b> description with a backtick ` and a <!-- sneaky comment -->.",
				Default:     "<{default}>",
				Example:     "<{example}>",
				Examples: map[string]docmodel.Example{
					"sample": {Value: "<{ex}>", Summary: "a <summary> with {braces}"},
				},
				Constraints: &docmodel.Constraints{
					Pattern: "^<{[a-z]+}>$",
					Enum:    []string{"<a>", "{b}"},
				},
			},
		},
	})
	pages, err := Render(doc)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var all string
	for _, p := range pages {
		all += p.Content
	}

	// No *unescaped* JSX-triggering construct should survive: every literal
	// '<' or '{' that came from prose (escapeText) must be preceded by a
	// backslash. Check for the unescaped form specifically — "\<tags>"
	// legitimately contains "<tags>" as a substring, so the assertion must
	// anchor on the character immediately before '<' or '{' not being '\'.
	forbidden := []string{
		"<tags>",
		"<b>bold</b>",
		"<!-- sneaky comment -->",
		"<!-- comments -->",
	}
	for _, f := range forbidden {
		if idx := strings.Index(all, f); idx >= 0 && (idx == 0 || all[idx-1] != '\\') {
			t.Errorf("unescaped %q leaked into output:\n%s", f, all)
		}
	}
	// escapeText'd description text: only '\', '<', '{', and '`' are
	// backslashed; '}' and '>' are left as-is (escaping '<' is sufficient
	// to defang "<!--", since a comment can't open without an unescaped
	// '<', and a bare '}'/'>' with no preceding unescaped opener is inert).
	wantDescription := "Uses \\{braces} and \\<tags> and \\`backticks\\` and \\<!-- comments -->."
	if !strings.Contains(all, wantDescription) {
		t.Errorf("expected escaped schema-level description, got:\n%s", all)
	}
	// codeSpan'd values (default/example/enum) keep their raw braces/angle
	// brackets literally, but fenced in backticks so MDX reads them as text.
	if !strings.Contains(all, "default `<{default}>`") {
		t.Errorf("expected code-spanned default, got:\n%s", all)
	}
}
