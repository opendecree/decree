package schema

import (
	"testing"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

func BenchmarkMarshalSchemaYAML(b *testing.B) {
	doc := &SchemaYAML{
		Syntax: yamlSyntaxV1,
		Name:   "payments",
		Fields: map[string]SchemaFieldYAML{
			"payments.fee":      {Type: "string", Description: "Fee percentage"},
			"payments.currency": {Type: "string"},
			"payments.retries":  {Type: "integer"},
			"payments.enabled":  {Type: "bool"},
			"payments.timeout":  {Type: "duration"},
		},
	}
	for b.Loop() {
		_, _ = marshalSchemaYAML(doc)
	}
}

func BenchmarkUnmarshalSchemaYAML(b *testing.B) {
	data := []byte(`syntax: "v1"
name: payments
fields:
  payments.fee:
    type: string
    description: Fee percentage
  payments.currency:
    type: string
  payments.retries:
    type: integer
  payments.enabled:
    type: bool
  payments.timeout:
    type: duration
`)
	for b.Loop() {
		_, _ = unmarshalSchemaYAML(data)
	}
}

func BenchmarkSchemaToYAML(b *testing.B) {
	s := &pb.Schema{
		Name: "payments",
		Fields: []*pb.SchemaField{
			{Path: "payments.fee", Type: pb.FieldType_FIELD_TYPE_STRING},
			{Path: "payments.currency", Type: pb.FieldType_FIELD_TYPE_STRING},
			{Path: "payments.retries", Type: pb.FieldType_FIELD_TYPE_INT},
			{Path: "payments.enabled", Type: pb.FieldType_FIELD_TYPE_BOOL},
			{Path: "payments.timeout", Type: pb.FieldType_FIELD_TYPE_DURATION},
		},
	}
	for b.Loop() {
		schemaToYAML(s)
	}
}
