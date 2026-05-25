package authz

import (
	"context"

	"github.com/opendecree/decree/internal/auth"
)

// TenantScopeGuard verifies the caller has access to the resource tenant.
// No-ops when TenantID is empty.
type TenantScopeGuard struct{}

func (TenantScopeGuard) Check(ctx context.Context, action Action, r Resource) error {
	if action == ActionGlobal || r.TenantID == "" {
		return nil
	}
	return auth.CheckTenantAccess(ctx, r.TenantID)
}
