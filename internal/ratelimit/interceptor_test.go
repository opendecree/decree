package ratelimit_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
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

// fakeStream is a minimal grpc.ServerStream for testing StreamInterceptor.
type fakeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeStream) Context() context.Context { return f.ctx }
func (f *fakeStream) SendMsg(_ any) error      { return nil }
func (f *fakeStream) RecvMsg(_ any) error      { return nil }

func streamHandler(_ any, _ grpc.ServerStream) error { return nil }

func invokeStream(t *testing.T, i *ratelimit.Interceptor, ctx context.Context, method string) error {
	t.Helper()
	info := &grpc.StreamServerInfo{FullMethod: method}
	return i.StreamInterceptor()(nil, &fakeStream{ctx: ctx}, info, streamHandler)
}

// TestStream_UnderLimit: burst=2, first 2 stream calls pass.
func TestStream_UnderLimit(t *testing.T) {
	const burst = 2
	lim := ratelimit.NewInProcess(rate.Limit(1), burst)
	i := ratelimit.New(ratelimit.Config{Authenticated: lim})

	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{TenantIDs: []string{"tenant-a"}})

	for n := range burst {
		err := invokeStream(t, i, ctx, testMethod)
		require.NoError(t, err, "stream request %d should pass", n+1)
	}
}

// TestStream_OverLimit: burst=2, 3rd stream call returns ResourceExhausted.
func TestStream_OverLimit(t *testing.T) {
	const burst = 2
	lim := ratelimit.NewInProcess(rate.Limit(1), burst)
	i := ratelimit.New(ratelimit.Config{Authenticated: lim})

	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{TenantIDs: []string{"tenant-a"}})

	for n := range burst {
		err := invokeStream(t, i, ctx, testMethod)
		require.NoError(t, err, "stream request %d should pass", n+1)
	}

	err := invokeStream(t, i, ctx, testMethod)
	require.Error(t, err)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// TestStream_PerTenantIsolation: exhausting tenant A does not affect tenant B.
func TestStream_PerTenantIsolation(t *testing.T) {
	lim := ratelimit.NewInProcess(rate.Limit(1), 1) // burst=1
	i := ratelimit.New(ratelimit.Config{Authenticated: lim})

	ctxA := auth.ContextWithClaims(context.Background(), &auth.Claims{TenantIDs: []string{"tenant-a"}})
	ctxB := auth.ContextWithClaims(context.Background(), &auth.Claims{TenantIDs: []string{"tenant-b"}})

	// Exhaust tenant A.
	require.NoError(t, invokeStream(t, i, ctxA, testMethod))
	require.Error(t, invokeStream(t, i, ctxA, testMethod), "tenant A should be exhausted")

	// Tenant B is unaffected.
	require.NoError(t, invokeStream(t, i, ctxB, testMethod), "tenant B should still pass")
}

// TestStream_PerMethodIsolation: exhausting /foo does not block /bar.
func TestStream_PerMethodIsolation(t *testing.T) {
	lim := ratelimit.NewInProcess(rate.Limit(1), 1) // burst=1 per bucket
	i := ratelimit.New(ratelimit.Config{Authenticated: lim})

	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{TenantIDs: []string{"tenant-a"}})

	const methodFoo = "/centralconfig.v1.ConfigService/Foo"
	const methodBar = "/centralconfig.v1.ConfigService/Bar"

	// Exhaust /foo bucket.
	require.NoError(t, invokeStream(t, i, ctx, methodFoo))
	require.Error(t, invokeStream(t, i, ctx, methodFoo), "/foo should be exhausted")

	// /bar bucket is independent.
	require.NoError(t, invokeStream(t, i, ctx, methodBar), "/bar should still pass")
}

// TestStream_HealthCheckExempt: health check methods bypass a zero-capacity limiter.
func TestStream_HealthCheckExempt(t *testing.T) {
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
		err := invokeStream(t, i, ctx, m)
		assert.NoError(t, err, "health stream method %s must be exempt", m)
	}
}

// TestGlobalLimiterBlocksAllRoles: exhausting the global method bucket blocks
// subsequent callers regardless of their per-role limit.
func TestGlobalLimiterBlocksAllRoles(t *testing.T) {
	i := ratelimit.New(ratelimit.Config{
		Global:        ratelimit.NewInProcess(rate.Limit(1), 1), // burst=1 for the method
		Anonymous:     ratelimit.NewInProcess(rate.Limit(100), 100),
		Authenticated: ratelimit.NewInProcess(rate.Limit(100), 100),
		SuperAdmin:    ratelimit.NewInProcess(rate.Limit(100), 100),
	})

	ctxAnon := context.Background()
	ctxAuthed := auth.ContextWithClaims(context.Background(), &auth.Claims{TenantIDs: []string{"t1"}})

	require.NoError(t, invokeUnary(t, i, ctxAnon, testMethod), "first request should pass")

	err := invokeUnary(t, i, ctxAuthed, testMethod)
	require.Error(t, err, "second request (different role) should be blocked by global limit")
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// TestGlobalLimiterCapsSuperAdmin: superadmin with nil per-role limiter (unlimited)
// is still capped by the global limiter.
func TestGlobalLimiterCapsSuperAdmin(t *testing.T) {
	i := ratelimit.New(ratelimit.Config{
		Global:        ratelimit.NewInProcess(rate.Limit(1), 1),
		Authenticated: ratelimit.NewInProcess(rate.Limit(100), 100),
		// SuperAdmin intentionally nil = unlimited per-role
	})

	ctxSA := auth.ContextWithClaims(context.Background(), &auth.Claims{Role: auth.RoleSuperAdmin})

	require.NoError(t, invokeUnary(t, i, ctxSA, testMethod), "first superadmin request should pass")
	err := invokeUnary(t, i, ctxSA, testMethod)
	require.Error(t, err, "second superadmin request should be blocked by global limit")
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// TestAnonPerIPIsolation: two anonymous callers from different IPs have separate
// buckets and do not starve each other.
func TestAnonPerIPIsolation(t *testing.T) {
	lim := ratelimit.NewInProcess(rate.Limit(1), 1) // burst=1 per bucket
	i := ratelimit.New(ratelimit.Config{Anonymous: lim})

	ctxIP1 := peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 50000},
	})
	ctxIP2 := peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("5.6.7.8"), Port: 50001},
	})

	// Exhaust IP1.
	require.NoError(t, invokeUnary(t, i, ctxIP1, testMethod))
	require.Error(t, invokeUnary(t, i, ctxIP1, testMethod), "IP1 should be exhausted")

	// IP2 is unaffected.
	require.NoError(t, invokeUnary(t, i, ctxIP2, testMethod), "IP2 should still pass")
}

// TestStream_MessageMeteringExhausted: per-message rate limiting kicks in after
// the connect-time token is consumed — the (burst)th SendMsg call is rejected.
func TestStream_MessageMeteringExhausted(t *testing.T) {
	const burst = 3 // connect uses 1 token; 2 messages pass; 3rd message is rejected
	lim := ratelimit.NewInProcess(rate.Limit(1), burst)
	i := ratelimit.New(ratelimit.Config{Authenticated: lim})

	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{TenantIDs: []string{"tenant-a"}})

	sendCount := 0
	handler := func(_ any, ss grpc.ServerStream) error {
		for range burst {
			if err := ss.SendMsg(struct{}{}); err != nil {
				return err
			}
			sendCount++
		}
		return nil
	}

	info := &grpc.StreamServerInfo{FullMethod: testMethod}
	err := i.StreamInterceptor()(nil, &fakeStream{ctx: ctx}, info, handler)
	require.Error(t, err, "stream should be terminated when message limit is hit")
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	// connect-time uses 1 token; (burst-1) messages succeed before exhaustion
	assert.Equal(t, burst-1, sendCount, "expected %d messages before exhaustion", burst-1)
}

// TestAnonTrustedProxy: when TrustedProxy is set, x-forwarded-for is used as the IP key
// so two callers with different x-forwarded-for values have independent buckets.
func TestAnonTrustedProxy(t *testing.T) {
	lim := ratelimit.NewInProcess(rate.Limit(1), 1) // burst=1 per bucket
	i := ratelimit.New(ratelimit.Config{
		Anonymous:    lim,
		TrustedProxy: true,
	})

	ctxIP1 := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-forwarded-for", "10.0.0.1"))
	ctxIP2 := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-forwarded-for", "10.0.0.2"))

	// Exhaust IP1 bucket.
	require.NoError(t, invokeUnary(t, i, ctxIP1, testMethod))
	require.Error(t, invokeUnary(t, i, ctxIP1, testMethod), "IP1 via x-forwarded-for should be exhausted")

	// IP2 bucket is independent.
	require.NoError(t, invokeUnary(t, i, ctxIP2, testMethod), "IP2 via x-forwarded-for should still pass")
}
