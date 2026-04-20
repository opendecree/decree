package config

import (
	"testing"

	"github.com/opendecree/decree/internal/storage/domain"
)

func BenchmarkMarshalConfigYAML(b *testing.B) {
	doc := &ConfigYAML{
		SpecVersion: yamlSpecVersionV1,
		Version:     3,
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
	data := []byte(`spec_version: "v1"
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
	fieldTypes := map[string]domain.FieldType{
		"payments.fee":      domain.FieldTypeNumber,
		"payments.currency": domain.FieldTypeString,
		"payments.enabled":  domain.FieldTypeBool,
		"payments.retries":  domain.FieldTypeInteger,
		"payments.timeout":  domain.FieldTypeDuration,
	}
	for b.Loop() {
		configToYAML(3, "test", rows, fieldTypes)
	}
}
