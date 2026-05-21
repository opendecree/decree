package audit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage/domain"
)

func TestVerifyChain_Intact(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	for i := range 5 {
		err := store.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			TenantID:   "aaaa0000-0000-0000-0000-000000000001",
			Actor:      "actor",
			Action:     "set_field",
			ObjectKind: "field",
			FieldPath:  verifyPtr("path.field"),
			NewValue:   verifyPtr(string(rune('a' + i))),
		})
		require.NoError(t, err)
	}

	result, err := VerifyChain(ctx, store, "aaaa0000-0000-0000-0000-000000000001")
	require.NoError(t, err)
	assert.True(t, result.OK)
	assert.Equal(t, 5, result.Total)
	assert.Empty(t, result.Breaks)
}

func TestVerifyChain_DetectsTampering(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	tenantID := "bbbb0000-0000-0000-0000-000000000001"

	for range 5 {
		err := store.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			TenantID:   tenantID,
			Actor:      "actor",
			Action:     "set_field",
			ObjectKind: "field",
		})
		require.NoError(t, err)
	}

	// Directly mutate the middle entry's hash to simulate tampering.
	store.mu.Lock()
	for i, e := range store.writeLogs {
		if e.TenantID == tenantID {
			store.writeLogs[i].EntryHash = "tampered"
			break
		}
	}
	store.mu.Unlock()

	result, err := VerifyChain(ctx, store, tenantID)
	require.NoError(t, err)
	assert.False(t, result.OK)
	assert.NotEmpty(t, result.Breaks)
	assert.Equal(t, "tampered", result.Breaks[0].Got)
}

func TestVerifyChain_EmptyChain(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	result, err := VerifyChain(ctx, store, "cccc0000-0000-0000-0000-000000000001")
	require.NoError(t, err)
	assert.True(t, result.OK)
	assert.Equal(t, 0, result.Total)
}

func TestVerifyChain_ChainLinkage(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	tenantID := "dddd0000-0000-0000-0000-000000000001"

	for range 3 {
		require.NoError(t, store.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			TenantID:   tenantID,
			Actor:      "a",
			Action:     "set_field",
			ObjectKind: "field",
		}))
	}

	entries, err := store.GetAuditWriteLogOrdered(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	// First entry must have empty previous_hash.
	assert.Empty(t, entries[0].PreviousHash)
	// Each subsequent entry must chain to the previous.
	assert.Equal(t, entries[0].EntryHash, entries[1].PreviousHash)
	assert.Equal(t, entries[1].EntryHash, entries[2].PreviousHash)
}

// TestInsertAuditWriteLog_SchemaObjectKind verifies schema-level entries use
// empty tenant_id (global chain).
func TestInsertAuditWriteLog_GlobalChain(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// Insert a tenant entry and a global entry; chains must be separate.
	require.NoError(t, store.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
		TenantID:   "eeee0000-0000-0000-0000-000000000001",
		Actor:      "a",
		Action:     "set_field",
		ObjectKind: "field",
	}))
	require.NoError(t, store.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
		TenantID:   "", // global
		Actor:      "a",
		Action:     "create_schema",
		ObjectKind: "schema",
	}))

	tenantResult, err := VerifyChain(ctx, store, "eeee0000-0000-0000-0000-000000000001")
	require.NoError(t, err)
	assert.True(t, tenantResult.OK)
	assert.Equal(t, 1, tenantResult.Total)

	globalResult, err := VerifyChain(ctx, store, "")
	require.NoError(t, err)
	assert.True(t, globalResult.OK)
	assert.Equal(t, 1, globalResult.Total)
}

func TestVerifyChain_TriggerRollbackOnOldRow(t *testing.T) {
	// The trigger test is integration-only (requires a real DB).
	// Verified in store_pg_test.go when running make e2e.
	t.Skip("trigger tests run against real DB — see store_pg_test.go")
}

// TestVerifyChain_DetectsPayloadTampering asserts that mutating a payload field
// (NewValue) is detected by VerifyChain on epoch-1 entries.
func TestVerifyChain_DetectsPayloadTampering(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	tenantID := "pay10000-0000-0000-0000-000000000001"

	for i := range 3 {
		fp := "app.name"
		val := string(rune('a' + i))
		err := store.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			TenantID:   tenantID,
			Actor:      "actor",
			Action:     "set_field",
			ObjectKind: "field",
			FieldPath:  &fp,
			NewValue:   &val,
		})
		require.NoError(t, err)
	}

	// Mutate the NewValue of the first entry — payload tampered, hash unchanged.
	store.mu.Lock()
	for i, e := range store.writeLogs {
		if e.TenantID == tenantID {
			tampered := "TAMPERED"
			store.writeLogs[i].NewValue = &tampered
			break
		}
	}
	store.mu.Unlock()

	result, err := VerifyChain(ctx, store, tenantID)
	require.NoError(t, err)
	assert.False(t, result.OK, "chain should be broken after payload mutation")
	assert.NotEmpty(t, result.Breaks, "should report at least one break")
}

// TestConcurrentInserts_ChainIsLinear fires N concurrent writes and asserts the
// resulting chain has no forks — every entry except the first has a unique
// PreviousHash that matches exactly the preceding entry's EntryHash.
// This exercises the MemoryStore's mutex-based serialisation; the PGStore
// relies on pg_advisory_xact_lock for the same guarantee.
func TestConcurrentInserts_ChainIsLinear(t *testing.T) {
	const (
		tenantID = "cccc0000-0000-0000-0000-000000000001"
		workers  = 20
	)

	store := NewMemoryStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = store.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
				TenantID:   tenantID,
				Actor:      "worker",
				Action:     "set_field",
				ObjectKind: "field",
				NewValue:   verifyPtr(string(rune('a' + i%26))),
			})
		}(i)
	}
	wg.Wait()

	entries, err := store.GetAuditWriteLogOrdered(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, entries, workers)

	// Verify the chain is strictly linear: each entry's PreviousHash must
	// equal the preceding entry's EntryHash (no two entries share a PreviousHash).
	seen := make(map[string]bool, workers)
	seen[""] = true // empty string is valid for the first entry only
	for _, e := range entries {
		assert.False(t, seen[e.EntryHash], "duplicate EntryHash detected — chain forked")
		seen[e.EntryHash] = true
	}

	// Chain linkage: each entry must point to its predecessor.
	for i := 1; i < len(entries); i++ {
		assert.Equal(t, entries[i-1].EntryHash, entries[i].PreviousHash,
			"entry %d PreviousHash must equal entry %d EntryHash", i, i-1)
	}

	result, err := VerifyChain(ctx, store, tenantID)
	require.NoError(t, err)
	assert.True(t, result.OK)
}

// Helper for verify_test — avoids collision with store_memory_test.go's ptr.
func verifyPtr(s string) *string { return &s }

// Ensure MemoryStore implements Store (compile-time check for new methods).
var _ Store = (*MemoryStore)(nil)

// Smoke test for MemoryStore.GetAuditWriteLogOrdered with multiple tenants.
func TestGetAuditWriteLogOrdered_TenantIsolation(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	t1 := "f1f10000-0000-0000-0000-000000000001"
	t2 := "f2f20000-0000-0000-0000-000000000002"

	store.AddWriteLog(domain.AuditWriteLog{TenantID: t1, CreatedAt: time.Now()})
	store.AddWriteLog(domain.AuditWriteLog{TenantID: t2, CreatedAt: time.Now()})
	store.AddWriteLog(domain.AuditWriteLog{TenantID: t1, CreatedAt: time.Now().Add(time.Second)})

	rows, err := store.GetAuditWriteLogOrdered(ctx, t1)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	for _, r := range rows {
		assert.Equal(t, t1, r.TenantID)
	}
}
