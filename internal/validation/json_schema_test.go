package validation

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultLimits(t *testing.T) {
	l := DefaultLimits()
	assert.Equal(t, 5*time.Second, l.CompileTimeout)
	assert.Equal(t, 64, l.MaxDepth)
}

func TestNewJSONSchemaValidator_Compiles(t *testing.T) {
	doc := `{"type":"object","properties":{"name":{"type":"string"}}}`
	v, err := newJSONSchemaValidator(doc, DefaultLimits())
	require.NoError(t, err)
	require.NotNil(t, v)
	require.NoError(t, v.validate(`{"name":"x"}`))
	require.Error(t, v.validate(`{"name":1}`))
}

func TestNewJSONSchemaValidator_DepthExceeded(t *testing.T) {
	// Build a schema with nesting depth 10, then cap MaxDepth to 5.
	doc := strings.Repeat(`{"properties":{"x":`, 10) + `{"type":"string"}` + strings.Repeat(`}}`, 10)
	_, err := newJSONSchemaValidator(doc, Limits{MaxDepth: 5})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nesting depth exceeds limit of 5")
}

func TestNewJSONSchemaValidator_DepthDisabled(t *testing.T) {
	doc := strings.Repeat(`{"properties":{"x":`, 5) + `{"type":"string"}` + strings.Repeat(`}}`, 5)
	v, err := newJSONSchemaValidator(doc, Limits{MaxDepth: 0})
	require.NoError(t, err)
	require.NotNil(t, v)
}

func TestNewJSONSchemaValidator_MalformedJSONFallsThrough(t *testing.T) {
	// Pre-scan ignores non-JSON; compiler reports the syntax error.
	_, err := newJSONSchemaValidator(`not-json`, DefaultLimits())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json schema")
}

func TestNewJSONSchemaValidator_TimeoutZeroIsUnbounded(t *testing.T) {
	doc := `{"type":"string"}`
	v, err := newJSONSchemaValidator(doc, Limits{CompileTimeout: 0, MaxDepth: 0})
	require.NoError(t, err)
	require.NotNil(t, v)
}

func TestScanJSONDepth(t *testing.T) {
	// Object nesting.
	require.NoError(t, scanJSONDepth(`{"a":{"b":{"c":1}}}`, 5))
	require.Error(t, scanJSONDepth(`{"a":{"b":{"c":1}}}`, 2))

	// Array nesting counts too.
	require.NoError(t, scanJSONDepth(`[[[[1]]]]`, 5))
	require.Error(t, scanJSONDepth(`[[[[1]]]]`, 3))

	// Non-JSON: scan is a no-op.
	require.NoError(t, scanJSONDepth(`not json`, 0))
}
