//go:build e2e

package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zeevdr/central-config-service/sdk/adminclient"
	"github.com/zeevdr/central-config-service/sdk/configclient"
)

// --- Error Cases (SDK sentinel errors) ---

func TestErrorCases(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	t.Run("get nonexistent schema", func(t *testing.T) {
		_, err := admin.GetSchema(ctx, "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, adminclient.ErrNotFound)
	})

	t.Run("create tenant with unpublished schema", func(t *testing.T) {
		s, err := admin.CreateSchema(ctx, "unpublished-e2e", []adminclient.Field{
			{Path: "x", Type: "FIELD_TYPE_STRING"},
		}, "")
		require.NoError(t, err)

		_, err = admin.CreateTenant(ctx, "bad-tenant-e2e", s.ID, 1)
		assert.ErrorIs(t, err, adminclient.ErrFailedPrecondition)

		_ = admin.DeleteSchema(ctx, s.ID)
	})

	t.Run("get nonexistent config field", func(t *testing.T) {
		// Create schema + tenant to have a valid tenant ID.
		s, err := admin.CreateSchema(ctx, "err-cfg-e2e", []adminclient.Field{
			{Path: "x", Type: "FIELD_TYPE_STRING"},
		}, "")
		require.NoError(t, err)
		_, err = admin.PublishSchema(ctx, s.ID, 1)
		require.NoError(t, err)
		tenant, err := admin.CreateTenant(ctx, "err-cfg-tenant-e2e", s.ID, 1)
		require.NoError(t, err)

		_, err = cfg.Get(ctx, tenant.ID, "nonexistent.field")
		assert.ErrorIs(t, err, configclient.ErrNotFound)

		_ = admin.DeleteTenant(ctx, tenant.ID)
		_ = admin.DeleteSchema(ctx, s.ID)
	})
}
