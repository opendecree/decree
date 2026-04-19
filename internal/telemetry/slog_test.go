package telemetry

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
)

// capturingHandler records every Handle call and tracks whether
// Enabled/WithAttrs/WithGroup were invoked.
type capturingHandler struct {
	records      []slog.Record
	enabledCalls int
	enabledFor   []slog.Level
	withAttrs    [][]slog.Attr
	withGroup    []string
	enabledReply bool
}

func (h *capturingHandler) Enabled(_ context.Context, level slog.Level) bool {
	h.enabledCalls++
	h.enabledFor = append(h.enabledFor, level)
	return h.enabledReply
}

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *capturingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.withAttrs = append(h.withAttrs, attrs)
	return h
}

func (h *capturingHandler) WithGroup(name string) slog.Handler {
	h.withGroup = append(h.withGroup, name)
	return h
}

func attrsOf(r slog.Record) map[string]string {
	out := map[string]string{}
	r.Attrs(func(a slog.Attr) bool {
		out[a.Key] = a.Value.String()
		return true
	})
	return out
}

func TestNewLogHandler_WrapsInner(t *testing.T) {
	inner := &capturingHandler{}
	h := NewLogHandler(inner)
	require.NotNil(t, h)
	assert.NotSame(t, inner, h, "should return a new handler, not the inner one")
}

func TestLogHandler_Enabled_DelegatesToInner(t *testing.T) {
	inner := &capturingHandler{enabledReply: true}
	h := NewLogHandler(inner)

	require.True(t, h.Enabled(context.Background(), slog.LevelWarn))
	require.Equal(t, 1, inner.enabledCalls)
	require.Equal(t, []slog.Level{slog.LevelWarn}, inner.enabledFor)

	inner.enabledReply = false
	require.False(t, h.Enabled(context.Background(), slog.LevelDebug))
}

func TestLogHandler_Handle_NoSpan_PassesThrough(t *testing.T) {
	inner := &capturingHandler{}
	h := NewLogHandler(inner)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	r.AddAttrs(slog.String("k", "v"))

	require.NoError(t, h.Handle(context.Background(), r))
	require.Len(t, inner.records, 1)

	got := attrsOf(inner.records[0])
	assert.Equal(t, "v", got["k"])
	assert.NotContains(t, got, "trace_id")
	assert.NotContains(t, got, "span_id")
}

func TestLogHandler_Handle_WithSpan_InjectsTraceAndSpanIDs(t *testing.T) {
	inner := &capturingHandler{}
	h := NewLogHandler(inner)

	tp := trace.NewTracerProvider()
	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	require.NoError(t, h.Handle(ctx, r))
	require.Len(t, inner.records, 1)

	got := attrsOf(inner.records[0])
	assert.Equal(t, span.SpanContext().TraceID().String(), got["trace_id"])
	assert.Equal(t, span.SpanContext().SpanID().String(), got["span_id"])
}

func TestLogHandler_WithAttrs_WrapsInnerResult(t *testing.T) {
	inner := &capturingHandler{}
	h := NewLogHandler(inner)

	child := h.WithAttrs([]slog.Attr{slog.String("svc", "decree")})
	require.NotNil(t, child)
	// Child is a traceLogHandler wrapping inner; the call should have been
	// delegated to the inner handler.
	require.Len(t, inner.withAttrs, 1)
	assert.Equal(t, "decree", inner.withAttrs[0][0].Value.String())

	// Child must still inject trace IDs — i.e. it's still the tracing wrapper.
	tp := trace.NewTracerProvider()
	ctx, span := tp.Tracer("t").Start(context.Background(), "op")
	defer span.End()
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "m", 0)
	require.NoError(t, child.Handle(ctx, r))
	got := attrsOf(inner.records[len(inner.records)-1])
	assert.Equal(t, span.SpanContext().TraceID().String(), got["trace_id"])
}

func TestLogHandler_WithGroup_WrapsInnerResult(t *testing.T) {
	inner := &capturingHandler{}
	h := NewLogHandler(inner)

	child := h.WithGroup("api")
	require.NotNil(t, child)
	require.Equal(t, []string{"api"}, inner.withGroup)

	// Child must still inject trace IDs.
	tp := trace.NewTracerProvider()
	ctx, span := tp.Tracer("t").Start(context.Background(), "op")
	defer span.End()
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "m", 0)
	require.NoError(t, child.Handle(ctx, r))
	got := attrsOf(inner.records[len(inner.records)-1])
	assert.Equal(t, span.SpanContext().SpanID().String(), got["span_id"])
}
