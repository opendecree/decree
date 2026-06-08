//go:build e2e && upgrade

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/sdk/adminclient"
)

const (
	upgradeSchemaName = "upgrade-e2e-schema"
	upgradeTenantName = "upgrade-e2e-tenant"
)

// TestUpgrade_Populate writes fixture data via the previous-release binary.
// Run before goose migrations and before switching to the new binary.
func TestUpgrade_Populate(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	s, err := admin.CreateSchema(ctx, upgradeSchemaName, []adminclient.Field{
		{Path: "app.timeout", Type: adminclient.FieldTypeDuration},
		{Path: "app.region", Type: adminclient.FieldTypeString},
	}, "upgrade e2e fixture")
	require.NoError(t, err)

	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	tenant, err := admin.CreateTenant(ctx, upgradeTenantName, s.ID, 1)
	require.NoError(t, err)

	require.NoError(t, noVer(cfg.SetDuration(ctx, tenant.ID, "app.timeout", 30*time.Second)))
	require.NoError(t, noVer(cfg.Set(ctx, tenant.ID, "app.region", "us-east-1")))
}

// TestUpgrade_Assert verifies fixture data is intact after goose migrations,
// and that the new binary accepts new writes on the migrated schema.
func TestUpgrade_Assert(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	// Locate schema by name.
	schemaID, schemaVersion, err := admin.GetLatestPublishedSchemaVersion(ctx, upgradeSchemaName)
	require.NoError(t, err)
	assert.Equal(t, int32(1), schemaVersion)

	// Locate tenant by name.
	tenants, err := admin.ListTenants(ctx, schemaID)
	require.NoError(t, err)
	var tenantID string
	for _, tnt := range tenants {
		if tnt.Name == upgradeTenantName {
			tenantID = tnt.ID
			break
		}
	}
	require.NotEmpty(t, tenantID, "tenant %q not found after migration", upgradeTenantName)

	// Verify config values survived migration.
	vals, err := cfg.GetAll(ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "30s", vals["app.timeout"])
	assert.Equal(t, "us-east-1", vals["app.region"])

	// Verify new writes succeed on the migrated server.
	require.NoError(t, noVer(cfg.Set(ctx, tenantID, "app.region", "eu-west-1")))
	updated, err := cfg.Get(ctx, tenantID, "app.region")
	require.NoError(t, err)
	assert.Equal(t, "eu-west-1", updated)
}
