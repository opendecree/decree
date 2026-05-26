package storage

import (
	"context"
	"testing"
)

func TestWithTenantID_RoundTrip(t *testing.T) {
	ctx := WithTenantID(context.Background(), "abc-123")
	if got := TenantIDFromCtx(ctx); got != "abc-123" {
		t.Fatalf("got %q, want %q", got, "abc-123")
	}
}

func TestTenantIDFromCtx_Missing(t *testing.T) {
	if got := TenantIDFromCtx(context.Background()); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
