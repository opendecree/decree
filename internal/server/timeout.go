package server

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

func defaultTimeoutUnaryInterceptor(d time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}
		return handler(ctx, req)
	}
}

func defaultTimeoutStreamInterceptor(d time.Duration) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if _, ok := ss.Context().Deadline(); !ok {
			ctx, cancel := context.WithTimeout(ss.Context(), d)
			defer cancel()
			ss = &timeoutStream{ServerStream: ss, ctx: ctx}
		}
		return handler(srv, ss)
	}
}

type timeoutStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *timeoutStream) Context() context.Context { return s.ctx }
