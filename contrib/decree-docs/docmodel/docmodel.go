// Package docmodel defines the documentation model decree-docs builds from a
// decree schema, and its canonical JSON serialization.
//
// The JSON document written by [Document.EncodeJSON] is the contract for
// third-party renderers (the renderer goal stated in issue #117): the Go API
// is secondary and may evolve, but the JSON shape only changes together with
// the docModelVersion marker. Serialization rules:
//
//   - All keys are lowerCamel (OpenAPI-style: externalDocs, readOnly,
//     exclusiveMinimum, ...).
//   - Optional values are omitted when empty; absent and empty mean the same
//     thing.
//   - Fields are sorted by path, so output is deterministic for a given
//     schema regardless of source order.
//   - Output is indented with two spaces, HTML characters are not escaped
//     (schema text frequently contains URLs), and a trailing newline is
//     appended.
//
// The model is a complete superset of the minimal sdk/tools/docgen schema:
// every documented schema and field property is carried, including info,
// named examples, externalDocs, versionDescription, and allowedSchemes.
// Server-side bookkeeping (schema ID, parent version, checksum, published
// state, timestamps) is deliberately excluded so that a schema loaded from a
// YAML file and the same schema fetched from a server produce identical
// documents.
package docmodel

import (
	"encoding/json"
	"io"
)

// Version is the current doc model version, emitted as docModelVersion in
// the JSON root. It is incremented whenever the JSON shape changes
// incompatibly, so third-party renderers can detect breaking changes.
const Version = 2

// Document is the root of the doc model.
type Document struct {
	// DocModelVersion identifies the JSON shape; see [Version].
	DocModelVersion int `json:"docModelVersion"`
	// Schema is the documented schema.
	Schema Schema `json:"schema"`
}

// New wraps a schema in a Document stamped with the current model version.
func New(s Schema) *Document {
	return &Document{DocModelVersion: Version, Schema: s}
}

// EncodeJSON writes the document to w in its canonical JSON form (see the
// package documentation for the serialization rules).
func (d *Document) EncodeJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(d)
}

// Schema describes one decree schema.
type Schema struct {
	// Name is the schema slug.
	Name string `json:"name"`
	// Description is the schema-level description.
	Description string `json:"description,omitempty"`
	// Version is the schema version number; 0 means the version is not set.
	Version int32 `json:"version,omitempty"`
	// VersionDescription describes what changed in this version.
	VersionDescription string `json:"versionDescription,omitempty"`
	// Info is optional schema-level metadata.
	Info *Info `json:"info,omitempty"`
	// Fields lists the schema's fields, sorted by path.
	Fields []Field `json:"fields"`
	// Validations lists the schema's cross-field CEL validation rules.
	Validations []Validation `json:"validations,omitempty"`
}

// Validation describes a single cross-field CEL validation rule.
type Validation struct {
	// Rule is the CEL expression evaluated against the configuration.
	Rule string `json:"rule"`
	// Message explains the rule to a human when it fails.
	Message string `json:"message"`
	// Severity is "error" or "warning".
	Severity string `json:"severity,omitempty"`
}

// Info contains optional schema-level metadata.
type Info struct {
	Title   string            `json:"title,omitempty"`
	Author  string            `json:"author,omitempty"`
	Contact *Contact          `json:"contact,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
}

// Contact contains contact information for a schema owner.
type Contact struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// Field describes a single schema field.
type Field struct {
	// Path is the dot-separated field path (e.g. "payments.fee").
	Path string `json:"path"`
	// Type is the field's data type: "integer", "number", "string", "bool",
	// "time", "duration", "url", or "json".
	Type string `json:"type"`
	// Title is a human-readable display name.
	Title string `json:"title,omitempty"`
	// Description explains what the field controls.
	Description string `json:"description,omitempty"`
	// Default is the default value, rendered as a string.
	Default string `json:"default,omitempty"`
	// Nullable marks fields that accept null.
	Nullable bool `json:"nullable,omitempty"`
	// Deprecated marks fields that should no longer be used.
	Deprecated bool `json:"deprecated,omitempty"`
	// RedirectTo names the replacement path for a deprecated field.
	RedirectTo string `json:"redirectTo,omitempty"`
	// Example is a single inline example value.
	Example string `json:"example,omitempty"`
	// Examples holds named example values.
	Examples map[string]Example `json:"examples,omitempty"`
	// ExternalDocs links to external documentation.
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty"`
	// Tags categorize the field for grouping and filtering.
	Tags []string `json:"tags,omitempty"`
	// Format is an OpenAPI-style format hint (e.g. "uri", "email").
	Format string `json:"format,omitempty"`
	// ReadOnly marks fields that clients cannot write.
	ReadOnly bool `json:"readOnly,omitempty"`
	// WriteOnce marks fields that can only be set once.
	WriteOnce bool `json:"writeOnce,omitempty"`
	// Sensitive marks values that must not be displayed in clear text.
	Sensitive bool `json:"sensitive,omitempty"`
	// Constraints holds the field's validation rules.
	Constraints *Constraints `json:"constraints,omitempty"`
}

// Example is a named example value.
type Example struct {
	Value   string `json:"value"`
	Summary string `json:"summary,omitempty"`
}

// ExternalDocs links to external documentation.
type ExternalDocs struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// Constraints defines validation rules for a field, named OAS-style like the
// schema YAML format.
type Constraints struct {
	Minimum          *float64 `json:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	MinLength        *int32   `json:"minLength,omitempty"`
	MaxLength        *int32   `json:"maxLength,omitempty"`
	Pattern          string   `json:"pattern,omitempty"`
	Enum             []string `json:"enum,omitempty"`
	JSONSchema       string   `json:"jsonSchema,omitempty"`
	// AllowedSchemes lists the URI schemes accepted for url-typed fields.
	// When empty the server defaults to http and https.
	AllowedSchemes []string `json:"allowedSchemes,omitempty"`
}
