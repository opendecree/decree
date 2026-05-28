package config

// TestGetConfig_SingleflightDeduplicatesDBFetches proves that N concurrent
// GetConfig calls on the same cache-miss key collapse to exactly one DB fetch.
//
// Pattern:
//  1. Gate GetFullConfigAtVersion so it blocks until the test releases it.
//  2. Launch N goroutines; all miss the cache and enter singleflight.Do.
//  3. Wait for the gate to receive exactly one DB-fetch entry.
//  4. Release the gate; assert all N goroutines complete successfully.
//  5. Verify GetFullConfigAtVersion was called exactly once.

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/cache"
	"github.com/opendecree/decree/internal/storage/domain"
)

func TestGetConfig_NegativeCacheHit(t *testing.T) {
	t.Parallel()

	store := &mockStore{}
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)

	c := cache.NewMemoryCache(0)
	defer c.Stop()
	ctx := auth.WithoutAuth(context.Background())
	require.NoError(t, c.SetNegative(ctx, tenantID1, 1, time.Minute))

	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	svc := NewService(store, c, pub, sub, WithLogger(testLogger))

	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})

	require.NoError(t, err)
	assert.Empty(t, resp.Config.Values, "negative cache hit must return empty values")
	store.AssertNotCalled(t, "GetFullConfigAtVersion")
}

func TestGetConfig_EmptyConfig_SetsNegativeCache(t *testing.T) {
	t.Parallel()

	store := &mockStore{}
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  1,
	}).Return([]GetFullConfigAtVersionRow{}, nil)
	setupNoSensitiveFields(store)

	c := cache.NewMemoryCache(0)
	defer c.Stop()
	ctx := auth.WithoutAuth(context.Background())

	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	svc := NewService(store, c, pub, sub, WithLogger(testLogger))

	// First call: cache miss → DB fetch (empty) → sets negative cache.
	resp, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})
	require.NoError(t, err)
	assert.Empty(t, resp.Config.Values)

	// Second call: served from negative cache — DB must not be called again.
	resp2, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})
	require.NoError(t, err)
	assert.Empty(t, resp2.Config.Values)

	store.AssertNumberOfCalls(t, "GetFullConfigAtVersion", 1)
}

func TestGetConfig_SingleflightDeduplicatesDBFetches(t *testing.T) {
	t.Parallel()

	const N = 10

	// Gate synchronization: first goroutine to enter GetFullConfigAtVersion
	// signals started (capacity 1); all goroutines then block on proceed.
	started := make(chan struct{}, 1)
	proceed := make(chan struct{})
	var dbFetches atomic.Int32

	store := &mockStore{}
	store.On("GetLatestConfigVersion", mock.Anything, tenantID1).
		Return(domain.ConfigVersion{Version: 1}, nil)
	store.On("GetFullConfigAtVersion", mock.Anything, GetFullConfigAtVersionParams{
		TenantID: tenantID1,
		Version:  1,
	}).
		Run(func(_ mock.Arguments) {
			dbFetches.Add(1)
			select {
			case started <- struct{}{}: // signal first entry (non-blocking)
			default:
			}
			<-proceed // block until test releases
		}).
		Return([]GetFullConfigAtVersionRow{
			{FieldPath: "app.env", Value: strPtr("production")},
		}, nil)
	setupNoSensitiveFields(store)

	c := cache.NewMemoryCache(0)
	defer c.Stop()

	pub := &mockPublisher{}
	sub := &mockSubscriber{}
	svc := NewService(store, c, pub, sub, WithLogger(testLogger))

	ctx := auth.WithoutAuth(context.Background())

	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.GetConfig(ctx, &pb.GetConfigRequest{TenantId: tenantID1})
			errs <- err
		}()
	}

	// Wait for the DB fetch to start.
	select {
	case <-started:
	case <-time.After(testTimeout):
		t.Fatal("DB fetch never started — goroutines may not have reached singleflight")
	}

	// Give the remaining goroutines time to queue inside singleflight.Do.
	time.Sleep(20 * time.Millisecond)

	// Only 1 DB fetch should be in progress; the rest wait in singleflight.
	assert.Equal(t, int32(1), dbFetches.Load(), "singleflight must collapse N misses to 1 DB fetch")

	// Release the DB fetch so all goroutines can complete.
	close(proceed)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("goroutines did not complete after proceeding")
	}

	for i := 0; i < N; i++ {
		require.NoError(t, <-errs, "all goroutines must succeed")
	}

	assert.Equal(t, int32(1), dbFetches.Load(), "DB fetch count must remain 1 after all goroutines complete")
}
