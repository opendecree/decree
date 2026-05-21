package auth

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type withoutAuthKey struct{}

// WithoutAuth marks ctx as explicitly bypassing auth checks. Use only for
// legitimate internal callers (server-internal goroutines, test setup helpers).
// Every call site should be auditable — do not use to silence unexpected failures.
func WithoutAuth(ctx context.Context) context.Context {
	return context.WithValue(ctx, withoutAuthKey{}, true)
}

func isWithoutAuth(ctx context.Context) bool {
	v, _ := ctx.Value(withoutAuthKey{}).(bool)
	return v
}

// CheckTenantAccess verifies the caller has access to the given tenant.
// Returns nil for superadmins. Returns PermissionDenied if the tenant is
// not in the caller's tenant_ids list. Returns Unauthenticated when no claims
// are present unless WithoutAuth(ctx) is set.
func CheckTenantAccess(ctx context.Context, tenantID string) error {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		if isWithoutAuth(ctx) {
			return nil
		}
		return status.Error(codes.Unauthenticated, "authentication required")
	}
	if claims.HasTenantAccess(tenantID) {
		return nil
	}
	return status.Errorf(codes.PermissionDenied, "no access to tenant %s", tenantID)
}

// RequireSuperAdmin returns PermissionDenied if the caller is not superadmin.
// Returns Unauthenticated when no claims are present unless WithoutAuth(ctx) is set.
func RequireSuperAdmin(ctx context.Context) error {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		if isWithoutAuth(ctx) {
			return nil
		}
		return status.Error(codes.Unauthenticated, "authentication required")
	}
	if claims.IsSuperAdmin() {
		return nil
	}
	return status.Error(codes.PermissionDenied, "superadmin required")
}

// RequireAdminOrAbove returns PermissionDenied for the user (read-only) role.
// Returns Unauthenticated when no claims are present unless WithoutAuth(ctx) is set.
func RequireAdminOrAbove(ctx context.Context) error {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		if isWithoutAuth(ctx) {
			return nil
		}
		return status.Error(codes.Unauthenticated, "authentication required")
	}
	if claims.Role == RoleUser {
		return status.Error(codes.PermissionDenied, "admin or superadmin required")
	}
	return nil
}

// AllowedTenantIDs returns the caller's allowed tenant IDs.
// Returns nil for superadmins (meaning all tenants), and for internal callers
// marked with WithoutAuth. Callers without auth are blocked by auth guards
// before this function is typically reached.
func AllowedTenantIDs(ctx context.Context) []string {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil
	}
	if claims.IsSuperAdmin() {
		return nil
	}
	return claims.TenantIDs
}

// IsSuperAdmin reports whether the caller has the superadmin role.
// Returns true for internal callers marked with WithoutAuth.
func IsSuperAdmin(ctx context.Context) bool {
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return isWithoutAuth(ctx)
	}
	return claims.IsSuperAdmin()
}

// MustHaveClaims returns codes.Unauthenticated if no auth claims are present in ctx
// and ctx is not marked with WithoutAuth. Use in handler bodies as a defense-in-depth
// guard for RPCs that must never be reachable without authentication.
func MustHaveClaims(ctx context.Context) error {
	_, ok := ClaimsFromContext(ctx)
	if !ok && !isWithoutAuth(ctx) {
		return status.Error(codes.Unauthenticated, "authentication required")
	}
	return nil
}
