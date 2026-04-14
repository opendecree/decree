package config

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
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

func (s *PGStore) CreateConfigVersion(ctx context.Context, arg dbstore.CreateConfigVersionParams) (dbstore.ConfigVersion, error) {
	return s.write.CreateConfigVersion(ctx, arg)
}

func (s *PGStore) GetConfigVersion(ctx context.Context, arg dbstore.GetConfigVersionParams) (dbstore.ConfigVersion, error) {
	return s.read.GetConfigVersion(ctx, arg)
}

func (s *PGStore) GetLatestConfigVersion(ctx context.Context, tenantID pgtype.UUID) (dbstore.ConfigVersion, error) {
	return s.read.GetLatestConfigVersion(ctx, tenantID)
}

func (s *PGStore) ListConfigVersions(ctx context.Context, arg dbstore.ListConfigVersionsParams) ([]dbstore.ConfigVersion, error) {
	return s.read.ListConfigVersions(ctx, arg)
}

// Config values.

func (s *PGStore) SetConfigValue(ctx context.Context, arg dbstore.SetConfigValueParams) error {
	return s.write.SetConfigValue(ctx, arg)
}

func (s *PGStore) GetConfigValues(ctx context.Context, configVersionID pgtype.UUID) ([]dbstore.ConfigValue, error) {
	return s.read.GetConfigValues(ctx, configVersionID)
}

func (s *PGStore) GetConfigValueAtVersion(ctx context.Context, arg dbstore.GetConfigValueAtVersionParams) (dbstore.GetConfigValueAtVersionRow, error) {
	return s.read.GetConfigValueAtVersion(ctx, arg)
}

func (s *PGStore) GetFullConfigAtVersion(ctx context.Context, arg dbstore.GetFullConfigAtVersionParams) ([]dbstore.GetFullConfigAtVersionRow, error) {
	return s.read.GetFullConfigAtVersion(ctx, arg)
}

// Tenant/schema lookup.

func (s *PGStore) GetTenantByID(ctx context.Context, id pgtype.UUID) (dbstore.Tenant, error) {
	return s.read.GetTenantByID(ctx, id)
}

func (s *PGStore) GetSchemaFields(ctx context.Context, schemaVersionID pgtype.UUID) ([]dbstore.SchemaField, error) {
	return s.read.GetSchemaFields(ctx, schemaVersionID)
}

func (s *PGStore) GetSchemaVersion(ctx context.Context, arg dbstore.GetSchemaVersionParams) (dbstore.SchemaVersion, error) {
	return s.read.GetSchemaVersion(ctx, arg)
}

func (s *PGStore) GetFieldLocks(ctx context.Context, tenantID pgtype.UUID) ([]dbstore.TenantFieldLock, error) {
	return s.read.GetFieldLocks(ctx, tenantID)
}

// Audit.

func (s *PGStore) InsertAuditWriteLog(ctx context.Context, arg dbstore.InsertAuditWriteLogParams) error {
	return s.write.InsertAuditWriteLog(ctx, arg)
}
