package auth

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/internal/grpcutil"
)

// skipAuth returns true for gRPC methods that should bypass authentication.
//
// Methods listed here are exempt from BOTH authentication and authorization.
// Only public, side-effect-free RPCs belong here. Adding any authenticated
// method silently bypasses all auth and authz checks. Every entry requires
// an explicit security review.
//
// Allowed methods and rationale:
//   - /grpc.health.v1.Health/*             — standard liveness/readiness probes; no data access
//   - /centralconfig.v1.ServerService/GetServerInfo — read-only capability discovery (version,
//     commit, feature flags); explicitly documented as unauthenticated in the proto; no side effects
func skipAuth(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, "/grpc.health.v1.Health/") ||
		fullMethod == "/centralconfig.v1.ServerService/GetServerInfo"
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

// TenantResolver batch-resolves tenant identifiers (UUID or name slug) to UUIDs.
// The returned map must contain an entry for every input ID.
// Used by MetadataInterceptor to normalize x-tenant-id header values in one query.
type TenantResolver func(ctx context.Context, ids []string) (map[string]string, error)

// MetadataInterceptor extracts identity from gRPC metadata headers
// instead of JWT tokens. Used when JWT auth is disabled.
type MetadataInterceptor struct {
	resolveTenant             TenantResolver
	logger                    *slog.Logger
	insecureDefaultSuperadmin bool
}

// MetadataInterceptorOption configures a MetadataInterceptor.
type MetadataInterceptorOption func(*metadataInterceptorOptions)

type metadataInterceptorOptions struct {
	logger                    *slog.Logger
	insecureDefaultSuperadmin bool
}

// WithMetadataLogger sets the logger used for sanitised server-side errors.
// Defaults to slog.Default() when unset.
func WithMetadataLogger(l *slog.Logger) MetadataInterceptorOption {
	return func(o *metadataInterceptorOptions) { o.logger = l }
}

// WithInsecureDefaultSuperadmin restores the pre-v0.10 behaviour where a
// missing x-role header defaults to superadmin. Only for migration windows;
// never use in production. Set DECREE_INSECURE_DEFAULT_SUPERADMIN=1 to enable
// at runtime without code changes.
func WithInsecureDefaultSuperadmin() MetadataInterceptorOption {
	return func(o *metadataInterceptorOptions) { o.insecureDefaultSuperadmin = true }
}

// NewMetadataInterceptor creates a new metadata-based auth interceptor.
// If resolver is non-nil, tenant IDs in x-tenant-id headers are resolved
// from name slugs to UUIDs before storing in the auth context.
func NewMetadataInterceptor(resolver TenantResolver, opts ...MetadataInterceptorOption) *MetadataInterceptor {
	o := metadataInterceptorOptions{logger: slog.Default()}
	if os.Getenv("DECREE_INSECURE_DEFAULT_SUPERADMIN") == "1" {
		o.insecureDefaultSuperadmin = true
	}
	for _, opt := range opts {
		opt(&o)
	}
	m := &MetadataInterceptor{
		resolveTenant:             resolver,
		logger:                    o.logger,
		insecureDefaultSuperadmin: o.insecureDefaultSuperadmin,
	}
	if m.insecureDefaultSuperadmin {
		m.logger.Warn("DECREE_INSECURE_DEFAULT_SUPERADMIN enabled — clients without x-role get superadmin; do not use in production")
	}
	return m
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
		return handler(srv, grpcutil.NewWrappedStream(ss, newCtx))
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
		if m.insecureDefaultSuperadmin {
			role = RoleSuperAdmin
			m.logger.WarnContext(ctx, "insecure default superadmin role assigned", "subject", subject)
		} else {
			role = RoleUser
		}
	}
	switch role {
	case RoleSuperAdmin, RoleAdmin, RoleUser:
	default:
		m.logger.WarnContext(ctx, "metadata auth: unknown role", "role", rawRole, "subject", subject)
		return nil, status.Error(codes.PermissionDenied, "unknown role")
	}

	// Parse tenant IDs — comma-separated in x-tenant-id header.
	// If a resolver is configured, all IDs are resolved in a single call.
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
			tenantIDs = append(tenantIDs, id)
		}
		if m.resolveTenant != nil && len(tenantIDs) > 0 {
			resolved, err := m.resolveTenant(ctx, tenantIDs)
			if err != nil {
				m.logger.WarnContext(ctx, "metadata auth: tenant resolution failed", "error", err)
				return nil, status.Error(codes.InvalidArgument, "failed to resolve tenant")
			}
			for i, id := range tenantIDs {
				r, ok := resolved[id]
				if !ok {
					m.logger.WarnContext(ctx, "metadata auth: tenant not found", "tenant", id)
					return nil, status.Error(codes.InvalidArgument, "failed to resolve tenant")
				}
				tenantIDs[i] = r
			}
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
