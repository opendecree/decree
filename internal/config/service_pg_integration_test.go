//go:build integration

package config

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/cache"
	"github.com/opendecree/decree/internal/pubsub"
	"github.com/opendecree/decree/internal/schema"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/storage/pgtest"
)

// TestRetryOnVersionConflict_ConcurrentSetField hammers SetField from N
// goroutines on the same tenant simultaneously. Before issue #196, concurrent
// writers that lost the UNIQUE(tenant_id, version) race received codes.Internal.
// After the fix, the service retries transparently; the test asserts that
// codes.Internal is never returned and all N writes eventually commit.
func TestRetryOnVersionConflict_ConcurrentSetField(t *testing.T) {
	t.Parallel()

	const N = 20
	pool := pgtest.NewPool(t)
	cfgStore := NewPGStore(pool, pool)
	schStore := schema.NewPGStore(pool, pool)
	ctx := context.Background()

	// Create schema with N string fields.
	_, sv, tenant := setupFixture(t, schStore, t.Name())
	for i := 0; i < N; i++ {
		_, err := schStore.CreateSchemaField(ctx, schema.CreateSchemaFieldParams{
			SchemaVersionID: sv.ID,
			Path:            fmt.Sprintf("field_%d", i),
			FieldType:       domain.FieldTypeString,
		})
		require.NoError(t, err)
	}

	memCache := cache.NewMemoryCache(context.Background(), 0)
	memPub := pubsub.NewMemoryPubSub()
	svc := NewService(cfgStore, memCache, memPub, memPub, WithLogger(testLogger))

	svcCtx := auth.WithoutAuth(ctx)

	var (
		wg           sync.WaitGroup
		internalErrs atomic.Int64
		successes    atomic.Int64
	)

	start := make(chan struct{})

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			fieldPath := fmt.Sprintf("field_%d", i)
			req := &pb.SetFieldRequest{
				TenantId:  tenant.ID,
				FieldPath: fieldPath,
				Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: fmt.Sprintf("v%d", i)}},
			}

			<-start // synchronize all goroutines for maximum contention

			// Retry at the caller level on codes.Aborted (service budget exhausted).
			for attempt := 0; attempt < 15; attempt++ {
				_, err := svc.SetField(svcCtx, req)
				if err == nil {
					successes.Add(1)
					return
				}
				if status.Code(err) == codes.Internal {
					internalErrs.Add(1)
					return
				}
				if status.Code(err) == codes.Aborted {
					time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
					continue
				}
				// Any other error is unexpected.
				t.Errorf("unexpected error: %v", err)
				return
			}
			t.Errorf("field_%d: still aborted after 15 caller-level retries", i)
		}(i)
	}

	close(start) // release all goroutines simultaneously
	wg.Wait()

	assert.Equal(t, int64(0), internalErrs.Load(),
		"codes.Internal must never be returned for version conflicts — only codes.Aborted is acceptable when the retry budget is exhausted")
	assert.Equal(t, int64(N), successes.Load(),
		"all %d writes must eventually commit", N)
}
