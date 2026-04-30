package ratelimit_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/ratelimit"
)

const testMethod = "/centralconfig.v1.ConfigService/GetConfig"

func unaryHandler(_ context.Context, _ any) (any, error) { return nil, nil }

func invokeUnary(t *testing.T, i *ratelimit.Interceptor, ctx context.Context, method string) error {
	t.Helper()
	info := &grpc.UnaryServerInfo{FullMethod: method}
	_, err := i.UnaryInterceptor()(ctx, nil, info, unaryHandler)
	return err
}

// TestWindowExhausted: burst=2 limit, (N+1)th request returns ResourceExhausted.
func TestWindowExhausted(t *testing.T) {
	const burst = 2
	lim := ratelimit.NewInProcess(rate.Limit(1), burst)
	i := ratelimit.New(ratelimit.Config{Authenticated: lim})

	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{TenantIDs: []string{"tenant-a"}})

	for n := range burst {
		err := invokeUnary(t, i, ctx, testMethod)
		require.NoError(t, err, "request %d should pass", n+1)
	}

	err := invokeUnary(t, i, ctx, testMethod)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// TestPerTenantIsolation: exhausting tenant A does not affect tenant B.
func TestPerTenantIsolation(t *testing.T) {
	lim := ratelimit.NewInProcess(rate.Limit(1), 1) // burst=1
	i := ratelimit.New(ratelimit.Config{Authenticated: lim})

	ctxA := auth.ContextWithClaims(context.Background(), &auth.Claims{TenantIDs: []string{"tenant-a"}})
	ctxB := auth.ContextWithClaims(context.Background(), &auth.Claims{TenantIDs: []string{"tenant-b"}})

	// Exhaust tenant A.
	require.NoError(t, invokeUnary(t, i, ctxA, testMethod))
	require.Error(t, invokeUnary(t, i, ctxA, testMethod), "tenant A should be exhausted")

	// Tenant B is unaffected.
	require.NoError(t, invokeUnary(t, i, ctxB, testMethod), "tenant B should still pass")
}

// TestAnonymousSeparateBucket: anonymous principal uses its own bucket, not tenant buckets.
func TestAnonymousSeparateBucket(t *testing.T) {
	authedLim := ratelimit.NewInProcess(rate.Limit(1), 1) // burst=1 for authed
	anonLim := ratelimit.NewInProcess(rate.Limit(1), 1)   // burst=1 for anon
	i := ratelimit.New(ratelimit.Config{
		Authenticated: authedLim,
		Anonymous:     anonLim,
	})

	ctxAuthed := auth.ContextWithClaims(context.Background(), &auth.Claims{TenantIDs: []string{"tenant-a"}})
	ctxAnon := context.Background() // no claims = anonymous

	// Exhaust anonymous bucket.
	require.NoError(t, invokeUnary(t, i, ctxAnon, testMethod))
	require.Error(t, invokeUnary(t, i, ctxAnon, testMethod), "anonymous should be exhausted")

	// Authenticated bucket is unaffected.
	require.NoError(t, invokeUnary(t, i, ctxAuthed, testMethod), "authenticated should still pass")
}

// TestHealthCheckExempt: health check methods bypass the rate limiter entirely.
func TestHealthCheckExempt(t *testing.T) {
	// Zero-capacity limiter — would deny everything.
	denying := ratelimit.NewInProcess(rate.Limit(0), 0)
	i := ratelimit.New(ratelimit.Config{
		Anonymous:     denying,
		Authenticated: denying,
		SuperAdmin:    denying,
	})

	ctx := context.Background()
	healthMethods := []string{
		"/grpc.health.v1.Health/Check",
		"/grpc.health.v1.Health/Watch",
	}
	for _, m := range healthMethods {
		err := invokeUnary(t, i, ctx, m)
		assert.NoError(t, err, "health method %s must be exempt", m)
	}
}
