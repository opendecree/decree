package server

import (
	"context"
	"log/slog"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/telemetry"
)

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// logCapture is a minimal slog.Handler that records handled records.
type logCapture struct{ records []slog.Record }

func (c *logCapture) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (c *logCapture) Handle(_ context.Context, r slog.Record) error {
	c.records = append(c.records, r)
	return nil
}
func (c *logCapture) WithAttrs(_ []slog.Attr) slog.Handler { return c }
func (c *logCapture) WithGroup(_ string) slog.Handler      { return c }

func attrsMap(r slog.Record) map[string]string {
	out := map[string]string{}
	r.Attrs(func(a slog.Attr) bool {
		out[a.Key] = a.Value.String()
		return true
	})
	return out
}

func TestNewRequestID_UniqueUUIDv4(t *testing.T) {
	a := newRequestID()
	b := newRequestID()
	assert.Regexp(t, uuidRE, a)
	assert.NotEqual(t, a, b)
}

func TestLogFieldsUnaryInterceptor_InjectsFields(t *testing.T) {
	claims := &auth.Claims{Role: auth.RoleAdmin, TenantIDs: []string{"t1", "t2"}}
	claims.Subject = "alice"
	ctx := auth.ContextWithClaims(context.Background(), claims)

	var capturedCtx context.Context
	_, err := logFieldsUnaryInterceptor()(ctx, nil, &grpc.UnaryServerInfo{}, func(c context.Context, _ any) (any, error) {
		capturedCtx = c
		return nil, nil
	})
	require.NoError(t, err)

	cap := &logCapture{}
	logger := slog.New(telemetry.NewLogHandler(cap))
	logger.InfoContext(capturedCtx, "test")
	require.Len(t, cap.records, 1)

	attrs := attrsMap(cap.records[0])
	assert.Equal(t, "t1,t2", attrs["tenant_id"])
	assert.Equal(t, "alice", attrs["actor"])
	assert.Regexp(t, uuidRE, attrs["request_id"])
}

func TestLogFieldsUnaryInterceptor_NoClaims_RequestIDStillSet(t *testing.T) {
	var capturedCtx context.Context
	_, err := logFieldsUnaryInterceptor()(context.Background(), nil, &grpc.UnaryServerInfo{}, func(c context.Context, _ any) (any, error) {
		capturedCtx = c
		return nil, nil
	})
	require.NoError(t, err)

	cap := &logCapture{}
	logger := slog.New(telemetry.NewLogHandler(cap))
	logger.InfoContext(capturedCtx, "test")
	require.Len(t, cap.records, 1)

	attrs := attrsMap(cap.records[0])
	assert.NotContains(t, attrs, "tenant_id")
	assert.NotContains(t, attrs, "actor")
	assert.Regexp(t, uuidRE, attrs["request_id"])
}

func TestLogFieldsStreamInterceptor_InjectsFields(t *testing.T) {
	claims := &auth.Claims{Role: auth.RoleAdmin, TenantIDs: []string{"t1"}}
	claims.Subject = "bob"
	ctx := auth.ContextWithClaims(context.Background(), claims)

	ss := &fakeServerStream{ctx: ctx}
	var capturedCtx context.Context
	err := logFieldsStreamInterceptor()(nil, ss, &grpc.StreamServerInfo{}, func(_ any, s grpc.ServerStream) error {
		capturedCtx = s.Context()
		return nil
	})
	require.NoError(t, err)

	cap := &logCapture{}
	logger := slog.New(telemetry.NewLogHandler(cap))
	logger.InfoContext(capturedCtx, "test")
	require.Len(t, cap.records, 1)

	attrs := attrsMap(cap.records[0])
	assert.Equal(t, "t1", attrs["tenant_id"])
	assert.Equal(t, "bob", attrs["actor"])
	assert.Regexp(t, uuidRE, attrs["request_id"])
}

// fakeServerStream satisfies grpc.ServerStream with a fixed context.
type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeServerStream) Context() context.Context { return f.ctx }
