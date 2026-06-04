package cel

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
)

// checkRuleTypes walks a compiled rule's AST and reports type mismatches that
// the dyn-typed env cannot catch at compile time but that fail at evaluation
// time with an opaque "no such overload" error.
//
// The check deliberately mirrors cel-go's *runtime* coercion rules rather than
// its (stricter) static checker, so a rule lints clean iff it also evaluates
// without a type error:
//
//   - numeric operands (int/uint/double) cross-compare and cross-combine, since
//     the runtime promotes them — `self.i < self.n` (int vs number) is allowed;
//   - `==` / `!=` are heterogeneous and never flagged;
//   - genuinely incompatible operands (number vs string, bool in arithmetic,
//     string methods on a non-string) are rejected with the offending field
//     path named.
//
// leaves maps each declared field path to its CEL leaf type (built by
// selfDescriptor). A `self.<path>` chain that resolves to a parent prefix or to
// an unknown path is treated as dyn here — unknown-path resolution is lint
// rule 3's job and is reported separately, so this pass never double-reports.
//
// False negatives are acceptable (a missed mismatch surfaces at runtime, the
// pre-existing behaviour); false positives are not — anything not positively
// modelled infers as dyn and is never rejected.
func checkRuleTypes(root ast.Expr, leaves map[string]*cel.Type) []error {
	var errs []error
	inferType(root, leaves, func(msg string) {
		errs = append(errs, fmt.Errorf("%s", msg))
	})
	return errs
}

// inferType returns a best-effort CEL type for e and reports any modelled type
// mismatch through addErr. On a detected mismatch it still returns a plausible
// result type so a single error does not cascade into spurious follow-ons.
func inferType(e ast.Expr, leaves map[string]*cel.Type, addErr func(string)) *cel.Type {
	if e == nil {
		return cel.DynType
	}
	switch e.Kind() {
	case ast.LiteralKind:
		return literalType(e.AsLiteral())
	case ast.SelectKind:
		sel := e.AsSelect()
		if sel.IsTestOnly() { // has(self.x)
			return cel.BoolType
		}
		if chain, ok := selfChainOf(e); ok && len(chain) > 0 {
			if t, found := leaves[strings.Join(chain, ".")]; found {
				return t
			}
			return cel.DynType // parent prefix or unknown path (rule 3 handles)
		}
		if base, ok := identName(sel.Operand()); ok && base == "tenant" {
			return cel.StringType // tenant is map<string,string>
		}
		// Walk the operand so nested errors are still surfaced.
		inferType(sel.Operand(), leaves, addErr)
		return cel.DynType
	case ast.CallKind:
		return inferCall(e.AsCall(), leaves, addErr)
	case ast.ComprehensionKind:
		c := e.AsComprehension()
		// Bound variables are untyped to this pass; walk sub-expressions for
		// nested errors but do not type the comprehension result.
		inferType(c.IterRange(), leaves, addErr)
		inferType(c.LoopCondition(), leaves, addErr)
		inferType(c.LoopStep(), leaves, addErr)
		inferType(c.Result(), leaves, addErr)
		return cel.DynType
	case ast.ListKind:
		for _, el := range e.AsList().Elements() {
			inferType(el, leaves, addErr)
		}
		return cel.DynType
	case ast.MapKind:
		for _, entry := range e.AsMap().Entries() {
			m := entry.AsMapEntry()
			inferType(m.Key(), leaves, addErr)
			inferType(m.Value(), leaves, addErr)
		}
		return cel.DynType
	default:
		return cel.DynType
	}
}

func inferCall(call ast.CallExpr, leaves map[string]*cel.Type, addErr func(string)) *cel.Type {
	fn := call.FunctionName()
	args := call.Args()

	if call.IsMemberFunction() {
		recv := inferType(call.Target(), leaves, addErr)
		for _, a := range args {
			inferType(a, leaves, addErr)
		}
		switch fn {
		case "startsWith", "endsWith", "matches":
			if isConcrete(recv) && recv.Kind() != types.StringKind {
				addErr(fmt.Sprintf("%s() requires a string receiver, got %s",
					fn, describe(call.Target(), recv)))
			}
			return cel.BoolType
		}
		return cel.DynType
	}

	switch fn {
	case "_<_", "_<=_", "_>_", "_>=_":
		l := inferType(args[0], leaves, addErr)
		r := inferType(args[1], leaves, addErr)
		if comparableMismatch(l, r) {
			addErr(fmt.Sprintf("cannot order %s with %s",
				describe(args[0], l), describe(args[1], r)))
		}
		return cel.BoolType
	case "_==_", "_!=_":
		// Heterogeneous equality is valid in CEL; only walk for nested errors.
		inferType(args[0], leaves, addErr)
		inferType(args[1], leaves, addErr)
		return cel.BoolType
	case "_&&_", "_||_":
		for _, a := range args {
			t := inferType(a, leaves, addErr)
			if isConcrete(t) && t.Kind() != types.BoolKind {
				addErr(fmt.Sprintf("logical operand must be bool, got %s", describe(a, t)))
			}
		}
		return cel.BoolType
	case "!_":
		t := inferType(args[0], leaves, addErr)
		if isConcrete(t) && t.Kind() != types.BoolKind {
			addErr(fmt.Sprintf("logical operand must be bool, got %s", describe(args[0], t)))
		}
		return cel.BoolType
	case "_?_:_":
		cond := inferType(args[0], leaves, addErr)
		if isConcrete(cond) && cond.Kind() != types.BoolKind {
			addErr(fmt.Sprintf("ternary condition must be bool, got %s", describe(args[0], cond)))
		}
		return join(inferType(args[1], leaves, addErr), inferType(args[2], leaves, addErr))
	case "_+_":
		l := inferType(args[0], leaves, addErr)
		r := inferType(args[1], leaves, addErr)
		if arithMismatch(l, r, true) {
			addErr(fmt.Sprintf("cannot add %s and %s", describe(args[0], l), describe(args[1], r)))
		}
		return arithResult(l, r)
	case "_-_", "_*_", "_/_", "_%_":
		l := inferType(args[0], leaves, addErr)
		r := inferType(args[1], leaves, addErr)
		if arithMismatch(l, r, false) {
			addErr(fmt.Sprintf("cannot apply %s to %s and %s",
				strings.Trim(fn, "_"), describe(args[0], l), describe(args[1], r)))
		}
		return arithResult(l, r)
	case "-_":
		t := inferType(args[0], leaves, addErr)
		if isConcrete(t) && !isNumeric(t) {
			addErr(fmt.Sprintf("cannot negate %s", describe(args[0], t)))
		}
		return t
	default:
		// Unknown function or index (`_[_]`): walk args, do not model the result.
		for _, a := range args {
			inferType(a, leaves, addErr)
		}
		return cel.DynType
	}
}

// isConcrete reports whether t is a scalar leaf type this pass reasons about.
// dyn, map, list, and anything else are non-concrete and never trigger an error.
func isConcrete(t *cel.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case types.IntKind, types.UintKind, types.DoubleKind,
		types.StringKind, types.BoolKind, types.BytesKind,
		types.DurationKind, types.TimestampKind:
		return true
	default:
		return false
	}
}

func isNumeric(t *cel.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case types.IntKind, types.UintKind, types.DoubleKind:
		return true
	default:
		return false
	}
}

// comparableMismatch reports whether l and r cannot be ordered by < <= > >=.
// Both must be concrete to flag; numeric cross-type is allowed (runtime
// promotes); otherwise the kinds must match and be an ordered type.
func comparableMismatch(l, r *cel.Type) bool {
	if !isConcrete(l) || !isConcrete(r) {
		return false
	}
	if isNumeric(l) && isNumeric(r) {
		return false
	}
	if l.Kind() != r.Kind() {
		return true
	}
	switch l.Kind() {
	case types.StringKind, types.BytesKind, types.DurationKind, types.TimestampKind:
		return false // same ordered type
	default:
		return true // e.g. bool < bool has no overload
	}
}

// arithMismatch reports whether l and r cannot combine under an arithmetic op.
// Numeric cross-type is fine; `+` also allows string+string and bytes+bytes;
// any duration/timestamp operand is left unmodelled (returns false) to avoid
// false positives on temporal arithmetic.
func arithMismatch(l, r *cel.Type, isPlus bool) bool {
	if !isConcrete(l) || !isConcrete(r) {
		return false
	}
	if isTemporal(l) || isTemporal(r) {
		return false
	}
	if isNumeric(l) && isNumeric(r) {
		return false
	}
	if isPlus && l.Kind() == types.StringKind && r.Kind() == types.StringKind {
		return false
	}
	if isPlus && l.Kind() == types.BytesKind && r.Kind() == types.BytesKind {
		return false
	}
	return true
}

func isTemporal(t *cel.Type) bool {
	return t != nil && (t.Kind() == types.DurationKind || t.Kind() == types.TimestampKind)
}

// arithResult returns the numeric result type when both operands are numeric so
// downstream checks (e.g. `(self.i + self.j) < self.s`) still see a number;
// anything else collapses to dyn.
func arithResult(l, r *cel.Type) *cel.Type {
	if isNumeric(l) && isNumeric(r) {
		if l.Kind() == types.DoubleKind || r.Kind() == types.DoubleKind {
			return cel.DoubleType
		}
		if l.Kind() == types.UintKind && r.Kind() == types.UintKind {
			return cel.UintType
		}
		return cel.IntType
	}
	return cel.DynType
}

func join(a, b *cel.Type) *cel.Type {
	if a != nil && b != nil && a.Kind() == b.Kind() {
		return a
	}
	return cel.DynType
}

func literalType(v interface{ Value() any }) *cel.Type {
	if v == nil {
		return cel.DynType
	}
	switch v.Value().(type) {
	case int64:
		return cel.IntType
	case uint64:
		return cel.UintType
	case float64:
		return cel.DoubleType
	case string:
		return cel.StringType
	case bool:
		return cel.BoolType
	case []byte:
		return cel.BytesType
	default:
		return cel.DynType
	}
}

// describe renders an operand as "self.path (kind)" when it is a field chain,
// otherwise just "(kind)", so error messages point the schema author at the
// offending field.
func describe(e ast.Expr, t *cel.Type) string {
	if chain, ok := selfChainOf(e); ok && len(chain) > 0 {
		return fmt.Sprintf("self.%s (%s)", strings.Join(chain, "."), kindName(t))
	}
	return kindName(t)
}

func kindName(t *cel.Type) string {
	if t == nil {
		return "dyn"
	}
	return t.String()
}

func identName(e ast.Expr) (string, bool) {
	if e != nil && e.Kind() == ast.IdentKind {
		return e.AsIdent(), true
	}
	return "", false
}
