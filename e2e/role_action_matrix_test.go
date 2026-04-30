//go:build e2e

package e2e

// Matrix 1 — Role × Action.
//
// Iterates {superadmin, admin, user} × every authenticated RPC and asserts
// allow/deny. The point is to catch any new RPC that forgets to call
// auth.CheckTenantAccess or a role gate. Each cell is scoped so the caller
// HAS access to the target tenant; matrix 2 covers the out-of-scope case.
//
// Policy (docs/concepts/auth.md):
//
//   - superadmin: allowed on every RPC
//   - admin: read + write config, manage field locks; denied schema management and tenant creation
//   - user: read-only; denied all mutating RPCs
//
// Per-cell expected policy is declared in each rpcSpec.policy field.
// Update that field when adding a new RPC or changing the role policy.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
)

// allow is shorthand for a fully-allowed policy (all three roles).
var allow = map[roleName]bool{
	roleSuperadmin: true,
	roleAdmin:      true,
	roleUser:       true,
}

// superadminOnly is a policy that only superadmin can execute.
var superadminOnly = map[roleName]bool{
	roleSuperadmin: true,
	roleAdmin:      false,
	roleUser:       false,
}

// adminOrAbove is a policy that superadmin and admin can execute but user cannot.
var adminOrAbove = map[roleName]bool{
	roleSuperadmin: true,
	roleAdmin:      true,
	roleUser:       false,
}

// rpcSpec describes one RPC under test.
type rpcSpec struct {
	name string
	// invoke runs the RPC against a per-cell role-scoped clients bundle.
	// It may use bootstrapAdmin (always superadmin) to create throwaway
	// resources. The returned error is checked with isAuthDenied.
	invoke func(ctx context.Context, t *testing.T, role *clients, fx *matrixFixture) error
	// policy maps each roleName to true (expect allow) or false (expect deny).
	policy map[roleName]bool
}

// allRPCs covers every authenticated RPC across SchemaService,
// ConfigService, and AuditService. ServerService.GetServerInfo skips auth
// (see internal/auth.skipAuth) and is intentionally omitted.
func allRPCs() []rpcSpec {
	return []rpcSpec{
		// --- SchemaService — schema management ---
		{
			name:   "CreateSchema",
			policy: superadminOnly,
			invoke: func(ctx context.Context, t *testing.T, c *clients, _ *matrixFixture) error {
				_, err := c.admin.CreateSchema(ctx, fmt.Sprintf("m1-create-%s", randSuffix()), oneStringField(), "")
				return err
			},
		},
		{
			name:   "GetSchema",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.admin.GetSchema(ctx, fx.schemaID)
				return err
			},
		},
		{
			name:   "ListSchemas",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, _ *matrixFixture) error {
				_, err := c.admin.ListSchemas(ctx)
				return err
			},
		},
		{
			name:   "UpdateSchema",
			policy: superadminOnly,
			invoke: func(ctx context.Context, t *testing.T, c *clients, _ *matrixFixture) error {
				s := mustCreateThrowawaySchema(ctx, t, c.bootstrapAdmin, "m1-update")
				_, err := c.admin.UpdateSchema(ctx, s.ID,
					[]adminclient.Field{{Path: "extra", Type: "FIELD_TYPE_STRING"}}, nil, "matrix update")
				return err
			},
		},
		{
			name:   "PublishSchema",
			policy: superadminOnly,
			invoke: func(ctx context.Context, t *testing.T, c *clients, _ *matrixFixture) error {
				s := mustCreateThrowawaySchema(ctx, t, c.bootstrapAdmin, "m1-publish")
				_, err := c.admin.PublishSchema(ctx, s.ID, 1)
				return err
			},
		},
		{
			name:   "DeleteSchema",
			policy: superadminOnly,
			invoke: func(ctx context.Context, t *testing.T, c *clients, _ *matrixFixture) error {
				s := mustCreateThrowawaySchema(ctx, t, c.bootstrapAdmin, "m1-delete")
				return c.admin.DeleteSchema(ctx, s.ID)
			},
		},
		{
			name:   "ExportSchema",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.admin.ExportSchema(ctx, fx.schemaID, nil)
				return err
			},
		},
		{
			name:   "ImportSchema",
			policy: superadminOnly,
			invoke: func(ctx context.Context, t *testing.T, c *clients, _ *matrixFixture) error {
				yaml := []byte(fmt.Sprintf("name: m1-import-%s\nfields:\n  - path: x\n    type: FIELD_TYPE_STRING\n", randSuffix()))
				_, err := c.admin.ImportSchema(ctx, yaml)
				return err
			},
		},

		// --- SchemaService — tenant management ---
		{
			name:   "CreateTenant",
			policy: superadminOnly,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				// Non-superadmin callers are denied by the role check before
				// reaching the server-generated tenant ID scoping problem.
				_, err := c.admin.CreateTenant(ctx, fmt.Sprintf("m1-ct-%s", randSuffix()), fx.schemaID, 1)
				return err
			},
		},
		{
			name:   "GetTenant",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.admin.GetTenant(ctx, fx.tenantID)
				return err
			},
		},
		{
			name:   "ListTenants",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, _ *matrixFixture) error {
				_, err := c.admin.ListTenants(ctx, "")
				return err
			},
		},
		{
			name:   "UpdateTenant",
			policy: adminOrAbove,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				newName := fmt.Sprintf("m1-upd-%s", randSuffix())
				_, err := c.admin.UpdateTenant(ctx, fx.tenantID, &newName, nil)
				return err
			},
		},
		{
			name:   "DeleteTenant",
			policy: adminOrAbove,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				// Delete a throwaway tenant. Rebuild the caller's scope to
				// include the throwaway tenant UUID so CheckTenantAccess passes;
				// the role × action gate is what we are isolating here.
				// For user, RequireAdminOrAbove fires before CheckTenantAccess.
				tenant := mustCreateThrowawayTenant(ctx, t, c.bootstrapAdmin, "m1-dt", fx.schemaID)
				caller := scopedClients(t, c.conn, c.role, tenant.ID)
				return caller.admin.DeleteTenant(ctx, tenant.ID)
			},
		},
		{
			name:   "LockField",
			policy: adminOrAbove,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				field := fmt.Sprintf("app.lock-%s", randSuffix())
				// Field doesn't exist in schema, so server may return
				// InvalidArgument — that's fine, we only check auth.
				return c.admin.LockField(ctx, fx.tenantID, field)
			},
		},
		{
			name:   "UnlockField",
			policy: adminOrAbove,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				return c.admin.UnlockField(ctx, fx.tenantID, "app.name")
			},
		},
		{
			name:   "ListFieldLocks",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.admin.ListFieldLocks(ctx, fx.tenantID)
				return err
			},
		},

		// --- ConfigService ---
		{
			name:   "GetConfig",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.cfg.GetAll(ctx, fx.tenantID)
				return err
			},
		},
		{
			name:   "GetField",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.cfg.Get(ctx, fx.tenantID, "app.name")
				return err
			},
		},
		{
			name:   "GetFields",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.cfg.GetFields(ctx, fx.tenantID, []string{"app.name"})
				return err
			},
		},
		{
			name:   "SetField",
			policy: adminOrAbove,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				return c.cfg.Set(ctx, fx.tenantID, "app.name", fmt.Sprintf("m1-%s", randSuffix()))
			},
		},
		{
			name:   "SetFields",
			policy: adminOrAbove,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				return c.cfg.SetMany(ctx, fx.tenantID, map[string]string{
					"app.name": fmt.Sprintf("m1-many-%s", randSuffix()),
				}, "matrix")
			},
		},
		{
			name:   "ListVersions",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.admin.ListConfigVersions(ctx, fx.tenantID)
				return err
			},
		},
		{
			name:   "GetVersion",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.admin.GetConfigVersion(ctx, fx.tenantID, 1)
				return err
			},
		},
		{
			name:   "RollbackToVersion",
			policy: adminOrAbove,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				// Produce a second version so rollback has somewhere to go;
				// the fixture seeds version 1. For user, the Set call is also
				// denied but the error is discarded — RollbackConfig is the
				// RPC under test.
				_ = c.cfg.Set(ctx, fx.tenantID, "app.name", fmt.Sprintf("rb-%s", randSuffix()))
				_, err := c.admin.RollbackConfig(ctx, fx.tenantID, 1, "matrix rollback")
				return err
			},
		},
		{
			name:   "Subscribe",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				return invokeSubscribe(c, fx.tenantID)
			},
		},
		{
			name:   "ExportConfig",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.admin.ExportConfig(ctx, fx.tenantID, nil)
				return err
			},
		},
		{
			name:   "ImportConfig",
			policy: adminOrAbove,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				yaml := []byte(fmt.Sprintf("values:\n  app.name: import-%s\n", randSuffix()))
				_, err := c.admin.ImportConfig(ctx, fx.tenantID, yaml, "matrix import")
				return err
			},
		},

		// --- AuditService ---
		{
			name:   "QueryWriteLog",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.admin.QueryWriteLog(ctx, adminclient.WithAuditTenant(fx.tenantID))
				return err
			},
		},
		{
			name:   "GetFieldUsage",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.admin.GetFieldUsage(ctx, fx.tenantID, "app.name", nil, nil)
				return err
			},
		},
		{
			name:   "GetTenantUsage",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.admin.GetTenantUsage(ctx, fx.tenantID, nil, nil)
				return err
			},
		},
		{
			name:   "GetUnusedFields",
			policy: allow,
			invoke: func(ctx context.Context, t *testing.T, c *clients, fx *matrixFixture) error {
				_, err := c.admin.GetUnusedFields(ctx, fx.tenantID, time.Time{})
				return err
			},
		},
	}
}

func TestRoleActionMatrix(t *testing.T) {
	conn := dial(t)
	fx := bootstrapMatrixFixture(t, "m1")
	rpcs := allRPCs()

	for _, role := range allRoles {
		role := role
		t.Run(string(role), func(t *testing.T) {
			caller := buildRoleCaller(t, conn, role, fx.tenantID)
			for _, spec := range rpcs {
				spec := spec
				expectAllow := spec.policy[role]
				t.Run(spec.name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					err := spec.invoke(ctx, t, caller, fx)
					if expectAllow {
						assert.False(t, isAuthDenied(err),
							"role=%s rpc=%s expected allow; got: %v",
							role, spec.name, err)
					} else {
						assert.True(t, isAuthDenied(err),
							"role=%s rpc=%s expected auth denial; got: %v",
							role, spec.name, err)
					}
				})
			}
		})
	}
}

// buildRoleCaller returns a role-scoped clients bundle whose tenant scope
// includes the target tenant. Superadmin is built without a tenant scope.
func buildRoleCaller(t *testing.T, conn *grpc.ClientConn, role roleName, tenantID string) *clients {
	t.Helper()
	if role == roleSuperadmin {
		return scopedClients(t, conn, role)
	}
	return scopedClients(t, conn, role, tenantID)
}

// invokeSubscribe is its own function because Subscribe is a streaming RPC
// and the auth check fires when the stream is established / first message
// is read. We use a short deadline and ignore non-auth errors.
func invokeSubscribe(c *clients, tenantID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	sub, err := c.cfgTransport.Subscribe(ctx, &configclient.SubscribeRequest{TenantID: tenantID})
	if err != nil {
		return err
	}
	// First Recv either returns auth error (from server) or some other
	// (DeadlineExceeded / nil change) we treat as auth-passed.
	_, err = sub.Recv()
	return err
}

// --- shared throwaway helpers (used by matrix 1 + 2) ---

func oneStringField() []adminclient.Field {
	return []adminclient.Field{{Path: "x", Type: "FIELD_TYPE_STRING"}}
}

func mustCreateThrowawaySchema(ctx context.Context, t *testing.T, admin *adminclient.Client, prefix string) *adminclient.Schema {
	t.Helper()
	s, err := admin.CreateSchema(ctx, fmt.Sprintf("%s-%s", prefix, randSuffix()), oneStringField(), "")
	if err != nil {
		t.Fatalf("bootstrap schema: %v", err)
	}
	t.Cleanup(func() { _ = admin.DeleteSchema(context.Background(), s.ID) })
	return s
}

func mustCreateThrowawayTenant(ctx context.Context, t *testing.T, admin *adminclient.Client, prefix, schemaID string) *adminclient.Tenant {
	t.Helper()
	tenant, err := admin.CreateTenant(ctx, fmt.Sprintf("%s-%s", prefix, randSuffix()), schemaID, 1)
	if err != nil {
		t.Fatalf("bootstrap tenant: %v", err)
	}
	t.Cleanup(func() { _ = admin.DeleteTenant(context.Background(), tenant.ID) })
	return tenant
}
