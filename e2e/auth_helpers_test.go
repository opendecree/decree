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
	conn         *grpc.ClientConn
	admin        *adminclient.Client
	cfg          *configclient.Client
	cfgTransport *grpctransport.ConfigTransport // exposes Subscribe directly
	role         roleName
	tenantIDs    []string

	// bootstrapAdmin is always a superadmin client. Use it inside invoke
	// functions when a cell needs a throwaway resource (e.g. a schema to
	// delete) that should not depend on the role-under-test having access
	// to create it.
	bootstrapAdmin *adminclient.Client
}

// scopedClients returns SDK clients with x-role and x-tenant-id metadata
// for the given role + tenant scope. Pass no tenant IDs for superadmin.
func scopedClients(t *testing.T, conn *grpc.ClientConn, role roleName, tenantIDs ...string) *clients {
	t.Helper()
	subject := fmt.Sprintf("e2e-%s", role)
	opts := []grpctransport.Option{
		grpctransport.WithSubject(subject),
		grpctransport.WithRole(string(role)),
	}
	if len(tenantIDs) > 0 {
		opts = append(opts, grpctransport.WithTenantID(strings.Join(tenantIDs, ",")))
	}
	cfgTransport, err := grpctransport.NewConfigTransport(conn, opts...)
	if err != nil {
		t.Fatalf("NewConfigTransport: %v", err)
	}
	admin, err := grpctransport.NewAdminClient(conn, opts...)
	if err != nil {
		t.Fatalf("NewAdminClient: %v", err)
	}
	bootstrapAdmin, err := grpctransport.NewAdminClient(conn, grpctransport.WithSubject("e2e-bootstrap"), grpctransport.WithRole("superadmin"))
	if err != nil {
		t.Fatalf("NewAdminClient (bootstrap): %v", err)
	}
	return &clients{
		conn:           conn,
		admin:          admin,
		cfg:            configclient.New(cfgTransport),
		cfgTransport:   cfgTransport,
		role:           role,
		tenantIDs:      tenantIDs,
		bootstrapAdmin: bootstrapAdmin,
	}
}

// isAuthDenied reports whether err is a gRPC status that indicates an auth
// gate fired. The auth layer surfaces:
//   - codes.Unauthenticated — no valid credentials
//   - codes.PermissionDenied — role or non-tenant-scoped access denied
//   - codes.NotFound — tenant-scoped access denied, collapsed from
//     PermissionDenied to prevent slug enumeration (see decree#454)
//
// InvalidArgument and other domain errors do NOT count: matrices care only
// about whether the auth gate fired, not whether the request was otherwise
// valid. In the tenant-access matrices the fixture always creates the target
// tenant, so NotFound can only mean the access check fired (not a real miss).
//
// It also treats configclient.ErrPermissionDenied as an auth denial.
// The configclient SDK maps codes.PermissionDenied to ErrPermissionDenied
// (role / tenant-access denial) and codes.FailedPrecondition to ErrLocked
// (field-lock denial). Both matrices that invoke this helper guarantee no
// active field locks on the target fields, so ErrPermissionDenied here
// always means a genuine auth gate fired.
func isAuthDenied(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, configclient.ErrPermissionDenied) {
		return true
	}
	// adminclient SDK maps codes.NotFound → adminclient.ErrNotFound (a plain sentinel,
	// not a gRPC status). Accept it as an auth denial: tenant-scoped RPCs return
	// NotFound for access denials to prevent slug enumeration (decree#454).
	if errors.Is(err, adminclient.ErrNotFound) {
		return true
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.PermissionDenied || st.Code() == codes.Unauthenticated || st.Code() == codes.NotFound
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
		{Path: "app.name", Type: adminclient.FieldTypeString, Nullable: true},
		{Path: "app.retries", Type: adminclient.FieldTypeInteger, Nullable: true},
		{Path: "app.rate", Type: adminclient.FieldTypeNumber},
		{Path: "app.enabled", Type: adminclient.FieldTypeBool},
		{Path: "app.timeout", Type: adminclient.FieldTypeDuration},
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
