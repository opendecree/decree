package cel

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func TestBuildActivation_NestsByDottedPath(t *testing.T) {
	rows := []SnapshotRow{
		{FieldPath: "payments.min_amount", Value: strPtr("10.0")},
		{FieldPath: "payments.max_amount", Value: strPtr("100.0")},
		{FieldPath: "payments.refunds_enabled", Value: strPtr("true")},
	}
	types := map[string]pb.FieldType{
		"payments.min_amount":      pb.FieldType_FIELD_TYPE_NUMBER,
		"payments.max_amount":      pb.FieldType_FIELD_TYPE_NUMBER,
		"payments.refunds_enabled": pb.FieldType_FIELD_TYPE_BOOL,
	}

	act := BuildActivation(rows, types, TenantBinding{ID: "t1", Name: "tenant-one"})

	self := act["self"].(map[string]any)
	payments := self["payments"].(map[string]any)
	assert.Equal(t, 10.0, payments["min_amount"])
	assert.Equal(t, 100.0, payments["max_amount"])
	assert.Equal(t, true, payments["refunds_enabled"])

	tenant := act["tenant"].(map[string]any)
	assert.Equal(t, "t1", tenant["id"])
	assert.Equal(t, "tenant-one", tenant["name"])
}

func TestBuildActivation_NullValueSurfacesAsCelNull(t *testing.T) {
	rows := []SnapshotRow{
		{FieldPath: "payments.refund_window", Value: nil},
	}
	types := map[string]pb.FieldType{
		"payments.refund_window": pb.FieldType_FIELD_TYPE_DURATION,
	}
	act := BuildActivation(rows, types, TenantBinding{})
	self := act["self"].(map[string]any)
	payments := self["payments"].(map[string]any)
	assert.Nil(t, payments["refund_window"])
}

func TestBuildActivation_HyphenatedSegmentsStayAsMapKeys(t *testing.T) {
	rows := []SnapshotRow{
		{FieldPath: "app-name.title", Value: strPtr("MyApp")},
	}
	types := map[string]pb.FieldType{
		"app-name.title": pb.FieldType_FIELD_TYPE_STRING,
	}
	act := BuildActivation(rows, types, TenantBinding{})
	self := act["self"].(map[string]any)
	app := self["app-name"].(map[string]any)
	assert.Equal(t, "MyApp", app["title"])
}

func TestBuildActivation_ParsesTypedScalars(t *testing.T) {
	rows := []SnapshotRow{
		{FieldPath: "count", Value: strPtr("42")},
		{FieldPath: "started_at", Value: strPtr("2026-05-13T11:00:00Z")},
		{FieldPath: "ttl", Value: strPtr("30s")},
		{FieldPath: "meta", Value: strPtr(`{"k":1}`)},
	}
	types := map[string]pb.FieldType{
		"count":      pb.FieldType_FIELD_TYPE_INT,
		"started_at": pb.FieldType_FIELD_TYPE_TIME,
		"ttl":        pb.FieldType_FIELD_TYPE_DURATION,
		"meta":       pb.FieldType_FIELD_TYPE_JSON,
	}
	act := BuildActivation(rows, types, TenantBinding{})
	self := act["self"].(map[string]any)
	assert.EqualValues(t, 42, self["count"])
	assert.NotNil(t, self["started_at"])
	assert.NotNil(t, self["ttl"])
	meta := self["meta"].(map[string]any)
	assert.EqualValues(t, 1, meta["k"])
}

func TestEval_RuleHolds(t *testing.T) {
	programs, rules := compileRunnable(t, []ruleSpec{
		{rule: "self.payments.min_amount < self.payments.max_amount", message: "min < max"},
	})

	act := BuildActivation(
		[]SnapshotRow{
			{FieldPath: "payments.min_amount", Value: strPtr("10")},
			{FieldPath: "payments.max_amount", Value: strPtr("20")},
		},
		map[string]pb.FieldType{
			"payments.min_amount": pb.FieldType_FIELD_TYPE_NUMBER,
			"payments.max_amount": pb.FieldType_FIELD_TYPE_NUMBER,
		},
		TenantBinding{},
	)
	failed, _, err := Eval(programs, act, rules)
	require.NoError(t, err)
	assert.Empty(t, failed)
}

func TestEval_RuleFires(t *testing.T) {
	programs, rules := compileRunnable(t, []ruleSpec{
		{rule: "self.payments.min_amount < self.payments.max_amount", message: "min must be < max"},
	})

	act := BuildActivation(
		[]SnapshotRow{
			{FieldPath: "payments.min_amount", Value: strPtr("100")},
			{FieldPath: "payments.max_amount", Value: strPtr("100")},
		},
		map[string]pb.FieldType{
			"payments.min_amount": pb.FieldType_FIELD_TYPE_NUMBER,
			"payments.max_amount": pb.FieldType_FIELD_TYPE_NUMBER,
		},
		TenantBinding{},
	)
	failed, _, err := Eval(programs, act, rules)
	require.NoError(t, err)
	require.Len(t, failed, 1)
	assert.Equal(t, "min must be < max", failed[0].Message)
}

func TestEval_AggregatesMultipleFailures(t *testing.T) {
	programs, rules := compileRunnable(t, []ruleSpec{
		{rule: "self.payments.min_amount < self.payments.max_amount", message: "min < max"},
		{rule: "self.payments.max_amount <= 1000.0", message: "max <= 1000"},
	})

	act := BuildActivation(
		[]SnapshotRow{
			{FieldPath: "payments.min_amount", Value: strPtr("2000")},
			{FieldPath: "payments.max_amount", Value: strPtr("1500")},
		},
		map[string]pb.FieldType{
			"payments.min_amount": pb.FieldType_FIELD_TYPE_NUMBER,
			"payments.max_amount": pb.FieldType_FIELD_TYPE_NUMBER,
		},
		TenantBinding{},
	)
	failed, _, err := Eval(programs, act, rules)
	require.NoError(t, err)
	require.Len(t, failed, 2)
	assert.Equal(t, "min < max", failed[0].Message)
	assert.Equal(t, "max <= 1000", failed[1].Message)
}

func TestEval_LengthMismatchIsErr(t *testing.T) {
	programs, rules := compileRunnable(t, []ruleSpec{
		{rule: "self.payments.min_amount < self.payments.max_amount", message: "x"},
	})
	_, _, err := Eval(programs, map[string]any{}, append(rules, &pb.ValidationRule{Rule: "true"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "length mismatch")
}

func TestEval_CostLimitExceeded(t *testing.T) {
	t.Setenv(envCostLimit, "1")
	t.Setenv(envInterruptFreq, "1")
	programs, rules := compileRunnable(t, []ruleSpec{
		// A comprehension over a literal range pushes cost above the
		// 1-unit limit on the first iteration, so cel-go surfaces the
		// "operation cancelled: actual cost limit exceeded" signature.
		{rule: "[1, 2, 3, 4, 5].all(x, x < self.payments.max_amount)", message: "x"},
	})
	act := BuildActivation(
		[]SnapshotRow{{FieldPath: "payments.max_amount", Value: strPtr("100")}},
		map[string]pb.FieldType{"payments.max_amount": pb.FieldType_FIELD_TYPE_NUMBER},
		TenantBinding{},
	)
	_, _, err := Eval(programs, act, rules)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cost limit")
}

func TestIsCostLimit(t *testing.T) {
	assert.True(t, isCostLimit(stringError("operation cancelled: actual cost limit exceeded")))
	assert.True(t, isCostLimit(stringError("cost limit exceeded for rule x")))
	assert.False(t, isCostLimit(stringError("unrelated runtime error")))
	assert.False(t, isCostLimit(nil))
}

type stringError string

func (a stringError) Error() string { return string(a) }

func TestEval_NonBoolResultIsSoftError(t *testing.T) {
	programs, rules := compileRunnable(t, []ruleSpec{
		{rule: "self.payments.min_amount + 1.0", message: "not bool"},
	})
	act := BuildActivation(
		[]SnapshotRow{{FieldPath: "payments.min_amount", Value: strPtr("1")}},
		map[string]pb.FieldType{"payments.min_amount": pb.FieldType_FIELD_TYPE_NUMBER},
		TenantBinding{},
	)
	failed, softErrs, err := Eval(programs, act, rules)
	require.NoError(t, err)
	assert.Empty(t, failed, "non-bool rule must not count as a failure")
	require.Len(t, softErrs, 1)
	assert.Contains(t, softErrs[0].Error(), "did not evaluate to bool")
}

func TestEval_NullComparisonIsSoftError(t *testing.T) {
	// Rule references a field that is null in the activation. cel-go raises
	// a runtime error ("no such overload") that must surface as a soft
	// error rather than failing the write — otherwise authors would have to
	// wrap every field reference in has() to keep unrelated writes from
	// being rejected.
	programs, rules := compileRunnable(t, []ruleSpec{
		{rule: "self.payments.min_amount < self.payments.max_amount", message: "min < max"},
	})
	act := BuildActivation(
		[]SnapshotRow{{FieldPath: "payments.max_amount", Value: strPtr("100")}},
		map[string]pb.FieldType{
			"payments.min_amount": pb.FieldType_FIELD_TYPE_NUMBER,
			"payments.max_amount": pb.FieldType_FIELD_TYPE_NUMBER,
		},
		TenantBinding{},
	)
	failed, softErrs, err := Eval(programs, act, rules)
	require.NoError(t, err)
	assert.Empty(t, failed, "null comparison must not fail the write")
	require.Len(t, softErrs, 1)
}

func BenchmarkEval_TenRules(b *testing.B) {
	env, err := BuildEnv(showcaseFields())
	require.NoError(b, err)
	cache := NewCache()
	rules := make([]*pb.ValidationRule, 0, 10)
	for range 10 {
		rules = append(rules, &pb.ValidationRule{
			Rule:    "self.payments.min_amount < self.payments.max_amount",
			Message: "x",
		})
	}
	programs := make([]cel.Program, len(rules))
	for i, r := range rules {
		p, err := cache.ProgramFor(env, r, "schema-id", 1, i)
		require.NoError(b, err)
		programs[i] = p
	}
	act := BuildActivation(
		[]SnapshotRow{
			{FieldPath: "payments.min_amount", Value: strPtr("1")},
			{FieldPath: "payments.max_amount", Value: strPtr("2")},
		},
		map[string]pb.FieldType{
			"payments.min_amount": pb.FieldType_FIELD_TYPE_NUMBER,
			"payments.max_amount": pb.FieldType_FIELD_TYPE_NUMBER,
		},
		TenantBinding{},
	)

	for b.Loop() {
		_, _, _ = Eval(programs, act, rules)
	}
}

type ruleSpec struct {
	rule    string
	message string
}

func compileRunnable(t *testing.T, specs []ruleSpec) ([]cel.Program, []*pb.ValidationRule) {
	t.Helper()
	rules := make([]*pb.ValidationRule, 0, len(specs))
	for _, s := range specs {
		rules = append(rules, &pb.ValidationRule{Rule: s.rule, Message: s.message})
	}
	env, err := BuildEnv(showcaseFields())
	require.NoError(t, err)
	cache := NewCache()
	programs := make([]cel.Program, len(rules))
	for i, r := range rules {
		p, err := cache.ProgramFor(env, r, "schema-id", 1, i)
		require.NoError(t, err)
		programs[i] = p
	}
	return programs, rules
}

func strPtr(s string) *string { return &s }
