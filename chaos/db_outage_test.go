//go:build chaos

package chaos

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/sdk/adminclient"
)

// allowedDBErrCodes are the valid gRPC codes when postgres is unreachable.
// codes.Internal: errToStatus maps non-ErrNotFound store errors to Internal.
// codes.DeadlineExceeded: client-side context expiry before server responds.
var allowedDBErrCodes = []codes.Code{codes.Internal, codes.DeadlineExceeded, codes.Unavailable}

// TestDBOutage_MidFlight pauses postgres mid-flight, asserts writes return a
// valid gRPC error (not a raw transport error), then resumes and asserts recovery.
func TestDBOutage_MidFlight(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	schemaID, cleanupSchema := makeSchema(t, admin, "db-outage-schema")
	t.Cleanup(cleanupSchema)
	tenantID, cleanupTenant := makeTenant(t, admin, "db-outage-tenant", schemaID)
	t.Cleanup(cleanupTenant)

	// Seed a value (warms the Redis cache so reads may survive the outage).
	require.NoError(t, cfg.Set(ctx, tenantID, "chaos.field0", "before-outage"))

	// Pause postgres — simulates a network partition (no TCP RST, just freeze).
	containerPause(t, postgresContainer())
	t.Cleanup(func() { containerUnpause(t, postgresContainer()) })

	// Write operations must fail with a valid gRPC status code (not raw EOF/reset).
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := admin.CreateSchema(reqCtx, "db-outage-probe", []adminclient.Field{
		{Path: "probe.f", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.Error(t, err, "CreateSchema must fail while postgres is paused")

	st, ok := status.FromError(err)
	require.True(t, ok, "expected gRPC status error, got: %T %v", err, err)
	assert.Contains(t, allowedDBErrCodes, st.Code(),
		"unexpected error code %v: %v", st.Code(), st.Message())

	// Unpause postgres.
	containerUnpause(t, postgresContainer())

	// Server (pgxpool) must reconnect and serve writes again within 30s.
	eventually(t, 30*time.Second, func() bool {
		writeCtx, writeCancel := context.WithTimeout(ctx, 5*time.Second)
		defer writeCancel()
		return cfg.Set(writeCtx, tenantID, "chaos.field0", "after-outage") == nil
	})
}

// TestDBRecovery_AfterRestart stops postgres entirely, verifies writes fail with
// a proper gRPC status, then restarts and asserts pgxpool reconnects automatically.
func TestDBRecovery_AfterRestart(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	schemaID, cleanupSchema := makeSchema(t, admin, "db-recovery-schema")
	t.Cleanup(cleanupSchema)
	tenantID, cleanupTenant := makeTenant(t, admin, "db-recovery-tenant", schemaID)
	t.Cleanup(cleanupTenant)

	// Stop postgres (SIGTERM → container exit).
	containerStop(t, postgresContainer())
	t.Cleanup(func() {
		containerStart(t, postgresContainer())
		waitContainerHealthy(t, postgresContainer(), 60*time.Second)
	})

	// Writes fail — must be gRPC status errors.
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := admin.CreateSchema(reqCtx, "db-recovery-probe", []adminclient.Field{
		{Path: "probe.f", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.Error(t, err, "CreateSchema must fail while postgres is stopped")

	st, ok := status.FromError(err)
	require.True(t, ok, "expected gRPC status error, got: %T %v", err, err)
	assert.Contains(t, allowedDBErrCodes, st.Code(),
		"unexpected error code %v: %v", st.Code(), st.Message())

	// Restart postgres and wait for health check to pass.
	containerStart(t, postgresContainer())
	waitContainerHealthy(t, postgresContainer(), 60*time.Second)

	// pgxpool reconnects automatically; writes must succeed within 60s.
	eventually(t, 60*time.Second, func() bool {
		writeCtx, writeCancel := context.WithTimeout(ctx, 5*time.Second)
		defer writeCancel()
		return cfg.Set(writeCtx, tenantID, "chaos.field0", "after-db-restart") == nil
	})
}
