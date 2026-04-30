package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// NewLogHandler wraps an slog.Handler to inject context-carried fields into
// every log record: trace_id and span_id from the active OTel span (when
// present), and tenant_id, actor, and request_id set by WithLogFields.
func NewLogHandler(inner slog.Handler) slog.Handler {
	return &contextLogHandler{inner: inner}
}

type contextLogHandler struct {
	inner slog.Handler
}

func (h *contextLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *contextLogHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		record.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	if v, _ := ctx.Value(logTenantIDKey{}).(string); v != "" {
		record.AddAttrs(slog.String("tenant_id", v))
	}
	if v, _ := ctx.Value(logActorKey{}).(string); v != "" {
		record.AddAttrs(slog.String("actor", v))
	}
	if v, _ := ctx.Value(logRequestIDKey{}).(string); v != "" {
		record.AddAttrs(slog.String("request_id", v))
	}
	return h.inner.Handle(ctx, record)
}

func (h *contextLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextLogHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *contextLogHandler) WithGroup(name string) slog.Handler {
	return &contextLogHandler{inner: h.inner.WithGroup(name)}
}
