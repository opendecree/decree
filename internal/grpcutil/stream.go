package grpcutil

import (
	"context"

	"google.golang.org/grpc"
)

// WrappedStream wraps a grpc.ServerStream to override its context.
type WrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

// NewWrappedStream returns a WrappedStream that substitutes the given context
// for the stream's original context.
func NewWrappedStream(ss grpc.ServerStream, ctx context.Context) *WrappedStream {
	return &WrappedStream{ServerStream: ss, ctx: ctx}
}

func (w *WrappedStream) Context() context.Context {
	return w.ctx
}
