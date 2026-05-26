//go:build integration

package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage/pgtest"
)

// TestRLSTenantIsolation proves that a query without an explicit tenant_id filter
// is still restricted to the current tenant when app.tenant_id is pinned via
// SET LOCAL inside a transaction. This validates the RLS defense-in-depth guarantee
// added by migration 006_rls.sql.
//
// The test switches the session to decree_app (a non-superuser role created by the
// migration) so that RLS policies apply. The container runs as a superuser, which
// bypasses RLS — SET SESSION AUTHORIZATION is required to observe policy effects.
func TestRLSTenantIsolation(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.NewPool(t)

	// --- Seed: two schemas and two tenants as superuser (bypasses RLS). ---

	var schemaID string
	err := pool.QueryRow(ctx,
		`INSERT INTO schemas (name) VALUES ('rls-test-schema') RETURNING id::text`,
	).Scan(&schemaID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO schema_versions (schema_id, version, checksum, published)
		 VALUES ($1, 1, 'rls-test-checksum', true)`,
		schemaID,
	)
	require.NoError(t, err)

	var tenantAID, tenantBID string
	err = pool.QueryRow(ctx,
		`INSERT INTO tenants (name, schema_id, schema_version) VALUES ('rls-tenant-a', $1, 1) RETURNING id::text`,
		schemaID,
	).Scan(&tenantAID)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`INSERT INTO tenants (name, schema_id, schema_version) VALUES ('rls-tenant-b', $1, 1) RETURNING id::text`,
		schemaID,
	).Scan(&tenantBID)
	require.NoError(t, err)

	// Insert a config version for each tenant.
	_, err = pool.Exec(ctx,
		`INSERT INTO config_versions (tenant_id, version, created_by) VALUES ($1, 1, 'rls-test')`,
		tenantAID,
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO config_versions (tenant_id, version, created_by) VALUES ($1, 1, 'rls-test')`,
		tenantBID,
	)
	require.NoError(t, err)

	// Acquire a connection and switch to decree_app so RLS policies apply.
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	t.Cleanup(conn.Release)

	_, err = conn.Exec(ctx, "SET SESSION AUTHORIZATION decree_app")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = conn.Exec(ctx, "RESET SESSION AUTHORIZATION")
	})

	t.Run("filters to pinned tenant within tx", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		t.Cleanup(func() { _ = tx.Rollback(ctx) })

		_, err = tx.Exec(ctx, "SET LOCAL app.tenant_id = $1", tenantAID)
		require.NoError(t, err)

		// Raw SELECT with no WHERE clause — RLS must filter to tenant A only.
		rows, err := tx.Query(ctx, "SELECT tenant_id::text FROM config_versions")
		require.NoError(t, err)
		defer rows.Close()

		var got []string
		for rows.Next() {
			var tid string
			require.NoError(t, rows.Scan(&tid))
			got = append(got, tid)
		}
		require.NoError(t, rows.Err())

		require.Equal(t, []string{tenantAID}, got,
			"RLS must restrict config_versions to the pinned tenant; tenant B's row must be invisible")
	})

	t.Run("superadmin_mode bypasses isolation", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		t.Cleanup(func() { _ = tx.Rollback(ctx) })

		_, err = tx.Exec(ctx, "SET LOCAL app.superadmin_mode = 'true'")
		require.NoError(t, err)

		rows, err := tx.Query(ctx, "SELECT tenant_id::text FROM config_versions")
		require.NoError(t, err)
		defer rows.Close()

		var got []string
		for rows.Next() {
			var tid string
			require.NoError(t, rows.Scan(&tid))
			got = append(got, tid)
		}
		require.NoError(t, rows.Err())

		require.Len(t, got, 2,
			"superadmin_mode must expose all tenants' config_versions")
	})

	t.Run("no GUC set returns all rows", func(t *testing.T) {
		// Without GUC, the app layer is responsible for tenant filtering.
		// RLS must not block unscoped reads so existing non-tx code paths keep working.
		rows, err := conn.Query(ctx, "SELECT tenant_id::text FROM config_versions")
		require.NoError(t, err)
		defer rows.Close()

		var got []string
		for rows.Next() {
			var tid string
			require.NoError(t, rows.Scan(&tid))
			got = append(got, tid)
		}
		require.NoError(t, rows.Err())

		require.Len(t, got, 2,
			"without a tenant GUC, RLS must allow unrestricted reads (app layer provides WHERE filters)")
	})
}
