package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func ctxWithMetadata(kvs map[string]string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.New(kvs))
}

// --- UnaryInterceptor ---

func TestMetadata_ValidSuperadmin(t *testing.T) {
	interceptor := NewMetadataInterceptor()
	unary := interceptor.UnaryInterceptor()

	ctx := ctxWithMetadata(map[string]string{"x-subject": "admin@example.com"})
	resp, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestMetadata_DefaultsToSuperadmin(t *testing.T) {
	interceptor := NewMetadataInterceptor()
	unary := interceptor.UnaryInterceptor()

	ctx := ctxWithMetadata(map[string]string{"x-subject": "user@example.com"})

	var captured *Claims
	handler := func(ctx context.Context, _ any) (any, error) {
		captured, _ = ClaimsFromContext(ctx)
		return "ok", nil
	}

	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, handler)
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, "user@example.com", captured.Subject)
	assert.Equal(t, RoleSuperAdmin, captured.Role)
	assert.Empty(t, captured.TenantIDs)
}

func TestMetadata_AdminWithTenant(t *testing.T) {
	interceptor := NewMetadataInterceptor()
	unary := interceptor.UnaryInterceptor()

	ctx := ctxWithMetadata(map[string]string{
		"x-subject":   "admin@example.com",
		"x-role":      "admin",
		"x-tenant-id": "tenant-123",
	})

	var captured *Claims
	handler := func(ctx context.Context, _ any) (any, error) {
		captured, _ = ClaimsFromContext(ctx)
		return "ok", nil
	}

	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, handler)
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, "admin@example.com", captured.Subject)
	assert.Equal(t, RoleAdmin, captured.Role)
	assert.Equal(t, []string{"tenant-123"}, captured.TenantIDs)
}

func TestMetadata_UserWithTenant(t *testing.T) {
	interceptor := NewMetadataInterceptor()
	unary := interceptor.UnaryInterceptor()

	ctx := ctxWithMetadata(map[string]string{
		"x-subject":   "user@example.com",
		"x-role":      "user",
		"x-tenant-id": "tenant-456",
	})

	var captured *Claims
	handler := func(ctx context.Context, _ any) (any, error) {
		captured, _ = ClaimsFromContext(ctx)
		return "ok", nil
	}

	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, handler)
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, RoleUser, captured.Role)
}

func TestMetadata_MissingSubject(t *testing.T) {
	interceptor := NewMetadataInterceptor()
	unary := interceptor.UnaryInterceptor()

	ctx := ctxWithMetadata(map[string]string{"x-role": "admin"})
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "x-subject")
}

func TestMetadata_NoMetadata(t *testing.T) {
	interceptor := NewMetadataInterceptor()
	unary := interceptor.UnaryInterceptor()

	ctx := context.Background()
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestMetadata_UnknownRole(t *testing.T) {
	interceptor := NewMetadataInterceptor()
	unary := interceptor.UnaryInterceptor()

	ctx := ctxWithMetadata(map[string]string{
		"x-subject": "user@example.com",
		"x-role":    "editor",
	})
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)

	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Contains(t, err.Error(), "unknown role")
}

func TestMetadata_AdminMissingTenant(t *testing.T) {
	interceptor := NewMetadataInterceptor()
	unary := interceptor.UnaryInterceptor()

	ctx := ctxWithMetadata(map[string]string{
		"x-subject": "admin@example.com",
		"x-role":    "admin",
	})
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)

	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Contains(t, err.Error(), "x-tenant-id")
}

func TestMetadata_UserMissingTenant(t *testing.T) {
	interceptor := NewMetadataInterceptor()
	unary := interceptor.UnaryInterceptor()

	ctx := ctxWithMetadata(map[string]string{
		"x-subject": "user@example.com",
		"x-role":    "user",
	})
	_, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, noopHandler)

	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestMetadata_HealthCheckBypass(t *testing.T) {
	interceptor := NewMetadataInterceptor()
	unary := interceptor.UnaryInterceptor()

	// No headers at all — health checks skip auth.
	ctx := context.Background()
	resp, err := unary(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}, noopHandler)

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestMetadata_StreamInterceptor(t *testing.T) {
	interceptor := NewMetadataInterceptor()
	stream := interceptor.StreamInterceptor()

	ctx := ctxWithMetadata(map[string]string{
		"x-subject": "streamer@example.com",
		"x-role":    "superadmin",
	})

	var captured *Claims
	handler := func(_ any, ss grpc.ServerStream) error {
		captured, _ = ClaimsFromContext(ss.Context())
		return nil
	}

	err := stream(nil, &fakeServerStream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: "/test.Service/Stream"}, handler)
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, "streamer@example.com", captured.Subject)
	assert.Equal(t, RoleSuperAdmin, captured.Role)
}
