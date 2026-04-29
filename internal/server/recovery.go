package server

import (
	"context"
	"log/slog"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// genericInternalError is returned to clients on panic. It deliberately omits
// the panic value so handler internals are not leaked across the trust boundary.
const genericInternalError = "internal server error"

// recoveryUnaryInterceptor returns a unary interceptor that recovers from panics
// in downstream handlers, logs the panic with a stack trace, and returns
// codes.Internal with a generic message. It must be registered as the outermost
// interceptor so it covers auth and any future middleware.
func recoveryUnaryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(ctx, "panic in unary handler",
					"method", info.FullMethod,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = status.Error(codes.Internal, genericInternalError)
			}
		}()
		return handler(ctx, req)
	}
}

// recoveryStreamInterceptor is the streaming counterpart to recoveryUnaryInterceptor.
func recoveryStreamInterceptor(logger *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(ss.Context(), "panic in stream handler",
					"method", info.FullMethod,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = status.Error(codes.Internal, genericInternalError)
			}
		}()
		return handler(srv, ss)
	}
}
