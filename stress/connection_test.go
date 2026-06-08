//go:build stress

package stress

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionPool_ConcurrentLoad saturates the server with many concurrent
// requests and verifies that all succeed or return well-formed gRPC errors —
// no panics, hangs, or silent data corruption.
func TestConnectionPool_ConcurrentLoad(t *testing.T) {
	goroutines := 50
	requestsEach := 100
	if testing.Short() {
		goroutines = 10
		requestsEach = 20
	}

	conn := dial(t)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(t, admin, "stress-pool", 4)
	defer cleanSchema()
	tenantID, cleanTenant := makeTenant(t, admin, "stress-pool-tenant", schemaID)
	defer cleanTenant()
	require.NoError(t, noVer(cfg.Set(ctx, tenantID, "f.field_0", "seed")))

	var wg sync.WaitGroup
	var errCount atomic.Int64

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < requestsEach; i++ {
				_, err := cfg.GetAll(ctx, tenantID)
				if err != nil {
					errCount.Add(1)
				}
			}
		}()
	}

	wg.Wait()
	t.Logf("concurrent load: %d goroutines × %d requests, %d errors",
		goroutines, requestsEach, errCount.Load())
	assert.Equal(t, int64(0), errCount.Load(), "concurrent reads must not error")
}

// TestConnectionPool_LeakDetection runs sustained mixed load and verifies
// that repeated rounds of activity do not degrade — a symptom of leaked
// connections is progressively increasing latency or error rates.
func TestConnectionPool_LeakDetection(t *testing.T) {
	rounds := 5
	goroutines := 20
	if testing.Short() {
		rounds = 3
		goroutines = 5
	}

	conn := dial(t)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(t, admin, "stress-leak", 4)
	defer cleanSchema()
	tenantID, cleanTenant := makeTenant(t, admin, "stress-leak-tenant", schemaID)
	defer cleanTenant()
	require.NoError(t, noVer(cfg.Set(ctx, tenantID, "f.field_0", "seed")))

	latencies := make([]time.Duration, rounds)
	for round := 0; round < rounds; round++ {
		var wg sync.WaitGroup
		start := time.Now()
		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 10; i++ {
					_, _ = cfg.GetAll(ctx, tenantID)
				}
			}()
		}
		wg.Wait()
		latencies[round] = time.Since(start)
		t.Logf("round %d: %v", round, latencies[round])
	}

	// Last round must not be more than 3× slower than the first, which would
	// indicate resource exhaustion or connection leaks accumulating.
	if rounds > 1 {
		assert.Less(t, latencies[rounds-1], latencies[0]*3,
			"final round must not be 3× slower than first round (connection leak signal)")
	}
}

// BenchmarkConcurrentGetAll measures throughput of parallel GetAll calls
// across multiple goroutines.
func BenchmarkConcurrentGetAll(b *testing.B) {
	goroutines := 20
	if testing.Short() {
		goroutines = 5
	}

	conn := dial(b)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(b, admin, "stress-concurrent-get", 4)
	b.Cleanup(cleanSchema)
	tenantID, cleanTenant := makeTenant(b, admin, "stress-concurrent-tenant", schemaID)
	b.Cleanup(cleanTenant)
	require.NoError(b, noVer(cfg.Set(ctx, tenantID, "f.field_0", "seed")))

	b.ResetTimer()
	b.SetParallelism(goroutines)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cfg.GetAll(ctx, tenantID)
		}
	})
}

// BenchmarkLargePayload_GetAll measures GetAll latency when a tenant has many
// large field values (KB-range strings).
func BenchmarkLargePayload_GetAll(b *testing.B) {
	fieldCount := 20
	valueSize := 4 * 1024 // 4 KB per field
	if testing.Short() {
		fieldCount = 5
		valueSize = 512
	}

	conn := dial(b)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(b, admin, "stress-large-payload", fieldCount)
	b.Cleanup(cleanSchema)
	tenantID, cleanTenant := makeTenant(b, admin, "stress-lp-tenant", schemaID)
	b.Cleanup(cleanTenant)

	largeVal := strings.Repeat("x", valueSize)
	for i := 0; i < fieldCount; i++ {
		require.NoError(b, noVer(cfg.Set(ctx, tenantID, fmt.Sprintf("f.field_%d", i), largeVal)))
	}

	b.ResetTimer()
	b.SetBytes(int64(fieldCount * valueSize))
	for b.Loop() {
		_, _ = cfg.GetAll(ctx, tenantID)
	}
}

// BenchmarkBulkTenants_List measures ListTenants throughput as tenant count grows.
func BenchmarkBulkTenants_List(b *testing.B) {
	tenantCount := 200
	if testing.Short() {
		tenantCount = 20
	}

	conn := dial(b)
	admin := newAdmin(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(b, admin, "stress-bulk-list", 2)
	b.Cleanup(cleanSchema)

	for i := 0; i < tenantCount; i++ {
		_, cleanTenant := makeTenant(b, admin, fmt.Sprintf("stress-bl-%d", i), schemaID)
		b.Cleanup(cleanTenant)
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = admin.ListTenants(ctx, schemaID)
	}
}

// TestLargeSchema_CreateAndFetch verifies correct handling of a very large schema
// (hundreds of fields) from creation through config read.
func TestLargeSchema_CreateAndFetch(t *testing.T) {
	fieldCount := 500
	if testing.Short() {
		fieldCount = 50
	}

	conn := dial(t)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(t, admin, "stress-big-schema", fieldCount)
	defer cleanSchema()
	tenantID, cleanTenant := makeTenant(t, admin, "stress-bs-tenant", schemaID)
	defer cleanTenant()

	// Write one value per field.
	for i := 0; i < fieldCount; i++ {
		require.NoError(t, noVer(cfg.Set(ctx, tenantID, fmt.Sprintf("f.field_%d", i), fmt.Sprintf("val-%d", i))))
	}

	vals, err := cfg.GetAll(ctx, tenantID)
	require.NoError(t, err)
	assert.Len(t, vals, fieldCount, "GetAll must return all %d fields", fieldCount)
}

// TestLargePayload_SetAndGet verifies correct round-trip of a large field value.
func TestLargePayload_SetAndGet(t *testing.T) {
	valueSize := 512 * 1024 // 512 KB
	if testing.Short() {
		valueSize = 4 * 1024 // 4 KB
	}

	conn := dial(t)
	admin := newAdmin(conn)
	cfg := newConfig(conn)
	ctx := context.Background()

	schemaID, cleanSchema := makeSchema(t, admin, "stress-large-value", 1)
	defer cleanSchema()
	tenantID, cleanTenant := makeTenant(t, admin, "stress-lv-tenant", schemaID)
	defer cleanTenant()

	largeVal := strings.Repeat("abc", valueSize/3)
	require.NoError(t, noVer(cfg.Set(ctx, tenantID, "f.field_0", largeVal)))

	got, err := cfg.Get(ctx, tenantID, "f.field_0")
	require.NoError(t, err)
	assert.Equal(t, largeVal, got, "large value must round-trip exactly")
}
