package docgen

import (
	"strings"
	"testing"
)

func TestGenerate_Basic(t *testing.T) {
	schema := Schema{
		Name:        "payment-config",
		Description: "Payment settings",
		Version:     3,
		Fields: []Field{
			{Path: "payments.fee", Type: "string", Description: "Fee percentage"},
			{Path: "payments.currency", Type: "string", Description: "Currency code"},
		},
	}

	md := Generate(schema)

	assertContains(t, md, "# payment-config")
	assertContains(t, md, "Payment settings")
	assertContains(t, md, "**Version:** 3")
	assertContains(t, md, "## payments")
	assertContains(t, md, "### `payments.fee`")
	assertContains(t, md, "### `payments.currency`")
	assertContains(t, md, "Fee percentage")
}

func TestGenerate_VersionDescription(t *testing.T) {
	schema := Schema{
		Name:               "test",
		Version:            3,
		VersionDescription: "Added refund_window field",
		Fields:             []Field{{Path: "x", Type: "string"}},
	}

	md := Generate(schema)
	assertContains(t, md, "**Version:** 3 — Added refund_window field")
}

func TestGenerate_VersionDescriptionWithoutVersion(t *testing.T) {
	schema := Schema{
		Name:               "test",
		VersionDescription: "Added refund_window field",
		Fields:             []Field{{Path: "x", Type: "string"}},
	}

	md := Generate(schema)
	if strings.Contains(md, "Version") || strings.Contains(md, "Added refund_window field") {
		t.Errorf("expected no version line when Version is unset, got:\n%s", md)
	}
}

func TestGenerate_GroupsByPrefix(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "db.host", Type: "string"},
			{Path: "db.port", Type: "integer"},
			{Path: "cache.ttl", Type: "duration"},
		},
	}

	md := Generate(schema)
	assertContains(t, md, "## db")
	assertContains(t, md, "## cache")
}

func TestGenerate_WithoutGrouping(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "db.host", Type: "string"},
			{Path: "db.port", Type: "integer"},
		},
	}

	md := Generate(schema, WithoutGrouping())
	if strings.Contains(md, "## db") {
		t.Error("expected no grouping headers")
	}
	assertContains(t, md, "### `db.host`")
}

func TestGenerate_Constraints(t *testing.T) {
	min := float64(0)
	max := float64(100)
	exclMin := float64(0)
	exclMax := float64(1)
	minLen := int32(1)
	maxLen := int32(50)
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "rate", Type: "number", Constraints: &Constraints{Min: &min, Max: &max}},
			{Path: "share", Type: "number", Constraints: &Constraints{ExclusiveMin: &exclMin, ExclusiveMax: &exclMax}},
			{Path: "name", Type: "string", Constraints: &Constraints{MinLength: &minLen, MaxLength: &maxLen, Pattern: "^[a-z]+$", Enum: []string{"a", "b"}}},
			{Path: "payload", Type: "json", Constraints: &Constraints{JSONSchema: `{"type": "object"}`}},
		},
	}

	md := Generate(schema)
	assertContains(t, md, "Minimum: 0")
	assertContains(t, md, "Maximum: 100")
	assertContains(t, md, "Exclusive minimum: 0")
	assertContains(t, md, "Exclusive maximum: 1")
	assertContains(t, md, "Min length: 1")
	assertContains(t, md, "Max length: 50")
	assertContains(t, md, "Pattern: `^[a-z]+$`")
	assertContains(t, md, "Enum: a, b")
	assertContains(t, md, "JSON Schema: (see schema definition)")
}

func TestGenerate_AllowedSchemes(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "hook", Type: "url", Constraints: &Constraints{AllowedSchemes: []string{"https", "sftp"}}},
		},
	}

	md := Generate(schema)
	assertContains(t, md, "**Constraints:**")
	assertContains(t, md, "- Allowed schemes: https, sftp")
}

func TestGenerate_WithoutConstraints(t *testing.T) {
	min := float64(0)
	schema := Schema{
		Name:   "test",
		Fields: []Field{{Path: "x", Type: "integer", Constraints: &Constraints{Min: &min}}},
	}

	md := Generate(schema, WithoutConstraints())
	if strings.Contains(md, "Minimum") {
		t.Error("expected no constraints in output")
	}
}

func TestGenerate_Deprecated(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "old_field", Type: "string", Deprecated: true, RedirectTo: "new_field"},
			{Path: "new_field", Type: "string"},
		},
	}

	md := Generate(schema)
	assertContains(t, md, "> **Deprecated** — use `new_field` instead.")
}

func TestGenerate_DeprecatedNoRedirect(t *testing.T) {
	schema := Schema{
		Name:   "test",
		Fields: []Field{{Path: "old_field", Type: "string", Deprecated: true}},
	}

	md := Generate(schema)
	assertContains(t, md, "> **Deprecated.**")
}

func TestGenerate_WithoutDeprecated(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "old_field", Type: "string", Deprecated: true},
			{Path: "new_field", Type: "string"},
		},
	}

	md := Generate(schema, WithoutDeprecated())
	if strings.Contains(md, "old_field") {
		t.Error("expected deprecated field to be excluded")
	}
	assertContains(t, md, "new_field")
}

func TestGenerate_Nullable(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "a", Type: "string", Nullable: true},
			{Path: "b", Type: "string", Nullable: false},
		},
	}

	md := Generate(schema)
	assertContains(t, md, "### `a`\n\n*type: `string` · nullable*")
	assertContains(t, md, "### `b`\n\n*type: `string`*")
}

func TestGenerate_Default(t *testing.T) {
	schema := Schema{
		Name:   "test",
		Fields: []Field{{Path: "x", Type: "string", Default: "hello"}},
	}

	md := Generate(schema)
	assertContains(t, md, "default: `hello`")
}

func TestGenerate_NoPrefix(t *testing.T) {
	schema := Schema{
		Name:   "test",
		Fields: []Field{{Path: "standalone", Type: "string"}},
	}

	md := Generate(schema)
	assertContains(t, md, "## standalone")
	assertContains(t, md, "### `standalone`")
}

func TestGenerate_SchemaInfo(t *testing.T) {
	schema := Schema{
		Name: "test",
		Info: &SchemaInfo{
			Title:  "Test Configuration",
			Author: "platform-team",
			Contact: &SchemaContact{
				Name:  "Platform Team",
				Email: "platform@example.com",
			},
			Labels: map[string]string{"team": "platform", "env": "prod"},
		},
		Fields: []Field{{Path: "x", Type: "string"}},
	}

	md := Generate(schema)
	assertContains(t, md, "# Test Configuration")
	assertContains(t, md, "**Author:** platform-team")
	assertContains(t, md, "**Contact:** Platform Team <platform@example.com>")
	assertContains(t, md, "`env: prod`")
	assertContains(t, md, "`team: platform`")
}

func TestGenerate_SchemaInfoContactURL(t *testing.T) {
	schema := Schema{
		Name: "test",
		Info: &SchemaInfo{
			Contact: &SchemaContact{Name: "Wiki", URL: "https://wiki.example.com"},
		},
		Fields: []Field{{Path: "x", Type: "string"}},
	}

	md := Generate(schema)
	assertContains(t, md, "[Wiki](https://wiki.example.com)")
}

func TestGenerate_SchemaInfoContactNameOnly(t *testing.T) {
	schema := Schema{
		Name: "test",
		Info: &SchemaInfo{
			Contact: &SchemaContact{Name: "Alice"},
		},
		Fields: []Field{{Path: "x", Type: "string"}},
	}

	md := Generate(schema)
	assertContains(t, md, "**Contact:** Alice")
}

func TestGenerate_Title(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "payments.fee", Type: "number", Title: "Fee Rate"},
		},
	}

	md := Generate(schema)
	assertContains(t, md, "### Fee Rate (`payments.fee`)")
}

func TestGenerate_Example(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "x", Type: "string", Example: "hello"},
		},
	}

	md := Generate(schema)
	assertContains(t, md, "**Example:** `hello`")
}

func TestGenerate_Examples(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "x", Type: "number", Examples: map[string]FieldExample{
				"low":  {Value: "0.01", Summary: "Low rate"},
				"high": {Value: "0.99"},
			}},
		},
	}

	md := Generate(schema)
	// Examples render sorted by name for deterministic output.
	assertContains(t, md, "**Examples:**\n- **high:** `0.99`\n- **low:** `0.01` — Low rate\n")
}

func TestGenerate_ExternalDocs(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "x", Type: "string", ExternalDocs: &ExternalDocs{
				Description: "Full guide",
				URL:         "https://docs.example.com",
			}},
		},
	}

	md := Generate(schema)
	assertContains(t, md, "[Full guide](https://docs.example.com)")
}

func TestGenerate_ExternalDocsURLOnly(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "x", Type: "string", ExternalDocs: &ExternalDocs{URL: "https://docs.example.com"}},
		},
	}

	md := Generate(schema)
	assertContains(t, md, "**See also:** https://docs.example.com")
}

func TestGenerate_Tags(t *testing.T) {
	schema := Schema{
		Name:   "test",
		Fields: []Field{{Path: "x", Type: "string", Tags: []string{"billing", "critical"}}},
	}

	md := Generate(schema)
	assertContains(t, md, "tags: billing, critical")
}

func TestGenerate_Format(t *testing.T) {
	schema := Schema{
		Name:   "test",
		Fields: []Field{{Path: "x", Type: "string", Format: "email"}},
	}

	md := Generate(schema)
	assertContains(t, md, "format: email")
}

func TestGenerate_ReadOnlyWriteOnceSensitive(t *testing.T) {
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "a", Type: "string", ReadOnly: true},
			{Path: "b", Type: "string", WriteOnce: true},
			{Path: "c", Type: "string", Sensitive: true},
		},
	}

	md := Generate(schema)
	assertContains(t, md, "*type: `string` · read-only*")
	assertContains(t, md, "*type: `string` · write-once*")
	assertContains(t, md, "*type: `string` · sensitive*")
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, s)
	}
}
