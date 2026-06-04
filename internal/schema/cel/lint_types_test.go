package cel

import (
	"strings"
	"testing"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// typeCheckFields is the schema used across the type-lint tests: one field of
// each scalar type, plus a nested numeric pair mirroring the e2e fixture.
func typeCheckFields() []*pb.SchemaField {
	return []*pb.SchemaField{
		{Path: "i", Type: pb.FieldType_FIELD_TYPE_INT},
		{Path: "n", Type: pb.FieldType_FIELD_TYPE_NUMBER},
		{Path: "s", Type: pb.FieldType_FIELD_TYPE_STRING},
		{Path: "b", Type: pb.FieldType_FIELD_TYPE_BOOL},
		{Path: "payments.min_amount", Type: pb.FieldType_FIELD_TYPE_NUMBER},
		{Path: "payments.max_amount", Type: pb.FieldType_FIELD_TYPE_NUMBER},
	}
}

func TestLintValidations_TypeMismatch(t *testing.T) {
	fields := typeCheckFields()

	tests := []struct {
		name        string
		rule        string
		wantErr     bool
		msgContains string
	}{
		// Accepted: lints clean because it also evaluates without a type error.
		{"numeric cross-compare", "self.i < self.n", false, ""},
		{"heterogeneous equality", "self.s == self.i", false, ""},
		{"string method on string", "self.s.startsWith('x')", false, ""},
		{"nested numeric compare", "self.payments.min_amount < self.payments.max_amount", false, ""},
		{"null-guarded cross-field", "self.n == null || self.i < self.n", false, ""},
		{"numeric arithmetic compare", "self.i + self.n > self.payments.min_amount", false, ""},
		{"ternary with bool condition", "self.b ? self.i < self.n : true", false, ""},
		{"numeric subtraction compare", "self.payments.max_amount - self.payments.min_amount > self.n", false, ""},
		{"tenant binding compare", "self.s == tenant.id", false, ""},
		{"list membership", "self.s in ['a', 'b'] || self.b", false, ""},

		// Rejected: these fail at runtime with "no such overload", so they must
		// be caught at import. The field path is named in the message.
		{"order int vs string", "self.i < self.s", true, "self.i"},
		{"order string vs number", "self.s < self.n", true, "self.s"},
		{"string method on number", "self.n.startsWith('x')", true, "string receiver"},
		{"add int and string", "self.i + self.s > 0", true, "add"},
		{"order bool vs int", "self.b < self.i", true, "order"},
		{"logical operand not bool", "self.b && self.i", true, "must be bool"},
		{"negate non-bool", "!self.i", true, "self.i"},
		{"subtract string and int", "self.s - self.i > 0", true, "apply"},
		{"ternary non-bool condition", "self.i ? self.s : self.n", true, "ternary condition"},
		{"endsWith on number", "self.n.endsWith('x')", true, "string receiver"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := LintValidations([]*pb.ValidationRule{{Rule: tc.rule, Message: "m"}}, fields)
			if tc.wantErr && err == nil {
				t.Fatalf("rule %q: expected a type error, got nil", tc.rule)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("rule %q: expected clean lint, got %v", tc.rule, err)
			}
			if tc.wantErr && tc.msgContains != "" && !strings.Contains(err.Error(), tc.msgContains) {
				t.Fatalf("rule %q: error %q does not contain %q", tc.rule, err.Error(), tc.msgContains)
			}
		})
	}
}

// TestLintValidations_TypeCheckRunsAfterPathResolution guards against the type
// pass double-reporting an unknown path: an unresolved self.<path> infers as
// dyn, so only the rule-3 path error fires, not a spurious type error.
func TestLintValidations_UnknownPathNoTypeNoise(t *testing.T) {
	fields := typeCheckFields()
	err := LintValidations([]*pb.ValidationRule{{Rule: "self.missing < self.s", Message: "m"}}, fields)
	if err == nil {
		t.Fatal("expected an error for unknown path")
	}
	if !strings.Contains(err.Error(), "does not resolve") {
		t.Fatalf("expected a path-resolution error, got %v", err)
	}
	if strings.Contains(err.Error(), "order") {
		t.Fatalf("unknown path should infer as dyn, not raise a type error: %v", err)
	}
}
