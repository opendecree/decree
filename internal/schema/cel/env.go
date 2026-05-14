// Package cel builds CEL environments and compiles CEL validation rules for
// the schema service. The exported surface is intentionally small: callers
// pass a schema's field set in, get back a *cel.Env that subsequent compile
// and evaluation passes use.
//
// Phase 2 ships `self` as cel.DynType. cel-go's typed env declares top-level
// variables only; nested-map field-path typing would require a custom
// cel.TypeProvider. Lint rule 3 (AST-walk path resolution) compensates by
// catching unknown self.<path> chains at ImportSchema time without that
// extra surface.
package cel

import (
	"github.com/google/cel-go/cel"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// BuildEnv constructs the cel.Env that compile and evaluation passes share.
// The env exposes two bindings:
//
//   - self  — dyn; the post-merge config snapshot as a nested object keyed
//     by dotted field path. Typed access is enforced by lint rule 3, not by
//     cel-go's type checker.
//   - tenant — map<string, string>; carries id and name for rules that need
//     tenant context.
//
// fields is accepted for forward-compatibility with a future typed env that
// declares each field path's leaf type. v0.1.0 ignores the parameter at env
// construction; selfDescriptor builds the lookup table that downstream lint
// passes consume.
func BuildEnv(fields []*pb.SchemaField) (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("self", cel.DynType),
		cel.Variable("tenant", cel.MapType(cel.StringType, cel.StringType)),
	)
}

// celTypeFor maps a decree field type to its CEL counterpart per the design
// brief's type-mapping table. URL collapses to string (CEL has no URL type);
// JSON collapses to dyn (escape hatch, discouraged in docs).
func celTypeFor(ft pb.FieldType) *cel.Type {
	switch ft {
	case pb.FieldType_FIELD_TYPE_INT:
		return cel.IntType
	case pb.FieldType_FIELD_TYPE_NUMBER:
		return cel.DoubleType
	case pb.FieldType_FIELD_TYPE_STRING, pb.FieldType_FIELD_TYPE_URL:
		return cel.StringType
	case pb.FieldType_FIELD_TYPE_BOOL:
		return cel.BoolType
	case pb.FieldType_FIELD_TYPE_TIME:
		return cel.TimestampType
	case pb.FieldType_FIELD_TYPE_DURATION:
		return cel.DurationType
	default:
		return cel.DynType
	}
}

// selfDescriptor returns the field-path → CEL leaf-type map that lint rule 3
// uses to resolve self.<path> chains. Built once per schema import; the
// returned map is read-only for callers.
func selfDescriptor(fields []*pb.SchemaField) map[string]*cel.Type {
	descriptor := make(map[string]*cel.Type, len(fields))
	for _, f := range fields {
		descriptor[f.GetPath()] = celTypeFor(f.GetType())
	}
	return descriptor
}
