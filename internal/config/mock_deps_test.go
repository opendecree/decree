package config

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/opendecree/decree/internal/pubsub"
)

// mockCache implements cache.ConfigCache for testing.
type mockCache struct {
	mock.Mock
}

func (m *mockCache) Get(ctx context.Context, tenantID string, version int32) (map[string]string, error) {
	args := m.Called(ctx, tenantID, version)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(map[string]string), args.Error(1)
}

func (m *mockCache) Set(ctx context.Context, tenantID string, version int32, values map[string]string, ttl time.Duration) error {
	args := m.Called(ctx, tenantID, version, values, ttl)
	return args.Error(0)
}

func (m *mockCache) Invalidate(ctx context.Context, tenantID string) error {
	args := m.Called(ctx, tenantID)
	return args.Error(0)
}

// SetNegative and GetNegative are no-ops in the test mock; existing tests don't
// exercise negative-cache paths. Tests that need control over negative caching
// use the real MemoryCache.
func (m *mockCache) SetNegative(_ context.Context, _ string, _ int32, _ time.Duration) error {
	return nil
}

func (m *mockCache) GetNegative(_ context.Context, _ string, _ int32) (bool, error) {
	return false, nil
}

// mockPublisher implements pubsub.Publisher for testing.
type mockPublisher struct {
	mock.Mock
}

func (m *mockPublisher) Publish(ctx context.Context, event pubsub.ConfigChangeEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *mockPublisher) Close() error {
	return nil
}

// mockSubscriber implements pubsub.Subscriber for testing.
type mockSubscriber struct {
	mock.Mock
}

func (m *mockSubscriber) Subscribe(ctx context.Context, tenantID string) (<-chan pubsub.ConfigChangeEvent, context.CancelFunc, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(<-chan pubsub.ConfigChangeEvent), args.Get(1).(context.CancelFunc), args.Error(2)
}

func (m *mockSubscriber) Close() error {
	return nil
}
