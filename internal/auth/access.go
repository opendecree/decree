package auth

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
