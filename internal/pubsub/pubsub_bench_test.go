package pubsub

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

var benchEvent = ConfigChangeEvent{
	TenantID:  "bench-t1",
	Version:   1,
	Changes:   []FieldChange{{FieldPath: "app.rate_limit", OldValue: "100", NewValue: "200"}},
	ChangedBy: "bench",
	ChangedAt: time.Now(),
}

func BenchmarkMemoryPubSub_Publish_NoSubscribers(b *testing.B) {
	ps := NewMemoryPubSub()
	defer func() { _ = ps.Close() }()
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_ = ps.Publish(ctx, benchEvent)
	}
}

func BenchmarkMemoryPubSub_FanOut(b *testing.B) {
	for _, subs := range []int{1, 10, 50} {
		b.Run(fmt.Sprintf("%dsubs", subs), func(b *testing.B) {
			ps := NewMemoryPubSub()
			// Register Close first (LIFO: cancel runs before Close, preventing double-close).
			b.Cleanup(func() { _ = ps.Close() })
			ctx := context.Background()
			for range subs {
				_, cancel, _ := ps.Subscribe(ctx, benchEvent.TenantID)
				b.Cleanup(cancel)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = ps.Publish(ctx, benchEvent)
			}
		})
	}
}

func newRedisPubSubClient(b *testing.B) *redis.Client {
	b.Helper()
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		b.Skip("REDIS_URL not set")
	}
	opt, err := redis.ParseURL(addr)
	if err != nil {
		b.Fatalf("parse REDIS_URL: %v", err)
	}
	return redis.NewClient(opt)
}

func BenchmarkRedisPubSub_Publish(b *testing.B) {
	client := newRedisPubSubClient(b)
	defer func() { _ = client.Close() }()
	p := NewRedisPublisher(client)
	defer func() { _ = p.Close() }()
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_ = p.Publish(ctx, benchEvent)
	}
}

func BenchmarkRedisPubSub_FanOut(b *testing.B) {
	for _, subs := range []int{1, 5, 20} {
		b.Run(fmt.Sprintf("%dsubs", subs), func(b *testing.B) {
			client := newRedisPubSubClient(b)
			defer func() { _ = client.Close() }()
			p := NewRedisPublisher(client)
			defer func() { _ = p.Close() }()
			subClient := newRedisPubSubClient(b)
			defer func() { _ = subClient.Close() }()
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 10}))
			subscriber := NewRedisSubscriber(subClient, logger)
			defer func() { _ = subscriber.Close() }()
			ctx := context.Background()
			for range subs {
				_, cancel, err := subscriber.Subscribe(ctx, benchEvent.TenantID)
				if err != nil {
					b.Skip("redis subscribe failed:", err)
				}
				b.Cleanup(cancel)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = p.Publish(ctx, benchEvent)
			}
		})
	}
}
