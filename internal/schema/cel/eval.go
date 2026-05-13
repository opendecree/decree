package cel

import (
	"errors"
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
// Runtime errors fall into two buckets:
//
//   - cost-limit exceedance — a contrived rule trips
//     cel.CostLimit; surfaces as a separate error rather than
//     a normal rule failure so operators can distinguish DoS attempts.
//   - evaluation errors (type mismatch on a path lint should have caught,
//     missing keys in `dyn` traversal, etc.) — joined and returned so the
//     write path can decide between InvalidArgument and Internal.
//
// nil/nil means every rule held.
func Eval(programs []cel.Program, activation map[string]any, rules []*pb.ValidationRule) ([]FailedRule, error) {
	if len(programs) == 0 {
		return nil, nil
	}
	if len(programs) != len(rules) {
		return nil, fmt.Errorf("cel: programs (%d) and rules (%d) length mismatch", len(programs), len(rules))
	}
	var (
		failed   []FailedRule
		evalErrs []error
	)
	for i, prog := range programs {
		val, _, err := prog.Eval(activation)
		if err != nil {
			if isCostLimit(err) {
				return nil, fmt.Errorf("cel: cost limit exceeded for validations[%d]: %w", i, err)
			}
			evalErrs = append(evalErrs, fmt.Errorf("validations[%d]: %w", i, err))
			continue
		}
		if val == nil || val.Type() != types.BoolType {
			evalErrs = append(evalErrs, fmt.Errorf("validations[%d]: rule did not evaluate to bool (got %v)", i, valType(val)))
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
	if len(evalErrs) > 0 {
		return failed, errors.Join(evalErrs...)
	}
	return failed, nil
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
