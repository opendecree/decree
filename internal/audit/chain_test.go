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

func TestComputeEntryHash_Epoch0_NoPayloadSensitivity(t *testing.T) {
	fp := "app.name"
	base := ChainInput{
		PreviousHash: "prev",
		ID:           "id-1",
		TenantID:     "t-1",
		Actor:        "a",
		Action:       "set_field",
		ObjectKind:   "field",
		CreatedAt:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Epoch:        0,
	}
	withPayload := base
	withPayload.FieldPath = &fp
	withPayload.NewValue = func() *string { s := "hello"; return &s }()

	// Epoch 0 must produce the same hash regardless of payload — backward compat.
	assert.Equal(t, ComputeEntryHash(base), ComputeEntryHash(withPayload))
}

func TestComputeEntryHash_Epoch1_PayloadSensitivity(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	i32Ptr := func(i int32) *int32 { return &i }

	base := ChainInput{
		PreviousHash:  "prev",
		ID:            "id-1",
		TenantID:      "t-1",
		Actor:         "a",
		Action:        "set_field",
		ObjectKind:    "field",
		CreatedAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Epoch:         1,
		FieldPath:     strPtr("app.name"),
		OldValue:      strPtr("old"),
		NewValue:      strPtr("new"),
		ConfigVersion: i32Ptr(3),
		Metadata:      []byte(`{"k":"v"}`),
	}
	h1 := ComputeEntryHash(base)
	require.NotEmpty(t, h1)

	mutate := func(fn func(*ChainInput)) {
		m := base
		fn(&m)
		assert.NotEqual(t, h1, ComputeEntryHash(m), "mutation must change hash")
	}

	mutate(func(m *ChainInput) { m.FieldPath = strPtr("other.field") })
	mutate(func(m *ChainInput) { m.FieldPath = nil })
	mutate(func(m *ChainInput) { m.OldValue = strPtr("changed") })
	mutate(func(m *ChainInput) { m.OldValue = nil })
	mutate(func(m *ChainInput) { m.NewValue = strPtr("changed") })
	mutate(func(m *ChainInput) { m.NewValue = nil })
	mutate(func(m *ChainInput) { m.ConfigVersion = i32Ptr(99) })
	mutate(func(m *ChainInput) { m.ConfigVersion = nil })
	mutate(func(m *ChainInput) { m.Metadata = []byte(`{"k":"changed"}`) })
	mutate(func(m *ChainInput) { m.Metadata = nil })
}

func TestComputeEntryHash_Epoch1_NilVsEmpty(t *testing.T) {
	empty := ""
	base := ChainInput{
		Epoch: 1, ID: "id-1", Actor: "a", Action: "set_field", ObjectKind: "field",
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	withNil := base
	withEmpty := base
	withEmpty.FieldPath = &empty

	// nil and empty string must produce different hashes.
	assert.NotEqual(t, ComputeEntryHash(withNil), ComputeEntryHash(withEmpty))
}

// TestComputeEntryHash_Epoch1_MetadataJSONNormalization verifies that Go's
// compact json.Marshal output and PostgreSQL's JSONB text output (which adds
// a space after each colon and comma) produce the same hash.  This prevents
// chain breaks when VerifyChain reads Metadata back from a JSONB column.
func TestComputeEntryHash_Epoch1_MetadataJSONNormalization(t *testing.T) {
	base := ChainInput{
		Epoch: 1, ID: "id-1", Actor: "a", Action: "set_field", ObjectKind: "field",
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	// Go's compact form (no spaces) — written to DB at audit-entry creation time.
	compact := base
	compact.Metadata = []byte(`{"name":"sec-audit","schema_id":"abc","schema_version":1}`)

	// PostgreSQL JSONB text form (space after : and ,) — returned by DB on read.
	pgForm := base
	pgForm.Metadata = []byte(`{"name": "sec-audit", "schema_id": "abc", "schema_version": 1}`)

	assert.Equal(t, ComputeEntryHash(compact), ComputeEntryHash(pgForm),
		"compact JSON and PostgreSQL JSONB text output must produce the same hash")

	// A different value must still produce a different hash.
	different := base
	different.Metadata = []byte(`{"name":"other"}`)
	assert.NotEqual(t, ComputeEntryHash(compact), ComputeEntryHash(different))

	// Invalid JSON bytes are hashed unchanged (fallback path).
	invalidJSON := base
	invalidJSON.Metadata = []byte(`not-json`)
	assert.NotPanics(t, func() { ComputeEntryHash(invalidJSON) })
	// Two calls with the same invalid bytes produce the same hash.
	assert.Equal(t, ComputeEntryHash(invalidJSON), ComputeEntryHash(invalidJSON))
}
