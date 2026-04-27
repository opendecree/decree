package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Registry plumbing ---
//
// These tests exercise the Dispatch / MarshalSchemaAt entry points
// directly. The full v1 round-trip behavior is covered by yaml_test.go.

func TestSupportedVersions_IncludesV1(t *testing.T) {
	versions := SupportedVersions()
	require.NotEmpty(t, versions, "at least v1 must be registered via init()")
	assert.Contains(t, versions, "v1")
}

func TestLatestVersion_IsLexicographicMax(t *testing.T) {
	assert.Equal(t, "v1", LatestVersion(), "v1 is the only registered parser today")
}

func TestDispatch_RouteToV1(t *testing.T) {
	pb, err := Dispatch([]byte(`
spec_version: v1
name: payments
fields:
  payments.fee:
    type: number
`))
	require.NoError(t, err)
	require.NotNil(t, pb)
	assert.Equal(t, "payments", pb.Name)
	require.Len(t, pb.Fields, 1)
	assert.Equal(t, "payments.fee", pb.Fields[0].Path)
}

func TestDispatch_MissingSpecVersion(t *testing.T) {
	_, err := Dispatch([]byte(`
name: payments
fields:
  payments.fee:
    type: number
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec_version is required")
	// Error message must list the supported versions so users self-correct.
	assert.Contains(t, err.Error(), "v1")
}

func TestDispatch_UnknownSpecVersion(t *testing.T) {
	_, err := Dispatch([]byte(`
spec_version: v99
name: payments
fields:
  payments.fee:
    type: number
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported spec_version")
	assert.Contains(t, err.Error(), `"v99"`)
	assert.Contains(t, err.Error(), "v1") // supported list
}

func TestDispatch_MalformedYAML(t *testing.T) {
	_, err := Dispatch([]byte("not: valid: yaml: at: all"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid YAML")
}

func TestMarshalSchemaAt_DefaultsToLatest(t *testing.T) {
	schema, err := Dispatch([]byte(`
spec_version: v1
name: payments
fields:
  payments.fee:
    type: number
`))
	require.NoError(t, err)

	// Empty version string means "use LatestVersion".
	data, err := MarshalSchemaAt(schema, "")
	require.NoError(t, err)
	assert.Contains(t, string(data), "spec_version: v1")
	assert.Contains(t, string(data), "name: payments")
}

func TestMarshalSchemaAt_ExplicitV1(t *testing.T) {
	schema, err := Dispatch([]byte(`
spec_version: v1
name: payments
fields:
  payments.fee:
    type: number
`))
	require.NoError(t, err)

	data, err := MarshalSchemaAt(schema, "v1")
	require.NoError(t, err)
	assert.Contains(t, string(data), "spec_version: v1")
}

func TestMarshalSchemaAt_UnknownVersionRejected(t *testing.T) {
	schema, err := Dispatch([]byte(`
spec_version: v1
name: payments
fields:
  payments.fee:
    type: number
`))
	require.NoError(t, err)

	_, err = MarshalSchemaAt(schema, "v99")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"v99"`)
}

// TestDispatch_RoundTrip verifies that Dispatch → MarshalSchemaAt → Dispatch
// produces a structurally equivalent schema. Catches accidental field
// drops in either direction of the v1 parser.
func TestDispatch_RoundTrip(t *testing.T) {
	yamlIn := []byte(`
spec_version: v1
name: payments
description: payments service
version: 3
version_description: added refund window
info:
  title: Payments
  author: platform
fields:
  payments.fee:
    type: number
    constraints: { minimum: 0, maximum: 1 }
  payments.refunds_enabled:
    type: bool
  payments.refund_window:
    type: duration
    nullable: true
dependentRequired:
  payments.refunds_enabled: [payments.refund_window]
validations:
  - path: payments
    rule: "self.payments.fee >= 0"
    message: "fee must be non-negative"
`)
	first, err := Dispatch(yamlIn)
	require.NoError(t, err)

	out, err := MarshalSchemaAt(first, "v1")
	require.NoError(t, err)

	second, err := Dispatch(out)
	require.NoError(t, err)

	assert.Equal(t, first.Name, second.Name)
	assert.Equal(t, first.Description, second.Description)
	assert.Equal(t, first.Version, second.Version)
	assert.Equal(t, first.VersionDescription, second.VersionDescription)
	assert.Len(t, second.Fields, len(first.Fields))
	assert.Len(t, second.DependentRequired, len(first.DependentRequired))
	assert.Len(t, second.Validations, len(first.Validations))
}
