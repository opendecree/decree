package cel

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// FailedRule names a CEL rule that returned false against the current
// activation. The Index field points back into the schema's validations
// array; Message and Reason are forwarded from the rule so callers can
// surface either at the gRPC boundary.
type FailedRule struct {
	Index   int
	Message string
	Reason  string
}

// Eval runs every program in turn against the activation and returns the
// rules that failed. Programs and rules must align by index — the caller
// builds them together in factory.GetCelArtifacts.
//
// Three outcome shapes:
//
//   - rule returns false      → appended to the failed slice.
//   - rule returns true       → dropped.
//   - rule errors at runtime  → appended to the soft-error slice (returned
//     separately so the caller can log without failing the write). The
//     common runtime error is comparison against an unset (null) field;
//     authors should not need to wrap every reference in `has()` to keep
//     unrelated writes from being rejected.
//
// Two terminal errors:
//
//   - cost-limit exceedance — abort the whole evaluation; surfaces as the
//     returned error so the caller can return InvalidArgument and
//     telemetry can flag a potential DoS attempt.
//   - length mismatch       — programmer bug; abort.
//
// nil/nil/nil means every rule held cleanly.
func Eval(programs []cel.Program, activation map[string]any, rules []*pb.ValidationRule) ([]FailedRule, []error, error) {
	if len(programs) == 0 {
		return nil, nil, nil
	}
	if len(programs) != len(rules) {
		return nil, nil, fmt.Errorf("cel: programs (%d) and rules (%d) length mismatch", len(programs), len(rules))
	}
	var (
		failed   []FailedRule
		softErrs []error
	)
	for i, prog := range programs {
		val, _, err := prog.Eval(activation)
		if err != nil {
			if isCostLimit(err) {
				return nil, softErrs, fmt.Errorf("cel: cost limit exceeded for validations[%d]: %w", i, err)
			}
			softErrs = append(softErrs, fmt.Errorf("validations[%d]: %w", i, err))
			continue
		}
		if val == nil || val.Type() != types.BoolType {
			softErrs = append(softErrs, fmt.Errorf("validations[%d]: rule did not evaluate to bool (got %v)", i, valType(val)))
			continue
		}
		if val.Value() == false {
			r := rules[i]
			failed = append(failed, FailedRule{
				Index:   i,
				Message: r.GetMessage(),
				Reason:  r.GetReason(),
			})
		}
	}
	return failed, softErrs, nil
}

func valType(v ref.Val) string {
	if v == nil {
		return "nil"
	}
	return v.Type().TypeName()
}

// isCostLimit returns true when the error message matches cel-go's
// cost-limit exhaustion signature. cel-go does not expose a sentinel error
// for this so we fall back to a substring match — fragile but isolated.
func isCostLimit(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "operation cancelled") ||
		strings.Contains(msg, "cost limit")
}
