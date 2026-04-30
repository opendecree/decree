package authz

import (
	"context"

	"github.com/opendecree/decree/internal/auth"
)

// RolePolicyGuard enforces minimum role requirements based on the action.
// ActionAdmin requires superadmin; ActionWrite requires admin or above; ActionRead passes unconditionally.
type RolePolicyGuard struct{}

func (RolePolicyGuard) Check(ctx context.Context, action Action, _ Resource) error {
	switch action {
	case ActionAdmin:
		return auth.RequireSuperAdmin(ctx)
	case ActionWrite:
		return auth.RequireAdminOrAbove(ctx)
	default:
		return nil
	}
}
