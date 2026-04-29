package schema

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"gopkg.in/yaml.v3"
)

const (
	yamlSpecVersionV1 = "v1"
	// metaSchemaURL is the canonical URL of the meta-schema that validates
	// decree.schema.yaml documents at this spec version. Emitted on export.
	metaSchemaURL = "https://schemas.opendecree.dev/schema/v0.1.0/decree.json"
)

// schemaURNPattern matches decree schema URNs: urn:decree:schema:<segment>(:<segment>)*
// where each segment is [a-zA-Z0-9][a-zA-Z0-9._-]*
var schemaURNPattern = regexp.MustCompile(`^urn:decree:schema:[a-zA-Z0-9][a-zA-Z0-9._-]*(?::[a-zA-Z0-9][a-zA-Z0-9._-]*)*$`)

// fieldPathPattern is the grammar for map keys under `fields:`. Must start with
// an ASCII letter or underscore; subsequent characters may be letters, digits,
// underscore, dot, or hyphen. Enforced at parse time to catch pathological
// keys (empty, leading digit, whitespace, special chars) early.
var fieldPathPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.-]*$`)

// extensionKeyPattern is the grammar for OAS-style vendor extension keys.
// Any YAML key not matching a known struct field must match this pattern,
// otherwise the parser rejects the document as containing an unknown key.
var extensionKeyPattern = regexp.MustCompile(`^x-.+$`)

// SchemaYAML is the top-level YAML document for schema import/export.
type SchemaYAML struct {
	SpecVersion        string                     `yaml:"spec_version"`
	Schema             string                     `yaml:"$schema,omitempty"`
	ID                 string                     `yaml:"$id,omitempty"`
	Name               string                     `yaml:"name"`
	Description        string                     `yaml:"description,omitempty"`
	Version            int32                      `yaml:"version,omitempty"`
	VersionDescription string                     `yaml:"version_description,omitempty"`
	Info               *SchemaInfoYAML            `yaml:"info,omitempty"`
	Fields             map[string]SchemaFieldYAML `yaml:"fields"`
	// DependentRequired declares cross-field "B required when A present"
	// rules. Keys and values must reference paths defined in Fields. Matches
	// the JSON Schema 2020-12 keyword of the same name.
	DependentRequired map[string][]string `yaml:"dependentRequired,omitempty"`
	// Validations declares cross-field rules expressed in CEL. Reserved in
	// v0.1.0 — the parser persists rules and round-trips them, but the
	// engine that compiles and evaluates them ships in Phase 2 (issue #76).
	Validations []ValidationYAML `yaml:"validations,omitempty"`
	Extensions  map[string]any   `yaml:",inline"`
}

// ValidationYAML mirrors a single ValidationRule entry on the wire.
// Fields match the proto message but use YAML naming conventions.
type ValidationYAML struct {
	Path       string         `yaml:"path,omitempty"`
	Rule       string         `yaml:"rule"`
	Message    string         `yaml:"message"`
	Severity   string         `yaml:"severity,omitempty"`
	Reason     string         `yaml:"reason,omitempty"`
	Extensions map[string]any `yaml:",inline"`
}

// SchemaInfoYAML contains optional schema-level metadata.
type SchemaInfoYAML struct {
	Title      string             `yaml:"title,omitempty"`
	Author     string             `yaml:"author,omitempty"`
	Contact    *SchemaContactYAML `yaml:"contact,omitempty"`
	Labels     map[string]string  `yaml:"labels,omitempty"`
	Extensions map[string]any     `yaml:",inline"`
}

// SchemaContactYAML contains contact information for a schema owner.
type SchemaContactYAML struct {
	Name       string         `yaml:"name,omitempty"`
	Email      string         `yaml:"email,omitempty"`
	URL        string         `yaml:"url,omitempty"`
	Extensions map[string]any `yaml:",inline"`
}

// SchemaFieldYAML represents a single field in the YAML format.
type SchemaFieldYAML struct {
	Type         string                 `yaml:"type"`
	Description  string                 `yaml:"description,omitempty"`
	Default      string                 `yaml:"default,omitempty"`
	Nullable     bool                   `yaml:"nullable,omitempty"`
	Deprecated   bool                   `yaml:"deprecated,omitempty"`
	RedirectTo   string                 `yaml:"redirect_to,omitempty"`
	Constraints  *ConstraintsYAML       `yaml:"constraints,omitempty"`
	Title        string                 `yaml:"title,omitempty"`
	Example      string                 `yaml:"example,omitempty"`
	Examples     map[string]ExampleYAML `yaml:"examples,omitempty"`
	ExternalDocs *ExternalDocsYAML      `yaml:"externalDocs,omitempty"`
	Tags         []string               `yaml:"tags,omitempty"`
	Format       string                 `yaml:"format,omitempty"`
	ReadOnly     bool                   `yaml:"readOnly,omitempty"`
	WriteOnce    bool                   `yaml:"writeOnce,omitempty"`
	Sensitive    bool                   `yaml:"sensitive,omitempty"`
	Extensions   map[string]any         `yaml:",inline"`
}

// ExampleYAML represents a named example value.
type ExampleYAML struct {
	Value      string         `yaml:"value"`
	Summary    string         `yaml:"summary,omitempty"`
	Extensions map[string]any `yaml:",inline"`
}

// ExternalDocsYAML links to external documentation.
type ExternalDocsYAML struct {
	Description string         `yaml:"description,omitempty"`
	URL         string         `yaml:"url"`
	Extensions  map[string]any `yaml:",inline"`
}

// ConstraintsYAML uses OAS-style naming for field constraints.
type ConstraintsYAML struct {
	Minimum          *float64       `yaml:"minimum,omitempty"`
	Maximum          *float64       `yaml:"maximum,omitempty"`
	ExclusiveMinimum *float64       `yaml:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64       `yaml:"exclusiveMaximum,omitempty"`
	MinLength        *int32         `yaml:"minLength,omitempty"`
	MaxLength        *int32         `yaml:"maxLength,omitempty"`
	Pattern          string         `yaml:"pattern,omitempty"`
	Enum             []string       `yaml:"enum,omitempty"`
	JSONSchema       string         `yaml:"json_schema,omitempty"`
	Extensions       map[string]any `yaml:",inline"`
}

// --- Validation ---

func validateSchemaYAML(doc *SchemaYAML) error {
	if doc.SpecVersion == "" {
		return fmt.Errorf("spec_version is required")
	}
	if doc.SpecVersion != yamlSpecVersionV1 {
		return fmt.Errorf("unsupported spec_version: %s", doc.SpecVersion)
	}
	if doc.Schema != "" {
		u, err := url.Parse(doc.Schema)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("$schema must be an HTTPS URL, got %q", doc.Schema)
		}
	}
	if doc.ID != "" && !schemaURNPattern.MatchString(doc.ID) {
		return fmt.Errorf("$id must match urn:decree:schema:<segment>(:<segment>)*, got %q", doc.ID)
	}
	if err := validateExtensions("", doc.Extensions); err != nil {
		return err
	}
	if doc.Info != nil {
		if err := validateExtensions("info", doc.Info.Extensions); err != nil {
			return err
		}
		if doc.Info.Contact != nil {
			if err := validateExtensions("info.contact", doc.Info.Contact.Extensions); err != nil {
				return err
			}
		}
	}
	if doc.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !isValidSlug(doc.Name) {
		return fmt.Errorf("name must be a slug: lowercase alphanumeric and hyphens, 1-63 chars")
	}
	if len(doc.Fields) == 0 {
		return fmt.Errorf("at least one field is required")
	}
	if err := validateDependentRequiredYAML(doc); err != nil {
		return err
	}
	if err := validateValidationsYAML(doc); err != nil {
		return err
	}
	for path, f := range doc.Fields {
		if !fieldPathPattern.MatchString(path) {
			return fmt.Errorf("invalid field path %q: must match %s", path, fieldPathPattern)
		}
		if _, ok := yamlTypeToProto(f.Type); !ok {
			return fmt.Errorf("field %s: unknown type %q", path, f.Type)
		}
		if err := validateExtensions("fields."+path, f.Extensions); err != nil {
			return err
		}
		if f.Constraints != nil {
			if err := validateExtensions("fields."+path+".constraints", f.Constraints.Extensions); err != nil {
				return err
			}
		}
		if f.ExternalDocs != nil {
			if err := validateExtensions("fields."+path+".externalDocs", f.ExternalDocs.Extensions); err != nil {
				return err
			}
		}
		for exName, ex := range f.Examples {
			if err := validateExtensions("fields."+path+".examples."+exName, ex.Extensions); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateDependentRequiredYAML lint-checks the `dependentRequired:` map at
// schema-validate time: every trigger key must reference a real field in the
// schema; every dependent path must reference a real field; the trigger may
// not appear in its own dependents list (a self-referential rule is always
// vacuously true and indicates author error).
func validateDependentRequiredYAML(doc *SchemaYAML) error {
	if len(doc.DependentRequired) == 0 {
		return nil
	}
	for trigger, dependents := range doc.DependentRequired {
		if _, ok := doc.Fields[trigger]; !ok {
			return fmt.Errorf("dependentRequired: trigger %q is not a defined field", trigger)
		}
		seen := make(map[string]struct{}, len(dependents))
		for _, dep := range dependents {
			if dep == trigger {
				return fmt.Errorf("dependentRequired: trigger %q cannot list itself as a dependent", trigger)
			}
			if _, ok := doc.Fields[dep]; !ok {
				return fmt.Errorf("dependentRequired: dependent %q (under trigger %q) is not a defined field", dep, trigger)
			}
			if _, dup := seen[dep]; dup {
				return fmt.Errorf("dependentRequired: dependent %q listed twice under trigger %q", dep, trigger)
			}
			seen[dep] = struct{}{}
		}
	}
	return nil
}

// validateValidationsYAML structurally lint-checks the `validations:` list
// at schema-validate time. v0.1.0 reserves the key in the format spec but
// does not yet ship the CEL engine — so this routine only enforces what
// the wire shape requires:
//
//   - rule and message are required, non-empty
//   - severity is empty or one of "error" | "warning"
//
// CEL compilation, field-reference resolution, and contradiction
// detection are deferred to Phase 2 (see .agents/context/cel-validation.md).
func validateValidationsYAML(doc *SchemaYAML) error {
	if len(doc.Validations) == 0 {
		return nil
	}
	for i, v := range doc.Validations {
		if v.Rule == "" {
			return fmt.Errorf("validations[%d]: rule is required", i)
		}
		if v.Message == "" {
			return fmt.Errorf("validations[%d]: message is required", i)
		}
		switch v.Severity {
		case "", "error", "warning":
			// ok
		default:
			return fmt.Errorf("validations[%d]: severity %q must be \"error\" or \"warning\"", i, v.Severity)
		}
		if err := validateExtensions(fmt.Sprintf("validations[%d]", i), v.Extensions); err != nil {
			return err
		}
	}
	return nil
}

// validateExtensions rejects any keys in the inline-extension map that do not
// match the x-* vendor-extension pattern. The path prefix is included in the
// error so users can locate the offending key in large documents.
func validateExtensions(pathPrefix string, ext map[string]any) error {
	for key := range ext {
		if !extensionKeyPattern.MatchString(key) {
			loc := "top level"
			if pathPrefix != "" {
				loc = pathPrefix
			}
			return fmt.Errorf("unknown key %q at %s: only known fields or x-* vendor extensions are allowed", key, loc)
		}
	}
	return nil
}

// --- Type mapping ---

func yamlTypeToProto(t string) (pb.FieldType, bool) {
	switch t {
	case "integer":
		return pb.FieldType_FIELD_TYPE_INT, true
	case "number":
		return pb.FieldType_FIELD_TYPE_NUMBER, true
	case "string":
		return pb.FieldType_FIELD_TYPE_STRING, true
	case "bool":
		return pb.FieldType_FIELD_TYPE_BOOL, true
	case "time":
		return pb.FieldType_FIELD_TYPE_TIME, true
	case "duration":
		return pb.FieldType_FIELD_TYPE_DURATION, true
	case "url":
		return pb.FieldType_FIELD_TYPE_URL, true
	case "json":
		return pb.FieldType_FIELD_TYPE_JSON, true
	default:
		return pb.FieldType_FIELD_TYPE_UNSPECIFIED, false
	}
}

func protoTypeToYAML(t pb.FieldType) string {
	switch t {
	case pb.FieldType_FIELD_TYPE_INT:
		return "integer"
	case pb.FieldType_FIELD_TYPE_NUMBER:
		return "number"
	case pb.FieldType_FIELD_TYPE_STRING:
		return "string"
	case pb.FieldType_FIELD_TYPE_BOOL:
		return "bool"
	case pb.FieldType_FIELD_TYPE_TIME:
		return "time"
	case pb.FieldType_FIELD_TYPE_DURATION:
		return "duration"
	case pb.FieldType_FIELD_TYPE_URL:
		return "url"
	case pb.FieldType_FIELD_TYPE_JSON:
		return "json"
	default:
		return "string"
	}
}

// --- Proto → YAML ---

func schemaToYAML(s *pb.Schema) *SchemaYAML {
	doc := &SchemaYAML{
		SpecVersion:        yamlSpecVersionV1,
		Schema:             metaSchemaURL,
		ID:                 fmt.Sprintf("urn:decree:schema:%s:v%d", s.Name, s.Version),
		Name:               s.Name,
		Description:        s.Description,
		Version:            s.Version,
		VersionDescription: s.VersionDescription,
		Info:               schemaInfoToYAML(s.Info),
		Fields:             make(map[string]SchemaFieldYAML, len(s.Fields)),
		DependentRequired:  protoDependentRequiredToYAML(s.DependentRequired),
		Validations:        protoValidationsToYAML(s.Validations),
	}

	for _, f := range s.Fields {
		yf := SchemaFieldYAML{
			Type:       protoTypeToYAML(f.Type),
			Nullable:   f.Nullable,
			Deprecated: f.Deprecated,
			ReadOnly:   f.ReadOnly,
			WriteOnce:  f.WriteOnce,
			Sensitive:  f.Sensitive,
		}
		if f.Description != nil {
			yf.Description = *f.Description
		}
		if f.DefaultValue != nil {
			yf.Default = *f.DefaultValue
		}
		if f.RedirectTo != nil {
			yf.RedirectTo = *f.RedirectTo
		}
		if f.Title != nil {
			yf.Title = *f.Title
		}
		if f.Example != nil {
			yf.Example = *f.Example
		}
		if f.Format != nil {
			yf.Format = *f.Format
		}
		if len(f.Tags) > 0 {
			yf.Tags = f.Tags
		}
		if len(f.Examples) > 0 {
			yf.Examples = make(map[string]ExampleYAML, len(f.Examples))
			for k, v := range f.Examples {
				yf.Examples[k] = ExampleYAML{Value: v.Value, Summary: v.Summary}
			}
		}
		if f.ExternalDocs != nil {
			yf.ExternalDocs = &ExternalDocsYAML{
				Description: f.ExternalDocs.Description,
				URL:         f.ExternalDocs.Url,
			}
		}
		if f.Constraints != nil {
			yf.Constraints = protoConstraintsToYAML(f.Constraints)
		}
		doc.Fields[f.Path] = yf
	}

	return doc
}

func schemaInfoToYAML(info *pb.SchemaInfo) *SchemaInfoYAML {
	if info == nil {
		return nil
	}
	yi := &SchemaInfoYAML{
		Title:  info.Title,
		Author: info.Author,
	}
	if info.Contact != nil {
		yi.Contact = &SchemaContactYAML{
			Name:  info.Contact.Name,
			Email: info.Contact.Email,
			URL:   info.Contact.Url,
		}
	}
	if len(info.Labels) > 0 {
		yi.Labels = info.Labels
	}
	if yi.Title == "" && yi.Author == "" && yi.Contact == nil && len(yi.Labels) == 0 {
		return nil
	}
	return yi
}

func protoConstraintsToYAML(c *pb.FieldConstraints) *ConstraintsYAML {
	if c == nil {
		return nil
	}
	yc := &ConstraintsYAML{
		Minimum:          c.Min,
		Maximum:          c.Max,
		ExclusiveMinimum: c.ExclusiveMin,
		ExclusiveMaximum: c.ExclusiveMax,
		MinLength:        c.MinLength,
		MaxLength:        c.MaxLength,
		JSONSchema:       c.GetJsonSchema(),
	}
	if c.Regex != nil {
		yc.Pattern = *c.Regex
	}
	if len(c.EnumValues) > 0 {
		yc.Enum = c.EnumValues
	}
	// Return nil if all fields are zero-valued.
	if yc.Minimum == nil && yc.Maximum == nil && yc.ExclusiveMinimum == nil && yc.ExclusiveMaximum == nil &&
		yc.MinLength == nil && yc.MaxLength == nil && yc.Pattern == "" && len(yc.Enum) == 0 && yc.JSONSchema == "" {
		return nil
	}
	return yc
}

// --- YAML → Proto ---

// yamlToProtoValidations converts the YAML validations list into proto
// ValidationRule entries. Order is preserved — schema authors typically
// expect rules to appear in the same order they declared them.
func yamlToProtoValidations(in []ValidationYAML) []*pb.ValidationRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]*pb.ValidationRule, 0, len(in))
	for _, v := range in {
		out = append(out, &pb.ValidationRule{
			Path:     v.Path,
			Rule:     v.Rule,
			Message:  v.Message,
			Severity: v.Severity,
			Reason:   v.Reason,
		})
	}
	return out
}

// protoValidationsToYAML converts proto ValidationRule entries back to
// YAML form for export. Returns nil for an empty input so the YAML key
// is omitted.
func protoValidationsToYAML(in []*pb.ValidationRule) []ValidationYAML {
	if len(in) == 0 {
		return nil
	}
	out := make([]ValidationYAML, 0, len(in))
	for _, v := range in {
		out = append(out, ValidationYAML{
			Path:     v.Path,
			Rule:     v.Rule,
			Message:  v.Message,
			Severity: v.Severity,
			Reason:   v.Reason,
		})
	}
	return out
}

// yamlToProtoDependentRequired converts the YAML map<trigger,[dependents]>
// shape into the proto repeated-entry shape. Returns nil for an empty map so
// the wire format stays compact.
func yamlToProtoDependentRequired(m map[string][]string) []*pb.DependentRequiredEntry {
	if len(m) == 0 {
		return nil
	}
	triggers := make([]string, 0, len(m))
	for k := range m {
		triggers = append(triggers, k)
	}
	sort.Strings(triggers)
	out := make([]*pb.DependentRequiredEntry, 0, len(triggers))
	for _, t := range triggers {
		deps := append([]string(nil), m[t]...)
		sort.Strings(deps)
		out = append(out, &pb.DependentRequiredEntry{
			TriggerField:    t,
			DependentFields: deps,
		})
	}
	return out
}

// protoDependentRequiredToYAML converts the proto repeated-entry shape back
// into the YAML map<trigger,[dependents]> shape. Returns nil when the input
// is empty so the YAML key is omitted.
func protoDependentRequiredToYAML(entries []*pb.DependentRequiredEntry) map[string][]string {
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string][]string, len(entries))
	for _, e := range entries {
		deps := append([]string(nil), e.DependentFields...)
		sort.Strings(deps)
		out[e.TriggerField] = deps
	}
	return out
}

func yamlToProtoFields(doc *SchemaYAML) []*pb.SchemaField {
	fields := make([]*pb.SchemaField, 0, len(doc.Fields))
	for path, yf := range doc.Fields {
		ft, _ := yamlTypeToProto(yf.Type) // already validated
		f := &pb.SchemaField{
			Path:       path,
			Type:       ft,
			Nullable:   yf.Nullable,
			Deprecated: yf.Deprecated,
			ReadOnly:   yf.ReadOnly,
			WriteOnce:  yf.WriteOnce,
			Sensitive:  yf.Sensitive,
			Tags:       yf.Tags,
		}
		if yf.Description != "" {
			f.Description = &yf.Description
		}
		if yf.Default != "" {
			f.DefaultValue = &yf.Default
		}
		if yf.RedirectTo != "" {
			f.RedirectTo = &yf.RedirectTo
		}
		if yf.Title != "" {
			f.Title = &yf.Title
		}
		if yf.Example != "" {
			f.Example = &yf.Example
		}
		if yf.Format != "" {
			f.Format = &yf.Format
		}
		if len(yf.Examples) > 0 {
			f.Examples = make(map[string]*pb.FieldExample, len(yf.Examples))
			for k, v := range yf.Examples {
				f.Examples[k] = &pb.FieldExample{Value: v.Value, Summary: v.Summary}
			}
		}
		if yf.ExternalDocs != nil {
			f.ExternalDocs = &pb.ExternalDocs{
				Description: yf.ExternalDocs.Description,
				Url:         yf.ExternalDocs.URL,
			}
		}
		if yf.Constraints != nil {
			f.Constraints = yamlConstraintsToProto(yf.Constraints)
		}
		fields = append(fields, f)
	}

	// Sort by path for deterministic output.
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Path < fields[j].Path
	})

	return fields
}

func yamlConstraintsToProto(yc *ConstraintsYAML) *pb.FieldConstraints {
	if yc == nil {
		return nil
	}
	c := &pb.FieldConstraints{
		Min:          yc.Minimum,
		Max:          yc.Maximum,
		ExclusiveMin: yc.ExclusiveMinimum,
		ExclusiveMax: yc.ExclusiveMaximum,
		MinLength:    yc.MinLength,
		MaxLength:    yc.MaxLength,
	}
	if yc.Pattern != "" {
		c.Regex = &yc.Pattern
	}
	if len(yc.Enum) > 0 {
		c.EnumValues = yc.Enum
	}
	if yc.JSONSchema != "" {
		c.JsonSchema = &yc.JSONSchema
	}
	return c
}

// --- Marshal / Unmarshal ---

func marshalSchemaYAML(doc *SchemaYAML) ([]byte, error) {
	return yaml.Marshal(doc)
}

func unmarshalSchemaYAML(data []byte) (*SchemaYAML, error) {
	var doc SchemaYAML
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if err := validateSchemaYAML(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}
