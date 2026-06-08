package schema

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/cespare/xxhash/v2"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/validation"
)

// SeedTenantConfigParams describes the initial config to materialize for a
// freshly created tenant. Values maps field path to the canonical string form
// of the schema field's default. The store writes these as config version 1.
type SeedTenantConfigParams struct {
	TenantID string
	Actor    string
	// Values maps field path -> default value (canonical string form).
	// Must be non-empty; callers skip seeding entirely when there are no defaults.
	Values map[string]SeedValue
}

// SeedValue is a single default value plus the checksum of its canonical
// string form, precomputed so both store implementations agree on the digest.
type SeedValue struct {
	Value    string
	Checksum string
}

// collectDefaultValues extracts the fields that carry a default_value and
// returns them keyed by field path, with each default's checksum precomputed.
// Every default is validated against its field's type and constraints first:
// schema publish does not validate default_value, so an out-of-range or
// type-mismatched default would otherwise be silently seeded as invalid config.
// Returns a nil map when no field has a default (caller then skips seeding).
func collectDefaultValues(fields []domain.SchemaField) (map[string]SeedValue, error) {
	var out map[string]SeedValue
	for _, f := range fields {
		if f.DefaultValue == nil {
			continue
		}
		if err := validateDefaultValue(f); err != nil {
			return nil, err
		}
		if out == nil {
			out = make(map[string]SeedValue)
		}
		out[f.Path] = SeedValue{
			Value:    *f.DefaultValue,
			Checksum: configValueChecksum(*f.DefaultValue),
		}
	}
	return out, nil
}

// validateDefaultValue checks that a field's default_value is a legal value for
// the field's type and constraints. It mirrors the per-field validation the
// config service applies on writes, so a default can never seed a value that a
// subsequent SetField would reject.
func validateDefaultValue(f domain.SchemaField) error {
	if f.DefaultValue == nil {
		return nil
	}
	ft := f.FieldType.ToProto()

	var opts []validation.Option
	if f.Nullable {
		opts = append(opts, validation.WithNullable())
	}
	if f.Sensitive {
		opts = append(opts, validation.WithSensitive())
	}
	if len(f.Constraints) > 0 {
		var c pb.FieldConstraints
		if err := json.Unmarshal(f.Constraints, &c); err != nil {
			return fmt.Errorf("field %s: invalid stored constraints: %w", f.Path, err)
		}
		opts = append(opts, validation.WithConstraints(&c))
	}

	tv, err := defaultToTypedValue(*f.DefaultValue, f.FieldType)
	if err != nil {
		return fmt.Errorf("field %s: invalid default value: %w", f.Path, err)
	}

	v := validation.NewFieldValidator(f.Path, ft, opts...)
	if err := v.Validate(tv); err != nil {
		return fmt.Errorf("invalid default value: %w", err)
	}
	return nil
}

// defaultToTypedValue parses a default's canonical string form into the
// TypedValue variant for the field type. Unlike the lenient DB-read conversion
// in the config package, parse failures here are returned as errors so an
// unparseable default (e.g. "abc" for an integer field) is rejected at tenant
// creation rather than stored as a zero value.
func defaultToTypedValue(s string, ft domain.FieldType) (*pb.TypedValue, error) {
	switch ft {
	case domain.FieldTypeInteger:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("not an integer: %q", s)
		}
		return &pb.TypedValue{Kind: &pb.TypedValue_IntegerValue{IntegerValue: n}}, nil
	case domain.FieldTypeNumber:
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("not a number: %q", s)
		}
		return &pb.TypedValue{Kind: &pb.TypedValue_NumberValue{NumberValue: n}}, nil
	case domain.FieldTypeString:
		return &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: s}}, nil
	case domain.FieldTypeBool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("not a bool: %q", s)
		}
		return &pb.TypedValue{Kind: &pb.TypedValue_BoolValue{BoolValue: b}}, nil
	case domain.FieldTypeTime:
		t, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return nil, fmt.Errorf("not an RFC3339 time: %q", s)
		}
		return &pb.TypedValue{Kind: &pb.TypedValue_TimeValue{TimeValue: timestamppb.New(t)}}, nil
	case domain.FieldTypeDuration:
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("not a duration: %q", s)
		}
		return &pb.TypedValue{Kind: &pb.TypedValue_DurationValue{DurationValue: durationpb.New(d)}}, nil
	case domain.FieldTypeURL:
		return &pb.TypedValue{Kind: &pb.TypedValue_UrlValue{UrlValue: s}}, nil
	case domain.FieldTypeJSON:
		return &pb.TypedValue{Kind: &pb.TypedValue_JsonValue{JsonValue: s}}, nil
	default:
		return &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: s}}, nil
	}
}

// configValueChecksum computes the checksum for a config value's canonical
// string form. It must match the config store's scheme (xxHash64, hex) so a
// seeded value's checksum is indistinguishable from one written via SetField.
func configValueChecksum(value string) string {
	return strconv.FormatUint(xxhash.Sum64String(value), 16)
}
