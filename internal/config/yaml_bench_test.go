package config

import (
	"testing"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

func BenchmarkMarshalConfigYAML(b *testing.B) {
	doc := &ConfigYAML{
		Syntax:  yamlSyntaxV1,
		Version: 3,
		Values: map[string]ConfigValueYAML{
			"payments.fee":      {Value: 0.025},
			"payments.currency": {Value: "USD"},
			"payments.enabled":  {Value: true},
			"payments.retries":  {Value: 5},
			"payments.timeout":  {Value: "24h"},
		},
	}
	for b.Loop() {
		_, _ = marshalConfigYAML(doc)
	}
}

func BenchmarkUnmarshalConfigYAML(b *testing.B) {
	data := []byte(`syntax: "v1"
version: 3
values:
  payments.fee:
    value: 0.025
  payments.currency:
    value: "USD"
  payments.enabled:
    value: true
  payments.retries:
    value: 5
  payments.timeout:
    value: "24h"
`)
	for b.Loop() {
		_, _ = unmarshalConfigYAML(data)
	}
}

func BenchmarkConfigToYAML(b *testing.B) {
	rows := []configRow{
		{FieldPath: "payments.fee", Value: "0.025"},
		{FieldPath: "payments.currency", Value: "USD"},
		{FieldPath: "payments.enabled", Value: "true"},
		{FieldPath: "payments.retries", Value: "5"},
		{FieldPath: "payments.timeout", Value: "24h"},
	}
	fieldTypes := map[string]pb.FieldType{
		"payments.fee":      pb.FieldType_FIELD_TYPE_NUMBER,
		"payments.currency": pb.FieldType_FIELD_TYPE_STRING,
		"payments.enabled":  pb.FieldType_FIELD_TYPE_BOOL,
		"payments.retries":  pb.FieldType_FIELD_TYPE_INT,
		"payments.timeout":  pb.FieldType_FIELD_TYPE_DURATION,
	}
	for b.Loop() {
		configToYAML(3, "test", rows, fieldTypes)
	}
}
