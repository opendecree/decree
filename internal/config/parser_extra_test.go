package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage/domain"
)

func TestParserV1_Parse_InvalidYAML(t *testing.T) {
	// Malformed YAML (unclosed flow sequence) fails at unmarshal.
	_, err := parserV1{}.Parse([]byte("spec_version: v1\nvalues: [oops"), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid YAML")
}

func TestParserV1_Parse_TypeMismatch(t *testing.T) {
	// Valid YAML that passes structural validation but fails type coercion:
	// a fractional value for a field declared as integer.
	_, err := parserV1{}.Parse([]byte(`
spec_version: v1
values:
  retries.max:
    value: 1.5
`), map[string]domain.FieldType{"retries.max": domain.FieldTypeInteger})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retries.max")
}

func TestRegister_DuplicatePanics(t *testing.T) {
	// v1 is already registered via init(); re-registering it is a programming
	// error and must panic. The check happens before insertion, so the global
	// registry is left untouched.
	assert.Panics(t, func() { Register(parserV1{}) })
}
