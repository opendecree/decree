package cel

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

func TestValType_Nil(t *testing.T) {
	assert.Equal(t, "nil", valType(nil))
}

func TestEval_EmptyProgramsIsNoOp(t *testing.T) {
	failed, soft, err := Eval(nil, nil, nil)
	require.NoError(t, err)
	assert.Nil(t, failed)
	assert.Nil(t, soft)
}

func TestEval_ProgramsRulesLengthMismatch(t *testing.T) {
	env, err := BuildEnv([]*pb.SchemaField{{Path: "x", Type: pb.FieldType_FIELD_TYPE_NUMBER}})
	require.NoError(t, err)
	prog, err := compileProgram(env, "self.x > 0.0")
	require.NoError(t, err)

	// One program, zero rules — a programmer bug that must abort.
	_, _, evalErr := Eval([]cel.Program{prog}, map[string]any{}, nil)
	require.Error(t, evalErr)
	assert.Contains(t, evalErr.Error(), "length mismatch")
}
