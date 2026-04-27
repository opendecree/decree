package schema

import (
	"encoding/json"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// validationWireEntry is the JSON shape stored in the
// schema_versions.validations column. Field names match the proto wire
// names (snake_case) so an external tool that reads the column directly
// sees the same shape it would over the gRPC API.
type validationWireEntry struct {
	Path     string `json:"path,omitempty"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// MarshalValidations encodes proto ValidationRule entries as the JSON
// array stored in the schema_versions.validations column. Always returns
// valid JSON — `[]` for empty input — so the column never holds NULL or
// junk.
func MarshalValidations(entries []*pb.ValidationRule) ([]byte, error) {
	if len(entries) == 0 {
		return []byte("[]"), nil
	}
	wire := make([]validationWireEntry, 0, len(entries))
	for _, e := range entries {
		wire = append(wire, validationWireEntry{
			Path:     e.Path,
			Rule:     e.Rule,
			Message:  e.Message,
			Severity: e.Severity,
			Reason:   e.Reason,
		})
	}
	return json.Marshal(wire)
}

// UnmarshalValidations decodes the JSON-stored rules back into proto
// entries. Returns nil for empty / `[]` / unparseable input — callers
// should treat that as "no rules". Exported so tooling can decode the
// column without re-inventing the wire format.
func UnmarshalValidations(raw []byte) []*pb.ValidationRule {
	if len(raw) == 0 {
		return nil
	}
	var wire []validationWireEntry
	if err := json.Unmarshal(raw, &wire); err != nil || len(wire) == 0 {
		return nil
	}
	out := make([]*pb.ValidationRule, 0, len(wire))
	for _, w := range wire {
		out = append(out, &pb.ValidationRule{
			Path:     w.Path,
			Rule:     w.Rule,
			Message:  w.Message,
			Severity: w.Severity,
			Reason:   w.Reason,
		})
	}
	return out
}
