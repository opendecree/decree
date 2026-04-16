package adminclient

import (
	"context"
)

// LockField prevents a configuration field from being modified by non-superadmin users.
// Optionally, lockedValues restricts only specific enum values from being set.
// If lockedValues is empty, the entire field is locked.
func (c *Client) LockField(ctx context.Context, tenantID, fieldPath string, lockedValues ...string) error {
	if c.schema == nil {
		return ErrServiceNotConfigured
	}
	return c.schema.LockField(ctx, tenantID, fieldPath, lockedValues)
}

// UnlockField removes a field lock, allowing modifications again.
func (c *Client) UnlockField(ctx context.Context, tenantID, fieldPath string) error {
	if c.schema == nil {
		return ErrServiceNotConfigured
	}
	return c.schema.UnlockField(ctx, tenantID, fieldPath)
}

// ListFieldLocks returns all active field locks for a tenant.
func (c *Client) ListFieldLocks(ctx context.Context, tenantID string) ([]FieldLock, error) {
	if c.schema == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.schema.ListFieldLocks(ctx, tenantID)
}
