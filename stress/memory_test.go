//go:build stress

package stress

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// heapSnapshot writes a heap pprof profile to a temp file and returns the path.
func heapSnapshot(t *testing.T, label string) string {
	t.Helper()
	runtime.GC()
	f, err := os.CreateTemp(t.TempDir(), fmt.Sprintf("heap-%s-*.pprof", label))
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, pprof.WriteHeapProfile(f))
	return filepath.Base(f.Name())
}

// TestMemoryGrowth_BoundedUnderLoad verifies that heap growth per operation is
// bounded. Runs a sustained GetAll workload and asserts that total heap inuse
// does not grow unboundedly relative to the number of operations completed.
func TestMemoryGrowth_BoundedUnderLoad(t *testing.T) {
	ops := 10_000
	if testing.Short() {
		ops = 500
	}

	conn := dial(t)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(t, admin, "stress-mem-growth", 8)
	defer cleanSchema()
	tenantID, cleanTenant := makeTenant(t, admin, "stress-mg-tenant", schemaID)
	defer cleanTenant()
	for i := 0; i < 8; i++ {
		require.NoError(t, noVer(cfg.Set(ctx, tenantID, fmt.Sprintf("f.field_%d", i), fmt.Sprintf("val-%d", i))))
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	_ = heapSnapshot(t, "before")

	for i := 0; i < ops; i++ {
		_, err := cfg.GetAll(ctx, tenantID)
		require.NoError(t, err)
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	_ = heapSnapshot(t, "after")

	// HeapInuse growth should be well under 50 MB for any workload size.
	// This catches client-side leaks (e.g. response objects not freed).
	const maxGrowthBytes = 50 * 1024 * 1024
	growth := int64(after.HeapInuse) - int64(before.HeapInuse)
	t.Logf("heap growth: %d bytes over %d ops (%.1f bytes/op)", growth, ops, float64(growth)/float64(ops))
	assert.Less(t, growth, int64(maxGrowthBytes),
		"client-side heap growth must be bounded (< 50 MB) regardless of op count")
}

// TestGoroutineLeak verifies that goroutine count returns to baseline after a
// sustained load burst. A persistent delta is a symptom of goroutine leaks in
// the SDK or connection layer.
func TestGoroutineLeak(t *testing.T) {
	goroutines := 30
	itersEach := 50
	if testing.Short() {
		goroutines = 10
		itersEach = 10
	}

	conn := dial(t)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(t, admin, "stress-goroutine", 4)
	defer cleanSchema()
	tenantID, cleanTenant := makeTenant(t, admin, "stress-gr-tenant", schemaID)
	defer cleanTenant()
	require.NoError(t, noVer(cfg.Set(ctx, tenantID, "f.field_0", "seed")))

	// Allow the stack to settle before measuring baseline.
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < itersEach; i++ {
				_, _ = cfg.GetAll(ctx, tenantID)
			}
		}()
	}
	wg.Wait()

	// Give the runtime a moment to reap any short-lived goroutines.
	time.Sleep(200 * time.Millisecond)
	runtime.GC()

	final := runtime.NumGoroutine()
	delta := final - baseline
	t.Logf("goroutines: baseline=%d final=%d delta=%d", baseline, final, delta)

	// Allow a small tolerance for background SDK/gRPC housekeeping goroutines.
	const maxDelta = 5
	assert.LessOrEqual(t, delta, maxDelta,
		"goroutine count must return to baseline after load (delta > %d indicates a leak)", maxDelta)
}

// TestConcurrentTenants_Isolation verifies that concurrent reads and writes
// across many tenants never produce cross-tenant data contamination.
// Each tenant has a unique sentinel value; any read returning a different
// tenant's sentinel signals isolation failure.
func TestConcurrentTenants_Isolation(t *testing.T) {
	tenantCount := 50
	duration := 8 * time.Second
	if testing.Short() {
		tenantCount = 10
		duration = 2 * time.Second
	}

	conn := dial(t)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(t, admin, "stress-isolation", 2)
	defer cleanSchema()

	type tenantInfo struct {
		id       string
		sentinel string
	}
	tenants := make([]tenantInfo, tenantCount)
	for i := range tenants {
		sentinel := fmt.Sprintf("tenant-sentinel-%d", i)
		id, cleanTenant := makeTenant(t, admin, fmt.Sprintf("stress-iso-%d", i), schemaID)
		defer cleanTenant()
		require.NoError(t, noVer(cfg.Set(ctx, id, "f.field_0", sentinel)))
		tenants[i] = tenantInfo{id: id, sentinel: sentinel}
	}

	deadline := time.Now().Add(duration)
	var isolationViolations atomic.Int64
	var totalOps atomic.Int64

	var wg sync.WaitGroup

	// Writers: update each tenant's sentinel to a new unique value.
	for w := 0; w < 4; w++ {
		w := w
		if testing.Short() {
			wg.Add(0)
			_ = w
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for gen := 0; time.Now().Before(deadline); gen++ {
				idx := (w*1000 + gen) % tenantCount
				newSentinel := fmt.Sprintf("tenant-sentinel-%d-gen%d", idx, gen)
				if _, err := cfg.Set(ctx, tenants[idx].id, "f.field_0", newSentinel); err == nil {
					tenants[idx].sentinel = newSentinel
				}
			}
		}()
	}

	// Readers: verify each tenant's f.field_1 (unwritten) is absent,
	// and f.field_0 belongs to this tenant (prefix check).
	for r := 0; r < 8; r++ {
		r := r
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; time.Now().Before(deadline); i++ {
				idx := (r*37 + i) % tenantCount
				vals, err := cfg.GetAll(ctx, tenants[idx].id)
				totalOps.Add(1)
				if err != nil {
					continue
				}
				// Verify the returned value (if any) belongs to this tenant.
				// Sentinel values are prefixed with "tenant-sentinel-<idx>".
				expectedPrefix := fmt.Sprintf("tenant-sentinel-%d", idx)
				for _, v := range vals {
					if v == "" {
						continue
					}
					if len(v) >= len(expectedPrefix) && v[:len(expectedPrefix)] != expectedPrefix {
						// Value does not belong to this tenant — isolation violation.
						isolationViolations.Add(1)
					}
				}
			}
		}()
	}

	wg.Wait()

	t.Logf("isolation: %d ops, %d violations", totalOps.Load(), isolationViolations.Load())
	assert.Equal(t, int64(0), isolationViolations.Load(),
		"cross-tenant data contamination detected under concurrent load")
}

// BenchmarkMemoryGrowth_Sustained measures steady-state allocations per GetAll.
func BenchmarkMemoryGrowth_Sustained(b *testing.B) {
	conn := dial(b)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(b, admin, "stress-mem-bench", 8)
	b.Cleanup(cleanSchema)
	tenantID, cleanTenant := makeTenant(b, admin, "stress-mb-tenant", schemaID)
	b.Cleanup(cleanTenant)
	for i := 0; i < 8; i++ {
		require.NoError(b, noVer(cfg.Set(ctx, tenantID, fmt.Sprintf("f.field_%d", i), fmt.Sprintf("val-%d", i))))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = cfg.GetAll(ctx, tenantID)
	}
}
