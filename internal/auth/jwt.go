package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Role represents a user's role in the system.
type Role string

const (
	RoleSuperAdmin Role = "superadmin"
	RoleAdmin      Role = "admin"
	RoleUser       Role = "user"
)

// Claims represents the JWT claims used by the service.
type Claims struct {
	jwt.RegisteredClaims
	Role      Role     `json:"role"`
	TenantIDs []string `json:"tenant_ids"`
}

// HasTenantAccess checks if the caller has access to a specific tenant.
// Superadmins have access to all tenants.
func (c *Claims) HasTenantAccess(tenantID string) bool {
	if c.Role == RoleSuperAdmin {
		return true
	}
	for _, id := range c.TenantIDs {
		if id == tenantID {
			return true
		}
	}
	return false
}

// IsSuperAdmin returns true if the caller has the superadmin role.
func (c *Claims) IsSuperAdmin() bool {
	return c.Role == RoleSuperAdmin
}

type claimsContextKey struct{}

// ClaimsFromContext extracts Claims from the context.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsContextKey{}).(*Claims)
	return c, ok
}

// ContextWithClaims returns a new context with the given claims. Intended for testing.
func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

// Interceptor provides gRPC interceptors for JWT authentication.
type Interceptor struct {
	jwks       keyfunc.Keyfunc
	jwksCancel context.CancelFunc
	issuer     string
	logger     *slog.Logger
}

// NewInterceptor creates a new auth interceptor.
// jwksURL is the JWKS endpoint for key discovery. issuer is optional.
func NewInterceptor(ctx context.Context, jwksURL, issuer string, logger *slog.Logger) (*Interceptor, error) {
	jwksCtx, jwksCancel := context.WithCancel(ctx)
	jwks, err := keyfunc.NewDefaultCtx(jwksCtx, []string{jwksURL})
	if err != nil {
		jwksCancel()
		return nil, fmt.Errorf("create jwks keyfunc: %w", err)
	}

	return &Interceptor{
		jwks:       jwks,
		jwksCancel: jwksCancel,
		issuer:     issuer,
		logger:     logger,
	}, nil
}

// Close cleans up the JWKS background goroutine.
func (i *Interceptor) Close() {
	i.jwksCancel()
}

// UnaryInterceptor returns a gRPC unary server interceptor.
func (i *Interceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Skip auth for health checks.
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return handler(ctx, req)
		}

		newCtx, err := i.authenticate(ctx)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// StreamInterceptor returns a gRPC stream server interceptor.
func (i *Interceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		newCtx, err := i.authenticate(ss.Context())
		if err != nil {
			return err
		}
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: newCtx})
	}
}

func (i *Interceptor) authenticate(ctx context.Context) (context.Context, error) {
	token, err := extractBearerToken(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	claims := &Claims{}
	opts := []jwt.ParserOption{jwt.WithValidMethods([]string{"RS256", "ES256"})}
	if i.issuer != "" {
		opts = append(opts, jwt.WithIssuer(i.issuer))
	}

	parsed, err := jwt.ParseWithClaims(token, claims, i.jwks.KeyfuncCtx(ctx), opts...)
	if err != nil {
		i.logger.DebugContext(ctx, "jwt validation failed", "error", err)
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	if !parsed.Valid {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	// Validate role.
	switch claims.Role {
	case RoleSuperAdmin, RoleAdmin, RoleUser:
	default:
		return nil, status.Errorf(codes.PermissionDenied, "unknown role: %s", claims.Role)
	}

	// Non-superadmin must have at least one tenant_id.
	if claims.Role != RoleSuperAdmin && len(claims.TenantIDs) == 0 {
		return nil, status.Error(codes.PermissionDenied, "tenant_ids required for non-superadmin")
	}

	return context.WithValue(ctx, claimsContextKey{}, claims), nil
}

func extractBearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("missing metadata")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", fmt.Errorf("missing authorization header")
	}
	token := values[0]
	if !strings.HasPrefix(token, "Bearer ") {
		return "", fmt.Errorf("invalid authorization format")
	}
	return strings.TrimPrefix(token, "Bearer "), nil
}

// wrappedStream wraps a grpc.ServerStream to override the context.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}
