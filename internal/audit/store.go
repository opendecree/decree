package audit

import (
	"context"

	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
)

// Store defines the data access interface for audit operations.
type Store interface {
	// Write log queries.
	QueryAuditWriteLog(ctx context.Context, arg dbstore.QueryAuditWriteLogParams) ([]dbstore.AuditWriteLog, error)

	// Usage stats.
	GetFieldUsage(ctx context.Context, arg dbstore.GetFieldUsageParams) ([]dbstore.UsageStat, error)
	GetTenantUsage(ctx context.Context, arg dbstore.GetTenantUsageParams) ([]dbstore.GetTenantUsageRow, error)
	GetUnusedFields(ctx context.Context, arg dbstore.GetUnusedFieldsParams) ([]string, error)
	UpsertUsageStats(ctx context.Context, arg dbstore.UpsertUsageStatsParams) error
}
