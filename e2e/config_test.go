//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeevdr/central-config-service/sdk/adminclient"
	"github.com/zeevdr/central-config-service/sdk/configclient"
)

// --- Full Flow: Schema → Tenant → Config → Audit (adminclient + configclient) ---

func TestFullFlow(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	// 1. Create and publish a schema.
	s, err := admin.CreateSchema(ctx, "settlement-e2e", []adminclient.Field{
		{Path: "settlement.window", Type: "FIELD_TYPE_DURATION"},
		{Path: "settlement.currency", Type: "FIELD_TYPE_STRING"},
		{Path: "settlement.fee", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	// 2. Create a tenant.
	tenant, err := admin.CreateTenant(ctx, "acme-e2e", s.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, "acme-e2e", tenant.Name)

	// Get tenant.
	got, err := admin.GetTenant(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, s.ID, got.SchemaID)

	// List tenants.
	tenants, err := admin.ListTenants(ctx, s.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(tenants), 1)

	// 3. Set config fields via configclient.
	require.NoError(t, cfg.Set(ctx, tenant.ID, "settlement.window", "24h"))

	require.NoError(t, cfg.SetMany(ctx, tenant.ID, map[string]string{
		"settlement.currency": "USD",
		"settlement.fee":      "0.5%",
	}, "Bulk config update"))

	// 4. Read config via configclient.
	allVals, err := cfg.GetAll(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "24h", allVals["settlement.window"])
	assert.Equal(t, "USD", allVals["settlement.currency"])
	assert.Equal(t, "0.5%", allVals["settlement.fee"])

	// Get single field.
	val, err := cfg.Get(ctx, tenant.ID, "settlement.currency")
	require.NoError(t, err)
	assert.Equal(t, "USD", val)

	// Get multiple fields.
	fields, err := cfg.GetFields(ctx, tenant.ID, []string{"settlement.window", "settlement.fee"})
	require.NoError(t, err)
	assert.Len(t, fields, 2)

	// 5. Snapshot reads (pinned version).
	snap, err := cfg.Snapshot(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Greater(t, snap.Version(), int32(0))

	snapVal, err := snap.Get(ctx, "settlement.fee")
	require.NoError(t, err)
	assert.Equal(t, "0.5%", snapVal)

	// 6. Config versioning via adminclient.
	versions, err := admin.ListConfigVersions(ctx, tenant.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(versions), 2)

	v, err := admin.GetConfigVersion(ctx, tenant.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, int32(1), v.Version)

	// 7. Optimistic concurrency via configclient.
	lv, err := cfg.GetForUpdate(ctx, tenant.ID, "settlement.fee")
	require.NoError(t, err)
	assert.Equal(t, "0.5%", lv.Value)

	// Set with correct checksum → succeeds.
	require.NoError(t, lv.Set(ctx, cfg, "0.3%"))

	// Set with stale checksum → fails.
	err = lv.Set(ctx, cfg, "0.1%")
	assert.ErrorIs(t, err, configclient.ErrChecksumMismatch)

	// 8. Update convenience CAS.
	require.NoError(t, cfg.Update(ctx, tenant.ID, "settlement.fee", func(current string) (string, error) {
		return "0.2%", nil
	}))
	updated, err := cfg.Get(ctx, tenant.ID, "settlement.fee")
	require.NoError(t, err)
	assert.Equal(t, "0.2%", updated)

	// 9. Rollback via adminclient.
	rv, err := admin.RollbackConfig(ctx, tenant.ID, 1, "Rollback to v1")
	require.NoError(t, err)
	assert.Greater(t, rv.Version, int32(3))

	afterRollback, err := cfg.GetAll(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "24h", afterRollback["settlement.window"])

	// 10. Field locking via adminclient.
	require.NoError(t, admin.LockField(ctx, tenant.ID, "settlement.currency"))

	locks, err := admin.ListFieldLocks(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Len(t, locks, 1)
	assert.Equal(t, "settlement.currency", locks[0].FieldPath)

	require.NoError(t, admin.UnlockField(ctx, tenant.ID, "settlement.currency"))

	locks, err = admin.ListFieldLocks(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Empty(t, locks)

	// 11. Audit log via adminclient.
	entries, err := admin.QueryWriteLog(ctx, adminclient.WithAuditTenant(tenant.ID))
	require.NoError(t, err)
	assert.Greater(t, len(entries), 0, "expected audit entries for config changes")

	// 12. Cleanup.
	require.NoError(t, admin.DeleteTenant(ctx, tenant.ID))
	require.NoError(t, admin.DeleteSchema(ctx, s.ID))
}

// --- Config Export/Import (adminclient + configclient) ---

func TestConfigExportImport(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	// 1. Create and publish a schema with typed fields.
	s, err := admin.CreateSchema(ctx, "config-export-e2e", []adminclient.Field{
		{Path: "app.enabled", Type: "FIELD_TYPE_BOOL"},
		{Path: "app.max_retries", Type: "FIELD_TYPE_INT"},
		{Path: "app.fee_rate", Type: "FIELD_TYPE_NUMBER"},
		{Path: "app.name", Type: "FIELD_TYPE_STRING"},
		{Path: "app.timeout", Type: "FIELD_TYPE_DURATION"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	// 2. Create a tenant.
	tenant, err := admin.CreateTenant(ctx, "config-export-tenant-e2e", s.ID, 1)
	require.NoError(t, err)

	// 3. Set config values via configclient.
	require.NoError(t, cfg.SetMany(ctx, tenant.ID, map[string]string{
		"app.enabled":     "true",
		"app.max_retries": "3",
		"app.fee_rate":    "0.025",
		"app.name":        "MyApp",
		"app.timeout":     "30s",
	}, "Initial config"))

	// 4. Export config via adminclient.
	yamlContent, err := admin.ExportConfig(ctx, tenant.ID, nil)
	require.NoError(t, err)
	require.NotEmpty(t, yamlContent)

	yamlStr := string(yamlContent)
	t.Logf("Exported config YAML:\n%s", yamlStr)
	assert.Contains(t, yamlStr, "value: true")
	assert.Contains(t, yamlStr, "value: 3")
	assert.Contains(t, yamlStr, "value: 0.025")

	// 5. Modify YAML and import.
	modified := bytes.Replace(yamlContent, []byte("value: 3"), []byte("value: 5"), 1)
	modified = bytes.Replace(modified, []byte("value: true"), []byte("value: false"), 1)

	v, err := admin.ImportConfig(ctx, tenant.ID, modified, "Updated via import")
	require.NoError(t, err)
	assert.Equal(t, int32(2), v.Version)

	// 6. Verify via configclient.
	vals, err := cfg.GetAll(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "false", vals["app.enabled"])
	assert.Equal(t, "5", vals["app.max_retries"])
	assert.Equal(t, "0.025", vals["app.fee_rate"])
	assert.Equal(t, "MyApp", vals["app.name"])
	assert.Equal(t, "30s", vals["app.timeout"])

	// Cleanup.
	_ = admin.DeleteTenant(ctx, tenant.ID)
	_ = admin.DeleteSchema(ctx, s.ID)
}
