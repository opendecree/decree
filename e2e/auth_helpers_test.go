//go:build e2e

package e2e

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/grpctransport"
)

// roleName is a stringly typed role for matrix table headers. The values
// match the x-role header strings the server accepts (see internal/auth).
type roleName string

const (
	roleSuperadmin roleName = "superadmin"
	roleAdmin      roleName = "admin"
	roleUser       roleName = "user"
)

// allRoles is the iteration order for matrix subtests.
var allRoles = []roleName{roleSuperadmin, roleAdmin, roleUser}

// clients bundles the SDK clients a matrix cell may need. The same conn is
// reused; only the metadata-header role/tenant scoping differs per cell.
type clients struct {
	conn            *grpc.ClientConn
	admin           *adminclient.Client
	cfg             *configclient.Client
	cfgTransport    *grpctransport.ConfigTransport // exposes Subscribe directly
	role            roleName
	tenantIDs       []string

	// bootstrapAdmin is always a superadmin client. Use it inside invoke
	// functions when a cell needs a throwaway resource (e.g. a schema to
	// delete) that should not depend on the role-under-test having access
	// to create it.
	bootstrapAdmin *adminclient.Client
}

// scopedClients is an alias kept for callers that read more naturally
// at the call site ("rebuild caller scoped to tenant X"). Identical to
// clientsFor.
func scopedClients(t *testing.T, conn *grpc.ClientConn, role roleName, tenantIDs ...string) *clients {
	return clientsFor(t, conn, role, tenantIDs...)
}

// clientsFor returns SDK clients with x-role and x-tenant-id metadata for
// the given role + tenant scope. Pass no tenant IDs for superadmin.
func clientsFor(t *testing.T, conn *grpc.ClientConn, role roleName, tenantIDs ...string) *clients {
	t.Helper()
	subject := fmt.Sprintf("e2e-%s", role)
	opts := []grpctransport.Option{
		grpctransport.WithSubject(subject),
		grpctransport.WithRole(string(role)),
	}
	if len(tenantIDs) > 0 {
		opts = append(opts, grpctransport.WithTenantID(strings.Join(tenantIDs, ",")))
	}
	cfgTransport := grpctransport.NewConfigTransport(conn, opts...)
	return &clients{
		conn:           conn,
		admin:          grpctransport.NewAdminClient(conn, opts...),
		cfg:            configclient.New(cfgTransport),
		cfgTransport:   cfgTransport,
		role:           role,
		tenantIDs:      tenantIDs,
		bootstrapAdmin: grpctransport.NewAdminClient(conn, grpctransport.WithSubject("e2e-bootstrap")),
	}
}

// isAuthDenied reports whether err is a gRPC status with PermissionDenied
// or Unauthenticated — the two codes the auth layer surfaces. NotFound,
// InvalidArgument, and other domain errors do NOT count: matrix 1 cares
// only about whether the auth gate fired, not whether the request was
// otherwise valid.
//
// It also treats configclient.ErrLocked as an auth denial because the
// configclient SDK collapses every codes.PermissionDenied response into
// ErrLocked (see sdk/grpctransport/errors.go:mapConfigError), losing the
// distinction between a field-lock denial and a tenant-access denial.
// Matrix tests that rely on this proxy must guarantee no actual field
// locks are in play, so any ErrLocked observed is in fact a tenant-scope
// denial.
func isAuthDenied(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, configclient.ErrLocked) {
		return true
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.PermissionDenied || st.Code() == codes.Unauthenticated
}

// matrixFixture is a shared schema + tenant used by matrix cells that need
// a real target. One fixture per test keeps wall time low — cells that
// need their own throwaway resources create them inline.
type matrixFixture struct {
	schemaID string
	tenantID string
}

// bootstrapMatrixFixture creates a schema with a representative set of
// fields, publishes v1, creates a tenant on it, and seeds one config
// version so version-history RPCs have something to read.
func bootstrapMatrixFixture(t *testing.T, namePrefix string) *matrixFixture {
	t.Helper()
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	schemaName := fmt.Sprintf("%s-%s", namePrefix, randSuffix())
	s, err := admin.CreateSchema(ctx, schemaName, []adminclient.Field{
		{Path: "app.name", Type: "FIELD_TYPE_STRING", Nullable: true},
		{Path: "app.retries", Type: "FIELD_TYPE_INT", Nullable: true},
		{Path: "app.rate", Type: "FIELD_TYPE_NUMBER"},
		{Path: "app.enabled", Type: "FIELD_TYPE_BOOL"},
		{Path: "app.timeout", Type: "FIELD_TYPE_DURATION"},
	}, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	tenantName := fmt.Sprintf("%s-tenant-%s", namePrefix, randSuffix())
	tenant, err := admin.CreateTenant(ctx, tenantName, s.ID, 1)
	require.NoError(t, err)

	// Seed one config write so RollbackToVersion / GetVersion / ListVersions
	// have content to act on.
	require.NoError(t, cfg.Set(ctx, tenant.ID, "app.name", "seed"))

	t.Cleanup(func() {
		_ = admin.DeleteTenant(ctx, tenant.ID)
		_ = admin.DeleteSchema(ctx, s.ID)
	})

	return &matrixFixture{schemaID: s.ID, tenantID: tenant.ID}
}

// randSuffix returns a short pseudo-unique suffix for schema/tenant names.
// Combines a per-process timestamp with a monotonic counter so names stay
// unique across test reruns against the same DB.
func randSuffix() string {
	return fmt.Sprintf("%d-%d", suffixEpoch, atomic.AddInt64(&suffixCounter, 1))
}

var (
	suffixCounter int64
	suffixEpoch   = time.Now().Unix()
)
