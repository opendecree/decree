package cel

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/google/cel-go/cel"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// Default DoS guards per the design brief. cel.CostLimit aborts a program
// whose internal cost counter exceeds the limit; cel.InterruptCheckFrequency
// pins how often the cost counter is sampled while a loop or comprehension
// runs. The brief picked 100_000 and 100; both are tunable per deployment.
const (
	defaultCostLimit     uint64 = 100_000
	defaultInterruptFreq uint   = 100

	envCostLimit     = "DECREE_CEL_COST_LIMIT"
	envInterruptFreq = "DECREE_CEL_INTERRUPT_FREQ"
)

// Cache memoises compiled CEL programs across config writes. Keys are tuples
// of (schemaID, schemaVersion, ruleIndex); the program holds its own copy of
// the env and is safe for concurrent evaluation. Invalidate by deleting all
// entries for a schemaID whenever a tenant's schema binding moves.
type Cache struct {
	entries sync.Map // cacheKey → cel.Program
}

type cacheKey struct {
	SchemaID  string
	Version   int32
	RuleIndex int
}

// NewCache returns an empty program cache.
func NewCache() *Cache { return &Cache{} }

// ProgramFor returns the cached program for the rule at the given index,
// compiling and caching it on first use. The rule string is taken verbatim
// from pb.ValidationRule.Rule. Compilation errors propagate to the caller
// — they have already been surfaced by LintValidations at schema-import
// time, but the runtime path defensively returns them rather than panicking
// on a corrupted schema row.
func (c *Cache) ProgramFor(env *cel.Env, rule *pb.ValidationRule, schemaID string, version int32, idx int) (cel.Program, error) {
	key := cacheKey{SchemaID: schemaID, Version: version, RuleIndex: idx}
	if v, ok := c.entries.Load(key); ok {
		return v.(cel.Program), nil
	}
	prog, err := compileProgram(env, rule.GetRule())
	if err != nil {
		return nil, err
	}
	actual, _ := c.entries.LoadOrStore(key, prog)
	return actual.(cel.Program), nil
}

// InvalidateSchema drops every cached program whose key belongs to the
// given schemaID. Called when a tenant's schema version moves so the next
// evaluation re-compiles against the new field set.
func (c *Cache) InvalidateSchema(schemaID string) {
	c.entries.Range(func(k, _ any) bool {
		if key, ok := k.(cacheKey); ok && key.SchemaID == schemaID {
			c.entries.Delete(key)
		}
		return true
	})
}

// compileProgram compiles a single CEL rule against the env and wraps it
// with the configured cost-limit and interrupt-check guards.
func compileProgram(env *cel.Env, rule string) (cel.Program, error) {
	ast, issues := env.Compile(rule)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}
	prog, err := env.Program(ast,
		cel.CostLimit(costLimit()),
		cel.InterruptCheckFrequency(interruptFreq()),
	)
	if err != nil {
		return nil, fmt.Errorf("build program: %w", err)
	}
	return prog, nil
}

func costLimit() uint64 {
	if v, ok := readUint64(envCostLimit); ok {
		return v
	}
	return defaultCostLimit
}

func interruptFreq() uint {
	if v, ok := readUint64(envInterruptFreq); ok {
		return uint(v)
	}
	return defaultInterruptFreq
}

func readUint64(name string) (uint64, bool) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
