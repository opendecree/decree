package validation

import (
	"testing"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	celpkg "github.com/opendecree/decree/internal/schema/cel"
)

// TestInvalidateSchemaPrograms_DropsCompiledPrograms verifies that the factory
// method evicts a schema's compiled programs from the shared cache, so the next
// access recompiles rather than returning the stale program. Without it the
// programs would linger for the process lifetime (issue #805, A3).
func TestInvalidateSchemaPrograms_DropsCompiledPrograms(t *testing.T) {
	f := NewValidatorFactory(nil)

	fields := []*pb.SchemaField{{Path: "a", Type: pb.FieldType_FIELD_TYPE_INT}}
	env, err := celpkg.BuildEnv(fields)
	if err != nil {
		t.Fatalf("BuildEnv: %v", err)
	}
	rule := &pb.ValidationRule{Rule: "self.a > 0"}

	p1, err := f.celCache.ProgramFor(env, rule, "s1", 1, 0)
	if err != nil {
		t.Fatalf("ProgramFor: %v", err)
	}
	p2, err := f.celCache.ProgramFor(env, rule, "s1", 1, 0)
	if err != nil {
		t.Fatalf("ProgramFor (cached): %v", err)
	}
	if p1 != p2 {
		t.Fatal("expected the same cached program instance before invalidation")
	}

	f.InvalidateSchemaPrograms("s1")

	p3, err := f.celCache.ProgramFor(env, rule, "s1", 1, 0)
	if err != nil {
		t.Fatalf("ProgramFor (post-invalidate): %v", err)
	}
	if p1 == p3 {
		t.Fatal("expected a freshly compiled program after InvalidateSchemaPrograms")
	}
}
