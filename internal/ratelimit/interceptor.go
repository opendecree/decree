package ratelimit

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	errdetailspb "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/opendecree/decree/internal/auth"
)

const healthServicePrefix = "/grpc.health.v1.Health/"

// Config holds per-role limiters. A nil Limiter for a role means unlimited.
type Config struct {
	// Anonymous limits unauthenticated callers via a shared global bucket per method.
	Anonymous Limiter
	// Authenticated limits tenant-scoped callers; keyed per tenant + method.
	Authenticated Limiter
	// SuperAdmin limits superadmin callers; keyed per subject + method. Nil = unlimited.
	SuperAdmin Limiter
}

// Interceptor enforces rate limits and implements server.GRPCInterceptor.
type Interceptor struct {
	cfg     Config
	logger  *slog.Logger
	counter metric.Int64Counter // nil when metrics are disabled
}

// Option configures an Interceptor.
type Option func(*Interceptor)

// WithInterceptorLogger sets the logger used for health-check debug logs.
func WithInterceptorLogger(l *slog.Logger) Option {
	return func(i *Interceptor) { i.logger = l }
}

// WithRejectedCounter sets the OTel counter incremented on every rejected request.
func WithRejectedCounter(c metric.Int64Counter) Option {
	return func(i *Interceptor) { i.counter = c }
}

// New returns a rate-limit interceptor with the given per-role limiters.
func New(cfg Config, opts ...Option) *Interceptor {
	in := &Interceptor{cfg: cfg, logger: slog.Default()}
	for _, o := range opts {
		o(in)
	}
	return in
}

// UnaryInterceptor returns a grpc.UnaryServerInterceptor that enforces rate limits.
func (i *Interceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, healthServicePrefix) {
			i.logger.DebugContext(ctx, "rate limit exempt", "method", info.FullMethod)
			return handler(ctx, req)
		}
		role, key := i.bucketKey(ctx, info.FullMethod)
		if !i.allow(role, key) {
			i.record(ctx, role, info.FullMethod)
			return nil, exhaustedErr()
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor returns a grpc.StreamServerInterceptor that enforces rate limits.
func (i *Interceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		if strings.HasPrefix(info.FullMethod, healthServicePrefix) {
			i.logger.DebugContext(ctx, "rate limit exempt", "method", info.FullMethod)
			return handler(srv, ss)
		}
		role, key := i.bucketKey(ctx, info.FullMethod)
		if !i.allow(role, key) {
			i.record(ctx, role, info.FullMethod)
			return exhaustedErr()
		}
		return handler(srv, ss)
	}
}

// bucketKey returns the role label and bucket key for the incoming request.
// Key format:
//   - anonymous:  "anon:<method>"
//   - superadmin: "sa:<subject>:<method>"
//   - tenant:     "t:<sorted_tenant_ids>:<method>"
func (i *Interceptor) bucketKey(ctx context.Context, method string) (role, key string) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return "anonymous", "anon:" + method
	}
	if claims.IsSuperAdmin() {
		return "superadmin", "sa:" + claims.Subject + ":" + method
	}
	tenantKey := strings.Join(sortedUniq(claims.TenantIDs), ",")
	return "authenticated", "t:" + tenantKey + ":" + method
}

func (i *Interceptor) allow(role, key string) bool {
	switch role {
	case "superadmin":
		return i.cfg.SuperAdmin == nil || i.cfg.SuperAdmin.Allow(key)
	case "authenticated":
		return i.cfg.Authenticated == nil || i.cfg.Authenticated.Allow(key)
	default: // anonymous
		return i.cfg.Anonymous == nil || i.cfg.Anonymous.Allow(key)
	}
}

func (i *Interceptor) record(ctx context.Context, role, method string) {
	if i.counter == nil {
		return
	}
	i.counter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("role", role),
		attribute.String("method", method),
	))
}

// exhaustedErr returns codes.ResourceExhausted with a RetryInfo detail (1 s hint).
func exhaustedErr() error {
	st := status.New(codes.ResourceExhausted, "rate limit exceeded; retry after 1s")
	if d, err := st.WithDetails(&errdetailspb.RetryInfo{
		RetryDelay: durationpb.New(time.Second),
	}); err == nil {
		return d.Err()
	}
	return st.Err()
}

func sortedUniq(ids []string) []string {
	cp := make([]string, len(ids))
	copy(cp, ids)
	slices.Sort(cp)
	return slices.Compact(cp)
}
