//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/sdk/adminclient"
)

func TestUsageTracking(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	// 1. Create schema + tenant + set values.
	s, err := admin.CreateSchema(ctx, "usage-e2e", []adminclient.Field{
		{Path: "billing.fee", Type: "FIELD_TYPE_STRING"},
		{Path: "billing.currency", Type: "FIELD_TYPE_STRING"},
		{Path: "billing.unused", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	tenant, err := admin.CreateTenant(ctx, "usage-tenant-e2e", s.ID, 1)
	require.NoError(t, err)

	require.NoError(t, cfg.Set(ctx, tenant.ID, "billing.fee", "2.5%"))
	require.NoError(t, cfg.Set(ctx, tenant.ID, "billing.currency", "USD"))
	require.NoError(t, cfg.Set(ctx, tenant.ID, "billing.unused", "n/a"))

	// 2. Read fields — triggers usage recording.
	_, err = cfg.Get(ctx, tenant.ID, "billing.fee")
	require.NoError(t, err)
	_, err = cfg.Get(ctx, tenant.ID, "billing.fee")
	require.NoError(t, err)
	_, err = cfg.Get(ctx, tenant.ID, "billing.currency")
	require.NoError(t, err)

	// Also read via GetAll (records all fields).
	_, err = cfg.GetAll(ctx, tenant.ID)
	require.NoError(t, err)

	// 3. Wait for flush (docker-compose sets USAGE_FLUSH_INTERVAL=1s).
	time.Sleep(3 * time.Second)

	// 4. Query usage via admin SDK.
	feeUsage, err := admin.GetFieldUsage(ctx, tenant.ID, "billing.fee", nil, nil)
	require.NoError(t, err)
	// 2 direct Get + 1 GetAll = 3 reads.
	assert.GreaterOrEqual(t, feeUsage.ReadCount, int64(3), "billing.fee should have at least 3 reads")

	currencyUsage, err := admin.GetFieldUsage(ctx, tenant.ID, "billing.currency", nil, nil)
	require.NoError(t, err)
	// 1 direct Get + 1 GetAll = 2 reads.
	assert.GreaterOrEqual(t, currencyUsage.ReadCount, int64(2), "billing.currency should have at least 2 reads")

	// 5. Tenant-wide usage.
	tenantUsage, err := admin.GetTenantUsage(ctx, tenant.ID, nil, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(tenantUsage), 3, "should have stats for all 3 fields")

	// 6. Unused fields — billing.unused was never read individually (only via GetAll).
	// After GetAll it has reads, so query for fields not read in the last second.
	// Since all fields were read (GetAll reads everything), unused should be empty.
	unusedFields, err := admin.GetUnusedFields(ctx, tenant.ID, time.Now().Add(-1*time.Hour))
	require.NoError(t, err)
	// All fields were touched via GetAll, so none should be unused.
	assert.Empty(t, unusedFields, "all fields were read via GetAll")
}
