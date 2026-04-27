//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/grpctransport"
)

// newRestrictedAdminClient builds an adminclient with x-role=admin and an
// x-tenant-id header restricted to the given tenant IDs (comma-joined).
// This exercises the non-superadmin code paths in ListTenants.
func newRestrictedAdminClient(conn *grpc.ClientConn, tenantIDs []string) *adminclient.Client {
	return grpctransport.NewAdminClient(conn,
		grpctransport.WithSubject("e2e-restricted"),
		grpctransport.WithRole("admin"),
		grpctransport.WithTenantID(strings.Join(tenantIDs, ",")),
	)
}

// --- UpdateTenant: rename only ---

func TestUpdateTenantName(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	ctx := context.Background()

	s, err := admin.CreateSchema(ctx, "rename-tenant-e2e", []adminclient.Field{
		{Path: "x", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	tenant, err := admin.CreateTenant(ctx, "tenant-rename-before", s.ID, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = admin.DeleteTenant(ctx, tenant.ID)
		_ = admin.DeleteSchema(ctx, s.ID)
	})

	updated, err := admin.UpdateTenantName(ctx, tenant.ID, "tenant-rename-after")
	require.NoError(t, err)
	assert.Equal(t, "tenant-rename-after", updated.Name)
	assert.Equal(t, tenant.ID, updated.ID)

	got, err := admin.GetTenant(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, "tenant-rename-after", got.Name)
}

// --- UpdateTenant: schema version upgrade invalidates the validator cache ---

func TestUpdateTenantSchemaVersion(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	// v1 has only `app.value` (string).
	s, err := admin.CreateSchema(ctx, "tenant-upgrade-e2e", []adminclient.Field{
		{Path: "app.value", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	// v2 drops `app.value`, adds `app.count`.
	_, err = admin.UpdateSchema(ctx, s.ID,
		[]adminclient.Field{{Path: "app.count", Type: "FIELD_TYPE_INT"}},
		[]string{"app.value"},
		"v2: swap fields",
	)
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 2)
	require.NoError(t, err)

	// Tenant on v1, set the v1-only field — populates the validator cache.
	tenant, err := admin.CreateTenant(ctx, "tenant-upgrade", s.ID, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = admin.DeleteTenant(ctx, tenant.ID)
		_ = admin.DeleteSchema(ctx, s.ID)
	})
	require.NoError(t, cfg.Set(ctx, tenant.ID, "app.value", "v1"))

	// Upgrade to v2.
	updated, err := admin.UpdateTenantSchema(ctx, tenant.ID, 2)
	require.NoError(t, err)
	assert.Equal(t, int32(2), updated.SchemaVersion)

	// Validator cache must reflect v2: setting v1's `app.value` now fails;
	// setting v2's `app.count` succeeds.
	err = cfg.Set(ctx, tenant.ID, "app.value", "still-v1")
	require.Error(t, err, "setting dropped field on upgraded tenant must fail")

	require.NoError(t, cfg.SetInt(ctx, tenant.ID, "app.count", 7))
	count, err := cfg.GetInt(ctx, tenant.ID, "app.count")
	require.NoError(t, err)
	assert.Equal(t, int64(7), count)
}

// --- UpdateTenant: rename + upgrade in a single call ---

func TestUpdateTenantBothFields(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	ctx := context.Background()

	s, err := admin.CreateSchema(ctx, "tenant-both-e2e", []adminclient.Field{
		{Path: "k", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	_, err = admin.UpdateSchema(ctx, s.ID,
		[]adminclient.Field{{Path: "k2", Type: "FIELD_TYPE_STRING"}},
		nil,
		"v2: add k2",
	)
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 2)
	require.NoError(t, err)

	tenant, err := admin.CreateTenant(ctx, "tenant-both-before", s.ID, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = admin.DeleteTenant(ctx, tenant.ID)
		_ = admin.DeleteSchema(ctx, s.ID)
	})

	newName := "tenant-both-after"
	newVersion := int32(2)
	updated, err := admin.UpdateTenant(ctx, tenant.ID, &newName, &newVersion)
	require.NoError(t, err)
	assert.Equal(t, newName, updated.Name)
	assert.Equal(t, newVersion, updated.SchemaVersion)
}

// --- ListTenants restricted-claim filtering (with and without SchemaId) ---

func TestListTenantsWithAccessFiltering(t *testing.T) {
	conn := dial(t)
	admin := newAdminClient(conn)
	ctx := context.Background()

	// Two schemas so we can also test the SchemaId-filtered path.
	sA, err := admin.CreateSchema(ctx, "tenant-filter-a", []adminclient.Field{
		{Path: "f", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, sA.ID, 1)
	require.NoError(t, err)

	sB, err := admin.CreateSchema(ctx, "tenant-filter-b", []adminclient.Field{
		{Path: "f", Type: "FIELD_TYPE_STRING"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, sB.ID, 1)
	require.NoError(t, err)

	tA1, err := admin.CreateTenant(ctx, "filter-a-1", sA.ID, 1)
	require.NoError(t, err)
	tA2, err := admin.CreateTenant(ctx, "filter-a-2", sA.ID, 1)
	require.NoError(t, err)
	tB1, err := admin.CreateTenant(ctx, "filter-b-1", sB.ID, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = admin.DeleteTenant(ctx, tA1.ID)
		_ = admin.DeleteTenant(ctx, tA2.ID)
		_ = admin.DeleteTenant(ctx, tB1.ID)
		_ = admin.DeleteSchema(ctx, sA.ID)
		_ = admin.DeleteSchema(ctx, sB.ID)
	})

	// Restrict the caller to {tA1, tB1}: tA2 must be filtered out.
	allowed := []string{tA1.ID, tB1.ID}
	restricted := newRestrictedAdminClient(conn, allowed)

	// No schema filter → exercises ListTenantsByIDs path.
	all, err := restricted.ListTenants(ctx, "")
	require.NoError(t, err)
	gotAll := tenantIDSet(all)
	assert.Contains(t, gotAll, tA1.ID)
	assert.Contains(t, gotAll, tB1.ID)
	assert.NotContains(t, gotAll, tA2.ID, "tA2 was not in the allowed set")

	// Schema filter → exercises ListTenantsBySchemaAndIDs path.
	bySchemaA, err := restricted.ListTenants(ctx, sA.ID)
	require.NoError(t, err)
	gotA := tenantIDSet(bySchemaA)
	assert.Contains(t, gotA, tA1.ID)
	assert.NotContains(t, gotA, tA2.ID, "tA2 was not in the allowed set")
	assert.NotContains(t, gotA, tB1.ID, "tB1 belongs to schema B")
}

func tenantIDSet(tenants []*adminclient.Tenant) map[string]struct{} {
	out := make(map[string]struct{}, len(tenants))
	for _, t := range tenants {
		out[t.ID] = struct{}{}
	}
	return out
}
