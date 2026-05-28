package config

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/pubsub"
	"github.com/opendecree/decree/internal/storage/domain"
)

// countingStore wraps MemoryStore and counts GetFieldLocks invocations.
type countingStore struct {
	*MemoryStore
	getLocksCalls atomic.Int64
}

func (c *countingStore) GetFieldLocks(ctx context.Context, tenantID string) ([]domain.TenantFieldLock, error) {
	c.getLocksCalls.Add(1)
	return c.MemoryStore.GetFieldLocks(ctx, tenantID)
}

const benchTenantID = "bbbbbbbb-0000-0000-0000-000000000001"

// BenchmarkSetFields_50Fields measures GetFieldLocks DB call count for a
// 50-field bulk update. With per-request memoization, GetFieldLocks is called
// exactly once per SetFields invocation regardless of the number of fields.
func BenchmarkSetFields_50Fields(b *testing.B) {
	const fieldCount = 50

	ms := NewMemoryStore()
	ms.SetTenant(domain.Tenant{ID: benchTenantID})

	cs := &countingStore{MemoryStore: ms}

	noop := &noopCache{}
	pub := &noopPublisher{}
	sub := &noopSubscriber{}

	svc := NewService(cs, noop, pub, sub, WithLogger(testLogger))

	ctx := auth.ContextWithClaims(context.Background(), &auth.Claims{
		Role:      auth.RoleAdmin,
		TenantIDs: []string{benchTenantID},
	})

	updates := make([]*pb.FieldUpdate, fieldCount)
	for i := range updates {
		updates[i] = &pb.FieldUpdate{
			FieldPath: fmt.Sprintf("field.key%d", i),
			Value:     &pb.TypedValue{Kind: &pb.TypedValue_StringValue{StringValue: "v"}},
		}
	}

	b.ResetTimer()
	for b.Loop() {
		_, err := svc.SetFields(ctx, &pb.SetFieldsRequest{
			TenantId: benchTenantID,
			Updates:  updates,
		})
		require.NoError(b, err)
	}
	b.StopTimer()

	calls := cs.getLocksCalls.Load()
	iterations := int64(b.N)
	// Exactly one GetFieldLocks DB call per SetFields invocation.
	if calls != iterations {
		b.Errorf("GetFieldLocks called %d times for %d iterations (want 1 per iteration)", calls, iterations)
	}
}

// --- minimal no-op implementations for benchmarks ---

type noopCache struct{}

func (noopCache) Get(_ context.Context, _ string, _, _ int32) (map[string]string, error) {
	return nil, nil
}

func (noopCache) Set(_ context.Context, _ string, _, _ int32, _ map[string]string, _ time.Duration) error {
	return nil
}
func (noopCache) Invalidate(_ context.Context, _ string) error { return nil }

type noopPublisher struct{}

func (noopPublisher) Publish(_ context.Context, _ pubsub.ConfigChangeEvent) error { return nil }
func (noopPublisher) Close() error                                                { return nil }

type noopSubscriber struct{}

func (noopSubscriber) Subscribe(_ context.Context, _ string) (<-chan pubsub.ConfigChangeEvent, context.CancelFunc, error) {
	ch := make(chan pubsub.ConfigChangeEvent)
	return ch, func() { close(ch) }, nil
}
func (noopSubscriber) Close() error { return nil }
