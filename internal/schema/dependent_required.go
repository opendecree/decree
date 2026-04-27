package schema

import (
	"encoding/json"
	"fmt"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// validateDependentRequiredAgainstFields lints a list of DependentRequiredEntry
// against the field set being imported: every trigger and every dependent
// must reference a real field, no trigger may list itself, and no dependent
// may appear twice under the same trigger. Mirrors the YAML-side check but
// runs over the proto representation, which is the form the rest of the
// import pipeline carries.
func validateDependentRequiredAgainstFields(entries []*pb.DependentRequiredEntry, fields []*pb.SchemaField) error {
	if len(entries) == 0 {
		return nil
	}
	known := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		known[f.Path] = struct{}{}
	}
	for _, e := range entries {
		if _, ok := known[e.TriggerField]; !ok {
			return fmt.Errorf("dependentRequired: trigger %q is not a defined field", e.TriggerField)
		}
		seen := make(map[string]struct{}, len(e.DependentFields))
		for _, dep := range e.DependentFields {
			if dep == e.TriggerField {
				return fmt.Errorf("dependentRequired: trigger %q cannot list itself as a dependent", e.TriggerField)
			}
			if _, ok := known[dep]; !ok {
				return fmt.Errorf("dependentRequired: dependent %q (under trigger %q) is not a defined field", dep, e.TriggerField)
			}
			if _, dup := seen[dep]; dup {
				return fmt.Errorf("dependentRequired: dependent %q listed twice under trigger %q", dep, e.TriggerField)
			}
			seen[dep] = struct{}{}
		}
	}
	return nil
}

// marshalDependentRequired encodes proto DependentRequiredEntry list as the
// JSON array stored in the schema_versions.dependent_required column. Always
// returns valid JSON — `[]` for empty input — so the column never holds NULL
// or junk.
func marshalDependentRequired(entries []*pb.DependentRequiredEntry) ([]byte, error) {
	if len(entries) == 0 {
		return []byte("[]"), nil
	}
	type wireEntry struct {
		TriggerField    string   `json:"trigger_field"`
		DependentFields []string `json:"dependent_fields"`
	}
	wire := make([]wireEntry, 0, len(entries))
	for _, e := range entries {
		wire = append(wire, wireEntry{
			TriggerField:    e.TriggerField,
			DependentFields: append([]string(nil), e.DependentFields...),
		})
	}
	return json.Marshal(wire)
}

// UnmarshalDependentRequired decodes the JSON-stored rules back into proto
// entries. Returns nil for empty / `[]` / unparseable input — callers should
// treat that case as "no rules". Exported so the config package can call it
// without re-inventing the wire format.
func UnmarshalDependentRequired(raw []byte) []*pb.DependentRequiredEntry {
	if len(raw) == 0 {
		return nil
	}
	type wireEntry struct {
		TriggerField    string   `json:"trigger_field"`
		DependentFields []string `json:"dependent_fields"`
	}
	var wire []wireEntry
	if err := json.Unmarshal(raw, &wire); err != nil || len(wire) == 0 {
		return nil
	}
	out := make([]*pb.DependentRequiredEntry, 0, len(wire))
	for _, w := range wire {
		out = append(out, &pb.DependentRequiredEntry{
			TriggerField:    w.TriggerField,
			DependentFields: w.DependentFields,
		})
	}
	return out
}

// CheckDependentRequired evaluates all rules against a post-merge value
// snapshot. For each rule, if the trigger field has a non-null value in the
// snapshot, every dependent path must also have a non-null value. Missing
// keys in the snapshot are treated as null. Returns the first violation
// encountered, formatted with both trigger and dependent paths so the
// caller's error message names the offending fields.
//
// Designed to run inside the same transaction that stages the write — the
// snapshot must include all in-flight changes already merged on top of the
// pre-tx state. Race-safety relies on Postgres MVCC + the caller's
// CreateConfigVersion UNIQUE(tenant_id, version) constraint to serialize
// concurrent writers.
func CheckDependentRequired(rules []*pb.DependentRequiredEntry, snapshot map[string]*pb.TypedValue) error {
	if len(rules) == 0 {
		return nil
	}
	for _, rule := range rules {
		tv, present := snapshot[rule.TriggerField]
		if !present || isNullTypedValue(tv) {
			continue
		}
		for _, dep := range rule.DependentFields {
			depTV, depPresent := snapshot[dep]
			if !depPresent || isNullTypedValue(depTV) {
				return fmt.Errorf("dependentRequired: %q has a value but required dependent %q is null", rule.TriggerField, dep)
			}
		}
	}
	return nil
}

// isNullTypedValue treats both a nil TypedValue and one with no kind set as
// "null". The wire protocol uses an unset oneof to mean null (per
// types.proto: "An unset oneof (no field present) represents a null value").
func isNullTypedValue(tv *pb.TypedValue) bool {
	return tv == nil || tv.Kind == nil
}
