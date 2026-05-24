package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	testKey    *rsa.PrivateKey
	testKID    = "test-key-1"
	testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
)

func init() {
	var err error
	testKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(fmt.Sprintf("generate test RSA key: %v", err))
	}
}

// makeJWKS returns a JWKS JSON document for the given RSA private key and kid.
func makeJWKS(key *rsa.PrivateKey, kid string) []byte {
	e := big.NewInt(int64(key.E))
	jwks := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": kid,
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(e.Bytes()),
			},
		},
	}
	b, _ := json.Marshal(jwks)
	return b
}

// jwksJSON returns a JWKS JSON document for the test RSA public key.
func jwksJSON() []byte {
	return makeJWKS(testKey, testKID)
}

// newTestInterceptor starts an httptest JWKS server and returns an Interceptor.
// extraOpts are appended after WithIssuer and WithLogger.
func newTestInterceptor(t *testing.T, issuer string, extraOpts ...InterceptorOption) *Interceptor {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksJSON())
	}))
	t.Cleanup(srv.Close)

	opts := append([]InterceptorOption{WithIssuer(issuer), WithLogger(testLogger)}, extraOpts...)
	ctx := context.Background()
	interceptor, err := NewInterceptor(ctx, srv.URL, opts...)
	require.NoError(t, err)
	t.Cleanup(interceptor.Close)
	return interceptor
}

// signToken creates a signed JWT string with the given claims.
func signToken(t *testing.T, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = testKID
	signed, err := token.SignedString(testKey)
	require.NoError(t, err)
	return signed
}

// ctxWithBearer creates a context with gRPC incoming metadata containing the bearer token.
func ctxWithBearer(token string) context.Context {
	md := metadata.New(map[string]string{
		"authorization": "Bearer " + token,
	})
	return metadata.NewIncomingContext(context.Background(), md)
}

func validClaims(role Role, tenantIDs ...string) Claims {
	// Filter out empty strings.
	var ids []string
	for _, id := range tenantIDs {
		if id != "" {
			ids = append(ids, id)
		}
	}
	return Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Role:      role,
		TenantIDs: ids,
	}
}

// --- ClaimsFromContext ---

func TestClaimsFromContext_Roundtrip(t *testing.T) {
	claims := &Claims{Role: RoleAdmin, TenantIDs: []string{"t1"}}
	ctx := ContextWithClaims(context.Background(), claims)

	got, ok := ClaimsFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, RoleAdmin, got.Role)
	assert.Equal(t, []string{"t1"}, got.TenantIDs)
}

func TestClaimsFromContext_Missing(t *testing.T) {
	_, ok := ClaimsFromContext(context.Background())
	assert.False(t, ok)
}

// --- UnaryInterceptor ---

// noopHandler is a gRPC unary handler that returns a fixed response.
func noopHandler(_ context.Context, _ any) (any, error) {
	return "ok", nil
}

func TestUnaryInterceptor_ValidToken(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	token := signToken(t, validClaims(RoleAdmin, "tenant-1"))
	ctx := ctxWithBearer(token)

	resp, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryInterceptor_SuperadminNoTenant(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	token := signToken(t, validClaims(RoleSuperAdmin, ""))
	ctx := ctxWithBearer(token)

	resp, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryInterceptor_HealthCheckBypass(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	// No auth metadata at all — should still pass for health checks.
	ctx := context.Background()
	resp, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryInterceptor_ServerServiceBypass(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	ctx := context.Background()
	resp, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/centralconfig.v1.ServerService/GetServerInfo"}, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryInterceptor_MissingMetadata(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	ctx := context.Background()
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryInterceptor_MissingAuthorizationHeader(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	md := metadata.New(map[string]string{"other": "value"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryInterceptor_InvalidBearerFormat(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	md := metadata.New(map[string]string{"authorization": "Basic abc123"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryInterceptor_InvalidToken(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	ctx := ctxWithBearer("not-a-valid-jwt")
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryInterceptor_ExpiredToken(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
		Role:      RoleAdmin,
		TenantIDs: []string{"tenant-1"},
	}
	ctx := ctxWithBearer(signToken(t, claims))

	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryInterceptor_WrongIssuer(t *testing.T) {
	interceptor := newTestInterceptor(t, "expected-issuer")
	unary := interceptor.UnaryInterceptor()

	claims := validClaims(RoleAdmin, "tenant-1")
	claims.Issuer = "wrong-issuer"
	ctx := ctxWithBearer(signToken(t, claims))

	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryInterceptor_CorrectIssuer(t *testing.T) {
	interceptor := newTestInterceptor(t, "my-issuer")
	unary := interceptor.UnaryInterceptor()

	claims := validClaims(RoleAdmin, "tenant-1")
	claims.Issuer = "my-issuer"
	ctx := ctxWithBearer(signToken(t, claims))

	resp, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryInterceptor_UnknownRole(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksJSON())
	}))
	t.Cleanup(srv.Close)

	interceptor, err := NewInterceptor(context.Background(), srv.URL, WithLogger(logger))
	require.NoError(t, err)
	t.Cleanup(interceptor.Close)
	unary := interceptor.UnaryInterceptor()

	claims := validClaims("editor", "tenant-1")
	ctx := ctxWithBearer(signToken(t, claims))

	_, err = unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	msg := status.Convert(err).Message()
	assert.Equal(t, "unknown role", msg)
	assert.NotContains(t, msg, "editor")

	logged := logBuf.String()
	assert.Contains(t, logged, "unknown role")
	assert.Contains(t, logged, "editor")
}

func TestUnaryInterceptor_NonSuperadminMissingTenantID(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	claims := validClaims(RoleAdmin, "")
	ctx := ctxWithBearer(signToken(t, claims))

	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Contains(t, status.Convert(err).Message(), "tenant_ids required")
}

func TestUnaryInterceptor_UserRoleWithTenantID(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	token := signToken(t, validClaims(RoleUser, "tenant-1"))
	ctx := ctxWithBearer(token)

	resp, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryInterceptor_ClaimsInContext(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	token := signToken(t, validClaims(RoleAdmin, "tenant-42"))

	handler := func(ctx context.Context, req any) (any, error) {
		claims, ok := ClaimsFromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, RoleAdmin, claims.Role)
		assert.Equal(t, []string{"tenant-42"}, claims.TenantIDs)
		return "ok", nil
	}

	ctx := ctxWithBearer(token)
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, handler)
	require.NoError(t, err)
}

// --- StreamInterceptor ---

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeServerStream) Context() context.Context { return f.ctx }

func TestStreamInterceptor_ValidToken(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	stream := interceptor.StreamInterceptor()

	token := signToken(t, validClaims(RoleAdmin, "tenant-1"))
	ss := &fakeServerStream{ctx: ctxWithBearer(token)}

	var capturedCtx context.Context
	handler := func(srv any, ss grpc.ServerStream) error {
		capturedCtx = ss.Context()
		return nil
	}

	err := stream(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test.Service/Stream"}, handler)
	require.NoError(t, err)

	claims, ok := ClaimsFromContext(capturedCtx)
	require.True(t, ok)
	assert.Equal(t, RoleAdmin, claims.Role)
}

func TestStreamInterceptor_InvalidToken(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	stream := interceptor.StreamInterceptor()

	ss := &fakeServerStream{ctx: ctxWithBearer("bad-token")}

	handler := func(srv any, ss grpc.ServerStream) error {
		t.Fatal("handler should not be called")
		return nil
	}

	err := stream(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test.Service/Stream"}, handler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestStreamInterceptor_MissingAuth(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	stream := interceptor.StreamInterceptor()

	ss := &fakeServerStream{ctx: context.Background()}

	err := stream(nil, ss, &grpc.StreamServerInfo{FullMethod: "/test.Service/Stream"}, nil)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// --- WrongSigningKey ---

func TestUnaryInterceptor_WrongSigningKey(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	// Sign with a different key that the JWKS server doesn't know about.
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	claims := validClaims(RoleAdmin, "tenant-1")
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = testKID // same kid, wrong key
	signed, err := token.SignedString(otherKey)
	require.NoError(t, err)

	ctx := ctxWithBearer(signed)
	_, err = unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// --- Expiry boundary / leeway ---

func TestUnaryInterceptor_ExpiryBoundaryRejected(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
		Role:      RoleAdmin,
		TenantIDs: []string{"t1"},
	}
	ctx := ctxWithBearer(signToken(t, claims))
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryInterceptor_ExpiryWithinLeeway(t *testing.T) {
	interceptor := newTestInterceptor(t, "", WithLeeway(30*time.Second))
	unary := interceptor.UnaryInterceptor()

	// Expired 10s ago — within the 30s leeway window.
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-10 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
		Role:      RoleAdmin,
		TenantIDs: []string{"t1"},
	}
	ctx := ctxWithBearer(signToken(t, claims))
	resp, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestUnaryInterceptor_ExpiryBeyondLeeway(t *testing.T) {
	interceptor := newTestInterceptor(t, "", WithLeeway(5*time.Second))
	unary := interceptor.UnaryInterceptor()

	// Expired 60s ago — beyond the 5s leeway.
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-60 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
		Role:      RoleAdmin,
		TenantIDs: []string{"t1"},
	}
	ctx := ctxWithBearer(signToken(t, claims))
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// --- JWKS rotation ---

func TestUnaryInterceptor_JWKSRotation(t *testing.T) {
	rotatedKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const rotatedKID = "test-key-rotated"

	var mu sync.Mutex
	currentJWKS := jwksJSON()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		body := currentJWKS
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	interceptor, err := NewInterceptor(context.Background(), srv.URL, WithLogger(testLogger))
	require.NoError(t, err)
	t.Cleanup(interceptor.Close)
	unary := interceptor.UnaryInterceptor()

	// Original key works before rotation.
	ctx := ctxWithBearer(signToken(t, validClaims(RoleAdmin, "t1")))
	_, err = unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.NoError(t, err)

	// Rotate: JWKS now serves only the new key.
	mu.Lock()
	currentJWKS = makeJWKS(rotatedKey, rotatedKID)
	mu.Unlock()

	// Token signed with rotated key — keyfunc refetches on unknown kid.
	rotatedClaims := validClaims(RoleAdmin, "t1")
	rotatedToken := jwt.NewWithClaims(jwt.SigningMethodRS256, rotatedClaims)
	rotatedToken.Header["kid"] = rotatedKID
	rotatedSigned, err := rotatedToken.SignedString(rotatedKey)
	require.NoError(t, err)

	ctx = ctxWithBearer(rotatedSigned)
	_, err = unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.NoError(t, err, "new key after rotation should validate")

	// Old token (original kid, no longer in JWKS) must be rejected.
	ctx = ctxWithBearer(signToken(t, validClaims(RoleAdmin, "t1")))
	_, err = unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err, "old kid after rotation must be rejected")
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// --- Clock skew (nbf) ---

func TestUnaryInterceptor_ClockSkew_NBF_Rejected(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	// nbf 10s in the future — no leeway configured.
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now().Add(10 * time.Second)),
		},
		Role:      RoleAdmin,
		TenantIDs: []string{"t1"},
	}
	ctx := ctxWithBearer(signToken(t, claims))
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryInterceptor_ClockSkew_NBF_WithinLeeway(t *testing.T) {
	interceptor := newTestInterceptor(t, "", WithLeeway(30*time.Second))
	unary := interceptor.UnaryInterceptor()

	// nbf 10s in the future — within 30s leeway.
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now().Add(10 * time.Second)),
		},
		Role:      RoleAdmin,
		TenantIDs: []string{"t1"},
	}
	ctx := ctxWithBearer(signToken(t, claims))
	resp, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

// --- Malformed tokens ---

func TestUnaryInterceptor_MalformedToken_TruncatedHeader(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	ctx := ctxWithBearer("eyJhbGci")
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryInterceptor_MalformedToken_NonBase64Segments(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	ctx := ctxWithBearer("abc.!!!.xyz")
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryInterceptor_MalformedToken_AlgNone(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"role":"superadmin","exp":9999999999}`))
	noneToken := header + "." + payload + "."

	ctx := ctxWithBearer(noneToken)
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestUnaryInterceptor_MalformedToken_UnsupportedAlgorithm(t *testing.T) {
	interceptor := newTestInterceptor(t, "")
	unary := interceptor.UnaryInterceptor()

	claims := validClaims(RoleAdmin, "t1")
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte("some-hmac-secret"))
	require.NoError(t, err)

	ctx := ctxWithBearer(signed)
	_, err = unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
