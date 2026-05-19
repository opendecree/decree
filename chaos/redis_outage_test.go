//go:build chaos

package chaos

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// TestRedisOutage_CacheFallback pauses Redis and asserts that:
//   - GetAll falls back to the DB (service.go:221: cache miss on error → store read)
//   - SetField succeeds (DB write; cache.Set failure is warn-only at service.go:274)
func TestRedisOutage_CacheFallback(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	schemaID, cleanupSchema := makeSchema(t, admin, "redis-cache-schema")
	t.Cleanup(cleanupSchema)
	tenantID, cleanupTenant := makeTenant(t, admin, "redis-cache-tenant", schemaID)
	t.Cleanup(cleanupTenant)

	// Seed a value — written to DB and cached in Redis.
	require.NoError(t, cfg.Set(ctx, tenantID, "chaos.field0", "seeded"))

	// Pause Redis.
	containerPause(t, redisContainer())
	t.Cleanup(func() { containerUnpause(t, redisContainer()) })

	// GetAll must return the seeded value via DB fallback (cache returns error → store read).
	readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
	defer readCancel()
	vals, err := cfg.GetAll(readCtx, tenantID)
	require.NoError(t, err, "GetAll must succeed via DB fallback while Redis is paused")
	assert.Equal(t, "seeded", vals["chaos.field0"])

	// SetField must succeed (DB write; Redis cache.Set and Invalidate are warn-only).
	writeCtx, writeCancel := context.WithTimeout(ctx, 10*time.Second)
	defer writeCancel()
	require.NoError(t, cfg.Set(writeCtx, tenantID, "chaos.field0", "written-during-outage"),
		"SetField must succeed while Redis is paused")

	// Verify the new value is readable via DB fallback.
	verifyCtx, verifyCancel := context.WithTimeout(ctx, 10*time.Second)
	defer verifyCancel()
	vals, err = cfg.GetAll(verifyCtx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "written-during-outage", vals["chaos.field0"])

	// Unpause Redis — subsequent requests use cache again (transparent to caller).
	containerUnpause(t, redisContainer())
}

// TestRedisOutage_PubSubDegraded pauses Redis and asserts that:
//   - Subscribe returns codes.Internal (subscriber.Subscribe fails; service.go:818)
//   - The server does not crash (SetField still works)
//   - Subscribe recovers after Redis comes back
func TestRedisOutage_PubSubDegraded(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	schemaID, cleanupSchema := makeSchema(t, admin, "redis-pubsub-schema")
	t.Cleanup(cleanupSchema)
	tenantID, cleanupTenant := makeTenant(t, admin, "redis-pubsub-tenant", schemaID)
	t.Cleanup(cleanupTenant)

	rawConn := dial(t)
	configSvc := newRawConfigClient(rawConn)

	// Pause Redis.
	containerPause(t, redisContainer())
	t.Cleanup(func() { containerUnpause(t, redisContainer()) })

	// Subscribe must fail with codes.Internal (Redis Subscribe times out at service.go:816-818).
	subCtx, subCancel := context.WithTimeout(
		metadata.NewOutgoingContext(ctx, metadata.Pairs(
			"x-subject", "chaos-test",
			"x-role", "superadmin",
		)),
		10*time.Second,
	)
	defer subCancel()
	stream, err := configSvc.Subscribe(subCtx, &pb.SubscribeRequest{
		TenantId:   tenantID,
		FieldPaths: []string{"chaos.field0"},
	})
	// Subscribe may fail at call time or on first Recv (streaming semantics).
	if err == nil {
		_, err = stream.Recv()
	}
	require.Error(t, err, "Subscribe must fail while Redis is paused")
	st, ok := status.FromError(err)
	require.True(t, ok, "expected gRPC status error: %T %v", err, err)
	assert.Equal(t, codes.Internal, st.Code())

	// Server must still serve non-pubsub requests (no crash).
	writeCtx, writeCancel := context.WithTimeout(ctx, 10*time.Second)
	defer writeCancel()
	require.NoError(t, cfg.Set(writeCtx, tenantID, "chaos.field0", "set-during-redis-outage"),
		"SetField must work while Redis is paused (publish failure is warn-only)")

	// Unpause Redis.
	containerUnpause(t, redisContainer())

	// Subscribe must recover: attempt succeeds (no codes.Internal) within 15s.
	// A Canceled code is acceptable — it means Redis subscribed successfully and
	// the context expired before a change event arrived.
	eventually(t, 15*time.Second, func() bool {
		rCtx, rCancel := context.WithTimeout(
			metadata.NewOutgoingContext(ctx, metadata.Pairs(
				"x-subject", "chaos-test",
				"x-role", "superadmin",
			)),
			2*time.Second,
		)
		defer rCancel()
		s, subErr := configSvc.Subscribe(rCtx, &pb.SubscribeRequest{
			TenantId:   tenantID,
			FieldPaths: []string{"chaos.field0"},
		})
		if subErr != nil {
			return false
		}
		_, recvErr := s.Recv()
		// codes.Canceled = context expired = Redis is up (subscription succeeded).
		// codes.Internal = Redis still failing.
		return status.Code(recvErr) != codes.Internal
	})
}
