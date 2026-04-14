package schema

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

// NewPGStore creates a new PostgreSQL-backed schema store.
func NewPGStore(writePool, readPool *pgxpool.Pool) *PGStore {
	return &PGStore{
		write: dbstore.New(writePool),
		read:  dbstore.New(readPool),
	}
}

// Schema CRUD — writes go to write pool, reads to read pool.

func (s *PGStore) CreateSchema(ctx context.Context, arg dbstore.CreateSchemaParams) (dbstore.Schema, error) {
	return s.write.CreateSchema(ctx, arg)
}

func (s *PGStore) GetSchemaByID(ctx context.Context, id pgUUID) (dbstore.Schema, error) {
	return s.read.GetSchemaByID(ctx, id)
}

func (s *PGStore) GetSchemaByName(ctx context.Context, name string) (dbstore.Schema, error) {
	return s.read.GetSchemaByName(ctx, name)
}

func (s *PGStore) ListSchemas(ctx context.Context, arg dbstore.ListSchemasParams) ([]dbstore.Schema, error) {
	return s.read.ListSchemas(ctx, arg)
}

func (s *PGStore) DeleteSchema(ctx context.Context, id pgUUID) error {
	return s.write.DeleteSchema(ctx, id)
}

// Schema versions.

func (s *PGStore) CreateSchemaVersion(ctx context.Context, arg dbstore.CreateSchemaVersionParams) (dbstore.SchemaVersion, error) {
	return s.write.CreateSchemaVersion(ctx, arg)
}

func (s *PGStore) GetSchemaVersion(ctx context.Context, arg dbstore.GetSchemaVersionParams) (dbstore.SchemaVersion, error) {
	return s.read.GetSchemaVersion(ctx, arg)
}

func (s *PGStore) GetLatestSchemaVersion(ctx context.Context, schemaID pgUUID) (dbstore.SchemaVersion, error) {
	return s.read.GetLatestSchemaVersion(ctx, schemaID)
}

func (s *PGStore) PublishSchemaVersion(ctx context.Context, arg dbstore.PublishSchemaVersionParams) (dbstore.SchemaVersion, error) {
	return s.write.PublishSchemaVersion(ctx, arg)
}

// Schema fields.

func (s *PGStore) CreateSchemaField(ctx context.Context, arg dbstore.CreateSchemaFieldParams) (dbstore.SchemaField, error) {
	return s.write.CreateSchemaField(ctx, arg)
}

func (s *PGStore) GetSchemaFields(ctx context.Context, schemaVersionID pgUUID) ([]dbstore.SchemaField, error) {
	return s.read.GetSchemaFields(ctx, schemaVersionID)
}

func (s *PGStore) DeleteSchemaField(ctx context.Context, arg dbstore.DeleteSchemaFieldParams) error {
	return s.write.DeleteSchemaField(ctx, arg)
}

// Tenants.

func (s *PGStore) CreateTenant(ctx context.Context, arg dbstore.CreateTenantParams) (dbstore.Tenant, error) {
	return s.write.CreateTenant(ctx, arg)
}

func (s *PGStore) GetTenantByID(ctx context.Context, id pgUUID) (dbstore.Tenant, error) {
	return s.read.GetTenantByID(ctx, id)
}

func (s *PGStore) ListTenants(ctx context.Context, arg dbstore.ListTenantsParams) ([]dbstore.Tenant, error) {
	return s.read.ListTenants(ctx, arg)
}

func (s *PGStore) ListTenantsBySchema(ctx context.Context, arg dbstore.ListTenantsBySchemaParams) ([]dbstore.Tenant, error) {
	return s.read.ListTenantsBySchema(ctx, arg)
}

func (s *PGStore) UpdateTenantName(ctx context.Context, arg dbstore.UpdateTenantNameParams) (dbstore.Tenant, error) {
	return s.write.UpdateTenantName(ctx, arg)
}

func (s *PGStore) UpdateTenantSchemaVersion(ctx context.Context, arg dbstore.UpdateTenantSchemaVersionParams) (dbstore.Tenant, error) {
	return s.write.UpdateTenantSchemaVersion(ctx, arg)
}

func (s *PGStore) DeleteTenant(ctx context.Context, id pgUUID) error {
	return s.write.DeleteTenant(ctx, id)
}

// Field locks.

func (s *PGStore) CreateFieldLock(ctx context.Context, arg dbstore.CreateFieldLockParams) error {
	return s.write.CreateFieldLock(ctx, arg)
}

func (s *PGStore) DeleteFieldLock(ctx context.Context, arg dbstore.DeleteFieldLockParams) error {
	return s.write.DeleteFieldLock(ctx, arg)
}

func (s *PGStore) GetFieldLocks(ctx context.Context, tenantID pgUUID) ([]dbstore.TenantFieldLock, error) {
	return s.read.GetFieldLocks(ctx, tenantID)
}
