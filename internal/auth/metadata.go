package auth

import (
	"context"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	headerSubject  = "x-subject"
	headerRole     = "x-role"
	headerTenantID = "x-tenant-id"
)

// MetadataInterceptor extracts identity from gRPC metadata headers
// instead of JWT tokens. Used when JWT auth is disabled.
type MetadataInterceptor struct{}

// NewMetadataInterceptor creates a new metadata-based auth interceptor.
func NewMetadataInterceptor() *MetadataInterceptor {
	return &MetadataInterceptor{}
}

// UnaryInterceptor returns a gRPC unary server interceptor.
func (m *MetadataInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
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

	subject := firstMetadataValue(md, headerSubject)
	if subject == "" {
		return nil, status.Error(codes.Unauthenticated, "x-subject header is required")
	}

	role := Role(firstMetadataValue(md, headerRole))
	if role == "" {
		role = RoleSuperAdmin
	}
	switch role {
	case RoleSuperAdmin, RoleAdmin, RoleUser:
	default:
		return nil, status.Errorf(codes.PermissionDenied, "unknown role: %s", role)
	}

	// Parse tenant IDs — comma-separated in x-tenant-id header.
	var tenantIDs []string
	rawTenantID := firstMetadataValue(md, headerTenantID)
	if rawTenantID != "" {
		for _, id := range strings.Split(rawTenantID, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				tenantIDs = append(tenantIDs, id)
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
