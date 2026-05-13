package cel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func TestCache_ProgramFor_CompilesAndMemoises(t *testing.T) {
	env, err := BuildEnv(showcaseFields())
	require.NoError(t, err)

	cache := NewCache()
	rule := &pb.ValidationRule{Rule: "self.payments.min_amount < self.payments.max_amount"}

	first, err := cache.ProgramFor(env, rule, "schema-1", 1, 0)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := cache.ProgramFor(env, rule, "schema-1", 1, 0)
	require.NoError(t, err)
	assert.Same(t, first, second, "second lookup must return the cached program")
}

func TestCache_ProgramFor_DifferentKeysHoldDifferentPrograms(t *testing.T) {
	env, err := BuildEnv(showcaseFields())
	require.NoError(t, err)

	cache := NewCache()
	rule := &pb.ValidationRule{Rule: "self.payments.min_amount < self.payments.max_amount"}

	a, err := cache.ProgramFor(env, rule, "schema-1", 1, 0)
	require.NoError(t, err)
	b, err := cache.ProgramFor(env, rule, "schema-1", 2, 0)
	require.NoError(t, err)
	assert.NotSame(t, a, b, "different versions key separately")
}

func TestCache_InvalidateSchema_DropsMatchingEntriesOnly(t *testing.T) {
	env, err := BuildEnv(showcaseFields())
	require.NoError(t, err)

	cache := NewCache()
	rule := &pb.ValidationRule{Rule: "self.payments.min_amount < self.payments.max_amount"}

	prog1, err := cache.ProgramFor(env, rule, "schema-1", 1, 0)
	require.NoError(t, err)
	prog2, err := cache.ProgramFor(env, rule, "schema-2", 1, 0)
	require.NoError(t, err)

	cache.InvalidateSchema("schema-1")

	reprog1, err := cache.ProgramFor(env, rule, "schema-1", 1, 0)
	require.NoError(t, err)
	assert.NotSame(t, prog1, reprog1, "schema-1 entry must be recompiled after invalidation")

	stillCached, err := cache.ProgramFor(env, rule, "schema-2", 1, 0)
	require.NoError(t, err)
	assert.Same(t, prog2, stillCached, "schema-2 entries must remain cached")
}

func TestCache_ProgramFor_ReportsCompileFailure(t *testing.T) {
	env, err := BuildEnv(showcaseFields())
	require.NoError(t, err)

	cache := NewCache()
	rule := &pb.ValidationRule{Rule: "self.payments.min_amount <"}

	_, err = cache.ProgramFor(env, rule, "schema-1", 1, 0)
	require.Error(t, err)
}

func TestCostLimit_ReadsFromEnv(t *testing.T) {
	t.Setenv(envCostLimit, "12345")
	assert.Equal(t, uint64(12345), costLimit())
}

func TestCostLimit_DefaultsWhenUnset(t *testing.T) {
	t.Setenv(envCostLimit, "")
	assert.Equal(t, defaultCostLimit, costLimit())
}

func TestCostLimit_DefaultsOnGarbageInput(t *testing.T) {
	t.Setenv(envCostLimit, "not-a-number")
	assert.Equal(t, defaultCostLimit, costLimit())
}

func TestInterruptFreq_ReadsFromEnv(t *testing.T) {
	t.Setenv(envInterruptFreq, "50")
	assert.Equal(t, uint(50), interruptFreq())
}

func TestInterruptFreq_DefaultsOnOverflow(t *testing.T) {
	t.Setenv(envInterruptFreq, "9999999999999")
	assert.Equal(t, defaultInterruptFreq, interruptFreq())
}
