package schema

import (
	"fmt"
	"sort"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"gopkg.in/yaml.v3"
)

const yamlSyntaxV1 = "v1"

// SchemaYAML is the top-level YAML document for schema import/export.
type SchemaYAML struct {
	Syntax             string                     `yaml:"syntax"`
	Name               string                     `yaml:"name"`
	Description        string                     `yaml:"description,omitempty"`
	Version            int32                      `yaml:"version,omitempty"`
	VersionDescription string                     `yaml:"version_description,omitempty"`
	Fields             map[string]SchemaFieldYAML `yaml:"fields"`
}

// SchemaFieldYAML represents a single field in the YAML format.
type SchemaFieldYAML struct {
	Type        string           `yaml:"type"`
	Description string           `yaml:"description,omitempty"`
	Default     string           `yaml:"default,omitempty"`
	Nullable    bool             `yaml:"nullable,omitempty"`
	Deprecated  bool             `yaml:"deprecated,omitempty"`
	RedirectTo  string           `yaml:"redirect_to,omitempty"`
	Constraints *ConstraintsYAML `yaml:"constraints,omitempty"`
}

// ConstraintsYAML uses OAS-style naming for field constraints.
type ConstraintsYAML struct {
	Minimum          *float64 `yaml:"minimum,omitempty"`
	Maximum          *float64 `yaml:"maximum,omitempty"`
	ExclusiveMinimum *float64 `yaml:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `yaml:"exclusiveMaximum,omitempty"`
	MinLength        *int32   `yaml:"minLength,omitempty"`
	MaxLength        *int32   `yaml:"maxLength,omitempty"`
	Pattern          string   `yaml:"pattern,omitempty"`
	Enum             []string `yaml:"enum,omitempty"`
	JSONSchema       string   `yaml:"json_schema,omitempty"`
}

// --- Validation ---

func validateSchemaYAML(doc *SchemaYAML) error {
	if doc.Syntax == "" {
		return fmt.Errorf("syntax is required")
	}
	if doc.Syntax != yamlSyntaxV1 {
		return fmt.Errorf("unsupported syntax version: %s", doc.Syntax)
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
	for path, f := range doc.Fields {
		if path == "" {
			return fmt.Errorf("field path cannot be empty")
		}
		if _, ok := yamlTypeToProto(f.Type); !ok {
			return fmt.Errorf("field %s: unknown type %q", path, f.Type)
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
		Syntax:             yamlSyntaxV1,
		Name:               s.Name,
		Description:        s.Description,
		Version:            s.Version,
		VersionDescription: s.VersionDescription,
		Fields:             make(map[string]SchemaFieldYAML, len(s.Fields)),
	}

	for _, f := range s.Fields {
		yf := SchemaFieldYAML{
			Type:       protoTypeToYAML(f.Type),
			Nullable:   f.Nullable,
			Deprecated: f.Deprecated,
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
		if f.Constraints != nil {
			yf.Constraints = protoConstraintsToYAML(f.Constraints)
		}
		doc.Fields[f.Path] = yf
	}

	return doc
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

func yamlToProtoFields(doc *SchemaYAML) []*pb.SchemaField {
	fields := make([]*pb.SchemaField, 0, len(doc.Fields))
	for path, yf := range doc.Fields {
		ft, _ := yamlTypeToProto(yf.Type) // already validated
		f := &pb.SchemaField{
			Path:       path,
			Type:       ft,
			Nullable:   yf.Nullable,
			Deprecated: yf.Deprecated,
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
