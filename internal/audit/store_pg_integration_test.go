//go:build integration

package audit_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/audit"
	"github.com/opendecree/decree/internal/schema"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/storage/pgtest"
)

// setupTenant creates the minimal schema → version → tenant chain.
func setupTenant(t *testing.T, schStore *schema.PGStore, tag string) domain.Tenant {
	t.Helper()
	ctx := context.Background()

	sch, err := schStore.CreateSchema(ctx, schema.CreateSchemaParams{
		Name: fmt.Sprintf("audit-schema-%s", tag),
	})
	require.NoError(t, err)

	sv, err := schStore.CreateSchemaVersion(ctx, schema.CreateSchemaVersionParams{
		SchemaID: sch.ID,
		Version:  1,
		Checksum: "x",
	})
	require.NoError(t, err)

	ten, err := schStore.CreateTenant(ctx, schema.CreateTenantParams{
		Name:          fmt.Sprintf("audit-tenant-%s", tag),
		SchemaID:      sch.ID,
		SchemaVersion: sv.Version,
	})
	require.NoError(t, err)
	return ten
}

func TestAuditPGStore(t *testing.T) {
	pool := pgtest.NewPool(t)
	store := audit.NewPGStore(pool, pool)
	schStore := schema.NewPGStore(pool, pool)
	ctx := context.Background()

	t.Run("InsertAndList", func(t *testing.T) {
		ten := setupTenant(t, schStore, t.Name())

		fp := "app.fee"
		old := "0.01"
		newv := "0.02"
		cv := int32(1)

		err := store.InsertAuditWriteLog(ctx, audit.InsertAuditWriteLogParams{
			TenantID:      ten.ID,
			Actor:         "admin",
			Action:        "set_field",
			ObjectKind:    "field",
			FieldPath:     &fp,
			OldValue:      &old,
			NewValue:      &newv,
			ConfigVersion: &cv,
			Metadata:      []byte(`{}`),
		})
		require.NoError(t, err)

		entries, err := store.GetAuditWriteLogOrdered(ctx, ten.ID)
		require.NoError(t, err)
		require.Len(t, entries, 1)

		e := entries[0]
		assert.Equal(t, ten.ID, e.TenantID)
		assert.Equal(t, "admin", e.Actor)
		assert.Equal(t, "set_field", e.Action)
		assert.Equal(t, "app.fee", *e.FieldPath)
		assert.Equal(t, "0.01", *e.OldValue)
		assert.Equal(t, "0.02", *e.NewValue)
		assert.NotEmpty(t, e.EntryHash)
	})

	t.Run("HashChain", func(t *testing.T) {
		ten := setupTenant(t, schStore, t.Name())

		for i := range 3 {
			fp := fmt.Sprintf("app.field%d", i)
			err := store.InsertAuditWriteLog(ctx, audit.InsertAuditWriteLogParams{
				TenantID:   ten.ID,
				Actor:      "bot",
				Action:     "set_field",
				ObjectKind: "field",
				FieldPath:  &fp,
			})
			require.NoError(t, err)
		}

		entries, err := store.GetAuditWriteLogOrdered(ctx, ten.ID)
		require.NoError(t, err)
		require.Len(t, entries, 3)

		// Each entry's PreviousHash must equal the prior entry's EntryHash.
		assert.Empty(t, entries[0].PreviousHash, "first entry has no previous hash")
		assert.Equal(t, entries[0].EntryHash, entries[1].PreviousHash)
		assert.Equal(t, entries[1].EntryHash, entries[2].PreviousHash)
	})

	t.Run("GlobalChain", func(t *testing.T) {
		// An empty TenantID writes to the global (schema-level) chain.
		err := store.InsertAuditWriteLog(ctx, audit.InsertAuditWriteLogParams{
			Actor:      "superadmin",
			Action:     "create_schema",
			ObjectKind: "schema",
		})
		require.NoError(t, err)

		entries, err := store.GetAuditWriteLogOrdered(ctx, "")
		require.NoError(t, err)
		assert.NotEmpty(t, entries)
	})

	t.Run("QueryFilter", func(t *testing.T) {
		ten := setupTenant(t, schStore, t.Name())

		fp1 := "app.fee"
		fp2 := "app.name"

		require.NoError(t, store.InsertAuditWriteLog(ctx, audit.InsertAuditWriteLogParams{
			TenantID: ten.ID, Actor: "alice", Action: "set_field", ObjectKind: "field", FieldPath: &fp1,
		}))
		require.NoError(t, store.InsertAuditWriteLog(ctx, audit.InsertAuditWriteLogParams{
			TenantID: ten.ID, Actor: "bob", Action: "set_field", ObjectKind: "field", FieldPath: &fp2,
		}))

		// Filter by actor.
		results, err := store.QueryAuditWriteLog(ctx, audit.QueryWriteLogParams{
			TenantID: ten.ID,
			Actor:    "alice",
			Limit:    100,
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "alice", results[0].Actor)

		// Filter by field path.
		results, err = store.QueryAuditWriteLog(ctx, audit.QueryWriteLogParams{
			TenantID:  ten.ID,
			FieldPath: "app.name",
			Limit:     100,
		})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "bob", results[0].Actor)

		// Time filter: future start → empty result.
		future := time.Now().Add(time.Hour)
		results, err = store.QueryAuditWriteLog(ctx, audit.QueryWriteLogParams{
			TenantID:  ten.ID,
			StartTime: &future,
			Limit:     100,
		})
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("UsageStats", func(t *testing.T) {
		ten := setupTenant(t, schStore, t.Name())

		// Upsert initial usage.
		reader := "service-a"
		now := time.Now().UTC().Truncate(time.Hour)
		err := store.UpsertUsageStats(ctx, audit.UpsertUsageStatsParams{
			TenantID:    ten.ID,
			FieldPath:   "app.timeout",
			PeriodStart: now,
			ReadCount:   5,
			LastReadBy:  &reader,
			LastReadAt:  now,
		})
		require.NoError(t, err)

		// GetFieldUsage
		usage, err := store.GetFieldUsage(ctx, audit.GetFieldUsageParams{
			TenantID:  ten.ID,
			FieldPath: "app.timeout",
		})
		require.NoError(t, err)
		require.Len(t, usage, 1)
		assert.Equal(t, int64(5), usage[0].ReadCount)
		assert.Equal(t, "service-a", *usage[0].LastReadBy)

		// GetTenantUsage
		tenUsage, err := store.GetTenantUsage(ctx, audit.GetTenantUsageParams{
			TenantID: ten.ID,
		})
		require.NoError(t, err)
		require.NotEmpty(t, tenUsage)
		assert.Equal(t, "app.timeout", tenUsage[0].FieldPath)
		assert.Equal(t, int64(5), tenUsage[0].ReadCount)

		// Upsert increments read count.
		err = store.UpsertUsageStats(ctx, audit.UpsertUsageStatsParams{
			TenantID:    ten.ID,
			FieldPath:   "app.timeout",
			PeriodStart: now,
			ReadCount:   3,
			LastReadAt:  now,
		})
		require.NoError(t, err)

		usage2, err := store.GetFieldUsage(ctx, audit.GetFieldUsageParams{
			TenantID:  ten.ID,
			FieldPath: "app.timeout",
		})
		require.NoError(t, err)
		require.Len(t, usage2, 1)
		assert.Equal(t, int64(8), usage2[0].ReadCount)
	})

	t.Run("GetUnusedFields", func(t *testing.T) {
		ten := setupTenant(t, schStore, t.Name())

		// Add a schema field so there's something to check usage against.
		sv, err := schStore.GetSchemaVersion(ctx, schema.GetSchemaVersionParams{
			SchemaID: ten.SchemaID,
			Version:  ten.SchemaVersion,
		})
		require.NoError(t, err)

		_, err = schStore.CreateSchemaField(ctx, schema.CreateSchemaFieldParams{
			SchemaVersionID: sv.ID,
			Path:            "app.unused",
			FieldType:       domain.FieldTypeString,
		})
		require.NoError(t, err)

		// With no usage stats, the field should appear as unused.
		unused, err := store.GetUnusedFields(ctx, audit.GetUnusedFieldsParams{
			TenantID: ten.ID,
			Since:    time.Now(),
		})
		require.NoError(t, err)
		assert.Contains(t, unused, "app.unused")

		// After recording usage, the field is no longer unused.
		now := time.Now().UTC().Truncate(time.Hour)
		err = store.UpsertUsageStats(ctx, audit.UpsertUsageStatsParams{
			TenantID:    ten.ID,
			FieldPath:   "app.unused",
			PeriodStart: now,
			ReadCount:   1,
			LastReadAt:  now,
		})
		require.NoError(t, err)

		// "Since" set to past → field is considered used.
		past := now.Add(-time.Minute)
		unused2, err := store.GetUnusedFields(ctx, audit.GetUnusedFieldsParams{
			TenantID: ten.ID,
			Since:    past,
		})
		require.NoError(t, err)
		assert.NotContains(t, unused2, "app.unused")
	})

	t.Run("LargeMetadata", func(t *testing.T) {
		// Verify that JSONB columns accept large payloads (>1 MB).
		ten := setupTenant(t, schStore, t.Name())
		largeJSON := `{"data":"` + strings.Repeat("x", 1024*1024) + `"}`

		err := store.InsertAuditWriteLog(ctx, audit.InsertAuditWriteLogParams{
			TenantID:   ten.ID,
			Actor:      "loader",
			Action:     "bulk_import",
			ObjectKind: "field",
			Metadata:   []byte(largeJSON),
		})
		require.NoError(t, err)

		entries, err := store.GetAuditWriteLogOrdered(ctx, ten.ID)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Greater(t, len(entries[0].Metadata), 1024*1024)
	})
}
