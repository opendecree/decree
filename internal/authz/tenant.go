package authz

import (
	"context"

	"github.com/opendecree/decree/internal/auth"
)

// TenantScopeGuard verifies the caller has access to the resource tenant.
// No-ops when TenantID is empty.
type TenantScopeGuard struct{}

func (TenantScopeGuard) Check(ctx context.Context, _ Action, r Resource) error {
	if r.TenantID == "" {
		return nil
	}
	return auth.CheckTenantAccess(ctx, r.TenantID)
}
