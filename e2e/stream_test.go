//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"github.com/zeevdr/central-config-service/sdk/adminclient"
)

// --- Streaming Subscription (raw proto — Subscribe is not in configclient SDK) ---

func TestConfigSubscription(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	configSvc := pb.NewConfigServiceClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	// Setup: schema + tenant via adminclient.
	s, err := admin.CreateSchema(ctx, "stream-e2e", []adminclient.Field{
		{Path: "notify.enabled", Type: "FIELD_TYPE_STRING"},
		{Path: "notify.channel", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	tenant, err := admin.CreateTenant(ctx, "stream-tenant-e2e", s.ID, 1)
	require.NoError(t, err)

	// Subscribe with raw proto client (not available in SDK).
	subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	stream, err := configSvc.Subscribe(subCtx, &pb.SubscribeRequest{
		TenantId:   tenant.ID,
		FieldPaths: []string{"notify.enabled"},
	})
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	// Write via configclient SDK.
	require.NoError(t, cfg.Set(ctx, tenant.ID, "notify.enabled", "true"))

	// Read from raw stream.
	change, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "notify.enabled", change.Change.FieldPath)
	assert.Equal(t, "true", change.Change.NewValue)

	cancel()

	// Cleanup.
	require.NoError(t, admin.DeleteTenant(context.Background(), tenant.ID))
	require.NoError(t, admin.DeleteSchema(context.Background(), s.ID))
}
