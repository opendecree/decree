package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeEntryHash_Deterministic(t *testing.T) {
	in := ChainInput{
		PreviousHash: "abc",
		ID:           "id-1",
		TenantID:     "tenant-1",
		Actor:        "user@example.com",
		Action:       "set_field",
		ObjectKind:   "field",
		CreatedAt:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	h1 := ComputeEntryHash(in)
	h2 := ComputeEntryHash(in)
	assert.Equal(t, h1, h2, "hash must be deterministic")
	assert.Len(t, h1, 64, "expected SHA-256 hex string")
}

func TestComputeEntryHash_ChainSensitivity(t *testing.T) {
	base := ChainInput{
		PreviousHash: "",
		ID:           "id-1",
		TenantID:     "tenant-1",
		Actor:        "a",
		Action:       "set_field",
		ObjectKind:   "field",
		CreatedAt:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	h1 := ComputeEntryHash(base)
	require.NotEmpty(t, h1)

	// Changing previous_hash must change the entry hash.
	modified := base
	modified.PreviousHash = "different"
	assert.NotEqual(t, h1, ComputeEntryHash(modified))

	// Changing actor must change the entry hash.
	modified = base
	modified.Actor = "b"
	assert.NotEqual(t, h1, ComputeEntryHash(modified))

	// Changing object_kind must change the entry hash.
	modified = base
	modified.ObjectKind = "schema"
	assert.NotEqual(t, h1, ComputeEntryHash(modified))
}
