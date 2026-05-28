package cache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func BenchmarkMemoryCache_Hit(b *testing.B) {
	c := NewMemoryCache(0)
	defer c.Stop()
	ctx := context.Background()
	_ = c.Set(ctx, "t1", 1, map[string]string{"a": "1", "b": "2", "c": "3"}, time.Minute)
	b.ReportAllocs()
	for b.Loop() {
		_, _ = c.Get(ctx, "t1", 1)
	}
}

func BenchmarkMemoryCache_Miss(b *testing.B) {
	c := NewMemoryCache(0)
	defer c.Stop()
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = c.Get(ctx, "t1", 1)
	}
}

func BenchmarkMemoryCache_SetEvictsOldest(b *testing.B) {
	const cap = 100
	c := NewMemoryCache(cap)
	defer c.Stop()
	ctx := context.Background()
	for i := range cap {
		_ = c.Set(ctx, fmt.Sprintf("seed%d", i), 1, map[string]string{"v": "x"}, time.Minute)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = c.Set(ctx, fmt.Sprintf("new%d", i), 1, map[string]string{"v": "x"}, time.Minute)
	}
}

func newRedisClient(b *testing.B) *redis.Client {
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

func BenchmarkRedisCache_Get(b *testing.B) {
	client := newRedisClient(b)
	defer func() { _ = client.Close() }()
	c := NewRedisCache(client)
	ctx := context.Background()
	_ = c.Set(ctx, "bench-t1", 1, map[string]string{"a": "1", "b": "2"}, time.Minute)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = c.Get(ctx, "bench-t1", 1)
	}
}

func BenchmarkRedisCache_Set(b *testing.B) {
	client := newRedisClient(b)
	defer func() { _ = client.Close() }()
	c := NewRedisCache(client)
	ctx := context.Background()
	vals := map[string]string{"a": "1", "b": "2", "c": "3"}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_ = c.Set(ctx, fmt.Sprintf("bench-t%d", i%10), 1, vals, time.Minute)
	}
}

func BenchmarkRedisCache_Invalidate(b *testing.B) {
	client := newRedisClient(b)
	defer func() { _ = client.Close() }()
	c := NewRedisCache(client)
	ctx := context.Background()
	vals := map[string]string{"a": "1"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		tid := fmt.Sprintf("bench-inv%d", i%5)
		_ = c.Set(ctx, tid, 1, vals, time.Minute)
		_ = c.Invalidate(ctx, tid)
	}
}
