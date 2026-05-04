package server

import (
	"log/slog"

	"google.golang.org/grpc"
)

// Option configures a Server.
type Option func(*options)

type options struct {
	enableServices   []string
	logger           *slog.Logger
	extraOptions     []grpc.ServerOption
	maxRecvMsgBytes  int
	maxSendMsgBytes  int
	tls              *TLSConfig
	insecure         bool
	rateLimiter      GRPCInterceptor // optional; runs after auth
	enableReflection bool
}

// WithLogger sets the server logger. Defaults to slog.Default() when unset.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.logger = l }
}

// WithEnableServices lists which services the server treats as enabled
// (used by IsServiceEnabled to gate registration).
func WithEnableServices(svcs []string) Option {
	return func(o *options) { o.enableServices = svcs }
}

// WithGRPCServerOptions appends extra grpc.ServerOption values applied
// alongside the built-in interceptors and message-size caps.
func WithGRPCServerOptions(opts ...grpc.ServerOption) Option {
	return func(o *options) { o.extraOptions = append(o.extraOptions, opts...) }
}

// WithMaxRecvMsgBytes caps inbound gRPC message size. Zero or negative falls
// back to DefaultMaxMsgBytes.
func WithMaxRecvMsgBytes(n int) Option {
	return func(o *options) { o.maxRecvMsgBytes = n }
}

// WithMaxSendMsgBytes caps outbound gRPC message size. Zero or negative falls
// back to DefaultMaxMsgBytes.
func WithMaxSendMsgBytes(n int) Option {
	return func(o *options) { o.maxSendMsgBytes = n }
}

// WithTLS enables transport security with the given config. Required unless
// WithInsecure is set; mutually exclusive with WithInsecure.
func WithTLS(cfg *TLSConfig) Option {
	return func(o *options) { o.tls = cfg }
}

// WithInsecure listens in plaintext. Intended for local dev only
// (INSECURE_LISTEN=1). Mutually exclusive with WithTLS.
func WithInsecure() Option {
	return func(o *options) { o.insecure = true }
}

// WithRateLimiter adds a rate-limit interceptor that runs after authentication.
// Pass nil to disable rate limiting.
func WithRateLimiter(rl GRPCInterceptor) Option {
	return func(o *options) { o.rateLimiter = rl }
}

// WithReflection enables gRPC server reflection. Off by default; enable for
// local dev or tooling environments (grpcurl, grpc-gateway introspection).
func WithReflection() Option {
	return func(o *options) { o.enableReflection = true }
}
