package config

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeChecksum_Deterministic(t *testing.T) {
	c1 := computeChecksum("hello world")
	c2 := computeChecksum("hello world")
	assert.Equal(t, c1, c2)
}

func TestComputeChecksum_DifferentValues(t *testing.T) {
	c1 := computeChecksum("hello")
	c2 := computeChecksum("world")
	assert.NotEqual(t, c1, c2)
}

func TestParseUUID_Valid(t *testing.T) {
	id, err := parseUUID("550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	assert.True(t, id.Valid)
}

func TestParseUUID_Invalid(t *testing.T) {
	_, err := parseUUID("bad")
	require.Error(t, err)
}

func TestUUIDToString_InvalidReturnsEmpty(t *testing.T) {
	var id pgtype.UUID
	assert.Equal(t, "", uuidToString(id))
}

func TestPtrString_Empty(t *testing.T) {
	assert.Nil(t, ptrString(""))
}

func TestPtrString_NonEmpty(t *testing.T) {
	s := ptrString("test")
	require.NotNil(t, s)
	assert.Equal(t, "test", *s)
}
