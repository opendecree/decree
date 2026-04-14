package validation

import (
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// jsonSchemaValidator validates JSON values against a JSON Schema document.
type jsonSchemaValidator struct {
	schema *jsonschema.Schema
}

// newJSONSchemaValidator compiles a JSON Schema document for validation.
func newJSONSchemaValidator(schemaDoc string) (*jsonSchemaValidator, error) {
	c := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaDoc))
	if err != nil {
		return nil, fmt.Errorf("invalid json schema: %w", err)
	}
	if err := c.AddResource("schema.json", doc); err != nil {
		return nil, fmt.Errorf("add json schema resource: %w", err)
	}
	schema, err := c.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile json schema: %w", err)
	}
	return &jsonSchemaValidator{schema: schema}, nil
}

// validate checks a JSON string against the compiled schema.
func (v *jsonSchemaValidator) validate(jsonStr string) error {
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(jsonStr))
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := v.schema.Validate(inst); err != nil {
		return fmt.Errorf("json schema validation failed: %w", err)
	}
	return nil
}
