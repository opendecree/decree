//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/sdk/adminclient"
)

// benchAuditEnv creates a schema + tenant with n config writes to seed the audit log.
func benchAuditEnv(b *testing.B, name string, writes int) (*adminclient.Client, string, func()) {
	b.Helper()
	conn := dialBench(b)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	s, err := admin.CreateSchema(ctx, name, []adminclient.Field{
		{Path: "bench.string", Type: adminclient.FieldTypeString},
	}, "")
	require.NoError(b, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(b, err)

	tenant, err := admin.CreateTenant(ctx, name+"-tenant", s.ID, 1)
	require.NoError(b, err)

	for i := range writes {
		require.NoError(b, noVer(cfg.Set(ctx, tenant.ID, "bench.string", fmt.Sprintf("val-%d", i))))
	}

	return admin, tenant.ID, func() {
		_ = admin.DeleteTenant(ctx, tenant.ID)
		_ = admin.DeleteSchema(ctx, s.ID)
	}
}

func BenchmarkQueryWriteLog(b *testing.B) {
	admin, tenantID, cleanup := benchAuditEnv(b, "bench-audit-query", 20)
	defer cleanup()
	ctx := context.Background()

	latencies := make([]float64, 0)
	b.ResetTimer()
	for b.Loop() {
		start := time.Now()
		_, _ = admin.QueryWriteLog(ctx, adminclient.WithAuditTenant(tenantID))
		latencies = append(latencies, float64(time.Since(start).Nanoseconds()))
	}
	reportLatencyPercentiles(b, latencies)
}

func BenchmarkVerifyChain(b *testing.B) {
	admin, tenantID, cleanup := benchAuditEnv(b, "bench-audit-verify", 20)
	defer cleanup()
	ctx := context.Background()

	latencies := make([]float64, 0)
	b.ResetTimer()
	for b.Loop() {
		start := time.Now()
		_, _ = admin.VerifyChain(ctx, tenantID)
		latencies = append(latencies, float64(time.Since(start).Nanoseconds()))
	}
	reportLatencyPercentiles(b, latencies)
}
