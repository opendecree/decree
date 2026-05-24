//go:build e2e

package e2e

// JWT auth e2e suite — decree#470.
//
// Requires:
//
//	SERVICE_JWT_ADDR=localhost:9091   (service-jwt in docker-compose.yml)
//	JWKS_SERVER_ADDR=localhost:8090  (jwks-server in docker-compose.yml, optional)
//
// All tests are skipped when SERVICE_JWT_ADDR is unset. Run via:
//
//	make e2e-jwt

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/grpctransport"
)

// TestJWTRoleActionMatrix mirrors TestRoleActionMatrix but authenticates every
// call via a signed JWT bearer token. The policy table is identical; this test
// verifies the JWT interceptor enforces the same RBAC rules as metadata auth.
func TestJWTRoleActionMatrix(t *testing.T) {
	if jwtServiceAddr() == "" {
		t.Skip("skip: SERVICE_JWT_ADDR not set — run via `make e2e-jwt`")
	}

	issuer := newJWTIssuer(t)
	conn := dialAddr(t, jwtServiceAddr())
	fx := bootstrapJWTMatrixFixture(t, issuer, conn, "jm1")
	rpcs := allRPCs()

	for _, role := range allRoles {
		role := role
		t.Run(string(role), func(t *testing.T) {
			caller := buildJWTRoleCaller(t, issuer, conn, role, fx.tenantID)
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

// TestJWTTenantAccessMatrix mirrors TestTenantAccessMatrix: for every write RPC,
// verifies that an in-scope JWT caller is allowed and an out-of-scope JWT caller
// is denied, irrespective of the role expressed in the token.
func TestJWTTenantAccessMatrix(t *testing.T) {
	if jwtServiceAddr() == "" {
		t.Skip("skip: SERVICE_JWT_ADDR not set — run via `make e2e-jwt`")
	}

	issuer := newJWTIssuer(t)
	conn := dialAddr(t, jwtServiceAddr())
	rpcs := writeRPCs()

	for _, spec := range rpcs {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			t.Run("in_scope_allowed", func(t *testing.T) {
				fx := bootstrapJWTMatrixFixture(t, issuer, conn, "jm2-in")
				caller := jwtScopedClients(t, issuer, conn, roleAdmin, fx.tenantID)

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := spec.invoke(ctx, t, caller, fx.tenantID, fx.schemaID)
				assert.False(t, isAuthDenied(err),
					"in-scope JWT admin must not get auth denial on %s; got: %v", spec.name, err)
			})

			t.Run("out_of_scope_denied", func(t *testing.T) {
				targetFx := bootstrapJWTMatrixFixture(t, issuer, conn, "jm2-out-target")
				otherFx := bootstrapJWTMatrixFixture(t, issuer, conn, "jm2-out-other")

				caller := jwtScopedClients(t, issuer, conn, roleAdmin, otherFx.tenantID)

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				err := spec.invoke(ctx, t, caller, targetFx.tenantID, targetFx.schemaID)
				assert.True(t, isAuthDenied(err),
					"out-of-scope JWT admin must be denied on %s; got: %v", spec.name, err)
			})
		})
	}
}

// TestJWTRotation verifies graceful key rotation:
//
//  1. Mint T1 with the initial key K1 — verify it works.
//  2. Rotate JWKS: K2 becomes the active signing key; K1 stays in JWKS.
//  3. Mint T2 with K2 — verify it works.
//  4. T1 (signed with K1) must still work: K1 is still in JWKS during the
//     grace period.
func TestJWTRotation(t *testing.T) {
	if jwtServiceAddr() == "" {
		t.Skip("skip: SERVICE_JWT_ADDR not set — run via `make e2e-jwt`")
	}

	issuer := newJWTIssuer(t)
	conn := dialAddr(t, jwtServiceAddr())
	ctx := context.Background()

	// Step 1: mint T1 with K1 and verify it works.
	t1 := issuer.mustSign(t, "superadmin")
	c1 := jwtClientWithToken(t, conn, t1)
	_, err := c1.admin.ListSchemas(ctx)
	require.NoError(t, err, "T1 (K1) must work before rotation")

	// Step 2: rotate — K2 becomes active, K1 remains in JWKS.
	retiredKID, activeKID := issuer.rotate(t)
	t.Logf("rotated: retired=%s active=%s", retiredKID, activeKID)

	// Step 3: mint T2 with K2 and verify it works.
	c2 := jwtScopedClients(t, issuer, conn, roleSuperadmin)
	_, err = c2.admin.ListSchemas(ctx)
	require.NoError(t, err, "T2 (K2) must work after rotation")

	// Step 4: T1 (signed with K1) must still work — K1 is still in JWKS.
	_, err = c1.admin.ListSchemas(ctx)
	require.NoError(t, err, "T1 (K1) must still work while K1 is in JWKS (grace period)")
}

// TestJWTExpiredToken verifies that a token with exp in the past is rejected
// with codes.Unauthenticated before any handler logic runs.
func TestJWTExpiredToken(t *testing.T) {
	if jwtServiceAddr() == "" {
		t.Skip("skip: SERVICE_JWT_ADDR not set — run via `make e2e-jwt`")
	}

	issuer := newJWTIssuer(t)
	conn := dialAddr(t, jwtServiceAddr())

	expiredToken := issuer.mustSignExpired(t, "superadmin")
	c := jwtClientWithToken(t, conn, expiredToken)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.admin.ListSchemas(ctx)
	require.Error(t, err, "expired token must be rejected")

	st, ok := status.FromError(err)
	require.True(t, ok, "error must be a gRPC status")
	assert.Equal(t, codes.Unauthenticated, st.Code(),
		"expired token must produce Unauthenticated, got %v", st.Code())
}

// TestJWTServerInfo verifies that the service-jwt reports jwt_auth=true.
func TestJWTServerInfo(t *testing.T) {
	if jwtServiceAddr() == "" {
		t.Skip("skip: SERVICE_JWT_ADDR not set — run via `make e2e-jwt`")
	}

	conn := dialAddr(t, jwtServiceAddr())
	issuer := newJWTIssuer(t)
	sa := jwtScopedClients(t, issuer, conn, roleSuperadmin)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := sa.admin.GetServerInfo(ctx)
	require.NoError(t, err)
	assert.True(t, info.Features["jwt_auth"], "service-jwt must advertise jwt_auth=true")
}

// jwtClientWithToken builds a minimal clients bundle authenticated by a
// pre-minted token string. Used to verify a specific token still works (or
// fails) without asking the issuer to mint a new one.
func jwtClientWithToken(t *testing.T, conn *grpc.ClientConn, token string) *clients {
	t.Helper()
	opts := []grpctransport.Option{grpctransport.WithBearerToken(token)}
	cfgTransport, err := grpctransport.NewConfigTransport(conn, opts...)
	if err != nil {
		t.Fatalf("NewConfigTransport: %v", err)
	}
	admin, err := grpctransport.NewAdminClient(conn, opts...)
	if err != nil {
		t.Fatalf("NewAdminClient: %v", err)
	}
	return &clients{
		conn:         conn,
		admin:        admin,
		cfg:          configclient.New(cfgTransport),
		cfgTransport: cfgTransport,
		role:         roleSuperadmin,
	}
}
