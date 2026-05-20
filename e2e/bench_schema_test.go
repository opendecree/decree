//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/opendecree/decree/sdk/adminclient"
)

func BenchmarkCreateSchema(b *testing.B) {
	conn := dialBench(b)
	admin := newAdminClient(conn)
	ctx := context.Background()

	var ids []string
	b.Cleanup(func() {
		for _, id := range ids {
			_ = admin.DeleteSchema(ctx, id)
		}
	})

	latencies := make([]float64, 0)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		start := time.Now()
		s, _ := admin.CreateSchema(ctx, fmt.Sprintf("bench-create-%d", i), []adminclient.Field{
			{Path: "f.value", Type: "FIELD_TYPE_STRING"},
		}, "")
		latencies = append(latencies, float64(time.Since(start).Nanoseconds()))
		if s != nil {
			ids = append(ids, s.ID)
		}
	}
	reportLatencyPercentiles(b, latencies)
}

func BenchmarkPublishSchemaVersion(b *testing.B) {
	conn := dialBench(b)
	admin := newAdminClient(conn)
	ctx := context.Background()

	var ids []string
	b.Cleanup(func() {
		for _, id := range ids {
			_ = admin.DeleteSchema(ctx, id)
		}
	})

	latencies := make([]float64, 0)
	for i := 0; b.Loop(); i++ {
		b.StopTimer()
		s, err := admin.CreateSchema(ctx, fmt.Sprintf("bench-pub-%d", i), []adminclient.Field{
			{Path: "f.value", Type: "FIELD_TYPE_STRING"},
		}, "")
		if err != nil || s == nil {
			b.StartTimer()
			continue
		}
		ids = append(ids, s.ID)
		b.StartTimer()

		start := time.Now()
		_, _ = admin.PublishSchema(ctx, s.ID, 1)
		latencies = append(latencies, float64(time.Since(start).Nanoseconds()))
	}
	reportLatencyPercentiles(b, latencies)
}

func reportLatencyPercentiles(b *testing.B, ns []float64) {
	b.Helper()
	n := len(ns)
	if n == 0 {
		return
	}
	sort.Float64s(ns)
	b.ReportMetric(ns[n/2], "p50_ns")
	b.ReportMetric(ns[min(int(float64(n)*0.95), n-1)], "p95_ns")
	b.ReportMetric(ns[min(int(float64(n)*0.99), n-1)], "p99_ns")
}
