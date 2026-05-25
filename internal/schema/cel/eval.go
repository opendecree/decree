package cel

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// EvalOption configures optional behaviour for Eval. All options are
// additive and backward-compatible; callers that pass no options get the
// same behaviour as before this feature was added.
type EvalOption func(*evalConfig)

type evalConfig struct {
	capCounter metric.Int64Counter
	tenantID   string
}

// WithCapCounter wires an OTEL counter that is incremented (with the
// given tenantID attribute) whenever the aggregate cost cap fires.
// Pass nil to disable (no-op — equivalent to omitting the option).
func WithCapCounter(c metric.Int64Counter, tenantID string) EvalOption {
	return func(cfg *evalConfig) {
		cfg.capCounter = c
		cfg.tenantID = tenantID
	}
}

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
// Three terminal errors:
//
//   - per-rule cost-limit exceedance  — abort; caller returns InvalidArgument.
//   - aggregate cost-limit exceedance — abort after summing cost across all
//     rules so far; bounds total cost when a tenant has many rules.
//   - length mismatch                 — programmer bug; abort.
//
// nil/nil/nil means every rule held cleanly.
func Eval(programs []cel.Program, activation map[string]any, rules []*pb.ValidationRule, opts ...EvalOption) ([]FailedRule, []error, error) {
	if len(programs) == 0 {
		return nil, nil, nil
	}
	if len(programs) != len(rules) {
		return nil, nil, fmt.Errorf("cel: programs (%d) and rules (%d) length mismatch", len(programs), len(rules))
	}
	cfg := &evalConfig{}
	for _, o := range opts {
		o(cfg)
	}
	cap := aggregateCostCap()
	var (
		failed        []FailedRule
		softErrs      []error
		aggregateCost uint64
	)
	for i, prog := range programs {
		val, details, err := prog.Eval(activation)
		if err != nil {
			if isCostLimit(err) {
				return nil, softErrs, fmt.Errorf("cel: cost limit exceeded for validations[%d]: %w", i, err)
			}
			softErrs = append(softErrs, fmt.Errorf("validations[%d]: %w", i, err))
			continue
		}
		if details != nil {
			if c := details.ActualCost(); c != nil {
				aggregateCost += *c
				if aggregateCost > cap {
					if cfg.capCounter != nil {
						cfg.capCounter.Add(context.Background(), 1, metric.WithAttributes(
							attribute.String("tenant_id", cfg.tenantID),
						))
					}
					return nil, softErrs, fmt.Errorf("cel: aggregate cost limit exceeded (cost %d > cap %d)", aggregateCost, cap)
				}
			}
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
