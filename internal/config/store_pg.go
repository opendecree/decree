package config

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/opendecree/decree/internal/audit"
	"github.com/opendecree/decree/internal/storage"
	"github.com/opendecree/decree/internal/storage/dbstore"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/storage/pgconv"
)

// PGStore implements Store using PostgreSQL via sqlc-generated queries.
type PGStore struct {
	writePool *pgxpool.Pool
	write     *dbstore.Queries
	read      *dbstore.Queries
	tx        pgx.Tx // non-nil when bound to a transaction
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
//
// Both write and read query handles are bound to the same transaction so
// that reads inside fn observe the transaction's own staged writes. This
// matters for cross-field validators (e.g. dependentRequired) that need to
// evaluate against the post-merge snapshot before commit — reading from
// the read pool would return pre-tx state and miss the new values.
func (s *PGStore) RunInTx(ctx context.Context, fn func(Store) error) error {
	tx, err := s.writePool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after commit

	if tid := storage.TenantIDFromCtx(ctx); tid != "" {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tid); err != nil {
			return fmt.Errorf("set tenant guc: %w", err)
		}
	}

	txQueries := s.write.WithTx(tx)
	txStore := &PGStore{
		writePool: s.writePool,
		write:     txQueries,
		read:      txQueries,
		tx:        tx,
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ConfigVersion{}, ErrVersionConflict
		}
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

func (s *PGStore) GetConfigValuesSince(ctx context.Context, arg GetConfigValuesSinceParams) ([]ConfigValueSince, error) {
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.GetConfigValuesSince(ctx, dbstore.GetConfigValuesSinceParams{
		TenantID: tenantUUID,
		Version:  arg.StartVersion,
	})
	if err != nil {
		return nil, err
	}
	result := make([]ConfigValueSince, len(rows))
	for i, r := range rows {
		result[i] = ConfigValueSince{
			FieldPath: r.FieldPath,
			Value:     r.Value,
			Version:   r.Version,
			CreatedBy: r.CreatedBy,
			ChangedAt: r.CreatedAt.Time,
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

func (s *PGStore) GetTenantByName(ctx context.Context, name string) (domain.Tenant, error) {
	row, err := s.read.GetTenantByName(ctx, name)
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

// batcher returns the BatchSender appropriate for the current context:
// the active transaction when inside RunInTx, otherwise the write pool.
func (s *PGStore) batcher() interface {
	SendBatch(context.Context, *pgx.Batch) pgx.BatchResults
} {
	if s.tx != nil {
		return s.tx
	}
	return s.writePool
}

const bulkSetConfigValueSQL = `INSERT INTO config_values (config_version_id, field_path, value, checksum, description) VALUES ($1, $2, $3, $4, $5)`

func (s *PGStore) BulkSetConfigValues(ctx context.Context, args []SetConfigValueParams) error {
	if len(args) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, a := range args {
		cvUUID, err := pgconv.StringToUUID(a.ConfigVersionID)
		if err != nil {
			return err
		}
		batch.Queue(bulkSetConfigValueSQL, cvUUID, a.FieldPath, a.Value, a.Checksum, a.Description)
	}
	br := s.batcher().SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()
	for range args {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("bulk set config value: %w", err)
		}
	}
	return br.Close()
}

func (s *PGStore) BulkInsertAuditWriteLog(ctx context.Context, args []InsertAuditWriteLogParams) error {
	if len(args) == 0 {
		return nil
	}
	tenantUUID, err := pgconv.StringToUUID(args[0].TenantID)
	if err != nil {
		return err
	}

	// Acquire the advisory lock before reading the previous hash and writing
	// entries. This prevents concurrent writers for the same tenant from forking
	// the chain. The lock is held until the surrounding transaction commits or
	// rolls back.
	lockKey := "audit_chain:" + args[0].TenantID
	if s.tx != nil {
		if _, err := s.tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1)::bigint)", lockKey); err != nil {
			return fmt.Errorf("acquire audit chain lock: %w", err)
		}
	}

	prevHash, err := s.write.GetLastAuditHashForTenant(ctx, tenantUUID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get last audit hash: %w", err)
	}

	for _, arg := range args {
		id, err := configGenUUID()
		if err != nil {
			return err
		}
		kind := arg.ObjectKind
		if kind == "" {
			kind = "field"
		}
		now, err := s.configDBNow(ctx)
		if err != nil {
			return err
		}
		hash := audit.ComputeEntryHash(audit.ChainInput{
			PreviousHash:  prevHash,
			ID:            pgconv.UUIDToString(id),
			TenantID:      arg.TenantID,
			Actor:         arg.Actor,
			Action:        arg.Action,
			ObjectKind:    kind,
			CreatedAt:     now,
			Epoch:         1,
			FieldPath:     arg.FieldPath,
			OldValue:      arg.OldValue,
			NewValue:      arg.NewValue,
			ConfigVersion: arg.ConfigVersion,
			Metadata:      arg.Metadata,
		})
		if err := s.write.InsertAuditWriteLog(ctx, dbstore.InsertAuditWriteLogParams{
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
			CreatedAt:     pgconv.TimeToTimestamptz(now),
			ChainEpoch:    1,
		}); err != nil {
			return fmt.Errorf("insert audit log: %w", err)
		}
		prevHash = hash
	}
	return nil
}

// Audit.

// configDBNow fetches the current timestamp from the database, mirroring the
// audit store's dbNow pattern. Using the DB clock ensures the timestamp stored
// in PG round-trips exactly when VerifyChain recomputes the hash, avoiding
// any sub-microsecond skew from the application server clock.
func (s *PGStore) configDBNow(ctx context.Context) (time.Time, error) {
	var ts pgtype.Timestamptz
	var err error
	if s.tx != nil {
		err = s.tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&ts)
	} else {
		err = s.writePool.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&ts)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("fetch db timestamp: %w", err)
	}
	return ts.Time.Truncate(time.Microsecond), nil
}

func configGenUUID() (pgtype.UUID, error) {
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
	tenantUUID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return err
	}

	// Acquire the advisory lock before reading the previous hash and inserting.
	// This prevents concurrent writers for the same tenant from forking the chain.
	// The lock is held until the surrounding transaction commits or rolls back.
	lockKey := "audit_chain:" + arg.TenantID
	if s.tx != nil {
		if _, err := s.tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1)::bigint)", lockKey); err != nil {
			return fmt.Errorf("acquire audit chain lock: %w", err)
		}
	}

	prevHash, err := s.write.GetLastAuditHashForTenant(ctx, tenantUUID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get last audit hash: %w", err)
	}

	id, err := configGenUUID()
	if err != nil {
		return err
	}

	kind := arg.ObjectKind
	if kind == "" {
		kind = "field"
	}
	// Use the DB clock (same pattern as internal/audit/store_pg.go dbNow) so the
	// timestamp round-trips exactly when VerifyChain recomputes the hash.
	now, err := s.configDBNow(ctx)
	if err != nil {
		return err
	}
	hash := audit.ComputeEntryHash(audit.ChainInput{
		PreviousHash:  prevHash,
		ID:            pgconv.UUIDToString(id),
		TenantID:      arg.TenantID,
		Actor:         arg.Actor,
		Action:        arg.Action,
		ObjectKind:    kind,
		CreatedAt:     now,
		Epoch:         1,
		FieldPath:     arg.FieldPath,
		OldValue:      arg.OldValue,
		NewValue:      arg.NewValue,
		ConfigVersion: arg.ConfigVersion,
		Metadata:      arg.Metadata,
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
		CreatedAt:     pgconv.TimeToTimestamptz(now),
		ChainEpoch:    1,
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
		Sensitive:       r.Sensitive,
		ReadOnly:        r.ReadOnly,
		WriteOnce:       r.WriteOnce,
	}
}

func schemaVersionFromDB(r dbstore.SchemaVersion) domain.SchemaVersion {
	return domain.SchemaVersion{
		ID:                pgconv.UUIDToString(r.ID),
		SchemaID:          pgconv.UUIDToString(r.SchemaID),
		Version:           r.Version,
		ParentVersion:     r.ParentVersion,
		Description:       r.Description,
		Checksum:          r.Checksum,
		Published:         r.Published,
		DependentRequired: r.DependentRequired,
		Validations:       r.Validations,
		CreatedAt:         pgconv.TimestamptzToTime(r.CreatedAt),
	}
}

func fieldLockFromDB(r dbstore.TenantFieldLock) domain.TenantFieldLock {
	return domain.TenantFieldLock{
		TenantID:     pgconv.UUIDToString(r.TenantID),
		FieldPath:    r.FieldPath,
		LockedValues: r.LockedValues,
	}
}
