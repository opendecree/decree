package storage

import "context"

type ctxKeyTenantID struct{}

// WithTenantID returns a copy of ctx carrying tenantID for use by RunInTx.
// Call this before invoking Store.RunInTx inside tenant-scoped service methods.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, ctxKeyTenantID{}, tenantID)
}

// TenantIDFromCtx returns the tenant ID stored by WithTenantID, or empty string.
func TenantIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyTenantID{}).(string)
	return v
}
