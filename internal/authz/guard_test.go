package authz_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/authz"
	"github.com/opendecree/decree/internal/storage/domain"
)

// --- helpers ---

func superAdminCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role: auth.RoleSuperAdmin,
	})
}

func adminCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: []string{tenant1},
	})
}

func userCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleUser,
		TenantIDs: []string{tenant1},
	})
}

func noAuthCtx() context.Context { return context.Background() }

const (
	tenant1 = "00000001-0000-0000-0000-000000000001"
	tenant2 = "00000002-0000-0000-0000-000000000002"
)

// --- TenantScopeGuard ---

func TestTenantScopeGuard_EmptyTenantID(t *testing.T) {
	g := authz.TenantScopeGuard{}
	err := g.Check(userCtx(), authz.ActionRead, authz.Resource{})
	require.NoError(t, err)
}

func TestTenantScopeGuard_NoAuth(t *testing.T) {
	g := authz.TenantScopeGuard{}
	err := g.Check(noAuthCtx(), authz.ActionRead, authz.Resource{TenantID: tenant1})
	require.NoError(t, err)
}

func TestTenantScopeGuard_SuperAdmin(t *testing.T) {
	g := authz.TenantScopeGuard{}
	err := g.Check(superAdminCtx(), authz.ActionRead, authz.Resource{TenantID: tenant1})
	require.NoError(t, err)
}

func TestTenantScopeGuard_MatchingTenant(t *testing.T) {
	g := authz.TenantScopeGuard{}
	err := g.Check(adminCtx(), authz.ActionRead, authz.Resource{TenantID: tenant1})
	require.NoError(t, err)
}

func TestTenantScopeGuard_WrongTenant(t *testing.T) {
	g := authz.TenantScopeGuard{}
	err := g.Check(adminCtx(), authz.ActionRead, authz.Resource{TenantID: tenant2})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

// --- RolePolicyGuard ---

func TestRolePolicyGuard_ReadAlwaysPasses(t *testing.T) {
	g := authz.RolePolicyGuard{}
	for _, ctx := range []context.Context{superAdminCtx(), adminCtx(), userCtx(), noAuthCtx()} {
		require.NoError(t, g.Check(ctx, authz.ActionRead, authz.Resource{}))
	}
}

func TestRolePolicyGuard_WriteRequiresAdmin(t *testing.T) {
	g := authz.RolePolicyGuard{}
	assert.NoError(t, g.Check(superAdminCtx(), authz.ActionWrite, authz.Resource{}))
	assert.NoError(t, g.Check(adminCtx(), authz.ActionWrite, authz.Resource{}))
	assert.NoError(t, g.Check(noAuthCtx(), authz.ActionWrite, authz.Resource{}))

	err := g.Check(userCtx(), authz.ActionWrite, authz.Resource{})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestRolePolicyGuard_AdminRequiresSuperAdmin(t *testing.T) {
	g := authz.RolePolicyGuard{}
	assert.NoError(t, g.Check(superAdminCtx(), authz.ActionAdmin, authz.Resource{}))
	assert.NoError(t, g.Check(noAuthCtx(), authz.ActionAdmin, authz.Resource{}))

	for _, ctx := range []context.Context{adminCtx(), userCtx()} {
		err := g.Check(ctx, authz.ActionAdmin, authz.Resource{})
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	}
}

// --- FieldLockGuard ---

type stubLockStore struct {
	locks []domain.TenantFieldLock
	err   error
}

func (s *stubLockStore) GetFieldLocks(_ context.Context, _ string) ([]domain.TenantFieldLock, error) {
	return s.locks, s.err
}

func TestFieldLockGuard_ReadSkipped(t *testing.T) {
	g := authz.NewFieldLockGuard(&stubLockStore{err: errors.New("should not be called")})
	err := g.Check(adminCtx(), authz.ActionRead, authz.Resource{TenantID: tenant1, FieldPath: "a.b"})
	require.NoError(t, err)
}

func TestFieldLockGuard_EmptyFieldPath(t *testing.T) {
	g := authz.NewFieldLockGuard(&stubLockStore{err: errors.New("should not be called")})
	err := g.Check(adminCtx(), authz.ActionWrite, authz.Resource{TenantID: tenant1})
	require.NoError(t, err)
}

func TestFieldLockGuard_SuperAdminBypasses(t *testing.T) {
	g := authz.NewFieldLockGuard(&stubLockStore{
		locks: []domain.TenantFieldLock{{TenantID: tenant1, FieldPath: "a.b"}},
	})
	err := g.Check(superAdminCtx(), authz.ActionWrite, authz.Resource{TenantID: tenant1, FieldPath: "a.b"})
	require.NoError(t, err)
}

func TestFieldLockGuard_NoAuthBypasses(t *testing.T) {
	g := authz.NewFieldLockGuard(&stubLockStore{
		locks: []domain.TenantFieldLock{{TenantID: tenant1, FieldPath: "a.b"}},
	})
	err := g.Check(noAuthCtx(), authz.ActionWrite, authz.Resource{TenantID: tenant1, FieldPath: "a.b"})
	require.NoError(t, err)
}

func TestFieldLockGuard_UnlockedField(t *testing.T) {
	g := authz.NewFieldLockGuard(&stubLockStore{
		locks: []domain.TenantFieldLock{{TenantID: tenant1, FieldPath: "other.field"}},
	})
	err := g.Check(adminCtx(), authz.ActionWrite, authz.Resource{TenantID: tenant1, FieldPath: "a.b"})
	require.NoError(t, err)
}

func TestFieldLockGuard_LockedField(t *testing.T) {
	g := authz.NewFieldLockGuard(&stubLockStore{
		locks: []domain.TenantFieldLock{{TenantID: tenant1, FieldPath: "a.b"}},
	})
	err := g.Check(adminCtx(), authz.ActionWrite, authz.Resource{TenantID: tenant1, FieldPath: "a.b"})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestFieldLockGuard_StoreError(t *testing.T) {
	g := authz.NewFieldLockGuard(&stubLockStore{err: errors.New("db down")})
	err := g.Check(adminCtx(), authz.ActionWrite, authz.Resource{TenantID: tenant1, FieldPath: "a.b"})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// --- ChainGuard ---

func TestChainGuard_Empty(t *testing.T) {
	chain := authz.Chain()
	require.NoError(t, chain.Check(context.Background(), authz.ActionRead, authz.Resource{}))
}

func TestChainGuard_ShortCircuitsOnError(t *testing.T) {
	// user role fails RolePolicyGuard; TenantScopeGuard never runs (wrong tenant would also fail).
	chain := authz.Chain(authz.RolePolicyGuard{}, authz.TenantScopeGuard{})
	err := chain.Check(userCtx(), authz.ActionWrite, authz.Resource{TenantID: tenant2})
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestChainGuard_AllPass(t *testing.T) {
	chain := authz.Chain(authz.TenantScopeGuard{}, authz.RolePolicyGuard{})
	err := chain.Check(adminCtx(), authz.ActionWrite, authz.Resource{TenantID: tenant1})
	require.NoError(t, err)
}
