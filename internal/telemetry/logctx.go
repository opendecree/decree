package telemetry

import "context"

type (
	logTenantIDKey  struct{}
	logActorKey     struct{}
	logRequestIDKey struct{}
)

// WithLogFields stores tenant_id, actor, and request_id in ctx so the slog
// handler injects them into every log record for the request lifetime.
func WithLogFields(ctx context.Context, tenantID, actor, requestID string) context.Context {
	ctx = context.WithValue(ctx, logTenantIDKey{}, tenantID)
	ctx = context.WithValue(ctx, logActorKey{}, actor)
	ctx = context.WithValue(ctx, logRequestIDKey{}, requestID)
	return ctx
}
