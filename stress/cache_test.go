//go:build stress

package stress

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BenchmarkManyTenants_GetAll measures cache performance across many tenants.
// Each iteration picks a random tenant and fetches its full config.
func BenchmarkManyTenants_GetAll(b *testing.B) {
	n := 200
	if testing.Short() {
		n = 20
	}

	conn := dial(b)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(b, admin, "stress-many-tenants", 4)
	b.Cleanup(cleanSchema)

	tenantIDs := make([]string, n)
	for i := range tenantIDs {
		id, cleanTenant := makeTenant(b, admin, fmt.Sprintf("stress-mt-%d", i), schemaID)
		tenantIDs[i] = id
		b.Cleanup(cleanTenant)
		require.NoError(b, noVer(cfg.Set(ctx, id, "f.field_0", fmt.Sprintf("val-%d", i))))
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, _ = cfg.GetAll(ctx, tenantIDs[rand.IntN(n)])
	}
}

// BenchmarkManyFields_GetAll measures GetAll latency for schemas with large field counts.
func BenchmarkManyFields_GetAll(b *testing.B) {
	fieldCount := 500
	if testing.Short() {
		fieldCount = 50
	}

	conn := dial(b)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(b, admin, "stress-many-fields", fieldCount)
	b.Cleanup(cleanSchema)
	tenantID, cleanTenant := makeTenant(b, admin, "stress-mf-tenant", schemaID)
	b.Cleanup(cleanTenant)

	for i := 0; i < fieldCount; i++ {
		require.NoError(b, noVer(cfg.Set(ctx, tenantID, fmt.Sprintf("f.field_%d", i), fmt.Sprintf("val-%d", i))))
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = cfg.GetAll(ctx, tenantID)
	}
}

// BenchmarkRapidInvalidation measures Set throughput — each write invalidates
// the tenant's cache entry and forces the next read to repopulate from DB.
func BenchmarkRapidInvalidation(b *testing.B) {
	conn := dial(b)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(b, admin, "stress-invalidation", 4)
	b.Cleanup(cleanSchema)
	tenantID, cleanTenant := makeTenant(b, admin, "stress-inv-tenant", schemaID)
	b.Cleanup(cleanTenant)
	require.NoError(b, noVer(cfg.Set(ctx, tenantID, "f.field_0", "seed")))

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, _ = cfg.Set(ctx, tenantID, "f.field_0", fmt.Sprintf("rapid-%d", i))
	}
}

// TestCacheEviction verifies data correctness under heavy cache churn.
// Writers continually invalidate cache entries while readers concurrently
// fetch the same tenants. All reads must return valid, non-empty results.
func TestCacheEviction(t *testing.T) {
	tenantCount := 100
	duration := 10 * time.Second
	if testing.Short() {
		tenantCount = 20
		duration = 3 * time.Second
	}

	conn := dial(t)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(t, admin, "stress-eviction", 4)
	defer cleanSchema()

	tenantIDs := make([]string, tenantCount)
	for i := range tenantIDs {
		id, cleanTenant := makeTenant(t, admin, fmt.Sprintf("stress-ev-%d", i), schemaID)
		tenantIDs[i] = id
		defer cleanTenant()
		require.NoError(t, noVer(cfg.Set(ctx, id, "f.field_0", fmt.Sprintf("seed-%d", i))))
	}

	deadline := time.Now().Add(duration)
	var readErrors atomic.Int64
	var emptyReads atomic.Int64
	var totalReads atomic.Int64

	var wg sync.WaitGroup

	writerCount := 4
	if testing.Short() {
		writerCount = 2
	}
	for w := 0; w < writerCount; w++ {
		w := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; time.Now().Before(deadline); i++ {
				tid := tenantIDs[(w*100+i)%tenantCount]
				_, _ = cfg.Set(ctx, tid, "f.field_0", fmt.Sprintf("write-%d-%d", w, i))
			}
		}()
	}

	readerCount := 8
	if testing.Short() {
		readerCount = 4
	}
	for r := 0; r < readerCount; r++ {
		r := r
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; time.Now().Before(deadline); i++ {
				tid := tenantIDs[(r*73+i)%tenantCount]
				vals, err := cfg.GetAll(ctx, tid)
				totalReads.Add(1)
				if err != nil {
					readErrors.Add(1)
					continue
				}
				if len(vals) == 0 {
					emptyReads.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	total := totalReads.Load()
	t.Logf("cache eviction: %d reads, %d errors, %d empty", total, readErrors.Load(), emptyReads.Load())

	assert.Equal(t, int64(0), readErrors.Load(), "reads must not error under cache churn")
	assert.Equal(t, int64(0), emptyReads.Load(), "reads must return non-empty config under cache churn")
	assert.Greater(t, total, int64(0), "must complete at least one read")
}
