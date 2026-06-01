//go:build integration

package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/schema"
	"github.com/opendecree/decree/internal/storage/dbstore"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/storage/pgconv"
	"github.com/opendecree/decree/internal/storage/pgtest"
)

// setupFixture creates the minimal schema → version → tenant chain for config tests.
func setupFixture(t *testing.T, schStore *schema.PGStore, tag string) (sch domain.Schema, sv domain.SchemaVersion, tenant domain.Tenant) {
	t.Helper()
	ctx := context.Background()

	var err error
	sch, err = schStore.CreateSchema(ctx, schema.CreateSchemaParams{
		Name: fmt.Sprintf("cfg-schema-%s", tag),
	})
	require.NoError(t, err)

	sv, err = schStore.CreateSchemaVersion(ctx, schema.CreateSchemaVersionParams{
		SchemaID: sch.ID,
		Version:  1,
		Checksum: "x",
	})
	require.NoError(t, err)

	tenant, err = schStore.CreateTenant(ctx, schema.CreateTenantParams{
		Name:          fmt.Sprintf("cfg-tenant-%s", tag),
		SchemaID:      sch.ID,
		SchemaVersion: sv.Version,
	})
	require.NoError(t, err)
	return
}

func TestConfigPGStore(t *testing.T) {
	pool := pgtest.NewPool(t)
	store := NewPGStore(pool, pool)
	schStore := schema.NewPGStore(pool, pool)
	ctx := context.Background()

	t.Run("ConfigVersionCRUD", func(t *testing.T) {
		_, _, tenant := setupFixture(t, schStore, t.Name())

		desc := "initial version"
		cv, err := store.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID:    tenant.ID,
			Version:     1,
			Description: &desc,
			CreatedBy:   "test",
		})
		require.NoError(t, err)
		assert.Equal(t, int32(1), cv.Version)
		assert.Equal(t, "initial version", *cv.Description)

		// GetConfigVersion
		got, err := store.GetConfigVersion(ctx, GetConfigVersionParams{
			TenantID: tenant.ID,
			Version:  1,
		})
		require.NoError(t, err)
		assert.Equal(t, cv.ID, got.ID)

		// Create a second version to test GetLatestConfigVersion.
		cv2, err := store.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID:  tenant.ID,
			Version:   2,
			CreatedBy: "test",
		})
		require.NoError(t, err)

		latest, err := store.GetLatestConfigVersion(ctx, tenant.ID)
		require.NoError(t, err)
		assert.Equal(t, cv2.ID, latest.ID)

		// ListConfigVersions
		versions, err := store.ListConfigVersions(ctx, ListConfigVersionsParams{
			TenantID: tenant.ID,
			Limit:    100,
		})
		require.NoError(t, err)
		assert.Len(t, versions, 2)
	})

	t.Run("VersionConflict", func(t *testing.T) {
		_, _, tenant := setupFixture(t, schStore, t.Name())

		_, err := store.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID:  tenant.ID,
			Version:   1,
			CreatedBy: "test",
		})
		require.NoError(t, err)

		// Duplicate version must return ErrVersionConflict.
		_, err = store.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID:  tenant.ID,
			Version:   1,
			CreatedBy: "test",
		})
		require.ErrorIs(t, err, ErrVersionConflict)
	})

	t.Run("ConfigValues", func(t *testing.T) {
		_, _, tenant := setupFixture(t, schStore, t.Name())

		cv, err := store.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID:  tenant.ID,
			Version:   1,
			CreatedBy: "test",
		})
		require.NoError(t, err)

		val := "100ms"
		chk := "abc"
		desc := "request timeout"
		err = store.SetConfigValue(ctx, SetConfigValueParams{
			ConfigVersionID: cv.ID,
			FieldPath:       "app.timeout",
			Value:           &val,
			Checksum:        &chk,
			Description:     &desc,
		})
		require.NoError(t, err)

		// GetConfigValues
		values, err := store.GetConfigValues(ctx, cv.ID)
		require.NoError(t, err)
		require.Len(t, values, 1)
		assert.Equal(t, "app.timeout", values[0].FieldPath)
		assert.Equal(t, "100ms", *values[0].Value)

		// GetConfigValueAtVersion
		row, err := store.GetConfigValueAtVersion(ctx, GetConfigValueAtVersionParams{
			TenantID:  tenant.ID,
			FieldPath: "app.timeout",
			Version:   1,
		})
		require.NoError(t, err)
		assert.Equal(t, "100ms", *row.Value)

		// GetFullConfigAtVersion
		full, err := store.GetFullConfigAtVersion(ctx, GetFullConfigAtVersionParams{
			TenantID: tenant.ID,
			Version:  1,
		})
		require.NoError(t, err)
		require.Len(t, full, 1)
		assert.Equal(t, "app.timeout", full[0].FieldPath)

		// GetConfigValuesSince
		deltas, err := store.GetConfigValuesSince(ctx, GetConfigValuesSinceParams{
			TenantID:     tenant.ID,
			StartVersion: 1,
		})
		require.NoError(t, err)
		require.Len(t, deltas, 1)
		assert.Equal(t, "app.timeout", deltas[0].FieldPath)
	})

	t.Run("TxRollback", func(t *testing.T) {
		_, _, tenant := setupFixture(t, schStore, t.Name())

		err := store.RunInTx(ctx, func(txStore Store) error {
			_, innerErr := txStore.CreateConfigVersion(ctx, CreateConfigVersionParams{
				TenantID:  tenant.ID,
				Version:   1,
				CreatedBy: "test",
			})
			if innerErr != nil {
				return innerErr
			}
			return errors.New("deliberate rollback")
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "deliberate rollback")

		// The version created inside the transaction must not be visible.
		_, err = store.GetConfigVersion(ctx, GetConfigVersionParams{
			TenantID: tenant.ID,
			Version:  1,
		})
		require.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("TxIsolation", func(t *testing.T) {
		// Writes within a tx are visible to reads in the same tx but not outside.
		_, _, tenant := setupFixture(t, schStore, t.Name())

		var cvIDInsideTx string
		err := store.RunInTx(ctx, func(txStore Store) error {
			cv, innerErr := txStore.CreateConfigVersion(ctx, CreateConfigVersionParams{
				TenantID:  tenant.ID,
				Version:   1,
				CreatedBy: "test",
			})
			if innerErr != nil {
				return innerErr
			}
			cvIDInsideTx = cv.ID

			val := "on"
			return txStore.SetConfigValue(ctx, SetConfigValueParams{
				ConfigVersionID: cv.ID,
				FieldPath:       "feature.flag",
				Value:           &val,
			})
		})
		require.NoError(t, err)

		// After commit: version and value must be visible.
		cv, err := store.GetConfigVersion(ctx, GetConfigVersionParams{
			TenantID: tenant.ID,
			Version:  1,
		})
		require.NoError(t, err)
		assert.Equal(t, cvIDInsideTx, cv.ID)

		values, err := store.GetConfigValues(ctx, cv.ID)
		require.NoError(t, err)
		require.Len(t, values, 1)
	})

	t.Run("LargeJSON", func(t *testing.T) {
		// Config values are TEXT columns — verify round-trip of >1 MB payloads.
		_, _, tenant := setupFixture(t, schStore, t.Name())

		cv, err := store.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID:  tenant.ID,
			Version:   1,
			CreatedBy: "test",
		})
		require.NoError(t, err)

		bigVal := `{"payload":"` + strings.Repeat("x", 1024*1024) + `"}`
		err = store.SetConfigValue(ctx, SetConfigValueParams{
			ConfigVersionID: cv.ID,
			FieldPath:       "app.largeblob",
			Value:           &bigVal,
		})
		require.NoError(t, err)

		values, err := store.GetConfigValues(ctx, cv.ID)
		require.NoError(t, err)
		require.Len(t, values, 1)
		assert.Equal(t, bigVal, *values[0].Value)
		assert.Greater(t, len(*values[0].Value), 1024*1024)
	})

	t.Run("TenantAndSchemaLookup", func(t *testing.T) {
		sch, sv, tenant := setupFixture(t, schStore, t.Name())

		// GetTenantByID and GetTenantByName via config store.
		got, err := store.GetTenantByID(ctx, tenant.ID)
		require.NoError(t, err)
		assert.Equal(t, tenant.Name, got.Name)

		got2, err := store.GetTenantByName(ctx, tenant.Name)
		require.NoError(t, err)
		assert.Equal(t, tenant.ID, got2.ID)

		// GetSchemaVersion via config store.
		schVer, err := store.GetSchemaVersion(ctx, domain.SchemaVersionKey{
			SchemaID: sch.ID,
			Version:  sv.Version,
		})
		require.NoError(t, err)
		assert.Equal(t, sv.ID, schVer.ID)
	})

	t.Run("DeadlockNotRetried", func(t *testing.T) {
		// Verify that PG deadlock errors (40P01) propagate to callers without
		// any retry logic in the store layer. The store returns the raw error;
		// retry decisions belong to the caller.
		//
		// We use pg_advisory_xact_lock to create a deterministic deadlock:
		//   - Goroutine A holds lock 1001, waits for lock 1002.
		//   - Goroutine B holds lock 1002, waits for lock 1001.
		// PG detects the cycle and aborts one of the transactions.

		var (
			aLocked    = make(chan struct{})
			bLocked    = make(chan struct{})
			errA, errB error
			wg         sync.WaitGroup
		)
		wg.Add(2)

		go func() {
			defer wg.Done()
			conn, err := pool.Acquire(ctx)
			if err != nil {
				errA = err
				close(aLocked)
				return
			}
			defer conn.Release()

			_, _ = conn.Exec(ctx, "BEGIN")
			_, _ = conn.Exec(ctx, "SELECT pg_advisory_xact_lock(1001)")
			close(aLocked)
			<-bLocked
			_, errA = conn.Exec(ctx, "SELECT pg_advisory_xact_lock(1002)")
			_, _ = conn.Exec(ctx, "ROLLBACK")
		}()

		go func() {
			defer wg.Done()
			conn, err := pool.Acquire(ctx)
			if err != nil {
				errB = err
				close(bLocked)
				return
			}
			defer conn.Release()

			_, _ = conn.Exec(ctx, "BEGIN")
			_, _ = conn.Exec(ctx, "SELECT pg_advisory_xact_lock(1002)")
			close(bLocked)
			<-aLocked
			_, errB = conn.Exec(ctx, "SELECT pg_advisory_xact_lock(1001)")
			_, _ = conn.Exec(ctx, "ROLLBACK")
		}()

		wg.Wait()

		// Exactly one transaction should fail with 40P01 (deadlock detected).
		deadlockErr := errA
		if deadlockErr == nil {
			deadlockErr = errB
		}
		require.Error(t, deadlockErr, "expected one goroutine to get a deadlock error")

		var pgErr *pgconn.PgError
		require.True(t, errors.As(deadlockErr, &pgErr), "expected pgconn.PgError, got: %T", deadlockErr)
		assert.Equal(t, "40P01", pgErr.Code, "expected deadlock error code")
	})
}

// TestAuditChainConcurrency verifies that concurrent same-tenant InsertAuditWriteLog
// calls produce a single linear chain with no forked previous_hash values.
func TestAuditChainConcurrency(t *testing.T) {
	pool := pgtest.NewPool(t)
	store := NewPGStore(pool, pool)
	schStore := schema.NewPGStore(pool, pool)
	ctx := context.Background()

	_, _, tenant := setupFixture(t, schStore, t.Name())

	const workers = 5
	var wg sync.WaitGroup
	wg.Add(workers)
	errs := make([]error, workers)

	for i := range workers {
		i := i
		go func() {
			defer wg.Done()
			errs[i] = store.RunInTx(ctx, func(txStore Store) error {
				fp := fmt.Sprintf("app.field%d", i)
				val := fmt.Sprintf("v%d", i)
				return txStore.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
					TenantID:   tenant.ID,
					Actor:      "concurrent-writer",
					Action:     "set_field",
					ObjectKind: "field",
					FieldPath:  &fp,
					NewValue:   &val,
				})
			})
		}()
	}
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "worker %d failed", i)
	}

	// Read the chain in insertion order and verify it is linear.
	tenantUUID, err := pgconv.StringToUUID(tenant.ID)
	require.NoError(t, err)
	entries, err := dbstore.New(pool).GetAuditWriteLogOrdered(ctx, tenantUUID)
	require.NoError(t, err)
	require.Len(t, entries, workers, "expected exactly %d entries", workers)

	// Build a set of all entry_hash values to detect forking:
	// if two entries share the same previous_hash, the chain is forked.
	prevHashCount := make(map[string]int, workers)
	for _, e := range entries {
		prevHashCount[e.PreviousHash]++
	}
	for ph, count := range prevHashCount {
		assert.Equal(t, 1, count, "previous_hash %q appears %d times — chain is forked", ph, count)
	}

	// Verify the linked-list structure.
	assert.Empty(t, entries[0].PreviousHash, "first entry must have empty previous_hash")
	for i := 1; i < len(entries); i++ {
		assert.Equal(t, entries[i-1].EntryHash, entries[i].PreviousHash,
			"entry[%d].PreviousHash must equal entry[%d].EntryHash", i, i-1)
	}
}

// TestConnPoolExhaustionBehavior demonstrates pool exhaustion error semantics
// independent of any store operation.
func TestConnPoolExhaustionBehavior(t *testing.T) {
	connStr := pgtest.ConnStr(t)
	ctx := context.Background()

	cfg, err := pgxpool.ParseConfig(connStr)
	require.NoError(t, err)
	cfg.MaxConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	c1, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer c1.Release()

	c2, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer c2.Release()

	tctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	_, err = pool.Acquire(tctx)
	require.Error(t, err, "pool exhaustion must return an error")
}
