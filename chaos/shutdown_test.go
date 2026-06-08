//go:build chaos

package chaos

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestGracefulShutdown_UnderLoad launches 1000 goroutines doing concurrent
// GetAll RPCs, sends SIGTERM to the server container, and asserts:
//   - All goroutines terminate (no hang)
//   - Shutdown completes within 30s (server has a 10s drain timeout)
//   - Every error is a proper gRPC status code (no raw transport failures)
//
// Run last: this test stops the server container. t.Cleanup restarts it.
func TestGracefulShutdown_UnderLoad(t *testing.T) {
	const (
		numConns      = 10
		numGoroutines = 1000
	)

	// Open connection pool and seed data before launching goroutines.
	conns := make([]*grpc.ClientConn, numConns)
	for i := range conns {
		conns[i] = dial(t)
	}
	admin := newAdminClient(conns[0])
	cfg0 := newConfigClient(conns[0])
	ctx := context.Background()

	schemaID, cleanupSchema := makeSchema(t, admin, "shutdown-schema")
	// cleanupSchema/cleanupTenant registered before restart so they run after restart (LIFO).
	t.Cleanup(cleanupSchema)
	tenantID, cleanupTenant := makeTenant(t, admin, "shutdown-tenant", schemaID)
	t.Cleanup(cleanupTenant)
	require.NoError(t, noVer(cfg0.Set(ctx, tenantID, "chaos.field0", "initial")))

	// Warm up all connections so goroutines are immediately in-flight on start.
	for _, c := range conns {
		cl := newConfigClient(c)
		_, err := cl.GetAll(ctx, tenantID)
		require.NoError(t, err, "warmup RPC failed")
	}

	// Launch goroutines; each loops until the server shuts down.
	type result struct{ err error }
	results := make([]result, numGoroutines)
	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Add(1)
		cl := newConfigClient(conns[i%numConns])
		go func(idx int) {
			defer wg.Done()
			for {
				_, err := cl.GetAll(context.Background(), tenantID)
				if err != nil {
					results[idx].err = err
					return
				}
			}
		}(i)
	}

	// Brief pause so goroutines enter their first RPC before SIGTERM.
	time.Sleep(100 * time.Millisecond)

	// Send SIGTERM — triggers the server's graceful shutdown (10s drain timeout).
	start := time.Now()
	containerSignal(t, serverContainer(), "SIGTERM")

	// Restart is registered LAST → runs FIRST during cleanup (LIFO), before schema/tenant delete.
	t.Cleanup(func() {
		containerStart(t, serverContainer())
		waitReachable(t, 30*time.Second)
	})

	// All goroutines must terminate; 60s is 6× the server's drain timeout.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("goroutines did not complete within 60s after SIGTERM")
	}

	elapsed := time.Since(start)
	t.Logf("shutdown elapsed: %v, goroutines terminated: %d", elapsed, numGoroutines)
	assert.Less(t, elapsed, 30*time.Second, "graceful shutdown must complete within 30s")

	// Every error must be a gRPC status error — not a raw connection reset.
	// codes.Unknown would indicate an unclassified/dropped request.
	for i, r := range results {
		if r.err == nil {
			continue
		}
		st, ok := status.FromError(r.err)
		assert.True(t, ok, "goroutine %d: non-gRPC error (possible dropped request): %T %v",
			i, r.err, r.err)
		if ok {
			assert.NotEqual(t, codes.Unknown, st.Code(),
				"goroutine %d: codes.Unknown suggests a dropped request: %v", i, st.Message())
		}
	}
}
