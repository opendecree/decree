package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCheckTenantAccess_NoClaims_Permissive(t *testing.T) {
	// No claims in context — permissive (tests, internal calls).
	assert.NoError(t, CheckTenantAccess(context.Background(), "t1"))
}

func TestCheckTenantAccess_Superadmin_AllowsAny(t *testing.T) {
	ctx := ContextWithClaims(context.Background(), &Claims{Role: RoleSuperAdmin})
	assert.NoError(t, CheckTenantAccess(ctx, "any-tenant"))
}

func TestCheckTenantAccess_Admin_AllowsListed(t *testing.T) {
	ctx := ContextWithClaims(context.Background(), &Claims{
		Role:      RoleAdmin,
		TenantIDs: []string{"t1", "t2"},
	})
	assert.NoError(t, CheckTenantAccess(ctx, "t1"))
	assert.NoError(t, CheckTenantAccess(ctx, "t2"))
}

func TestCheckTenantAccess_Admin_RejectsUnlisted(t *testing.T) {
	ctx := ContextWithClaims(context.Background(), &Claims{
		Role:      RoleAdmin,
		TenantIDs: []string{"t1"},
	})
	err := CheckTenantAccess(ctx, "other")
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestAllowedTenantIDs_NoClaims_Nil(t *testing.T) {
	assert.Nil(t, AllowedTenantIDs(context.Background()))
}

func TestAllowedTenantIDs_Superadmin_Nil(t *testing.T) {
	ctx := ContextWithClaims(context.Background(), &Claims{Role: RoleSuperAdmin})
	assert.Nil(t, AllowedTenantIDs(ctx), "superadmin gets nil meaning all tenants")
}

func TestAllowedTenantIDs_Admin_ReturnsList(t *testing.T) {
	ctx := ContextWithClaims(context.Background(), &Claims{
		Role:      RoleAdmin,
		TenantIDs: []string{"t1", "t2"},
	})
	assert.Equal(t, []string{"t1", "t2"}, AllowedTenantIDs(ctx))
}

func TestClaims_HasTenantAccess_Superadmin(t *testing.T) {
	c := &Claims{Role: RoleSuperAdmin}
	assert.True(t, c.HasTenantAccess("anything"))
}

func TestClaims_HasTenantAccess_ListedTenant(t *testing.T) {
	c := &Claims{Role: RoleUser, TenantIDs: []string{"a", "b"}}
	assert.True(t, c.HasTenantAccess("a"))
	assert.True(t, c.HasTenantAccess("b"))
	assert.False(t, c.HasTenantAccess("c"))
}

func TestClaims_IsSuperAdmin(t *testing.T) {
	assert.True(t, (&Claims{Role: RoleSuperAdmin}).IsSuperAdmin())
	assert.False(t, (&Claims{Role: RoleAdmin}).IsSuperAdmin())
	assert.False(t, (&Claims{Role: RoleUser}).IsSuperAdmin())
}
