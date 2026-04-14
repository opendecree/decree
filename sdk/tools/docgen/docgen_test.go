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
	minLen := int32(1)
	schema := Schema{
		Name: "test",
		Fields: []Field{
			{Path: "rate", Type: "number", Constraints: &Constraints{Min: &min, Max: &max}},
			{Path: "name", Type: "string", Constraints: &Constraints{MinLength: &minLen, Pattern: "^[a-z]+$", Enum: []string{"a", "b"}}},
		},
	}

	md := Generate(schema)
	assertContains(t, md, "Minimum: 0")
	assertContains(t, md, "Maximum: 100")
	assertContains(t, md, "Min length: 1")
	assertContains(t, md, "Pattern: `^[a-z]+$`")
	assertContains(t, md, "Enum: a, b")
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
	assertContains(t, md, "Deprecated | yes")
	assertContains(t, md, "Redirect | `new_field`")
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
	assertContains(t, md, "Nullable | yes")
	assertContains(t, md, "Nullable | no")
}

func TestGenerate_Default(t *testing.T) {
	schema := Schema{
		Name:   "test",
		Fields: []Field{{Path: "x", Type: "string", Default: "hello"}},
	}

	md := Generate(schema)
	assertContains(t, md, "Default | `hello`")
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

func TestFieldTypeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"FIELD_TYPE_STRING", "string"},
		{"FIELD_TYPE_INT", "integer"},
		{"FIELD_TYPE_NUMBER", "number"},
		{"FIELD_TYPE_BOOL", "bool"},
		{"FIELD_TYPE_TIME", "time"},
		{"FIELD_TYPE_DURATION", "duration"},
		{"FIELD_TYPE_URL", "url"},
		{"FIELD_TYPE_JSON", "json"},
		{"UNKNOWN", "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := FieldTypeName(tt.input); got != tt.want {
			t.Errorf("FieldTypeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
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
	assertContains(t, md, "**Examples:**")
	assertContains(t, md, "**low:** `0.01` — Low rate")
	assertContains(t, md, "**high:** `0.99`")
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
	assertContains(t, md, "| Tags | billing, critical |")
}

func TestGenerate_Format(t *testing.T) {
	schema := Schema{
		Name:   "test",
		Fields: []Field{{Path: "x", Type: "string", Format: "email"}},
	}

	md := Generate(schema)
	assertContains(t, md, "| Format | email |")
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
	assertContains(t, md, "| Read-only | yes |")
	assertContains(t, md, "| Write-once | yes |")
	assertContains(t, md, "| Sensitive | yes |")
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, s)
	}
}
