package adminclient

import (
	"context"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

// LockField prevents a configuration field from being modified by non-superadmin users.
// Optionally, lockedValues restricts only specific enum values from being set.
// If lockedValues is empty, the entire field is locked.
func (c *Client) LockField(ctx context.Context, tenantID, fieldPath string, lockedValues ...string) error {
	if c.schema == nil {
		return ErrServiceNotConfigured
	}
	_, err := c.schema.LockField(c.withAuth(ctx), &pb.LockFieldRequest{
		TenantId:     tenantID,
		FieldPath:    fieldPath,
		LockedValues: lockedValues,
	})
	return mapError(err)
}

// UnlockField removes a field lock, allowing modifications again.
func (c *Client) UnlockField(ctx context.Context, tenantID, fieldPath string) error {
	if c.schema == nil {
		return ErrServiceNotConfigured
	}
	_, err := c.schema.UnlockField(c.withAuth(ctx), &pb.UnlockFieldRequest{
		TenantId:  tenantID,
		FieldPath: fieldPath,
	})
	return mapError(err)
}

// ListFieldLocks returns all active field locks for a tenant.
func (c *Client) ListFieldLocks(ctx context.Context, tenantID string) ([]FieldLock, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	resp, err := c.schema.ListFieldLocks(c.withAuth(ctx), &pb.ListFieldLocksRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]FieldLock, len(resp.Locks))
	for i, l := range resp.Locks {
		result[i] = FieldLock{
			TenantID:     l.TenantId,
			FieldPath:    l.FieldPath,
			LockedValues: l.LockedValues,
		}
	}
	return result, nil
}
