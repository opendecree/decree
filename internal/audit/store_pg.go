package audit

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opendecree/decree/internal/storage/dbstore"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/storage/pgconv"
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

func (s *PGStore) QueryAuditWriteLog(ctx context.Context, arg QueryWriteLogParams) ([]domain.AuditWriteLog, error) {
	var tenantUUID pgtype.UUID
	if arg.TenantID != "" {
		var err error
		tenantUUID, err = pgconv.StringToUUID(arg.TenantID)
		if err != nil {
			return nil, err
		}
	}

	rows, err := s.read.QueryAuditWriteLog(ctx, dbstore.QueryAuditWriteLogParams{
		Column1: tenantUUID,
		Column2: arg.Actor,
		Column3: arg.FieldPath,
		Column4: pgconv.OptionalTimeToTimestamptz(arg.StartTime),
		Column5: pgconv.OptionalTimeToTimestamptz(arg.EndTime),
		Limit:   arg.Limit,
		Offset:  arg.Offset,
	})
	if err != nil {
		return nil, err
	}

	result := make([]domain.AuditWriteLog, len(rows))
	for i, r := range rows {
		result[i] = auditWriteLogFromDB(r)
	}
	return result, nil
}

func (s *PGStore) GetFieldUsage(ctx context.Context, arg GetFieldUsageParams) ([]domain.UsageStat, error) {
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return nil, err
	}

	rows, err := s.read.GetFieldUsage(ctx, dbstore.GetFieldUsageParams{
		TenantID:  tenantUUID,
		FieldPath: arg.FieldPath,
		Column3:   pgconv.OptionalTimeToTimestamptz(arg.StartTime),
		Column4:   pgconv.OptionalTimeToTimestamptz(arg.EndTime),
	})
	if err != nil {
		return nil, err
	}

	result := make([]domain.UsageStat, len(rows))
	for i, r := range rows {
		result[i] = usageStatFromDB(r)
	}
	return result, nil
}

func (s *PGStore) GetTenantUsage(ctx context.Context, arg GetTenantUsageParams) ([]domain.TenantUsageRow, error) {
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return nil, err
	}

	rows, err := s.read.GetTenantUsage(ctx, dbstore.GetTenantUsageParams{
		TenantID: tenantUUID,
		Column2:  pgconv.OptionalTimeToTimestamptz(arg.StartTime),
		Column3:  pgconv.OptionalTimeToTimestamptz(arg.EndTime),
	})
	if err != nil {
		return nil, err
	}

	result := make([]domain.TenantUsageRow, len(rows))
	for i, r := range rows {
		result[i] = tenantUsageRowFromDB(r)
	}
	return result, nil
}

func (s *PGStore) GetUnusedFields(ctx context.Context, arg GetUnusedFieldsParams) ([]string, error) {
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return nil, err
	}

	return s.read.GetUnusedFields(ctx, dbstore.GetUnusedFieldsParams{
		ID:         tenantUUID,
		LastReadAt: pgconv.TimeToTimestamptz(arg.Since),
	})
}

func (s *PGStore) UpsertUsageStats(ctx context.Context, arg UpsertUsageStatsParams) error {
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return err
	}

	return s.write.UpsertUsageStats(ctx, dbstore.UpsertUsageStatsParams{
		TenantID:    tenantUUID,
		FieldPath:   arg.FieldPath,
		PeriodStart: pgconv.TimeToTimestamptz(arg.PeriodStart),
		ReadCount:   arg.ReadCount,
		LastReadBy:  arg.LastReadBy,
		LastReadAt:  pgconv.TimeToTimestamptz(arg.LastReadAt),
	})
}

// --- DB → domain conversion helpers ---

func auditWriteLogFromDB(r dbstore.AuditWriteLog) domain.AuditWriteLog {
	return domain.AuditWriteLog{
		ID:            pgconv.UUIDToString(r.ID),
		TenantID:      pgconv.UUIDToString(r.TenantID),
		Actor:         r.Actor,
		Action:        r.Action,
		FieldPath:     r.FieldPath,
		OldValue:      r.OldValue,
		NewValue:      r.NewValue,
		ConfigVersion: r.ConfigVersion,
		Metadata:      r.Metadata,
		CreatedAt:     pgconv.TimestamptzToTime(r.CreatedAt),
	}
}

func usageStatFromDB(r dbstore.UsageStat) domain.UsageStat {
	return domain.UsageStat{
		TenantID:    pgconv.UUIDToString(r.TenantID),
		FieldPath:   r.FieldPath,
		PeriodStart: pgconv.TimestamptzToTime(r.PeriodStart),
		ReadCount:   r.ReadCount,
		LastReadBy:  r.LastReadBy,
		LastReadAt:  pgconv.TimestamptzToOptionalTime(r.LastReadAt),
	}
}

func tenantUsageRowFromDB(r dbstore.GetTenantUsageRow) domain.TenantUsageRow {
	row := domain.TenantUsageRow{
		FieldPath: r.FieldPath,
		ReadCount: r.ReadCount,
	}
	// LastReadAt comes as interface{} from MAX() aggregate.
	if t, ok := r.LastReadAt.(pgtype.Timestamptz); ok && t.Valid {
		row.LastReadAt = &t.Time
	}
	return row
}
