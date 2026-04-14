package audit

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
)

// PGStore implements Store using PostgreSQL via sqlc-generated queries.
type PGStore struct {
	write *dbstore.Queries
	read  *dbstore.Queries
}

// NewPGStore creates a new PostgreSQL-backed audit store.
func NewPGStore(writePool, readPool *pgxpool.Pool) *PGStore {
	return &PGStore{
		write: dbstore.New(writePool),
		read:  dbstore.New(readPool),
	}
}

func (s *PGStore) QueryAuditWriteLog(ctx context.Context, arg dbstore.QueryAuditWriteLogParams) ([]dbstore.AuditWriteLog, error) {
	return s.read.QueryAuditWriteLog(ctx, arg)
}

func (s *PGStore) GetFieldUsage(ctx context.Context, arg dbstore.GetFieldUsageParams) ([]dbstore.UsageStat, error) {
	return s.read.GetFieldUsage(ctx, arg)
}

func (s *PGStore) GetTenantUsage(ctx context.Context, arg dbstore.GetTenantUsageParams) ([]dbstore.GetTenantUsageRow, error) {
	return s.read.GetTenantUsage(ctx, arg)
}

func (s *PGStore) GetUnusedFields(ctx context.Context, arg dbstore.GetUnusedFieldsParams) ([]string, error) {
	return s.read.GetUnusedFields(ctx, arg)
}

func (s *PGStore) UpsertUsageStats(ctx context.Context, arg dbstore.UpsertUsageStatsParams) error {
	return s.write.UpsertUsageStats(ctx, arg)
}
