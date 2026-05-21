package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCheckTenantAccess_NoClaims_Denied(t *testing.T) {
	err := CheckTenantAccess(context.Background(), "t1")
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestCheckTenantAccess_WithoutAuth_Allowed(t *testing.T) {
	assert.NoError(t, CheckTenantAccess(WithoutAuth(context.Background()), "t1"))
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

func TestMustHaveClaims_NoClaims(t *testing.T) {
	err := MustHaveClaims(context.Background())
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestMustHaveClaims_WithoutAuth(t *testing.T) {
	assert.NoError(t, MustHaveClaims(WithoutAuth(context.Background())))
}

func TestMustHaveClaims_WithClaims(t *testing.T) {
	ctx := ContextWithClaims(context.Background(), &Claims{Role: RoleSuperAdmin})
	assert.NoError(t, MustHaveClaims(ctx))
}

func TestRequireSuperAdmin(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		wantErr codes.Code
	}{
		{"no claims — denied", context.Background(), codes.Unauthenticated},
		{"WithoutAuth — allowed", WithoutAuth(context.Background()), codes.OK},
		{"superadmin — allowed", ContextWithClaims(context.Background(), &Claims{Role: RoleSuperAdmin}), codes.OK},
		{"admin — denied", ContextWithClaims(context.Background(), &Claims{Role: RoleAdmin}), codes.PermissionDenied},
		{"user — denied", ContextWithClaims(context.Background(), &Claims{Role: RoleUser}), codes.PermissionDenied},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireSuperAdmin(tc.ctx)
			if tc.wantErr == codes.OK {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tc.wantErr, status.Code(err))
			}
		})
	}
}

func TestRequireAdminOrAbove(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		wantErr codes.Code
	}{
		{"no claims — denied", context.Background(), codes.Unauthenticated},
		{"WithoutAuth — allowed", WithoutAuth(context.Background()), codes.OK},
		{"superadmin — allowed", ContextWithClaims(context.Background(), &Claims{Role: RoleSuperAdmin}), codes.OK},
		{"admin — allowed", ContextWithClaims(context.Background(), &Claims{Role: RoleAdmin}), codes.OK},
		{"user — denied", ContextWithClaims(context.Background(), &Claims{Role: RoleUser}), codes.PermissionDenied},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := RequireAdminOrAbove(tc.ctx)
			if tc.wantErr == codes.OK {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tc.wantErr, status.Code(err))
			}
		})
	}
}

func TestIsSuperAdmin(t *testing.T) {
	assert.False(t, IsSuperAdmin(context.Background()), "no claims — denied")
	assert.True(t, IsSuperAdmin(WithoutAuth(context.Background())), "WithoutAuth — allowed")
	assert.True(t, IsSuperAdmin(ContextWithClaims(context.Background(), &Claims{Role: RoleSuperAdmin})))
	assert.False(t, IsSuperAdmin(ContextWithClaims(context.Background(), &Claims{Role: RoleAdmin})))
	assert.False(t, IsSuperAdmin(ContextWithClaims(context.Background(), &Claims{Role: RoleUser})))
}
