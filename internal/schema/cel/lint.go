package cel

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// LintValidations runs every lint rule over every entry in rules and returns
// the aggregated set of failures. Each rule is checked independently; one
// rule's failure does not short-circuit the rest. The returned error wraps
// every per-rule error via errors.Join so callers receive a single multi-line
// message at the gRPC boundary.
//
// Lint rules (all blocking at ImportSchema time):
//
//  1. cel.Compile succeeds; syntax errors surface with line and column.
//  2. The compiled AST references at least one self.* path; pure-constant
//     and tenant-only rules are rejected.
//  3. Every self.<path> chain resolves to a declared field path (with
//     intermediate prefixes tolerated when their full path is a parent of
//     a real leaf).
//  4. The rule cannot be expressed by a native constraint or
//     dependentRequired entry; if it can, the suggested keyword is named.
func LintValidations(rules []*pb.ValidationRule, fields []*pb.SchemaField) error {
	if len(rules) == 0 {
		return nil
	}
	env, err := BuildEnv(fields)
	if err != nil {
		return fmt.Errorf("build CEL env: %w", err)
	}
	leafPaths, parentPaths := buildPathSets(fields)

	var errs []error
	for i, r := range rules {
		if err := lintOne(env, leafPaths, parentPaths, r); err != nil {
			errs = append(errs, fmt.Errorf("validations[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

func lintOne(env *cel.Env, leaves, parents map[string]struct{}, r *pb.ValidationRule) error {
	celAst, issues := env.Compile(r.GetRule())
	if issues != nil && issues.Err() != nil {
		return issues.Err()
	}

	root := celAst.NativeRep().Expr()
	chains := collectSelfChains(root)

	if len(chains) == 0 {
		return fmt.Errorf("rule must reference at least one self.* field")
	}

	var resolutionErrs []error
	for _, c := range chains {
		path := strings.Join(c, ".")
		if _, ok := leaves[path]; ok {
			continue
		}
		if _, ok := parents[path]; ok {
			continue
		}
		resolutionErrs = append(resolutionErrs, fmt.Errorf("self.%s does not resolve to a declared field", path))
	}

	if suggestion, sub := patternSubstitutable(root); sub {
		resolutionErrs = append(resolutionErrs, fmt.Errorf("rule is expressible with a native constraint — %s", suggestion))
	}

	return errors.Join(resolutionErrs...)
}

// buildPathSets returns two sets: leaf paths (exact field paths declared on
// the schema) and parent paths (every dotted prefix of a leaf path).
// Intermediate self.foo accesses where foo.bar is a declared leaf are
// tolerated so that rules like `has(self.foo) || self.foo.bar > 0` lint
// cleanly.
func buildPathSets(fields []*pb.SchemaField) (leaves, parents map[string]struct{}) {
	leaves = make(map[string]struct{}, len(fields))
	parents = make(map[string]struct{}, len(fields))
	for _, f := range fields {
		p := f.GetPath()
		leaves[p] = struct{}{}
		segments := strings.Split(p, ".")
		for i := 1; i < len(segments); i++ {
			parents[strings.Join(segments[:i], ".")] = struct{}{}
		}
	}
	return leaves, parents
}

// collectSelfChains walks the AST and returns every dotted-path chain rooted
// at the self identifier. Both `self.x.y` (Select chains) and `self["x"].y`
// (Index plus Select) collapse to ["x", "y"]. Chains that terminate in a
// non-self operand are ignored. The bare `self` identifier (an empty chain)
// is filtered out — buildPathSets cannot resolve it and a rule that does
// nothing but reference `self` itself is rejected by rule 2's empty-chain
// count check.
func collectSelfChains(root ast.Expr) [][]string {
	var chains [][]string
	visit(root, func(e ast.Expr) {
		if chain, ok := selfChainOf(e); ok && len(chain) > 0 {
			chains = append(chains, chain)
		}
	})
	dedupe := chains[:0]
	seen := make(map[string]struct{}, len(chains))
	for _, c := range chains {
		key := strings.Join(c, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dedupe = append(dedupe, c)
	}
	sort.Slice(dedupe, func(i, j int) bool {
		return strings.Join(dedupe[i], ".") < strings.Join(dedupe[j], ".")
	})
	return dedupe
}

// selfChainOf returns the dotted path of a Select/Index chain rooted at
// `self`, or false when the expression is not such a chain. The shallowest
// expression rooted at `self.x` returns ["x"]; nested chains return the full
// path. Only the *outermost* self chain is returned per call site — the
// recursive walker emits intermediate chains too, which is fine because
// `buildPathSets` accepts parents as resolved paths.
func selfChainOf(e ast.Expr) ([]string, bool) {
	if e == nil {
		return nil, false
	}
	switch e.Kind() {
	case ast.SelectKind:
		sel := e.AsSelect()
		base, ok := selfChainOf(sel.Operand())
		if !ok {
			return nil, false
		}
		return append(base, sel.FieldName()), true
	case ast.CallKind:
		call := e.AsCall()
		if call.FunctionName() != "_[_]" || len(call.Args()) != 2 {
			return nil, false
		}
		base, ok := selfChainOf(call.Args()[0])
		if !ok {
			return nil, false
		}
		idx := call.Args()[1]
		if idx.Kind() != ast.LiteralKind {
			return nil, false
		}
		if str, isStr := stringLiteral(idx.AsLiteral()); isStr {
			return append(base, str), true
		}
		return nil, false
	case ast.IdentKind:
		if e.AsIdent() == "self" {
			return []string{}, true
		}
	}
	return nil, false
}

// patternSubstitutable returns a suggestion describing the native constraint
// that subsumes the rule, when the rule's AST matches one of the well-known
// substitutable shapes. False-negatives are accepted; false-positives are
// not — a genuine cross-field rule must never match.
func patternSubstitutable(root ast.Expr) (string, bool) {
	if root.Kind() != ast.CallKind {
		return "", false
	}
	call := root.AsCall()
	args := call.Args()
	switch call.FunctionName() {
	case "_<_", "_<=_", "_>_", "_>=_":
		return suggestComparisonAgainstLiteral(call.FunctionName(), args)
	case "_==_":
		return suggestEqualityLiteral(args)
	case "_||_":
		if s, ok := suggestEnumChain(args); ok {
			return s, true
		}
		if s, ok := suggestDependentRequiredImplication(args); ok {
			return s, true
		}
	case "_&&_":
		return suggestBoundedRange(args)
	}
	return "", false
}

func suggestComparisonAgainstLiteral(fn string, args []ast.Expr) (string, bool) {
	if len(args) != 2 {
		return "", false
	}
	_, leftIsSelf := singleSelfChain(args[0])
	rightLit, rightIsLit := constLiteral(args[1])
	leftLit, leftIsLit := constLiteral(args[0])
	_, rightIsSelf := singleSelfChain(args[1])

	if leftIsSelf && rightIsLit {
		return comparisonSuggestion(fn, rightLit, false), true
	}
	if rightIsSelf && leftIsLit {
		return comparisonSuggestion(fn, leftLit, true), true
	}
	return "", false
}

func comparisonSuggestion(fn, literal string, flipped bool) string {
	op := fn
	if flipped {
		op = flipOp(fn)
	}
	switch op {
	case "_>_":
		return fmt.Sprintf("use exclusiveMinimum: %s on the field", literal)
	case "_>=_":
		return fmt.Sprintf("use minimum: %s on the field", literal)
	case "_<_":
		return fmt.Sprintf("use exclusiveMaximum: %s on the field", literal)
	case "_<=_":
		return fmt.Sprintf("use maximum: %s on the field", literal)
	}
	return "use a native bound on the field"
}

func flipOp(fn string) string {
	switch fn {
	case "_<_":
		return "_>_"
	case "_<=_":
		return "_>=_"
	case "_>_":
		return "_<_"
	case "_>=_":
		return "_<=_"
	}
	return fn
}

func suggestEqualityLiteral(args []ast.Expr) (string, bool) {
	if len(args) != 2 {
		return "", false
	}
	if _, ok := singleSelfChain(args[0]); ok {
		if lit, isLit := constLiteral(args[1]); isLit {
			return fmt.Sprintf("use enum: [%s] on the field", lit), true
		}
	}
	if _, ok := singleSelfChain(args[1]); ok {
		if lit, isLit := constLiteral(args[0]); isLit {
			return fmt.Sprintf("use enum: [%s] on the field", lit), true
		}
	}
	return "", false
}

// suggestEnumChain matches `self.x == "a" || self.x == "b" || ...` —
// every leaf of the OR tree must be `self.x == literal` with the same self
// path on the left or right side.
func suggestEnumChain(args []ast.Expr) (string, bool) {
	if len(args) != 2 {
		return "", false
	}
	var path string
	var values []string
	if !collectEqualityLeaves(args[0], &path, &values) {
		return "", false
	}
	if !collectEqualityLeaves(args[1], &path, &values) {
		return "", false
	}
	if path == "" || len(values) < 2 {
		return "", false
	}
	return fmt.Sprintf("use enum: [%s] on field %s", strings.Join(values, ", "), path), true
}

func collectEqualityLeaves(e ast.Expr, path *string, values *[]string) bool {
	if e.Kind() != ast.CallKind {
		return false
	}
	call := e.AsCall()
	if call.FunctionName() == "_||_" {
		args := call.Args()
		if len(args) != 2 {
			return false
		}
		return collectEqualityLeaves(args[0], path, values) && collectEqualityLeaves(args[1], path, values)
	}
	if call.FunctionName() != "_==_" {
		return false
	}
	args := call.Args()
	if len(args) != 2 {
		return false
	}
	chain, lit, ok := equalityComponents(args)
	if !ok {
		return false
	}
	got := strings.Join(chain, ".")
	if *path != "" && *path != got {
		return false
	}
	*path = got
	*values = append(*values, lit)
	return true
}

func equalityComponents(args []ast.Expr) (chain []string, literal string, ok bool) {
	if c, isSelf := singleSelfChain(args[0]); isSelf {
		if lit, isLit := constLiteral(args[1]); isLit {
			return c, lit, true
		}
	}
	if c, isSelf := singleSelfChain(args[1]); isSelf {
		if lit, isLit := constLiteral(args[0]); isLit {
			return c, lit, true
		}
	}
	return nil, "", false
}

// suggestDependentRequiredImplication matches `!has(self.a) || has(self.b)`
// (the CEL desugaring of `has(self.a) -> has(self.b)`) and suggests the
// equivalent dependentRequired entry.
func suggestDependentRequiredImplication(args []ast.Expr) (string, bool) {
	if len(args) != 2 {
		return "", false
	}
	trigger, ok := negatedHasSelf(args[0])
	if !ok {
		return "", false
	}
	dependent, ok := hasSelf(args[1])
	if !ok {
		return "", false
	}
	return fmt.Sprintf("use dependentRequired: { %s: [%s] }", trigger, dependent), true
}

func negatedHasSelf(e ast.Expr) (string, bool) {
	if e.Kind() != ast.CallKind {
		return "", false
	}
	call := e.AsCall()
	if call.FunctionName() != "!_" || len(call.Args()) != 1 {
		return "", false
	}
	return hasSelf(call.Args()[0])
}

// hasSelf matches the `has(self.<path>)` macro. cel-go's parser desugars the
// macro into a SelectKind with IsTestOnly()=true, so we detect that flag
// rather than searching for a CallKind named "has".
func hasSelf(e ast.Expr) (string, bool) {
	if e.Kind() != ast.SelectKind {
		return "", false
	}
	sel := e.AsSelect()
	if !sel.IsTestOnly() {
		return "", false
	}
	chain, ok := selfChainOf(e)
	if !ok || len(chain) == 0 {
		return "", false
	}
	return strings.Join(chain, "."), true
}

// suggestBoundedRange matches `lo <op> self.x && self.x <op> hi` with both
// operands referencing the same self path. Either side may carry the lower
// bound.
func suggestBoundedRange(args []ast.Expr) (string, bool) {
	if len(args) != 2 {
		return "", false
	}
	leftChain, leftLit, leftOk := boundComponents(args[0])
	rightChain, rightLit, rightOk := boundComponents(args[1])
	if !leftOk || !rightOk {
		return "", false
	}
	if strings.Join(leftChain, ".") != strings.Join(rightChain, ".") {
		return "", false
	}
	return fmt.Sprintf("use minimum: %s and maximum: %s on field %s", leftLit, rightLit, strings.Join(leftChain, ".")), true
}

func boundComponents(e ast.Expr) (chain []string, literal string, ok bool) {
	if e.Kind() != ast.CallKind {
		return nil, "", false
	}
	call := e.AsCall()
	switch call.FunctionName() {
	case "_<_", "_<=_", "_>_", "_>=_":
	default:
		return nil, "", false
	}
	args := call.Args()
	if len(args) != 2 {
		return nil, "", false
	}
	if c, isSelf := singleSelfChain(args[0]); isSelf {
		if lit, isLit := constLiteral(args[1]); isLit {
			return c, lit, true
		}
	}
	if c, isSelf := singleSelfChain(args[1]); isSelf {
		if lit, isLit := constLiteral(args[0]); isLit {
			return c, lit, true
		}
	}
	return nil, "", false
}

// singleSelfChain returns the dotted-path chain of the expression iff the
// expression is itself a self.* chain and contains no other identifiers. It
// rejects expressions with nested calls, comparisons, or arithmetic.
func singleSelfChain(e ast.Expr) ([]string, bool) {
	chain, ok := selfChainOf(e)
	if !ok || len(chain) == 0 {
		return nil, false
	}
	return chain, true
}

func constLiteral(e ast.Expr) (string, bool) {
	if e.Kind() != ast.LiteralKind {
		return "", false
	}
	return literalString(e.AsLiteral()), true
}

func stringLiteral(v ref.Val) (string, bool) {
	if v == nil {
		return "", false
	}
	if v.Type() == types.StringType {
		s, ok := v.Value().(string)
		return s, ok
	}
	return "", false
}

func literalString(v ref.Val) string {
	if v == nil {
		return ""
	}
	switch raw := v.Value().(type) {
	case string:
		return fmt.Sprintf("%q", raw)
	case bool:
		return fmt.Sprintf("%t", raw)
	default:
		return fmt.Sprintf("%v", raw)
	}
}

// visit performs a pre-order traversal of the AST and invokes fn for every
// non-nil expression. Used by the chain collector; rule-4 patterns inspect
// only the root and a small fixed depth, so they do not need a visitor.
func visit(e ast.Expr, fn func(ast.Expr)) {
	if e == nil {
		return
	}
	fn(e)
	switch e.Kind() {
	case ast.CallKind:
		call := e.AsCall()
		if call.IsMemberFunction() {
			visit(call.Target(), fn)
		}
		for _, a := range call.Args() {
			visit(a, fn)
		}
	case ast.SelectKind:
		visit(e.AsSelect().Operand(), fn)
	case ast.ListKind:
		for _, el := range e.AsList().Elements() {
			visit(el, fn)
		}
	case ast.MapKind:
		for _, entry := range e.AsMap().Entries() {
			m := entry.AsMapEntry()
			visit(m.Key(), fn)
			visit(m.Value(), fn)
		}
	case ast.StructKind:
		for _, f := range e.AsStruct().Fields() {
			visit(f.AsStructField().Value(), fn)
		}
	case ast.ComprehensionKind:
		c := e.AsComprehension()
		visit(c.IterRange(), fn)
		visit(c.AccuInit(), fn)
		visit(c.LoopCondition(), fn)
		visit(c.LoopStep(), fn)
		visit(c.Result(), fn)
	}
}
