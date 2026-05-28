//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/grpctransport"
)

// jwtServiceAddr returns the address of the JWT-mode service (service-jwt).
// Returns "" when the JWT e2e suite is not enabled, causing JWT tests to skip.
func jwtServiceAddr() string {
	return os.Getenv("SERVICE_JWT_ADDR")
}

// jwksServerAddr returns the HTTP address of the jwks-server admin API.
func jwksServerAddr() string {
	if addr := os.Getenv("JWKS_SERVER_ADDR"); addr != "" {
		return addr
	}
	return "localhost:8090"
}

// dialAddr dials an explicit service address and registers cleanup.
func dialAddr(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("failed to connect to %s: %v", addr, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// jwtIssuer communicates with the jwks-server admin API to mint tokens and
// trigger key rotation. It requires no local crypto deps — all key material
// stays inside the jwks-server container.
type jwtIssuer struct {
	baseURL string
	hc      *http.Client
}

// newJWTIssuer constructs a jwtIssuer pointed at jwksServerAddr() and verifies
// the JWKS endpoint is reachable.
func newJWTIssuer(t *testing.T) *jwtIssuer {
	t.Helper()
	addr := jwksServerAddr()
	baseURL := "http://" + addr
	hc := &http.Client{Timeout: 10 * time.Second}

	resp, err := hc.Get(baseURL + "/.well-known/jwks.json")
	if err != nil {
		t.Fatalf("jwks-server not reachable at %s: %v", addr, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("jwks-server returned %d for JWKS endpoint", resp.StatusCode)
	}

	return &jwtIssuer{baseURL: baseURL, hc: hc}
}

type jwtSignRequest struct {
	Role      string   `json:"role"`
	Subject   string   `json:"subject"`
	TenantIDs []string `json:"tenant_ids"`
	ExpiresIn string   `json:"expires_in"`
}

type jwtSignResponse struct {
	Token string `json:"token"`
	Kid   string `json:"kid"`
}

type jwtRotateResponse struct {
	RetiredKID string `json:"retired_kid"`
	ActiveKID  string `json:"active_kid"`
}

// mustSign mints a valid JWT for the given role and optional tenant IDs.
// The token expires in 5 minutes.
func (j *jwtIssuer) mustSign(t *testing.T, role string, tenantIDs ...string) string {
	t.Helper()
	return j.mustSignWithExpiry(t, role, "5m", tenantIDs...)
}

// mustSignExpired mints a JWT with an exp already in the past.
func (j *jwtIssuer) mustSignExpired(t *testing.T, role string) string {
	t.Helper()
	return j.mustSignWithExpiry(t, role, "-1s")
}

func (j *jwtIssuer) mustSignWithExpiry(t *testing.T, role, expiresIn string, tenantIDs ...string) string {
	t.Helper()
	req := jwtSignRequest{
		Role:      role,
		Subject:   fmt.Sprintf("e2e-%s", role),
		TenantIDs: tenantIDs,
		ExpiresIn: expiresIn,
	}
	body, _ := json.Marshal(req)
	resp, err := j.hc.Post(j.baseURL+"/admin/sign", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("sign request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sign returned %d", resp.StatusCode)
	}
	var r jwtSignResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("decode sign response: %v", err)
	}
	return r.Token
}

// rotate triggers a JWKS key rotation: a new signing key is generated and
// added to JWKS. The old key stays in JWKS for graceful token handoff.
// Returns (retiredKID, activeKID).
func (j *jwtIssuer) rotate(t *testing.T) (retiredKID, activeKID string) {
	t.Helper()
	resp, err := j.hc.Post(j.baseURL+"/admin/rotate", "application/json",
		strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("rotate request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rotate returned %d", resp.StatusCode)
	}
	var r jwtRotateResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	return r.RetiredKID, r.ActiveKID
}

// jwtScopedClients builds a clients bundle that authenticates via a JWT
// bearer token instead of metadata headers. A fresh superadmin token is
// minted for bootstrapAdmin.
func jwtScopedClients(t *testing.T, issuer *jwtIssuer, conn *grpc.ClientConn, role roleName, tenantIDs ...string) *clients {
	t.Helper()
	token := issuer.mustSign(t, string(role), tenantIDs...)
	bootstrapToken := issuer.mustSign(t, "superadmin")

	opts := []grpctransport.Option{grpctransport.WithBearerToken(token)}
	bootstrapOpts := []grpctransport.Option{grpctransport.WithBearerToken(bootstrapToken)}

	cfgTransport, err := grpctransport.NewConfigTransport(conn, opts...)
	if err != nil {
		t.Fatalf("NewConfigTransport: %v", err)
	}
	admin, err := grpctransport.NewAdminClient(conn, opts...)
	if err != nil {
		t.Fatalf("NewAdminClient: %v", err)
	}
	bootstrapAdmin, err := grpctransport.NewAdminClient(conn, bootstrapOpts...)
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

// buildJWTRoleCaller returns a role-scoped JWT clients bundle whose tenant
// scope includes the target tenant. Superadmin is built without a tenant scope.
func buildJWTRoleCaller(t *testing.T, issuer *jwtIssuer, conn *grpc.ClientConn, role roleName, tenantID string) *clients {
	t.Helper()
	if role == roleSuperadmin {
		return jwtScopedClients(t, issuer, conn, role)
	}
	return jwtScopedClients(t, issuer, conn, role, tenantID)
}

// bootstrapJWTMatrixFixture creates the shared schema + tenant fixture using
// a superadmin JWT. Mirrors bootstrapMatrixFixture but authenticates via JWT.
func bootstrapJWTMatrixFixture(t *testing.T, issuer *jwtIssuer, conn *grpc.ClientConn, namePrefix string) *matrixFixture {
	t.Helper()
	sa := jwtScopedClients(t, issuer, conn, roleSuperadmin)
	admin := sa.admin
	cfg := sa.cfg
	ctx := context.Background()

	schemaName := fmt.Sprintf("%s-%s", namePrefix, randSuffix())
	s, err := admin.CreateSchema(ctx, schemaName, []adminclient.Field{
		{Path: "app.name", Type: adminclient.FieldTypeString, Nullable: true},
		{Path: "app.retries", Type: adminclient.FieldTypeInteger, Nullable: true},
		{Path: "app.rate", Type: adminclient.FieldTypeNumber},
		{Path: "app.enabled", Type: adminclient.FieldTypeBool},
		{Path: "app.timeout", Type: adminclient.FieldTypeDuration},
	}, "")
	if err != nil {
		t.Fatalf("bootstrapJWTMatrixFixture CreateSchema: %v", err)
	}
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	if err != nil {
		t.Fatalf("bootstrapJWTMatrixFixture PublishSchema: %v", err)
	}

	tenantName := fmt.Sprintf("%s-tenant-%s", namePrefix, randSuffix())
	tenant, err := admin.CreateTenant(ctx, tenantName, s.ID, 1)
	if err != nil {
		t.Fatalf("bootstrapJWTMatrixFixture CreateTenant: %v", err)
	}

	if err := cfg.Set(ctx, tenant.ID, "app.name", "seed"); err != nil {
		t.Fatalf("bootstrapJWTMatrixFixture seed config: %v", err)
	}

	t.Cleanup(func() {
		_ = admin.DeleteTenant(context.Background(), tenant.ID)
		_ = admin.DeleteSchema(context.Background(), s.ID)
	})
	return &matrixFixture{schemaID: s.ID, tenantID: tenant.ID}
}
