package cel

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// Baseline numbers captured on a single workstation are kept inline below as
// rough sanity checks for future reviewers; absolute values vary by host but
// the cost ratios between benches should stay in the same ballpark.
//
// Captured on Linux 6.17 / Go 1.25 / i7-1265U (2026-05-13):
//
//   BenchmarkCompile-12                          ~44 µs/op    25 KB/op    374 allocs/op
//   BenchmarkBuildActivation_LargeSchema-12     ~240 µs/op   161 KB/op  2 543 allocs/op  (1000 fields)
//   BenchmarkEval_FullCycle_LargeSchema-12       ~68 µs/op    39 KB/op  1 005 allocs/op  (50 rules × 1000 fields)
//   BenchmarkCache_ProgramFor_Cold-12            ~45 µs/op    25 KB/op
//   BenchmarkCache_ProgramFor_Cached-12          ~26 ns/op     0 B/op       0 allocs/op  (sync.Map hit)
//
// Headline takeaways:
//
//   - Per-write cost at 50 rules × 1000 fields is well under 1 ms; the
//     activation builder dominates, not eval.
//   - The cache turns the per-rule compile cost into a one-time cost on
//     the order of a sync.Map lookup (~3 orders of magnitude faster).
//   - cel.CostLimit at 100 000 gives roughly 100× headroom over a typical
//     full-cycle eval; raising it is unlikely to pay off until rule
//     complexity grows substantially.
//
// If a future cel-go bump moves any of these by >2× without an explainable
// API change, the bump warrants a closer look.

// largeSchemaFields builds a 1000-field schema rooted at `payments.*` plus a
// few sibling groups. The path layout mirrors what a production schema looks
// like — most fields in a single hot group with a long tail in others.
func largeSchemaFields() []*pb.SchemaField {
	const total = 1000
	fields := make([]*pb.SchemaField, 0, total)
	for i := range total {
		group := "payments"
		switch {
		case i >= 900:
			group = "settlement"
		case i >= 850:
			group = "audit"
		}
		fields = append(fields, &pb.SchemaField{
			Path: fmt.Sprintf("%s.field_%d", group, i),
			Type: pb.FieldType_FIELD_TYPE_NUMBER,
		})
	}
	return fields
}

// largeSchemaSnapshot returns a snapshot row for every field in
// largeSchemaFields. Half the values are unset (Value=nil) to model a
// realistic partial-population scenario.
func largeSchemaSnapshot() ([]SnapshotRow, map[string]pb.FieldType) {
	fields := largeSchemaFields()
	rows := make([]SnapshotRow, 0, len(fields))
	types := make(map[string]pb.FieldType, len(fields))
	for i, f := range fields {
		types[f.Path] = f.Type
		if i%2 == 0 {
			v := strconv.Itoa(i)
			rows = append(rows, SnapshotRow{FieldPath: f.Path, Value: &v})
		} else {
			rows = append(rows, SnapshotRow{FieldPath: f.Path, Value: nil})
		}
	}
	return rows, types
}

// largeSchemaRules returns N rules that touch the high-density `payments.*`
// group. The point of varying N is to model cumulative per-write cost when
// many rules apply to the same write.
func largeSchemaRules(n int) []*pb.ValidationRule {
	rules := make([]*pb.ValidationRule, 0, n)
	for i := range n {
		rules = append(rules, &pb.ValidationRule{
			Rule:    fmt.Sprintf("self.payments.field_%d < self.payments.field_%d", i*2, i*2+1),
			Message: fmt.Sprintf("rule %d", i),
		})
	}
	return rules
}

// BenchmarkCompile measures the cost of compiling one rule from source.
// This is the ImportSchema hot path: every rule on a fresh schema goes
// through Compile once before the program is cached.
func BenchmarkCompile(b *testing.B) {
	env, err := BuildEnv(largeSchemaFields())
	require.NoError(b, err)
	rule := "self.payments.field_0 < self.payments.field_1"

	for b.Loop() {
		ast, issues := env.Compile(rule)
		if issues != nil && issues.Err() != nil {
			b.Fatal(issues.Err())
		}
		_, err := env.Program(ast,
			cel.CostLimit(defaultCostLimit),
			cel.InterruptCheckFrequency(defaultInterruptFreq),
		)
		require.NoError(b, err)
	}
}

// BenchmarkBuildActivation_LargeSchema measures the activation builder for a
// 1000-field schema. Allocation cost dominates here because every declared
// path is pre-populated with nil before the snapshot overlay.
func BenchmarkBuildActivation_LargeSchema(b *testing.B) {
	rows, types := largeSchemaSnapshot()

	for b.Loop() {
		_ = BuildActivation(rows, types, TenantBinding{ID: "tenant"})
	}
}

// BenchmarkEval_FullCycle_LargeSchema models the per-write cost at scale:
// 50 rules over a 1000-field activation. Mirrors the realistic upper bound
// for a single tenant's CEL surface.
func BenchmarkEval_FullCycle_LargeSchema(b *testing.B) {
	env, err := BuildEnv(largeSchemaFields())
	require.NoError(b, err)
	rules := largeSchemaRules(50)
	cache := NewCache()
	programs := make([]cel.Program, len(rules))
	for i, r := range rules {
		p, err := cache.ProgramFor(env, r, "schema-id", 1, i)
		require.NoError(b, err)
		programs[i] = p
	}
	rows, types := largeSchemaSnapshot()
	act := BuildActivation(rows, types, TenantBinding{ID: "tenant"})

	for b.Loop() {
		_, _, _ = Eval(programs, act, rules)
	}
}

// BenchmarkCache_ProgramFor_Cold measures the cost of compiling and caching
// a program for the first time. Combine with the Cached counterpart to
// confirm the cache turns this into a one-time cost.
func BenchmarkCache_ProgramFor_Cold(b *testing.B) {
	env, err := BuildEnv(largeSchemaFields())
	require.NoError(b, err)
	rule := &pb.ValidationRule{Rule: "self.payments.field_0 < self.payments.field_1"}

	for i := 0; b.Loop(); i++ {
		cache := NewCache()
		_, err := cache.ProgramFor(env, rule, "schema", int32(i), 0)
		require.NoError(b, err)
	}
}

// BenchmarkCache_ProgramFor_Cached measures the cost of a cache hit. Should
// be on the order of a sync.Map lookup; if it climbs noticeably, the cache
// is no longer doing its job.
func BenchmarkCache_ProgramFor_Cached(b *testing.B) {
	env, err := BuildEnv(largeSchemaFields())
	require.NoError(b, err)
	rule := &pb.ValidationRule{Rule: "self.payments.field_0 < self.payments.field_1"}
	cache := NewCache()
	_, err = cache.ProgramFor(env, rule, "schema", 1, 0)
	require.NoError(b, err)

	for b.Loop() {
		_, _ = cache.ProgramFor(env, rule, "schema", 1, 0)
	}
}

// BenchmarkEval_AggregateCostCap_EarlyAbort measures the overhead of
// aggregate cost tracking when the cap fires after the first rule. The
// happy path (cap never reached) is already covered by
// BenchmarkEval_FullCycle_LargeSchema; this bench captures the abort path
// specifically, where the extra cost-sum check terminates the loop early.
func BenchmarkEval_AggregateCostCap_EarlyAbort(b *testing.B) {
	b.Setenv(envAggregateCostCap, "1") // fires after rule 0
	env, err := BuildEnv(largeSchemaFields())
	require.NoError(b, err)
	rules := largeSchemaRules(50)
	cache := NewCache()
	programs := make([]cel.Program, len(rules))
	for i, r := range rules {
		p, err := cache.ProgramFor(env, r, "schema-id", 1, i)
		require.NoError(b, err)
		programs[i] = p
	}
	rows, types := largeSchemaSnapshot()
	act := BuildActivation(rows, types, TenantBinding{ID: "tenant"})

	for b.Loop() {
		_, _, _ = Eval(programs, act, rules)
	}
}
