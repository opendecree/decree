package schema

import (
	"fmt"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

// validateFieldConstraints checks that constraints are applicable to the field type.
// Returns an error if a constraint is applied to an incompatible type.
func validateFieldConstraints(field *pb.SchemaField) error {
	c := field.Constraints
	if c == nil {
		return nil
	}

	path := field.Path
	ft := field.Type

	// Numeric constraints: min, max, exclusiveMin, exclusiveMax
	numericOnly := ft == pb.FieldType_FIELD_TYPE_INT ||
		ft == pb.FieldType_FIELD_TYPE_NUMBER ||
		ft == pb.FieldType_FIELD_TYPE_DURATION

	if c.Min != nil && !numericOnly {
		return fmt.Errorf("field %s: 'minimum' constraint is not valid for type %s (only integer, number, duration)", path, ft)
	}
	if c.Max != nil && !numericOnly {
		return fmt.Errorf("field %s: 'maximum' constraint is not valid for type %s (only integer, number, duration)", path, ft)
	}
	if c.ExclusiveMin != nil && !numericOnly {
		return fmt.Errorf("field %s: 'exclusiveMinimum' constraint is not valid for type %s (only integer, number, duration)", path, ft)
	}
	if c.ExclusiveMax != nil && !numericOnly {
		return fmt.Errorf("field %s: 'exclusiveMaximum' constraint is not valid for type %s (only integer, number, duration)", path, ft)
	}

	// String length constraints: minLength, maxLength
	if c.MinLength != nil && ft != pb.FieldType_FIELD_TYPE_STRING {
		return fmt.Errorf("field %s: 'minLength' constraint is not valid for type %s (only string)", path, ft)
	}
	if c.MaxLength != nil && ft != pb.FieldType_FIELD_TYPE_STRING {
		return fmt.Errorf("field %s: 'maxLength' constraint is not valid for type %s (only string)", path, ft)
	}

	// Pattern: string only
	if c.Regex != nil && ft != pb.FieldType_FIELD_TYPE_STRING {
		return fmt.Errorf("field %s: 'pattern' constraint is not valid for type %s (only string)", path, ft)
	}

	// JSON Schema: json only
	if c.JsonSchema != nil && ft != pb.FieldType_FIELD_TYPE_JSON {
		return fmt.Errorf("field %s: 'json_schema' constraint is not valid for type %s (only json)", path, ft)
	}

	// enum: any type is fine — no restriction needed

	// Range sanity checks
	if c.Min != nil && c.Max != nil && *c.Min > *c.Max {
		return fmt.Errorf("field %s: minimum (%v) is greater than maximum (%v)", path, *c.Min, *c.Max)
	}
	if c.ExclusiveMin != nil && c.ExclusiveMax != nil && *c.ExclusiveMin >= *c.ExclusiveMax {
		return fmt.Errorf("field %s: exclusiveMinimum (%v) must be less than exclusiveMaximum (%v)", path, *c.ExclusiveMin, *c.ExclusiveMax)
	}
	if c.MinLength != nil && c.MaxLength != nil && *c.MinLength > *c.MaxLength {
		return fmt.Errorf("field %s: minLength (%d) is greater than maxLength (%d)", path, *c.MinLength, *c.MaxLength)
	}

	return nil
}
