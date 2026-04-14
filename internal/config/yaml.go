package config

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"gopkg.in/yaml.v3"
)

const yamlSyntaxV1 = "v1"

// ConfigYAML is the top-level YAML document for config import/export.
type ConfigYAML struct {
	Syntax      string                     `yaml:"syntax"`
	Version     int32                      `yaml:"version,omitempty"`
	Description string                     `yaml:"description,omitempty"`
	Values      map[string]ConfigValueYAML `yaml:"values"`
}

// ConfigValueYAML represents a single config value in the YAML format.
type ConfigValueYAML struct {
	Value       interface{} `yaml:"value"`
	Description string      `yaml:"description,omitempty"`
}

// configValueImport is the parsed result of a YAML config value, ready for DB storage.
type configValueImport struct {
	FieldPath   string
	Value       string
	Description *string
}

// --- Validation ---

func validateConfigYAML(doc *ConfigYAML) error {
	if doc.Syntax == "" {
		return fmt.Errorf("syntax is required")
	}
	if doc.Syntax != yamlSyntaxV1 {
		return fmt.Errorf("unsupported syntax version: %s", doc.Syntax)
	}
	if len(doc.Values) == 0 {
		return fmt.Errorf("at least one value is required")
	}
	for path := range doc.Values {
		if path == "" {
			return fmt.Errorf("field path cannot be empty")
		}
	}
	return nil
}

// --- Export: DB rows → YAML ---

// configToYAML converts config rows to a YAML document, using schema field types
// for typed value representation.
func configToYAML(version int32, description string, rows []configRow, fieldTypes map[string]pb.FieldType) *ConfigYAML {
	doc := &ConfigYAML{
		Syntax:      yamlSyntaxV1,
		Version:     version,
		Description: description,
		Values:      make(map[string]ConfigValueYAML, len(rows)),
	}

	for _, row := range rows {
		yv := ConfigValueYAML{
			Value: typedValue(row.Value, fieldTypes[row.FieldPath]),
		}
		if row.Description != nil {
			yv.Description = *row.Description
		}
		doc.Values[row.FieldPath] = yv
	}

	return doc
}

// configRow is a minimal representation of a config value row.
type configRow struct {
	FieldPath   string
	Value       string
	Description *string
}

// typedValue converts a string value to its native Go type based on the field type.
func typedValue(value string, ft pb.FieldType) interface{} {
	switch ft {
	case pb.FieldType_FIELD_TYPE_INT:
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			return v
		}
	case pb.FieldType_FIELD_TYPE_NUMBER:
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			return v
		}
	case pb.FieldType_FIELD_TYPE_BOOL:
		if v, err := strconv.ParseBool(value); err == nil {
			return v
		}
	case pb.FieldType_FIELD_TYPE_JSON:
		var v interface{}
		if err := json.Unmarshal([]byte(value), &v); err == nil {
			return v
		}
	}
	// string, time, duration, url, unknown → passthrough
	return value
}

// --- Import: YAML → string values ---

// yamlToConfigValues converts YAML values back to string representations,
// using schema field types for type-aware conversion.
func yamlToConfigValues(doc *ConfigYAML, fieldTypes map[string]pb.FieldType) ([]configValueImport, error) {
	result := make([]configValueImport, 0, len(doc.Values))

	// Sort for deterministic ordering.
	paths := make([]string, 0, len(doc.Values))
	for path := range doc.Values {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		yv := doc.Values[path]
		ft := fieldTypes[path]
		strVal, err := stringifyValue(yv.Value, ft)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", path, err)
		}
		cvi := configValueImport{
			FieldPath: path,
			Value:     strVal,
		}
		if yv.Description != "" {
			cvi.Description = &yv.Description
		}
		result = append(result, cvi)
	}

	return result, nil
}

// stringifyValue converts a YAML-native value back to its string representation.
func stringifyValue(value interface{}, ft pb.FieldType) (string, error) {
	if value == nil {
		return "", nil
	}

	switch ft {
	case pb.FieldType_FIELD_TYPE_INT:
		switch v := value.(type) {
		case int:
			return strconv.FormatInt(int64(v), 10), nil
		case int64:
			return strconv.FormatInt(v, 10), nil
		case float64:
			if v != math.Trunc(v) {
				return "", fmt.Errorf("expected integer, got %v", v)
			}
			return strconv.FormatInt(int64(v), 10), nil
		case string:
			return v, nil
		}
		return "", fmt.Errorf("cannot convert %T to integer", value)

	case pb.FieldType_FIELD_TYPE_NUMBER:
		switch v := value.(type) {
		case int:
			return strconv.FormatFloat(float64(v), 'f', -1, 64), nil
		case int64:
			return strconv.FormatFloat(float64(v), 'f', -1, 64), nil
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64), nil
		case string:
			return v, nil
		}
		return "", fmt.Errorf("cannot convert %T to number", value)

	case pb.FieldType_FIELD_TYPE_BOOL:
		switch v := value.(type) {
		case bool:
			return strconv.FormatBool(v), nil
		case string:
			return v, nil
		}
		return "", fmt.Errorf("cannot convert %T to bool", value)

	case pb.FieldType_FIELD_TYPE_JSON:
		switch v := value.(type) {
		case string:
			return v, nil
		default:
			data, err := json.Marshal(v)
			if err != nil {
				return "", fmt.Errorf("cannot marshal JSON: %w", err)
			}
			return string(data), nil
		}

	default:
		// string, time, duration, url → expect string
		switch v := value.(type) {
		case string:
			return v, nil
		default:
			return fmt.Sprintf("%v", v), nil
		}
	}
}

// --- Marshal / Unmarshal ---

func marshalConfigYAML(doc *ConfigYAML) ([]byte, error) {
	return yaml.Marshal(doc)
}

func unmarshalConfigYAML(data []byte) (*ConfigYAML, error) {
	var doc ConfigYAML
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if err := validateConfigYAML(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}
