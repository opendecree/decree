package config

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opendecree/decree/internal/storage/dbstore"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/storage/pgconv"
)

// PGStore implements Store using PostgreSQL via sqlc-generated queries.
type PGStore struct {
	writePool *pgxpool.Pool
	write     *dbstore.Queries
	read      *dbstore.Queries
}

// NewPGStore creates a new PostgreSQL-backed config store.
func NewPGStore(writePool, readPool *pgxpool.Pool) *PGStore {
	return &PGStore{
		writePool: writePool,
		write:     dbstore.New(writePool),
		read:      dbstore.New(readPool),
	}
}

// RunInTx executes fn within a database transaction.
func (s *PGStore) RunInTx(ctx context.Context, fn func(Store) error) error {
	tx, err := s.writePool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after commit

	txStore := &PGStore{
		writePool: s.writePool,
		write:     s.write.WithTx(tx),
		read:      s.read,
	}

	if err := fn(txStore); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// Config versions.

func (s *PGStore) CreateConfigVersion(ctx context.Context, arg CreateConfigVersionParams) (domain.ConfigVersion, error) {
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	row, err := s.write.CreateConfigVersion(ctx, dbstore.CreateConfigVersionParams{
		TenantID:    tenantUUID,
		Version:     arg.Version,
		Description: arg.Description,
		CreatedBy:   arg.CreatedBy,
	})
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	return configVersionFromDB(row), nil
}

func (s *PGStore) GetConfigVersion(ctx context.Context, arg GetConfigVersionParams) (domain.ConfigVersion, error) {
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	row, err := s.read.GetConfigVersion(ctx, dbstore.GetConfigVersionParams{
		TenantID: tenantUUID,
		Version:  arg.Version,
	})
	if err != nil {
		return domain.ConfigVersion{}, pgconv.WrapNotFound(err)
	}
	return configVersionFromDB(row), nil
}

func (s *PGStore) GetLatestConfigVersion(ctx context.Context, tenantID string) (domain.ConfigVersion, error) {
	tenantUUID, err := pgconv.StringToUUID(tenantID)
	if err != nil {
		return domain.ConfigVersion{}, err
	}
	row, err := s.read.GetLatestConfigVersion(ctx, tenantUUID)
	if err != nil {
		return domain.ConfigVersion{}, pgconv.WrapNotFound(err)
	}
	return configVersionFromDB(row), nil
}

func (s *PGStore) ListConfigVersions(ctx context.Context, arg ListConfigVersionsParams) ([]domain.ConfigVersion, error) {
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.ListConfigVersions(ctx, dbstore.ListConfigVersionsParams{
		TenantID: tenantUUID,
		Limit:    arg.Limit,
		Offset:   arg.Offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]domain.ConfigVersion, len(rows))
	for i, r := range rows {
		result[i] = configVersionFromDB(r)
	}
	return result, nil
}

// Config values.

func (s *PGStore) SetConfigValue(ctx context.Context, arg SetConfigValueParams) error {
	cvUUID, err := pgconv.StringToUUID(arg.ConfigVersionID)
	if err != nil {
		return err
	}
	return s.write.SetConfigValue(ctx, dbstore.SetConfigValueParams{
		ConfigVersionID: cvUUID,
		FieldPath:       arg.FieldPath,
		Value:           arg.Value,
		Checksum:        arg.Checksum,
		Description:     arg.Description,
	})
}

func (s *PGStore) GetConfigValues(ctx context.Context, configVersionID string) ([]domain.ConfigValue, error) {
	cvUUID, err := pgconv.StringToUUID(configVersionID)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.GetConfigValues(ctx, cvUUID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.ConfigValue, len(rows))
	for i, r := range rows {
		result[i] = configValueFromDB(r)
	}
	return result, nil
}

func (s *PGStore) GetConfigValueAtVersion(ctx context.Context, arg GetConfigValueAtVersionParams) (GetConfigValueAtVersionRow, error) {
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return GetConfigValueAtVersionRow{}, err
	}
	row, err := s.read.GetConfigValueAtVersion(ctx, dbstore.GetConfigValueAtVersionParams{
		TenantID:  tenantUUID,
		FieldPath: arg.FieldPath,
		Version:   arg.Version,
	})
	if err != nil {
		return GetConfigValueAtVersionRow{}, pgconv.WrapNotFound(err)
	}
	return GetConfigValueAtVersionRow{
		FieldPath:   row.FieldPath,
		Value:       row.Value,
		Checksum:    row.Checksum,
		Description: row.Description,
	}, nil
}

func (s *PGStore) GetFullConfigAtVersion(ctx context.Context, arg GetFullConfigAtVersionParams) ([]GetFullConfigAtVersionRow, error) {
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.GetFullConfigAtVersion(ctx, dbstore.GetFullConfigAtVersionParams{
		TenantID: tenantUUID,
		Version:  arg.Version,
	})
	if err != nil {
		return nil, err
	}
	result := make([]GetFullConfigAtVersionRow, len(rows))
	for i, r := range rows {
		result[i] = GetFullConfigAtVersionRow{
			FieldPath:   r.FieldPath,
			Value:       r.Value,
			Checksum:    r.Checksum,
			Description: r.Description,
		}
	}
	return result, nil
}

// Tenant/schema lookup.

func (s *PGStore) GetTenantByID(ctx context.Context, id string) (domain.Tenant, error) {
	idUUID, err := pgconv.StringToUUID(id)
	if err != nil {
		return domain.Tenant{}, err
	}
	row, err := s.read.GetTenantByID(ctx, idUUID)
	if err != nil {
		return domain.Tenant{}, pgconv.WrapNotFound(err)
	}
	return tenantFromDB(row), nil
}

func (s *PGStore) GetSchemaFields(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error) {
	svUUID, err := pgconv.StringToUUID(schemaVersionID)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.GetSchemaFields(ctx, svUUID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.SchemaField, len(rows))
	for i, r := range rows {
		result[i] = schemaFieldFromDB(r)
	}
	return result, nil
}

func (s *PGStore) GetSchemaVersion(ctx context.Context, arg domain.SchemaVersionKey) (domain.SchemaVersion, error) {
	schemaUUID, err := pgconv.StringToUUID(arg.SchemaID)
	if err != nil {
		return domain.SchemaVersion{}, err
	}
	row, err := s.read.GetSchemaVersion(ctx, dbstore.GetSchemaVersionParams{
		SchemaID: schemaUUID,
		Version:  arg.Version,
	})
	if err != nil {
		return domain.SchemaVersion{}, pgconv.WrapNotFound(err)
	}
	return schemaVersionFromDB(row), nil
}

func (s *PGStore) GetFieldLocks(ctx context.Context, tenantID string) ([]domain.TenantFieldLock, error) {
	tenantUUID, err := pgconv.StringToUUID(tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.GetFieldLocks(ctx, tenantUUID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.TenantFieldLock, len(rows))
	for i, r := range rows {
		result[i] = fieldLockFromDB(r)
	}
	return result, nil
}

// Audit.

func (s *PGStore) InsertAuditWriteLog(ctx context.Context, arg InsertAuditWriteLogParams) error {
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return err
	}
	return s.write.InsertAuditWriteLog(ctx, dbstore.InsertAuditWriteLogParams{
		TenantID:      tenantUUID,
		Actor:         arg.Actor,
		Action:        arg.Action,
		FieldPath:     arg.FieldPath,
		OldValue:      arg.OldValue,
		NewValue:      arg.NewValue,
		ConfigVersion: arg.ConfigVersion,
		Metadata:      arg.Metadata,
	})
}

// --- DB → domain conversion helpers ---

func configVersionFromDB(r dbstore.ConfigVersion) domain.ConfigVersion {
	return domain.ConfigVersion{
		ID:          pgconv.UUIDToString(r.ID),
		TenantID:    pgconv.UUIDToString(r.TenantID),
		Version:     r.Version,
		Description: r.Description,
		CreatedBy:   r.CreatedBy,
		CreatedAt:   pgconv.TimestamptzToTime(r.CreatedAt),
	}
}

func configValueFromDB(r dbstore.ConfigValue) domain.ConfigValue {
	return domain.ConfigValue{
		ConfigVersionID: pgconv.UUIDToString(r.ConfigVersionID),
		FieldPath:       r.FieldPath,
		Value:           r.Value,
		Checksum:        r.Checksum,
		Description:     r.Description,
	}
}

func tenantFromDB(r dbstore.Tenant) domain.Tenant {
	return domain.Tenant{
		ID:            pgconv.UUIDToString(r.ID),
		Name:          r.Name,
		SchemaID:      pgconv.UUIDToString(r.SchemaID),
		SchemaVersion: r.SchemaVersion,
		CreatedAt:     pgconv.TimestamptzToTime(r.CreatedAt),
		UpdatedAt:     pgconv.TimestamptzToTime(r.UpdatedAt),
	}
}

func schemaFieldFromDB(r dbstore.SchemaField) domain.SchemaField {
	return domain.SchemaField{
		ID:              pgconv.UUIDToString(r.ID),
		SchemaVersionID: pgconv.UUIDToString(r.SchemaVersionID),
		Path:            r.Path,
		FieldType:       domain.FieldType(r.FieldType),
		Constraints:     r.Constraints,
		Nullable:        r.Nullable,
		Deprecated:      r.Deprecated,
		RedirectTo:      r.RedirectTo,
		DefaultValue:    r.DefaultValue,
		Description:     r.Description,
	}
}

func schemaVersionFromDB(r dbstore.SchemaVersion) domain.SchemaVersion {
	return domain.SchemaVersion{
		ID:            pgconv.UUIDToString(r.ID),
		SchemaID:      pgconv.UUIDToString(r.SchemaID),
		Version:       r.Version,
		ParentVersion: r.ParentVersion,
		Description:   r.Description,
		Checksum:      r.Checksum,
		Published:     r.Published,
		CreatedAt:     pgconv.TimestamptzToTime(r.CreatedAt),
	}
}

func fieldLockFromDB(r dbstore.TenantFieldLock) domain.TenantFieldLock {
	return domain.TenantFieldLock{
		TenantID:     pgconv.UUIDToString(r.TenantID),
		FieldPath:    r.FieldPath,
		LockedValues: r.LockedValues,
	}
}
