package auth

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Access functions in this file are permissive when no auth claims are present in the
// context. The interceptor layer (MetadataInterceptor or JWTInterceptor) is the gate
// that requires authentication before claims are set.
//
// Consequence: any method accidentally added to skipAuth bypasses not just
// authentication but all authz checks too. Handlers for RPCs that must never be
// reachable without auth should call MustHaveClaims first as a defense-in-depth guard.

// CheckTenantAccess verifies the caller has access to the given tenant.
// Returns nil for superadmins. Returns PermissionDenied if the tenant is
// not in the caller's tenant_ids list.
func CheckTenantAccess(ctx context.Context, tenantID string) error {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		// No auth context — permissive (tests, internal calls).
		return nil
	}
	if claims.HasTenantAccess(tenantID) {
		return nil
	}
	return status.Errorf(codes.PermissionDenied, "no access to tenant %s", tenantID)
}

// RequireSuperAdmin returns PermissionDenied if the caller is not superadmin.
// No-ops when no auth context is present (internal/test calls).
func RequireSuperAdmin(ctx context.Context) error {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil
	}
	if claims.IsSuperAdmin() {
		return nil
	}
	return status.Error(codes.PermissionDenied, "superadmin required")
}

// RequireAdminOrAbove returns PermissionDenied for the user (read-only) role.
// No-ops when no auth context is present (internal/test calls).
func RequireAdminOrAbove(ctx context.Context) error {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil
	}
	if claims.Role == RoleUser {
		return status.Error(codes.PermissionDenied, "admin or superadmin required")
	}
	return nil
}

// AllowedTenantIDs returns the caller's allowed tenant IDs.
// Returns nil for superadmins (meaning all tenants).
func AllowedTenantIDs(ctx context.Context) []string {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil
	}
	if claims.IsSuperAdmin() {
		return nil // nil = all tenants
	}
	return claims.TenantIDs
}

// IsSuperAdmin reports whether the caller has the superadmin role.
// Returns true when no auth context is present (permissive, consistent with other access helpers).
func IsSuperAdmin(ctx context.Context) bool {
	claims, ok := ClaimsFromContext(ctx)
	return !ok || claims.IsSuperAdmin()
}

// MustHaveClaims returns codes.Unauthenticated if no auth claims are present in ctx.
// Use in handler bodies as a defense-in-depth guard for RPCs that must never be
// reachable without authentication, even if the method is accidentally added to skipAuth.
func MustHaveClaims(ctx context.Context) error {
	_, ok := ClaimsFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "authentication required")
	}
	return nil
}
