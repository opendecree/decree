package audit

import (
	"context"
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
