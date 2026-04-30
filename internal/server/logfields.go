package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"strings"

	"google.golang.org/grpc"

	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/telemetry"
)

// logFieldsUnaryInterceptor injects tenant_id, actor, and request_id into the
// context after auth so the slog handler attaches them to every log record.
func logFieldsUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(enrichLogFields(ctx), req)
	}
}

// logFieldsStreamInterceptor is the streaming counterpart to logFieldsUnaryInterceptor.
func logFieldsStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, &logFieldsStream{ServerStream: ss, ctx: enrichLogFields(ss.Context())})
	}
}

func enrichLogFields(ctx context.Context) context.Context {
	tenantID := ""
	actor := ""
	if claims, ok := auth.ClaimsFromContext(ctx); ok {
		actor = claims.Subject
		tenantID = strings.Join(claims.TenantIDs, ",")
	}
	return telemetry.WithLogFields(ctx, tenantID, actor, newRequestID())
}

// newRequestID returns a random UUID v4.
func newRequestID() string {
	var b [16]byte
	_, _ = io.ReadFull(rand.Reader, b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type logFieldsStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *logFieldsStream) Context() context.Context { return s.ctx }
