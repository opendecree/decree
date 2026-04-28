//go:build e2e

package e2e

// Matrix 2 — Tenant-access × Write RPCs.
//
// For every write RPC that takes a tenant id, verify that:
//
//   - in-scope caller (admin role, tenant in x-tenant-id) is allowed
//   - out-of-scope caller (admin role, tenant NOT in x-tenant-id) is denied
//     with codes.PermissionDenied
//
// This is the regression net for "someone added a new write RPC and
// forgot to call auth.CheckTenantAccess at the top of the handler."

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// writeRPCSpec describes one write RPC that should enforce tenant access.
type writeRPCSpec struct {
	name string
	// invoke runs the RPC against the given tenant. The caller is the
	// supplied clients bundle; it may need to derive a fresh client to
	// reach the right scope. Returned error is checked with isAuthDenied.
	invoke func(ctx context.Context, t *testing.T, c *clients, targetTenantID, targetSchemaID string) error
}

// writeRPCs covers every write RPC the issue (decree#114) calls out.
func writeRPCs() []writeRPCSpec {
	return []writeRPCSpec{
		// Note: SetField / SetFields go through the configclient SDK
		// which maps codes.PermissionDenied to ErrLocked. isAuthDenied
		// recognizes that mapping (matrix 2 setup guarantees no real
		// field locks, so ErrLocked here is always a tenant-access
		// denial in disguise).
		{
			name: "SetField",
			invoke: func(ctx context.Context, _ *testing.T, c *clients, tenantID, _ string) error {
				return c.cfg.Set(ctx, tenantID, "app.name", fmt.Sprintf("m2-%s", randSuffix()))
			},
		},
		{
			name: "SetFields",
			invoke: func(ctx context.Context, _ *testing.T, c *clients, tenantID, _ string) error {
				return c.cfg.SetMany(ctx, tenantID, map[string]string{
					"app.name": fmt.Sprintf("m2-many-%s", randSuffix()),
				}, "matrix2")
			},
		},
		{
			name: "LockField",
			invoke: func(ctx context.Context, _ *testing.T, c *clients, tenantID, _ string) error {
				return c.admin.LockField(ctx, tenantID, "app.name")
			},
		},
		{
			name: "UnlockField",
			invoke: func(ctx context.Context, _ *testing.T, c *clients, tenantID, _ string) error {
				return c.admin.UnlockField(ctx, tenantID, "app.name")
			},
		},
		{
			name: "UpdateTenant",
			invoke: func(ctx context.Context, _ *testing.T, c *clients, tenantID, _ string) error {
				newName := fmt.Sprintf("m2-rename-%s", randSuffix())
				_, err := c.admin.UpdateTenant(ctx, tenantID, &newName, nil)
				return err
			},
		},
		{
			name: "DeleteTenant",
			invoke: func(ctx context.Context, _ *testing.T, c *clients, tenantID, _ string) error {
				return c.admin.DeleteTenant(ctx, tenantID)
			},
		},
		{
			name: "RollbackToVersion",
			invoke: func(ctx context.Context, _ *testing.T, c *clients, tenantID, _ string) error {
				_, err := c.admin.RollbackConfig(ctx, tenantID, 1, "matrix2 rollback")
				return err
			},
		},
		{
			name: "ImportConfig",
			invoke: func(ctx context.Context, _ *testing.T, c *clients, tenantID, _ string) error {
				yaml := []byte(fmt.Sprintf("values:\n  app.name: m2-import-%s\n", randSuffix()))
				_, err := c.admin.ImportConfig(ctx, tenantID, yaml, "matrix2 import")
				return err
			},
		},
	}
}

func TestTenantAccessMatrix(t *testing.T) {
	conn := dial(t)
	rpcs := writeRPCs()

	for _, spec := range rpcs {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			t.Run("in_scope_allowed", func(t *testing.T) {
				// Every cell gets a fresh tenant: some RPCs (DeleteTenant,
				// LockField/UnlockField pair, RollbackToVersion) mutate or
				// destroy state that would interfere with later cells.
				fx := bootstrapMatrixFixture(t, "m2-in")
				caller := scopedClients(t, conn, roleAdmin, fx.tenantID)

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := spec.invoke(ctx, t, caller, fx.tenantID, fx.schemaID)
				assert.False(t, isAuthDenied(err),
					"in-scope admin must not get auth denial on %s; got: %v", spec.name, err)
			})

			t.Run("out_of_scope_denied", func(t *testing.T) {
				targetFx := bootstrapMatrixFixture(t, "m2-out-target")
				otherFx := bootstrapMatrixFixture(t, "m2-out-other")

				// Caller is scoped to a DIFFERENT tenant than the target.
				caller := scopedClients(t, conn, roleAdmin, otherFx.tenantID)

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := spec.invoke(ctx, t, caller, targetFx.tenantID, targetFx.schemaID)
				assert.True(t, isAuthDenied(err),
					"out-of-scope admin must be denied on %s; got: %v", spec.name, err)
			})
		})
	}
}
