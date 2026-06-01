//go:build integration

package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage"
	"github.com/opendecree/decree/internal/storage/pgtest"
)

func TestNewDB_Connect(t *testing.T) {
	connStr := pgtest.ConnStr(t)
	ctx := context.Background()

	db, err := storage.NewDB(ctx, connStr, "")
	require.NoError(t, err)
	t.Cleanup(db.Close)

	// Both pools point to the same pool when readDSN is empty.
	assert.Same(t, db.WritePool, db.ReadPool)

	// Both pools respond to ping.
	require.NoError(t, db.WritePool.Ping(ctx))
}

func TestNewDB_SeparateReadPool(t *testing.T) {
	connStr := pgtest.ConnStr(t)
	ctx := context.Background()

	db, err := storage.NewDB(ctx, connStr, connStr)
	require.NoError(t, err)
	t.Cleanup(db.Close)

	// Same DSN string → same pool (deduplication in NewDB).
	assert.Same(t, db.WritePool, db.ReadPool)
}

func TestNewDB_InvalidDSN(t *testing.T) {
	ctx := context.Background()
	_, err := storage.NewDB(ctx, "postgres://invalid-host:5432/nodb?connect_timeout=1", "")
	require.Error(t, err)
}

func TestDB_Close(t *testing.T) {
	connStr := pgtest.ConnStr(t)
	ctx := context.Background()

	db, err := storage.NewDB(ctx, connStr, "")
	require.NoError(t, err)

	db.Close()

	// Pool is closed; further pings must fail.
	assert.Error(t, db.WritePool.Ping(ctx))
}

// TestNewDB_AfterConnectSetsRole verifies that every connection acquired from
// the pool created by NewDB runs as decree_app (the non-superuser application
// role), which is required for PostgreSQL Row-Level Security policies to be
// effective on tables guarded with FORCE ROW LEVEL SECURITY.  This is the
// central acceptance test for issue #659.
func TestNewDB_AfterConnectSetsRole(t *testing.T) {
	connStr := pgtest.ConnStr(t)
	ctx := context.Background()

	db, err := storage.NewDB(ctx, connStr, "")
	require.NoError(t, err)
	t.Cleanup(db.Close)

	conn, err := db.WritePool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	var role string
	require.NoError(t, conn.QueryRow(ctx, "SELECT current_role").Scan(&role))
	assert.Equal(t, "decree_app", role,
		"every connection in the pool must run as decree_app so RLS policies are enforced")
}

func TestConnPoolExhaustion(t *testing.T) {
	connStr := pgtest.ConnStr(t)
	ctx := context.Background()

	cfg, err := pgxpool.ParseConfig(connStr)
	require.NoError(t, err)
	cfg.MaxConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// Acquire both connections and hold them.
	c1, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer c1.Release()

	c2, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer c2.Release()

	// A third acquire with a tight deadline must time out.
	tctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_, err = pool.Acquire(tctx)
	require.Error(t, err, "expected error when pool is exhausted")
}
