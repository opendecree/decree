package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage/domain"
)

// --- Registry plumbing ---

func TestSupportedVersions_IncludesV1(t *testing.T) {
	versions := SupportedVersions()
	require.NotEmpty(t, versions)
	assert.Contains(t, versions, "v1")
}

func TestLatestVersion_IsLexicographicMax(t *testing.T) {
	assert.Equal(t, "v1", LatestVersion())
}

func TestDispatchImport_RouteToV1(t *testing.T) {
	parsed, err := DispatchImport([]byte(`
spec_version: v1
description: import desc
values:
  payments.fee:
    value: 0.025
`), map[string]domain.FieldType{
		"payments.fee": domain.FieldTypeNumber,
	})
	require.NoError(t, err)
	assert.Equal(t, "import desc", parsed.Description)
	require.Len(t, parsed.Values, 1)
	assert.Equal(t, "payments.fee", parsed.Values[0].FieldPath)
	assert.Equal(t, "0.025", parsed.Values[0].Value)
}

func TestDispatchImport_MissingSpecVersion(t *testing.T) {
	_, err := DispatchImport([]byte(`
values:
  payments.fee:
    value: 0.025
`), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec_version is required")
}

func TestDispatchImport_UnknownSpecVersion(t *testing.T) {
	_, err := DispatchImport([]byte(`
spec_version: v99
values:
  payments.fee:
    value: 0.025
`), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"v99"`)
	assert.Contains(t, err.Error(), "v1")
}

func TestMarshalConfigAt_DefaultsToLatest(t *testing.T) {
	rows := []configRow{
		{FieldPath: "payments.fee", Value: "0.025"},
	}
	data, err := MarshalConfigAt(1, "first import", rows, map[string]domain.FieldType{
		"payments.fee": domain.FieldTypeNumber,
	}, "")
	require.NoError(t, err)
	assert.Contains(t, string(data), "spec_version: v1")
}

func TestMarshalConfigAt_UnknownVersion(t *testing.T) {
	_, err := MarshalConfigAt(1, "x", nil, nil, "v99")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"v99"`)
}
