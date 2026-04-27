package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// --- validateDependentRequiredAgainstFields ---

func TestValidateDependentRequiredAgainstFields_Empty(t *testing.T) {
	require.NoError(t, validateDependentRequiredAgainstFields(nil, nil))
	require.NoError(t, validateDependentRequiredAgainstFields([]*pb.DependentRequiredEntry{}, nil))
}

func TestValidateDependentRequiredAgainstFields_Valid(t *testing.T) {
	fields := []*pb.SchemaField{
		{Path: "a", Type: pb.FieldType_FIELD_TYPE_STRING},
		{Path: "b", Type: pb.FieldType_FIELD_TYPE_STRING},
		{Path: "c", Type: pb.FieldType_FIELD_TYPE_STRING},
	}
	entries := []*pb.DependentRequiredEntry{
		{TriggerField: "a", DependentFields: []string{"b", "c"}},
	}
	require.NoError(t, validateDependentRequiredAgainstFields(entries, fields))
}

func TestValidateDependentRequiredAgainstFields_UnknownTrigger(t *testing.T) {
	fields := []*pb.SchemaField{{Path: "a", Type: pb.FieldType_FIELD_TYPE_STRING}}
	entries := []*pb.DependentRequiredEntry{
		{TriggerField: "missing", DependentFields: []string{"a"}},
	}
	err := validateDependentRequiredAgainstFields(entries, fields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"missing"`)
	assert.Contains(t, err.Error(), "not a defined field")
}

func TestValidateDependentRequiredAgainstFields_UnknownDependent(t *testing.T) {
	fields := []*pb.SchemaField{{Path: "a", Type: pb.FieldType_FIELD_TYPE_STRING}}
	entries := []*pb.DependentRequiredEntry{
		{TriggerField: "a", DependentFields: []string{"ghost"}},
	}
	err := validateDependentRequiredAgainstFields(entries, fields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"ghost"`)
}

func TestValidateDependentRequiredAgainstFields_SelfReference(t *testing.T) {
	fields := []*pb.SchemaField{{Path: "a", Type: pb.FieldType_FIELD_TYPE_STRING}}
	entries := []*pb.DependentRequiredEntry{
		{TriggerField: "a", DependentFields: []string{"a"}},
	}
	err := validateDependentRequiredAgainstFields(entries, fields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot list itself")
}

func TestValidateDependentRequiredAgainstFields_Duplicate(t *testing.T) {
	fields := []*pb.SchemaField{
		{Path: "a", Type: pb.FieldType_FIELD_TYPE_STRING},
		{Path: "b", Type: pb.FieldType_FIELD_TYPE_STRING},
	}
	entries := []*pb.DependentRequiredEntry{
		{TriggerField: "a", DependentFields: []string{"b", "b"}},
	}
	err := validateDependentRequiredAgainstFields(entries, fields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listed twice")
}

// --- marshal / Unmarshal round-trip ---

func TestMarshalUnmarshalDependentRequired_RoundTrip(t *testing.T) {
	in := []*pb.DependentRequiredEntry{
		{TriggerField: "x", DependentFields: []string{"y", "z"}},
		{TriggerField: "a", DependentFields: []string{"b"}},
	}
	raw, err := marshalDependentRequired(in)
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	out := UnmarshalDependentRequired(raw)
	require.Len(t, out, 2)
	// Order preserved from input.
	assert.Equal(t, "x", out[0].TriggerField)
	assert.Equal(t, []string{"y", "z"}, out[0].DependentFields)
	assert.Equal(t, "a", out[1].TriggerField)
}

func TestMarshalDependentRequired_EmptyReturnsBracketArray(t *testing.T) {
	raw, err := marshalDependentRequired(nil)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(raw))
}

func TestUnmarshalDependentRequired_EmptyInputReturnsNil(t *testing.T) {
	assert.Nil(t, UnmarshalDependentRequired(nil))
	assert.Nil(t, UnmarshalDependentRequired([]byte("[]")))
}

func TestUnmarshalDependentRequired_MalformedReturnsNil(t *testing.T) {
	// Unparseable JSON degrades to "no rules" — never panics, never errors.
	assert.Nil(t, UnmarshalDependentRequired([]byte("not json")))
}

// --- CheckDependentRequired ---

func TestCheckDependentRequired_NoRules(t *testing.T) {
	require.NoError(t, CheckDependentRequired(nil, nil))
	require.NoError(t, CheckDependentRequired([]*pb.DependentRequiredEntry{}, map[string]struct{}{"a": {}}))
}

func TestCheckDependentRequired_TriggerAbsent_NoRequirement(t *testing.T) {
	rules := []*pb.DependentRequiredEntry{
		{TriggerField: "trigger", DependentFields: []string{"dep"}},
	}
	// Trigger not in presence set — rule does not fire even though dep is also absent.
	require.NoError(t, CheckDependentRequired(rules, map[string]struct{}{}))
}

func TestCheckDependentRequired_TriggerPresent_DependentsPresent_OK(t *testing.T) {
	rules := []*pb.DependentRequiredEntry{
		{TriggerField: "trigger", DependentFields: []string{"a", "b"}},
	}
	present := map[string]struct{}{"trigger": {}, "a": {}, "b": {}}
	require.NoError(t, CheckDependentRequired(rules, present))
}

func TestCheckDependentRequired_TriggerPresent_DependentMissing_Fails(t *testing.T) {
	rules := []*pb.DependentRequiredEntry{
		{TriggerField: "trigger", DependentFields: []string{"a", "b"}},
	}
	present := map[string]struct{}{"trigger": {}, "a": {}}
	err := CheckDependentRequired(rules, present)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"trigger"`)
	assert.Contains(t, err.Error(), `"b"`)
}

func TestCheckDependentRequired_FirstViolationReturned(t *testing.T) {
	rules := []*pb.DependentRequiredEntry{
		{TriggerField: "t1", DependentFields: []string{"d1"}},
		{TriggerField: "t2", DependentFields: []string{"d2"}},
	}
	// Both rules violate — only first reported.
	present := map[string]struct{}{"t1": {}, "t2": {}}
	err := CheckDependentRequired(rules, present)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"t1"`)
}

// --- proto <-> YAML conversion helpers ---

func TestYamlToProtoDependentRequired_StableOrder(t *testing.T) {
	// Map iteration is randomized in Go; the converter must sort triggers and
	// dependents so the proto/wire form is deterministic.
	in := map[string][]string{
		"z": {"c", "a", "b"},
		"a": {"y"},
	}
	out := yamlToProtoDependentRequired(in)
	require.Len(t, out, 2)
	assert.Equal(t, "a", out[0].TriggerField)
	assert.Equal(t, []string{"y"}, out[0].DependentFields)
	assert.Equal(t, "z", out[1].TriggerField)
	assert.Equal(t, []string{"a", "b", "c"}, out[1].DependentFields)
}

func TestYamlToProtoDependentRequired_EmptyReturnsNil(t *testing.T) {
	assert.Nil(t, yamlToProtoDependentRequired(nil))
	assert.Nil(t, yamlToProtoDependentRequired(map[string][]string{}))
}

func TestProtoDependentRequiredToYAML_RoundTrip(t *testing.T) {
	entries := []*pb.DependentRequiredEntry{
		{TriggerField: "a", DependentFields: []string{"b", "c"}},
	}
	yaml := protoDependentRequiredToYAML(entries)
	require.Len(t, yaml, 1)
	assert.Equal(t, []string{"b", "c"}, yaml["a"])

	back := yamlToProtoDependentRequired(yaml)
	require.Len(t, back, 1)
	assert.Equal(t, "a", back[0].TriggerField)
	assert.Equal(t, []string{"b", "c"}, back[0].DependentFields)
}

func TestProtoDependentRequiredToYAML_EmptyReturnsNil(t *testing.T) {
	assert.Nil(t, protoDependentRequiredToYAML(nil))
	assert.Nil(t, protoDependentRequiredToYAML([]*pb.DependentRequiredEntry{}))
}
