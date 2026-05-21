package pubsub

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var discardLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))

// Tests below use nil Redis clients. Only the constructor and Close paths are
// exercised — no actual Redis calls are made.

func TestNewRedisPublisher(t *testing.T) {
	p := NewRedisPublisher(nil)
	require.NotNil(t, p)
}

func TestRedisPublisher_Close(t *testing.T) {
	p := NewRedisPublisher(nil)
	require.NoError(t, p.Close())
}

func TestNewRedisSubscriber(t *testing.T) {
	s := NewRedisSubscriber(nil, discardLogger)
	require.NotNil(t, s)
}

func TestRedisSubscriber_Close(t *testing.T) {
	s := NewRedisSubscriber(nil, discardLogger)
	require.NoError(t, s.Close())
}

// --- miniredis integration tests ---

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr:       mr.Addr(),
		MaxRetries: 0,
	})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestRedisPubSub_PublishSubscribe_Happy(t *testing.T) {
	client := newTestRedisClient(t)
	pub := NewRedisPublisher(client)
	sub := NewRedisSubscriber(client, discardLogger)
	ctx := context.Background()

	ch, cancel, err := sub.Subscribe(ctx, "t1")
	require.NoError(t, err)
	defer cancel()

	event := ConfigChangeEvent{TenantID: "t1", FieldPath: "app.fee", NewValue: "0.02"}
	require.NoError(t, pub.Publish(ctx, event))

	select {
	case got := <-ch:
		assert.Equal(t, "app.fee", got.FieldPath)
		assert.Equal(t, "0.02", got.NewValue)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestRedisPubSub_TenantIsolation(t *testing.T) {
	client := newTestRedisClient(t)
	pub := NewRedisPublisher(client)
	sub := NewRedisSubscriber(client, discardLogger)
	ctx := context.Background()

	ch1, cancel1, err := sub.Subscribe(ctx, "t1")
	require.NoError(t, err)
	defer cancel1()

	ch2, cancel2, err := sub.Subscribe(ctx, "t2")
	require.NoError(t, err)
	defer cancel2()

	require.NoError(t, pub.Publish(ctx, ConfigChangeEvent{TenantID: "t1", FieldPath: "a"}))

	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("t1 should receive event")
	}
	select {
	case <-ch2:
		t.Fatal("t2 should not receive t1 event")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRedisPubSub_SlowSubscriber(t *testing.T) {
	client := newTestRedisClient(t)
	pub := NewRedisPublisher(client)
	sub := NewRedisSubscriber(client, discardLogger)
	ctx := context.Background()

	ch, cancel, err := sub.Subscribe(ctx, "t1")
	require.NoError(t, err)
	defer cancel()

	// Publish several messages without reading; the 64-slot buffer absorbs them.
	const n = 5
	for range n {
		require.NoError(t, pub.Publish(ctx, ConfigChangeEvent{TenantID: "t1", FieldPath: "x"}))
	}

	// Slow consumer reads all messages after a delay.
	time.Sleep(50 * time.Millisecond)
	received := 0
	for received < n {
		select {
		case _, ok := <-ch:
			require.True(t, ok)
			received++
		case <-time.After(time.Second):
			t.Fatalf("timed out after receiving %d/%d events", received, n)
		}
	}
	assert.Equal(t, n, received)
}

func TestRedisPubSub_SubscribeFailsWhenRedisDown(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr:       mr.Addr(),
		MaxRetries: 0,
	})
	t.Cleanup(func() { _ = client.Close() })
	sub := NewRedisSubscriber(client, discardLogger)

	mr.Close()

	_, _, err := sub.Subscribe(context.Background(), "t1")
	require.Error(t, err, "subscribe must fail when Redis is unreachable")
}

func TestRedisPubSub_CancelClosesChannel(t *testing.T) {
	client := newTestRedisClient(t)
	sub := NewRedisSubscriber(client, discardLogger)
	ctx := context.Background()

	ch, cancel, err := sub.Subscribe(ctx, "t1")
	require.NoError(t, err)

	cancel()

	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel must be closed after cancel")
	case <-time.After(time.Second):
		t.Fatal("channel did not close after cancel")
	}
}

func TestRedisPubSub_DuplicateDeliverySemantics(t *testing.T) {
	client := newTestRedisClient(t)
	pub := NewRedisPublisher(client)
	sub := NewRedisSubscriber(client, discardLogger)
	ctx := context.Background()

	ch, cancel, err := sub.Subscribe(ctx, "t1")
	require.NoError(t, err)
	defer cancel()

	require.NoError(t, pub.Publish(ctx, ConfigChangeEvent{TenantID: "t1", FieldPath: "x"}))

	select {
	case got := <-ch:
		assert.Equal(t, "x", got.FieldPath)
	case <-time.After(time.Second):
		t.Fatal("no event received")
	}

	// Redis pub/sub is at-most-once: exactly one delivery per publish.
	select {
	case extra, ok := <-ch:
		if ok {
			t.Fatalf("unexpected duplicate delivery: %+v", extra)
		}
	case <-time.After(50 * time.Millisecond):
		// Good: no duplicate.
	}
}
