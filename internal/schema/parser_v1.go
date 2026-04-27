package schema

import (
	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// parserV1 is the schema parser for spec_version "v1" — the only version
// in v0.1.0 of the schema spec. It delegates to the existing
// SchemaYAML-based unmarshal/marshal helpers in yaml.go; the registry
// indirection is what gives us the room to add v2 as a sibling file
// alongside this one without touching v1 code.
type parserV1 struct{}

func (parserV1) SpecVersion() string { return yamlSpecVersionV1 }

func (parserV1) Parse(data []byte) (*pb.Schema, error) {
	doc, err := unmarshalSchemaYAML(data)
	if err != nil {
		return nil, err
	}
	fields := yamlToProtoFields(doc)
	depReqs := yamlToProtoDependentRequired(doc.DependentRequired)
	validations := yamlToProtoValidations(doc.Validations)
	return &pb.Schema{
		Name:               doc.Name,
		Description:        doc.Description,
		Version:            doc.Version,
		VersionDescription: doc.VersionDescription,
		Info:               yamlToProtoInfo(doc.Info),
		Fields:             fields,
		DependentRequired:  depReqs,
		Validations:        validations,
	}, nil
}

func (parserV1) Marshal(s *pb.Schema) ([]byte, error) {
	return marshalSchemaYAML(schemaToYAML(s))
}

func init() {
	Register(parserV1{})
}

// yamlToProtoInfo lifts the optional info block out of the YAML doc into
// the proto SchemaInfo. Lives here rather than yaml.go because it's only
// used by the parser-side conversion; yaml.go's existing converters
// handle fields and the schema-level metadata separately.
func yamlToProtoInfo(yi *SchemaInfoYAML) *pb.SchemaInfo {
	if yi == nil {
		return nil
	}
	info := &pb.SchemaInfo{
		Title:  yi.Title,
		Author: yi.Author,
		Labels: yi.Labels,
	}
	if yi.Contact != nil {
		info.Contact = &pb.SchemaContact{
			Name:  yi.Contact.Name,
			Email: yi.Contact.Email,
			Url:   yi.Contact.URL,
		}
	}
	return info
}
