package audit

import (
	"context"
	"time"

	"github.com/opendecree/decree/internal/storage/domain"
)

// QueryWriteLogParams filters audit write log queries.
type QueryWriteLogParams struct {
	TenantID  string // optional — empty means all tenants
	Actor     string // optional
	FieldPath string // optional
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int32
	Offset    int32
}

// GetFieldUsageParams filters field usage queries.
type GetFieldUsageParams struct {
	TenantID  string
	FieldPath string
	StartTime *time.Time
	EndTime   *time.Time
}

// GetTenantUsageParams filters tenant usage queries.
type GetTenantUsageParams struct {
	TenantID  string
	StartTime *time.Time
	EndTime   *time.Time
}

// GetUnusedFieldsParams filters unused field queries.
type GetUnusedFieldsParams struct {
	TenantID string
	Since    time.Time
}

// UpsertUsageStatsParams contains parameters for upserting usage statistics.
type UpsertUsageStatsParams struct {
	TenantID    string
	FieldPath   string
	PeriodStart time.Time
	ReadCount   int64
	LastReadBy  *string
	LastReadAt  time.Time
}

// Store defines the data access interface for audit operations.
// Implementations must return [domain.ErrNotFound] when an entity is not found.
type Store interface {
	QueryAuditWriteLog(ctx context.Context, arg QueryWriteLogParams) ([]domain.AuditWriteLog, error)
	GetFieldUsage(ctx context.Context, arg GetFieldUsageParams) ([]domain.UsageStat, error)
	GetTenantUsage(ctx context.Context, arg GetTenantUsageParams) ([]domain.TenantUsageRow, error)
	GetUnusedFields(ctx context.Context, arg GetUnusedFieldsParams) ([]string, error)
	UpsertUsageStats(ctx context.Context, arg UpsertUsageStatsParams) error
}
