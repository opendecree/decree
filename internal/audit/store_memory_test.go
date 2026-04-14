package audit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage/domain"
)

func newTestMemoryStore() *MemoryStore {
	return NewMemoryStore()
}

func ptr[T any](v T) *T { return &v }

func TestMemoryStore_QueryWriteLog_Filters(t *testing.T) {
	ctx := context.Background()
	s := newTestMemoryStore()

	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	s.AddWriteLog(domain.AuditWriteLog{TenantID: "t1", Actor: "alice", Action: "set_field", FieldPath: ptr("app.fee"), CreatedAt: t1})
	s.AddWriteLog(domain.AuditWriteLog{TenantID: "t1", Actor: "bob", Action: "set_field", FieldPath: ptr("app.rate"), CreatedAt: t2})
	s.AddWriteLog(domain.AuditWriteLog{TenantID: "t2", Actor: "alice", Action: "set_field", FieldPath: ptr("app.fee"), CreatedAt: t3})

	// Filter by tenant.
	logs, err := s.QueryAuditWriteLog(ctx, QueryWriteLogParams{TenantID: "t1"})
	require.NoError(t, err)
	assert.Len(t, logs, 2)

	// Filter by actor.
	logs, err = s.QueryAuditWriteLog(ctx, QueryWriteLogParams{Actor: "alice"})
	require.NoError(t, err)
	assert.Len(t, logs, 2)

	// Filter by field path.
	logs, err = s.QueryAuditWriteLog(ctx, QueryWriteLogParams{FieldPath: "app.fee"})
	require.NoError(t, err)
	assert.Len(t, logs, 2)

	// Filter by time range.
	logs, err = s.QueryAuditWriteLog(ctx, QueryWriteLogParams{StartTime: &t2, EndTime: &t2})
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "bob", logs[0].Actor)

	// Combined filters.
	logs, err = s.QueryAuditWriteLog(ctx, QueryWriteLogParams{TenantID: "t1", Actor: "alice"})
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "app.fee", *logs[0].FieldPath)
}

func TestMemoryStore_QueryWriteLog_SortAndPagination(t *testing.T) {
	ctx := context.Background()
	s := newTestMemoryStore()

	for i := 0; i < 5; i++ {
		s.AddWriteLog(domain.AuditWriteLog{
			TenantID:  "t1",
			Actor:     "alice",
			Action:    "set_field",
			FieldPath: ptr("app.fee"),
			CreatedAt: time.Date(2026, 1, 1, i, 0, 0, 0, time.UTC),
		})
	}

	// Results should be sorted by CreatedAt DESC.
	logs, err := s.QueryAuditWriteLog(ctx, QueryWriteLogParams{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, logs, 3)
	assert.True(t, logs[0].CreatedAt.After(logs[1].CreatedAt))
	assert.True(t, logs[1].CreatedAt.After(logs[2].CreatedAt))

	// Offset.
	logs, err = s.QueryAuditWriteLog(ctx, QueryWriteLogParams{Limit: 2, Offset: 3})
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}

func TestMemoryStore_QueryWriteLog_Empty(t *testing.T) {
	ctx := context.Background()
	s := newTestMemoryStore()

	logs, err := s.QueryAuditWriteLog(ctx, QueryWriteLogParams{TenantID: "nonexistent"})
	require.NoError(t, err)
	assert.Empty(t, logs)
}

func TestMemoryStore_GetFieldUsage(t *testing.T) {
	ctx := context.Background()
	s := newTestMemoryStore()

	period1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	period2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	require.NoError(t, s.UpsertUsageStats(ctx, UpsertUsageStatsParams{
		TenantID: "t1", FieldPath: "app.fee", PeriodStart: period1,
		ReadCount: 10, LastReadBy: ptr("reader1"), LastReadAt: period1,
	}))
	require.NoError(t, s.UpsertUsageStats(ctx, UpsertUsageStatsParams{
		TenantID: "t1", FieldPath: "app.fee", PeriodStart: period2,
		ReadCount: 20, LastReadBy: ptr("reader2"), LastReadAt: period2,
	}))
	require.NoError(t, s.UpsertUsageStats(ctx, UpsertUsageStatsParams{
		TenantID: "t1", FieldPath: "app.rate", PeriodStart: period1,
		ReadCount: 5, LastReadBy: ptr("reader1"), LastReadAt: period1,
	}))

	// Get all periods for app.fee.
	stats, err := s.GetFieldUsage(ctx, GetFieldUsageParams{TenantID: "t1", FieldPath: "app.fee"})
	require.NoError(t, err)
	assert.Len(t, stats, 2)

	// Filter by time range.
	stats, err = s.GetFieldUsage(ctx, GetFieldUsageParams{
		TenantID: "t1", FieldPath: "app.fee", StartTime: &period2,
	})
	require.NoError(t, err)
	assert.Len(t, stats, 1)
	assert.Equal(t, int64(20), stats[0].ReadCount)

	// Wrong tenant.
	stats, err = s.GetFieldUsage(ctx, GetFieldUsageParams{TenantID: "t2", FieldPath: "app.fee"})
	require.NoError(t, err)
	assert.Empty(t, stats)
}

func TestMemoryStore_GetTenantUsage(t *testing.T) {
	ctx := context.Background()
	s := newTestMemoryStore()

	period1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	period2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	require.NoError(t, s.UpsertUsageStats(ctx, UpsertUsageStatsParams{
		TenantID: "t1", FieldPath: "app.fee", PeriodStart: period1,
		ReadCount: 10, LastReadBy: ptr("r1"), LastReadAt: period1,
	}))
	require.NoError(t, s.UpsertUsageStats(ctx, UpsertUsageStatsParams{
		TenantID: "t1", FieldPath: "app.fee", PeriodStart: period2,
		ReadCount: 20, LastReadBy: ptr("r2"), LastReadAt: period2,
	}))
	require.NoError(t, s.UpsertUsageStats(ctx, UpsertUsageStatsParams{
		TenantID: "t1", FieldPath: "app.rate", PeriodStart: period1,
		ReadCount: 5, LastReadBy: ptr("r1"), LastReadAt: period1,
	}))

	rows, err := s.GetTenantUsage(ctx, GetTenantUsageParams{TenantID: "t1"})
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	// Sorted by field path.
	assert.Equal(t, "app.fee", rows[0].FieldPath)
	assert.Equal(t, int64(30), rows[0].ReadCount)
	assert.Equal(t, period2, *rows[0].LastReadAt)

	assert.Equal(t, "app.rate", rows[1].FieldPath)
	assert.Equal(t, int64(5), rows[1].ReadCount)

	// Filter by time range: only period2.
	rows, err = s.GetTenantUsage(ctx, GetTenantUsageParams{TenantID: "t1", StartTime: &period2})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "app.fee", rows[0].FieldPath)
	assert.Equal(t, int64(20), rows[0].ReadCount)
}

func TestMemoryStore_GetTenantUsage_Empty(t *testing.T) {
	ctx := context.Background()
	s := newTestMemoryStore()

	rows, err := s.GetTenantUsage(ctx, GetTenantUsageParams{TenantID: "nonexistent"})
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestMemoryStore_UpsertUsageStats_InsertAndUpdate(t *testing.T) {
	ctx := context.Background()
	s := newTestMemoryStore()

	period := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Insert.
	require.NoError(t, s.UpsertUsageStats(ctx, UpsertUsageStatsParams{
		TenantID: "t1", FieldPath: "app.fee", PeriodStart: period,
		ReadCount: 10, LastReadBy: ptr("reader1"), LastReadAt: period,
	}))

	stats, err := s.GetFieldUsage(ctx, GetFieldUsageParams{TenantID: "t1", FieldPath: "app.fee"})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(10), stats[0].ReadCount)
	assert.Equal(t, "reader1", *stats[0].LastReadBy)

	// Update (same tenant + field + period): count should accumulate.
	laterTime := period.Add(time.Hour)
	require.NoError(t, s.UpsertUsageStats(ctx, UpsertUsageStatsParams{
		TenantID: "t1", FieldPath: "app.fee", PeriodStart: period,
		ReadCount: 5, LastReadBy: ptr("reader2"), LastReadAt: laterTime,
	}))

	stats, err = s.GetFieldUsage(ctx, GetFieldUsageParams{TenantID: "t1", FieldPath: "app.fee"})
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, int64(15), stats[0].ReadCount)
	assert.Equal(t, "reader2", *stats[0].LastReadBy)
	assert.Equal(t, laterTime, *stats[0].LastReadAt)
}

func TestMemoryStore_GetUnusedFields_ReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	s := newTestMemoryStore()

	paths, err := s.GetUnusedFields(ctx, GetUnusedFieldsParams{TenantID: "t1", Since: time.Now()})
	require.NoError(t, err)
	assert.Empty(t, paths)
}

func TestMemoryStore_InterfaceCompliance(t *testing.T) {
	var _ Store = (*MemoryStore)(nil)
}
