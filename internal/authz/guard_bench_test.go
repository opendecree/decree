package authz_test

import (
	"testing"

	"github.com/opendecree/decree/internal/authz"
	"github.com/opendecree/decree/internal/storage/domain"
)

// BenchmarkChain_TenantAndRole measures the two-guard pipeline used by most
// read/write RPCs: TenantScopeGuard → RolePolicyGuard.
func BenchmarkChain_TenantAndRole(b *testing.B) {
	chain := authz.Chain(authz.TenantScopeGuard{}, authz.RolePolicyGuard{})
	ctx := adminCtx()
	r := authz.Resource{TenantID: tenant1}
	b.ReportAllocs()
	for b.Loop() {
		_ = chain.Check(ctx, authz.ActionWrite, r)
	}
}

// BenchmarkChain_FullPipeline_CacheHit measures the full three-guard pipeline
// when field locks are already in the request context (no store round-trip).
func BenchmarkChain_FullPipeline_CacheHit(b *testing.B) {
	locks := []domain.TenantFieldLock{{TenantID: tenant1, FieldPath: "a.b"}}
	store := &stubLockStore{locks: locks}
	chain := authz.Chain(authz.TenantScopeGuard{}, authz.RolePolicyGuard{}, authz.NewFieldLockGuard(store))
	ctx := authz.WithFieldLockCache(adminCtx(), locks)
	r := authz.Resource{TenantID: tenant1, FieldPath: "c.d"} // unlocked → passes
	b.ReportAllocs()
	for b.Loop() {
		_ = chain.Check(ctx, authz.ActionWrite, r)
	}
}

// BenchmarkChain_FullPipeline_StoreLookup measures the full three-guard
// pipeline when field locks must be fetched from the store each call.
func BenchmarkChain_FullPipeline_StoreLookup(b *testing.B) {
	store := &stubLockStore{
		locks: []domain.TenantFieldLock{{TenantID: tenant1, FieldPath: "a.b"}},
	}
	chain := authz.Chain(authz.TenantScopeGuard{}, authz.RolePolicyGuard{}, authz.NewFieldLockGuard(store))
	ctx := adminCtx() // no context cache
	r := authz.Resource{TenantID: tenant1, FieldPath: "c.d"}
	b.ReportAllocs()
	for b.Loop() {
		_ = chain.Check(ctx, authz.ActionWrite, r)
	}
}
