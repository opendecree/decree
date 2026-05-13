package cel

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// SnapshotRow is one (path, value) pair from the post-merge config snapshot.
// The runtime layer passes its store-row equivalents through this shape so
// the cel package stays decoupled from internal/config types.
type SnapshotRow struct {
	FieldPath string
	Value     *string
}

// TenantBinding carries the subset of tenant metadata exposed via the
// `tenant.*` CEL binding. Use empty strings for missing values; the rule
// will see them as empty strings rather than CEL null, which is consistent
// with the brief.
type TenantBinding struct {
	ID   string
	Name string
}

// BuildActivation returns the activation map handed to cel.Program.Eval.
// The returned map has two top-level keys:
//
//   - self   — nested map keyed by dotted-path segments. Null/absent fields
//     surface as the CEL `null` literal (Go nil) so authors can write
//     `has(self.x)` or `self.x == null` interchangeably.
//   - tenant — flat `{id, name}` map of strings.
//
// types must cover every path that appears in rows; unknown paths fall back
// to a raw-string value, mirroring stringToTypedValue's default branch.
func BuildActivation(rows []SnapshotRow, types map[string]pb.FieldType, tenant TenantBinding) map[string]any {
	self := make(map[string]any)
	for _, row := range rows {
		ft := types[row.FieldPath]
		val := stringToCelNative(row.Value, ft)
		setNested(self, strings.Split(row.FieldPath, "."), val)
	}
	return map[string]any{
		"self": self,
		"tenant": map[string]any{
			"id":   tenant.ID,
			"name": tenant.Name,
		},
	}
}

// stringToCelNative parses the stored string into the Go type cel-go expects
// for the field's declared CEL counterpart. Nil string → nil (CEL null).
// Parse failures fall back to the original string; runtime evaluation will
// fail loudly rather than silently coerce, which is the correct behaviour
// when stored data does not match the declared type.
func stringToCelNative(s *string, ft pb.FieldType) any {
	if s == nil {
		return nil
	}
	switch ft {
	case pb.FieldType_FIELD_TYPE_INT:
		v, err := strconv.ParseInt(*s, 10, 64)
		if err != nil {
			return *s
		}
		return v
	case pb.FieldType_FIELD_TYPE_NUMBER:
		v, err := strconv.ParseFloat(*s, 64)
		if err != nil {
			return *s
		}
		return v
	case pb.FieldType_FIELD_TYPE_BOOL:
		v, err := strconv.ParseBool(*s)
		if err != nil {
			return *s
		}
		return v
	case pb.FieldType_FIELD_TYPE_TIME:
		v, err := time.Parse(time.RFC3339Nano, *s)
		if err != nil {
			return *s
		}
		return v
	case pb.FieldType_FIELD_TYPE_DURATION:
		v, err := time.ParseDuration(*s)
		if err != nil {
			return *s
		}
		return v
	case pb.FieldType_FIELD_TYPE_JSON:
		var v any
		if err := json.Unmarshal([]byte(*s), &v); err != nil {
			return *s
		}
		return v
	default:
		// string, url, unspecified
		return *s
	}
}

// setNested walks segments into m, creating intermediate maps when needed,
// and stores leaf at the terminal segment. Existing leaves are overwritten;
// because the schema's prefix-overlap lint rejects collisions, no path will
// ever try to overwrite an intermediate map with a leaf.
func setNested(m map[string]any, segments []string, leaf any) {
	if len(segments) == 0 {
		return
	}
	for i := 0; i < len(segments)-1; i++ {
		seg := segments[i]
		next, ok := m[seg].(map[string]any)
		if !ok {
			next = make(map[string]any)
			m[seg] = next
		}
		m = next
	}
	m[segments[len(segments)-1]] = leaf
}
