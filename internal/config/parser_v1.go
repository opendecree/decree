package config

import (
	"github.com/opendecree/decree/internal/storage/domain"
)

// parserV1 is the config parser for spec_version "v1" — the only version
// in v0.1.0 of the config format. It delegates to the existing
// ConfigYAML-based unmarshal/marshal helpers in yaml.go; the registry
// indirection is what gives us the room to add v2 as a sibling file
// alongside this one without touching v1 code.
type parserV1 struct{}

func (parserV1) SpecVersion() string { return yamlSpecVersionV1 }

func (parserV1) Parse(data []byte, fieldTypes map[string]domain.FieldType) (ParsedImport, error) {
	doc, err := unmarshalConfigYAML(data)
	if err != nil {
		return ParsedImport{}, err
	}
	values, err := yamlToConfigValues(doc, fieldTypes)
	if err != nil {
		return ParsedImport{}, err
	}
	return ParsedImport{
		Description: doc.Description,
		Values:      values,
	}, nil
}

func (parserV1) Marshal(version int32, description string, rows []configRow, fieldTypes map[string]domain.FieldType) ([]byte, error) {
	return marshalConfigYAML(configToYAML(version, description, rows, fieldTypes))
}

func init() {
	Register(parserV1{})
}
