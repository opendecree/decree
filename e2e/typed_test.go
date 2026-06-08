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

// --- Typed Values + Null Handling ---

func TestTypedValuesAndNull(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	// 1. Create schema with all types.
	s, err := admin.CreateSchema(ctx, "typed-e2e", []adminclient.Field{
		{Path: "app.retries", Type: adminclient.FieldTypeInteger, Nullable: true},
		{Path: "app.rate", Type: adminclient.FieldTypeNumber},
		{Path: "app.name", Type: adminclient.FieldTypeString, Nullable: true},
		{Path: "app.enabled", Type: adminclient.FieldTypeBool},
		{Path: "app.timeout", Type: adminclient.FieldTypeDuration},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	tenant, err := admin.CreateTenant(ctx, "typed-tenant-e2e", s.ID, 1)
	require.NoError(t, err)

	// 2. Set values with typed setters.
	require.NoError(t, noVer(cfg.SetInt(ctx, tenant.ID, "app.retries", 5)))
	require.NoError(t, noVer(cfg.SetFloat(ctx, tenant.ID, "app.rate", 0.025)))
	require.NoError(t, noVer(cfg.Set(ctx, tenant.ID, "app.name", "MyApp")))
	require.NoError(t, noVer(cfg.SetBool(ctx, tenant.ID, "app.enabled", true)))
	require.NoError(t, noVer(cfg.SetDuration(ctx, tenant.ID, "app.timeout", 30*time.Second)))

	// 3. Read with typed getters.
	retries, err := cfg.GetInt(ctx, tenant.ID, "app.retries")
	require.NoError(t, err)
	assert.Equal(t, int64(5), retries)

	rate, err := cfg.GetFloat(ctx, tenant.ID, "app.rate")
	require.NoError(t, err)
	assert.Equal(t, 0.025, rate)

	name, err := cfg.GetString(ctx, tenant.ID, "app.name")
	require.NoError(t, err)
	assert.Equal(t, "MyApp", name)

	enabled, err := cfg.GetBool(ctx, tenant.ID, "app.enabled")
	require.NoError(t, err)
	assert.True(t, enabled)

	timeout, err := cfg.GetDuration(ctx, tenant.ID, "app.timeout")
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, timeout)

	// 4. Get as string (always works).
	retriesStr, err := cfg.Get(ctx, tenant.ID, "app.retries")
	require.NoError(t, err)
	assert.Equal(t, "5", retriesStr)

	// 5. Null handling — set to null and verify.
	require.NoError(t, noVer(cfg.SetNull(ctx, tenant.ID, "app.retries")))

	// GetInt on null returns zero value.
	retriesAfterNull, err := cfg.GetInt(ctx, tenant.ID, "app.retries")
	require.NoError(t, err)
	assert.Equal(t, int64(0), retriesAfterNull)

	// GetIntNullable distinguishes null from zero.
	retriesNullable, err := cfg.GetIntNullable(ctx, tenant.ID, "app.retries")
	require.NoError(t, err)
	assert.Nil(t, retriesNullable)

	// 6. Null vs empty string distinction.
	require.NoError(t, noVer(cfg.Set(ctx, tenant.ID, "app.name", ""))) // empty string, not null
	nameNullable, err := cfg.GetStringNullable(ctx, tenant.ID, "app.name")
	require.NoError(t, err)
	require.NotNil(t, nameNullable, "empty string should not be null")
	assert.Equal(t, "", *nameNullable)

	require.NoError(t, noVer(cfg.SetNull(ctx, tenant.ID, "app.name"))) // now actually null
	nameNullable, err = cfg.GetStringNullable(ctx, tenant.ID, "app.name")
	require.NoError(t, err)
	assert.Nil(t, nameNullable, "should be null after SetNull")

	// Cleanup.
	_ = admin.DeleteTenant(ctx, tenant.ID)
	_ = admin.DeleteSchema(ctx, s.ID)
}
