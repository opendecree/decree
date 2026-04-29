package auth

import (
	"context"
	"log/slog"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// skipAuth returns true for gRPC methods that should bypass authentication.
func skipAuth(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, "/grpc.health.v1.Health/") ||
		strings.HasPrefix(fullMethod, "/centralconfig.v1.ServerService/")
}

const (
	headerSubject  = "x-subject"
	headerRole     = "x-role"
	headerTenantID = "x-tenant-id"

	// Bound metadata header sizes to keep auth interceptor allocations small.
	maxHeaderLen  = 1024
	maxSubjectLen = 256
	maxTenantIDs  = 32
	maxRoleLen    = 64
)

// TenantResolver resolves a tenant identifier (UUID or name slug) to a UUID.
// Used by MetadataInterceptor to normalize x-tenant-id header values.
type TenantResolver func(ctx context.Context, idOrName string) (string, error)

// MetadataInterceptor extracts identity from gRPC metadata headers
// instead of JWT tokens. Used when JWT auth is disabled.
type MetadataInterceptor struct {
	resolveTenant TenantResolver
	logger        *slog.Logger
}

// MetadataInterceptorOption configures a MetadataInterceptor.
type MetadataInterceptorOption func(*metadataInterceptorOptions)

type metadataInterceptorOptions struct {
	logger *slog.Logger
}

// WithMetadataLogger sets the logger used for sanitised server-side errors.
// Defaults to slog.Default() when unset.
func WithMetadataLogger(l *slog.Logger) MetadataInterceptorOption {
	return func(o *metadataInterceptorOptions) { o.logger = l }
}

// NewMetadataInterceptor creates a new metadata-based auth interceptor.
// If resolver is non-nil, tenant IDs in x-tenant-id headers are resolved
// from name slugs to UUIDs before storing in the auth context.
func NewMetadataInterceptor(resolver TenantResolver, opts ...MetadataInterceptorOption) *MetadataInterceptor {
	o := metadataInterceptorOptions{logger: slog.Default()}
	for _, opt := range opts {
		opt(&o)
	}
	return &MetadataInterceptor{resolveTenant: resolver, logger: o.logger}
}

// UnaryInterceptor returns a gRPC unary server interceptor.
func (m *MetadataInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if skipAuth(info.FullMethod) {
			return handler(ctx, req)
		}
		newCtx, err := m.extractClaims(ctx)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// StreamInterceptor returns a gRPC stream server interceptor.
func (m *MetadataInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		newCtx, err := m.extractClaims(ss.Context())
		if err != nil {
			return err
		}
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: newCtx})
	}
}

func (m *MetadataInterceptor) extractClaims(ctx context.Context) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)

	rawSubject := firstMetadataValue(md, headerSubject)
	if len(rawSubject) > maxHeaderLen {
		return nil, status.Errorf(codes.InvalidArgument, "%s header exceeds %d bytes", headerSubject, maxHeaderLen)
	}
	subject := rawSubject
	if subject == "" {
		return nil, status.Error(codes.Unauthenticated, "x-subject header is required")
	}
	if len(subject) > maxSubjectLen {
		return nil, status.Errorf(codes.InvalidArgument, "%s exceeds %d bytes", headerSubject, maxSubjectLen)
	}
	if !isPrintableASCII(subject) {
		return nil, status.Errorf(codes.InvalidArgument, "%s must be printable ASCII", headerSubject)
	}

	rawRole := firstMetadataValue(md, headerRole)
	if len(rawRole) > maxRoleLen {
		return nil, status.Errorf(codes.InvalidArgument, "%s header exceeds %d bytes", headerRole, maxRoleLen)
	}
	role := Role(rawRole)
	if role == "" {
		role = RoleSuperAdmin
	}
	switch role {
	case RoleSuperAdmin, RoleAdmin, RoleUser:
	default:
		m.logger.WarnContext(ctx, "metadata auth: unknown role", "role", rawRole, "subject", subject)
		return nil, status.Error(codes.PermissionDenied, "unknown role")
	}

	// Parse tenant IDs — comma-separated in x-tenant-id header.
	// If a resolver is configured, slugs are resolved to UUIDs.
	rawTenantID := firstMetadataValue(md, headerTenantID)
	if len(rawTenantID) > maxHeaderLen {
		return nil, status.Errorf(codes.InvalidArgument, "%s header exceeds %d bytes", headerTenantID, maxHeaderLen)
	}
	var tenantIDs []string
	if rawTenantID != "" {
		parts := strings.Split(rawTenantID, ",")
		if len(parts) > maxTenantIDs {
			return nil, status.Errorf(codes.InvalidArgument, "%s exceeds %d entries", headerTenantID, maxTenantIDs)
		}
		for _, id := range parts {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if m.resolveTenant != nil {
				resolved, err := m.resolveTenant(ctx, id)
				if err != nil {
					m.logger.WarnContext(ctx, "metadata auth: tenant resolution failed", "tenant", id, "error", err)
					return nil, status.Error(codes.InvalidArgument, "failed to resolve tenant")
				}
				id = resolved
			}
			tenantIDs = append(tenantIDs, id)
		}
	}
	if role != RoleSuperAdmin && len(tenantIDs) == 0 {
		return nil, status.Error(codes.PermissionDenied, "x-tenant-id required for non-superadmin")
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: subject},
		Role:             role,
		TenantIDs:        tenantIDs,
	}

	return context.WithValue(ctx, claimsContextKey{}, claims), nil
}

func firstMetadataValue(md metadata.MD, key string) string {
	if md == nil {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// isPrintableASCII reports whether s contains only printable ASCII (0x20–0x7E).
func isPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return true
}
