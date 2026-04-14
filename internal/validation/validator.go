// Package validation provides field-level constraint validation for config values.
// Validators are built from schema field definitions and check TypedValue instances
// against constraints (min/max, pattern, enum, JSON Schema, URL validity).
package validation

import (
	"fmt"
	"net/url"
	"regexp"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

// FieldValidator validates a TypedValue against a schema field's constraints.
type FieldValidator struct {
	fieldPath string
	fieldType pb.FieldType
	nullable  bool
	checks    []checkFunc
}

type checkFunc func(tv *pb.TypedValue) error

// Validate checks a TypedValue against this field's constraints.
// Returns nil if the value is valid. Returns an error describing the first violation.
func (v *FieldValidator) Validate(tv *pb.TypedValue) error {
	// Null check.
	if tv == nil || tv.Kind == nil {
		if v.nullable {
			return nil
		}
		return fmt.Errorf("field %s: value is required (not nullable)", v.fieldPath)
	}

	// Type check.
	if err := checkType(tv, v.fieldType); err != nil {
		return fmt.Errorf("field %s: %w", v.fieldPath, err)
	}

	// Constraint checks.
	for _, check := range v.checks {
		if err := check(tv); err != nil {
			return fmt.Errorf("field %s: %w", v.fieldPath, err)
		}
	}

	return nil
}

// NewFieldValidator creates a validator for a schema field.
func NewFieldValidator(fieldPath string, fieldType pb.FieldType, nullable bool, constraints *pb.FieldConstraints) *FieldValidator {
	v := &FieldValidator{
		fieldPath: fieldPath,
		fieldType: fieldType,
		nullable:  nullable,
	}

	// URL validity check is always applied (not constraint-dependent).
	if fieldType == pb.FieldType_FIELD_TYPE_URL {
		v.checks = append(v.checks, func(tv *pb.TypedValue) error {
			val := tv.Kind.(*pb.TypedValue_UrlValue).UrlValue
			u, err := url.Parse(val)
			if err != nil || !u.IsAbs() {
				return fmt.Errorf("value %q is not a valid absolute URL", val)
			}
			return nil
		})
	}

	if constraints == nil {
		return v
	}

	// Build constraint checks based on field type.
	switch fieldType {
	case pb.FieldType_FIELD_TYPE_INT:
		addNumericChecks(&v.checks, constraints, func(tv *pb.TypedValue) float64 {
			return float64(tv.Kind.(*pb.TypedValue_IntegerValue).IntegerValue)
		})

	case pb.FieldType_FIELD_TYPE_NUMBER:
		addNumericChecks(&v.checks, constraints, func(tv *pb.TypedValue) float64 {
			return tv.Kind.(*pb.TypedValue_NumberValue).NumberValue
		})

	case pb.FieldType_FIELD_TYPE_STRING:
		if constraints.MinLength != nil {
			min := int(*constraints.MinLength)
			v.checks = append(v.checks, func(tv *pb.TypedValue) error {
				val := tv.Kind.(*pb.TypedValue_StringValue).StringValue
				if len(val) < min {
					return fmt.Errorf("string length %d is less than minimum %d", len(val), min)
				}
				return nil
			})
		}
		if constraints.MaxLength != nil {
			max := int(*constraints.MaxLength)
			v.checks = append(v.checks, func(tv *pb.TypedValue) error {
				val := tv.Kind.(*pb.TypedValue_StringValue).StringValue
				if len(val) > max {
					return fmt.Errorf("string length %d is greater than maximum %d", len(val), max)
				}
				return nil
			})
		}
		if constraints.Regex != nil {
			re, err := regexp.Compile(*constraints.Regex)
			if err == nil {
				v.checks = append(v.checks, func(tv *pb.TypedValue) error {
					val := tv.Kind.(*pb.TypedValue_StringValue).StringValue
					if !re.MatchString(val) {
						return fmt.Errorf("value %q does not match pattern %s", val, re.String())
					}
					return nil
				})
			}
		}

	case pb.FieldType_FIELD_TYPE_DURATION:
		addNumericChecks(&v.checks, constraints, func(tv *pb.TypedValue) float64 {
			return tv.Kind.(*pb.TypedValue_DurationValue).DurationValue.AsDuration().Seconds()
		})

	case pb.FieldType_FIELD_TYPE_URL:
		// URL validity already checked above (unconditionally).

	case pb.FieldType_FIELD_TYPE_JSON:
		if constraints.JsonSchema != nil {
			jv, err := newJSONSchemaValidator(*constraints.JsonSchema)
			if err == nil {
				v.checks = append(v.checks, func(tv *pb.TypedValue) error {
					val := tv.Kind.(*pb.TypedValue_JsonValue).JsonValue
					return jv.validate(val)
				})
			}
		}
	}

	// Enum check applies to any type — compares string representation.
	if len(constraints.EnumValues) > 0 {
		allowed := make(map[string]struct{}, len(constraints.EnumValues))
		for _, e := range constraints.EnumValues {
			allowed[e] = struct{}{}
		}
		v.checks = append(v.checks, func(tv *pb.TypedValue) error {
			s := typedValueToString(tv)
			if _, ok := allowed[s]; !ok {
				return fmt.Errorf("value %q is not in allowed values %v", s, constraints.EnumValues)
			}
			return nil
		})
	}

	return v
}

// checkType verifies that a TypedValue matches the expected field type.
func checkType(tv *pb.TypedValue, expected pb.FieldType) error {
	switch expected {
	case pb.FieldType_FIELD_TYPE_INT:
		if _, ok := tv.Kind.(*pb.TypedValue_IntegerValue); !ok {
			return fmt.Errorf("expected integer value")
		}
	case pb.FieldType_FIELD_TYPE_NUMBER:
		if _, ok := tv.Kind.(*pb.TypedValue_NumberValue); !ok {
			return fmt.Errorf("expected number value")
		}
	case pb.FieldType_FIELD_TYPE_STRING:
		if _, ok := tv.Kind.(*pb.TypedValue_StringValue); !ok {
			return fmt.Errorf("expected string value")
		}
	case pb.FieldType_FIELD_TYPE_BOOL:
		if _, ok := tv.Kind.(*pb.TypedValue_BoolValue); !ok {
			return fmt.Errorf("expected bool value")
		}
	case pb.FieldType_FIELD_TYPE_TIME:
		if _, ok := tv.Kind.(*pb.TypedValue_TimeValue); !ok {
			return fmt.Errorf("expected time value")
		}
	case pb.FieldType_FIELD_TYPE_DURATION:
		if _, ok := tv.Kind.(*pb.TypedValue_DurationValue); !ok {
			return fmt.Errorf("expected duration value")
		}
	case pb.FieldType_FIELD_TYPE_URL:
		if _, ok := tv.Kind.(*pb.TypedValue_UrlValue); !ok {
			return fmt.Errorf("expected url value")
		}
	case pb.FieldType_FIELD_TYPE_JSON:
		if _, ok := tv.Kind.(*pb.TypedValue_JsonValue); !ok {
			return fmt.Errorf("expected json value")
		}
	}
	return nil
}

// addNumericChecks adds min/max and exclusiveMin/exclusiveMax checks for numeric types.
func addNumericChecks(checks *[]checkFunc, c *pb.FieldConstraints, extract func(*pb.TypedValue) float64) {
	if c.Min != nil {
		min := *c.Min
		*checks = append(*checks, func(tv *pb.TypedValue) error {
			if extract(tv) < min {
				return fmt.Errorf("value %v is less than minimum %v", extract(tv), min)
			}
			return nil
		})
	}
	if c.Max != nil {
		max := *c.Max
		*checks = append(*checks, func(tv *pb.TypedValue) error {
			if extract(tv) > max {
				return fmt.Errorf("value %v is greater than maximum %v", extract(tv), max)
			}
			return nil
		})
	}
	if c.ExclusiveMin != nil {
		emin := *c.ExclusiveMin
		*checks = append(*checks, func(tv *pb.TypedValue) error {
			if extract(tv) <= emin {
				return fmt.Errorf("value %v must be greater than %v", extract(tv), emin)
			}
			return nil
		})
	}
	if c.ExclusiveMax != nil {
		emax := *c.ExclusiveMax
		*checks = append(*checks, func(tv *pb.TypedValue) error {
			if extract(tv) >= emax {
				return fmt.Errorf("value %v must be less than %v", extract(tv), emax)
			}
			return nil
		})
	}
}

// typedValueToString extracts a string representation for enum comparison.
func typedValueToString(tv *pb.TypedValue) string {
	if tv == nil {
		return ""
	}
	switch v := tv.Kind.(type) {
	case *pb.TypedValue_IntegerValue:
		return fmt.Sprintf("%d", v.IntegerValue)
	case *pb.TypedValue_NumberValue:
		return fmt.Sprintf("%g", v.NumberValue)
	case *pb.TypedValue_StringValue:
		return v.StringValue
	case *pb.TypedValue_BoolValue:
		return fmt.Sprintf("%t", v.BoolValue)
	case *pb.TypedValue_UrlValue:
		return v.UrlValue
	case *pb.TypedValue_JsonValue:
		return v.JsonValue
	default:
		return ""
	}
}
