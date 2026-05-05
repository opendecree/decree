package audit

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

func genUUID() (pgtype.UUID, error) {
	var id pgtype.UUID
	if _, err := rand.Read(id.Bytes[:]); err != nil {
		return pgtype.UUID{}, fmt.Errorf("generate uuid: %w", err)
	}
	id.Bytes[6] = (id.Bytes[6] & 0x0f) | 0x40 // version 4
	id.Bytes[8] = (id.Bytes[8] & 0x3f) | 0x80 // variant 2
	id.Valid = true
	return id, nil
}

func (s *PGStore) InsertAuditWriteLog(ctx context.Context, arg InsertAuditWriteLogParams) error {
	var tenantUUID pgtype.UUID
	if arg.TenantID != "" {
		var err error
		tenantUUID, err = pgconv.StringToUUID(arg.TenantID)
		if err != nil {
			return err
		}
	}

	prevHash, err := s.write.GetLastAuditHashForTenant(ctx, tenantUUID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get last audit hash: %w", err)
	}

	id, err := genUUID()
	if err != nil {
		return err
	}

	kind := arg.ObjectKind
	if kind == "" {
		kind = "field"
	}
	now := time.Now()
	hash := ComputeEntryHash(ChainInput{
		PreviousHash: prevHash,
		ID:           pgconv.UUIDToString(id),
		TenantID:     arg.TenantID,
		Actor:        arg.Actor,
		Action:       arg.Action,
		ObjectKind:   kind,
		CreatedAt:    now,
	})

	return s.write.InsertAuditWriteLog(ctx, dbstore.InsertAuditWriteLogParams{
		ID:            id,
		TenantID:      tenantUUID,
		Actor:         arg.Actor,
		Action:        arg.Action,
		FieldPath:     arg.FieldPath,
		OldValue:      arg.OldValue,
		NewValue:      arg.NewValue,
		ConfigVersion: arg.ConfigVersion,
		Metadata:      arg.Metadata,
		ObjectKind:    kind,
		PreviousHash:  prevHash,
		EntryHash:     hash,
	})
}

func (s *PGStore) GetAuditWriteLogOrdered(ctx context.Context, tenantID string) ([]domain.AuditWriteLog, error) {
	var tenantUUID pgtype.UUID
	if tenantID != "" {
		var err error
		tenantUUID, err = pgconv.StringToUUID(tenantID)
		if err != nil {
			return nil, err
		}
	}
	rows, err := s.read.GetAuditWriteLogOrdered(ctx, tenantUUID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AuditWriteLog, len(rows))
	for i, r := range rows {
		result[i] = auditWriteLogFromDB(r)
	}
	return result, nil
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
		ObjectKind:    r.ObjectKind,
		FieldPath:     r.FieldPath,
		OldValue:      r.OldValue,
		NewValue:      r.NewValue,
		ConfigVersion: r.ConfigVersion,
		Metadata:      r.Metadata,
		PreviousHash:  r.PreviousHash,
		EntryHash:     r.EntryHash,
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
