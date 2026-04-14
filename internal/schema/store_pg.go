package schema

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

// NewPGStore creates a new PostgreSQL-backed schema store.
func NewPGStore(writePool, readPool *pgxpool.Pool) *PGStore {
	return &PGStore{
		write: dbstore.New(writePool),
		read:  dbstore.New(readPool),
	}
}

// --- Schema CRUD ---

func (s *PGStore) CreateSchema(ctx context.Context, arg CreateSchemaParams) (domain.Schema, error) {
	row, err := s.write.CreateSchema(ctx, dbstore.CreateSchemaParams{
		Name:        arg.Name,
		Description: arg.Description,
	})
	if err != nil {
		return domain.Schema{}, err
	}
	return schemaFromDB(row), nil
}

func (s *PGStore) GetSchemaByID(ctx context.Context, id string) (domain.Schema, error) {
	pgID, err := pgconv.StringToUUID(id)
	if err != nil {
		return domain.Schema{}, err
	}
	row, err := s.read.GetSchemaByID(ctx, pgID)
	if err != nil {
		return domain.Schema{}, pgconv.WrapNotFound(err)
	}
	return schemaFromDB(row), nil
}

func (s *PGStore) GetSchemaByName(ctx context.Context, name string) (domain.Schema, error) {
	row, err := s.read.GetSchemaByName(ctx, name)
	if err != nil {
		return domain.Schema{}, pgconv.WrapNotFound(err)
	}
	return schemaFromDB(row), nil
}

func (s *PGStore) ListSchemas(ctx context.Context, arg ListSchemasParams) ([]domain.Schema, error) {
	rows, err := s.read.ListSchemas(ctx, dbstore.ListSchemasParams{
		Limit:  arg.Limit,
		Offset: arg.Offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]domain.Schema, len(rows))
	for i, r := range rows {
		result[i] = schemaFromDB(r)
	}
	return result, nil
}

func (s *PGStore) DeleteSchema(ctx context.Context, id string) error {
	pgID, err := pgconv.StringToUUID(id)
	if err != nil {
		return err
	}
	return s.write.DeleteSchema(ctx, pgID)
}

// --- Schema versions ---

func (s *PGStore) CreateSchemaVersion(ctx context.Context, arg CreateSchemaVersionParams) (domain.SchemaVersion, error) {
	schemaID, err := pgconv.StringToUUID(arg.SchemaID)
	if err != nil {
		return domain.SchemaVersion{}, err
	}
	row, err := s.write.CreateSchemaVersion(ctx, dbstore.CreateSchemaVersionParams{
		SchemaID:      schemaID,
		Version:       arg.Version,
		ParentVersion: arg.ParentVersion,
		Description:   arg.Description,
		Checksum:      arg.Checksum,
	})
	if err != nil {
		return domain.SchemaVersion{}, err
	}
	return schemaVersionFromDB(row), nil
}

func (s *PGStore) GetSchemaVersion(ctx context.Context, arg GetSchemaVersionParams) (domain.SchemaVersion, error) {
	schemaID, err := pgconv.StringToUUID(arg.SchemaID)
	if err != nil {
		return domain.SchemaVersion{}, err
	}
	row, err := s.read.GetSchemaVersion(ctx, dbstore.GetSchemaVersionParams{
		SchemaID: schemaID,
		Version:  arg.Version,
	})
	if err != nil {
		return domain.SchemaVersion{}, pgconv.WrapNotFound(err)
	}
	return schemaVersionFromDB(row), nil
}

func (s *PGStore) GetLatestSchemaVersion(ctx context.Context, schemaID string) (domain.SchemaVersion, error) {
	pgID, err := pgconv.StringToUUID(schemaID)
	if err != nil {
		return domain.SchemaVersion{}, err
	}
	row, err := s.read.GetLatestSchemaVersion(ctx, pgID)
	if err != nil {
		return domain.SchemaVersion{}, pgconv.WrapNotFound(err)
	}
	return schemaVersionFromDB(row), nil
}

func (s *PGStore) PublishSchemaVersion(ctx context.Context, arg PublishSchemaVersionParams) (domain.SchemaVersion, error) {
	schemaID, err := pgconv.StringToUUID(arg.SchemaID)
	if err != nil {
		return domain.SchemaVersion{}, err
	}
	row, err := s.write.PublishSchemaVersion(ctx, dbstore.PublishSchemaVersionParams{
		SchemaID: schemaID,
		Version:  arg.Version,
	})
	if err != nil {
		return domain.SchemaVersion{}, pgconv.WrapNotFound(err)
	}
	return schemaVersionFromDB(row), nil
}

// --- Schema fields ---

func (s *PGStore) CreateSchemaField(ctx context.Context, arg CreateSchemaFieldParams) (domain.SchemaField, error) {
	svID, err := pgconv.StringToUUID(arg.SchemaVersionID)
	if err != nil {
		return domain.SchemaField{}, err
	}
	row, err := s.write.CreateSchemaField(ctx, dbstore.CreateSchemaFieldParams{
		SchemaVersionID: svID,
		Path:            arg.Path,
		FieldType:       dbstore.FieldType(arg.FieldType),
		Constraints:     arg.Constraints,
		Nullable:        arg.Nullable,
		Deprecated:      arg.Deprecated,
		RedirectTo:      arg.RedirectTo,
		DefaultValue:    arg.DefaultValue,
		Description:     arg.Description,
		Title:           arg.Title,
		Example:         arg.Example,
		Examples:        arg.Examples,
		ExternalDocs:    arg.ExternalDocs,
		Tags:            arg.Tags,
		Format:          arg.Format,
		ReadOnly:        arg.ReadOnly,
		WriteOnce:       arg.WriteOnce,
		Sensitive:       arg.Sensitive,
	})
	if err != nil {
		return domain.SchemaField{}, err
	}
	return schemaFieldFromDB(row), nil
}

func (s *PGStore) GetSchemaFields(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error) {
	svID, err := pgconv.StringToUUID(schemaVersionID)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.GetSchemaFields(ctx, svID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.SchemaField, len(rows))
	for i, r := range rows {
		result[i] = schemaFieldFromDB(r)
	}
	return result, nil
}

func (s *PGStore) DeleteSchemaField(ctx context.Context, arg DeleteSchemaFieldParams) error {
	svID, err := pgconv.StringToUUID(arg.SchemaVersionID)
	if err != nil {
		return err
	}
	return s.write.DeleteSchemaField(ctx, dbstore.DeleteSchemaFieldParams{
		SchemaVersionID: svID,
		Path:            arg.Path,
	})
}

// --- Tenants ---

func (s *PGStore) CreateTenant(ctx context.Context, arg CreateTenantParams) (domain.Tenant, error) {
	schemaID, err := pgconv.StringToUUID(arg.SchemaID)
	if err != nil {
		return domain.Tenant{}, err
	}
	row, err := s.write.CreateTenant(ctx, dbstore.CreateTenantParams{
		Name:          arg.Name,
		SchemaID:      schemaID,
		SchemaVersion: arg.SchemaVersion,
	})
	if err != nil {
		return domain.Tenant{}, err
	}
	return tenantFromDB(row), nil
}

func (s *PGStore) GetTenantByID(ctx context.Context, id string) (domain.Tenant, error) {
	pgID, err := pgconv.StringToUUID(id)
	if err != nil {
		return domain.Tenant{}, err
	}
	row, err := s.read.GetTenantByID(ctx, pgID)
	if err != nil {
		return domain.Tenant{}, pgconv.WrapNotFound(err)
	}
	return tenantFromDB(row), nil
}

func (s *PGStore) ListTenants(ctx context.Context, arg ListTenantsParams) ([]domain.Tenant, error) {
	if arg.AllowedTenantIDs != nil {
		pgIDs, err := stringsToUUIDs(arg.AllowedTenantIDs)
		if err != nil {
			return nil, err
		}
		rows, err := s.read.ListTenantsByIDs(ctx, dbstore.ListTenantsByIDsParams{
			Limit:      arg.Limit,
			Offset:     arg.Offset,
			AllowedIds: pgIDs,
		})
		if err != nil {
			return nil, err
		}
		return tenantsFromDB(rows), nil
	}
	rows, err := s.read.ListTenants(ctx, dbstore.ListTenantsParams{
		Limit:  arg.Limit,
		Offset: arg.Offset,
	})
	if err != nil {
		return nil, err
	}
	return tenantsFromDB(rows), nil
}

func (s *PGStore) ListTenantsBySchema(ctx context.Context, arg ListTenantsBySchemaParams) ([]domain.Tenant, error) {
	schemaID, err := pgconv.StringToUUID(arg.SchemaID)
	if err != nil {
		return nil, err
	}
	if arg.AllowedTenantIDs != nil {
		pgIDs, err := stringsToUUIDs(arg.AllowedTenantIDs)
		if err != nil {
			return nil, err
		}
		rows, err := s.read.ListTenantsBySchemaAndIDs(ctx, dbstore.ListTenantsBySchemaAndIDsParams{
			SchemaID:   schemaID,
			Limit:      arg.Limit,
			Offset:     arg.Offset,
			AllowedIds: pgIDs,
		})
		if err != nil {
			return nil, err
		}
		return tenantsFromDB(rows), nil
	}
	rows, err := s.read.ListTenantsBySchema(ctx, dbstore.ListTenantsBySchemaParams{
		SchemaID: schemaID,
		Limit:    arg.Limit,
		Offset:   arg.Offset,
	})
	if err != nil {
		return nil, err
	}
	return tenantsFromDB(rows), nil
}

func (s *PGStore) UpdateTenantName(ctx context.Context, arg UpdateTenantNameParams) (domain.Tenant, error) {
	pgID, err := pgconv.StringToUUID(arg.ID)
	if err != nil {
		return domain.Tenant{}, err
	}
	row, err := s.write.UpdateTenantName(ctx, dbstore.UpdateTenantNameParams{
		ID:   pgID,
		Name: arg.Name,
	})
	if err != nil {
		return domain.Tenant{}, pgconv.WrapNotFound(err)
	}
	return tenantFromDB(row), nil
}

func (s *PGStore) UpdateTenantSchemaVersion(ctx context.Context, arg UpdateTenantSchemaVersionParams) (domain.Tenant, error) {
	pgID, err := pgconv.StringToUUID(arg.ID)
	if err != nil {
		return domain.Tenant{}, err
	}
	row, err := s.write.UpdateTenantSchemaVersion(ctx, dbstore.UpdateTenantSchemaVersionParams{
		ID:            pgID,
		SchemaVersion: arg.SchemaVersion,
	})
	if err != nil {
		return domain.Tenant{}, pgconv.WrapNotFound(err)
	}
	return tenantFromDB(row), nil
}

func (s *PGStore) DeleteTenant(ctx context.Context, id string) error {
	pgID, err := pgconv.StringToUUID(id)
	if err != nil {
		return err
	}
	return s.write.DeleteTenant(ctx, pgID)
}

// --- Field locks ---

func (s *PGStore) CreateFieldLock(ctx context.Context, arg CreateFieldLockParams) error {
	tenantID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return err
	}
	return s.write.CreateFieldLock(ctx, dbstore.CreateFieldLockParams{
		TenantID:     tenantID,
		FieldPath:    arg.FieldPath,
		LockedValues: arg.LockedValues,
	})
}

func (s *PGStore) DeleteFieldLock(ctx context.Context, arg DeleteFieldLockParams) error {
	tenantID, err := pgconv.StringToUUID(arg.TenantID)
	if err != nil {
		return err
	}
	return s.write.DeleteFieldLock(ctx, dbstore.DeleteFieldLockParams{
		TenantID:  tenantID,
		FieldPath: arg.FieldPath,
	})
}

func (s *PGStore) GetFieldLocks(ctx context.Context, tenantID string) ([]domain.TenantFieldLock, error) {
	pgID, err := pgconv.StringToUUID(tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.GetFieldLocks(ctx, pgID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.TenantFieldLock, len(rows))
	for i, r := range rows {
		result[i] = domain.TenantFieldLock{
			TenantID:     pgconv.UUIDToString(r.TenantID),
			FieldPath:    r.FieldPath,
			LockedValues: r.LockedValues,
		}
	}
	return result, nil
}

// --- DB → domain conversion helpers ---

func schemaFromDB(r dbstore.Schema) domain.Schema {
	return domain.Schema{
		ID:          pgconv.UUIDToString(r.ID),
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   pgconv.TimestamptzToTime(r.CreatedAt),
		UpdatedAt:   pgconv.TimestamptzToTime(r.UpdatedAt),
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
		Title:           r.Title,
		Example:         r.Example,
		Examples:        r.Examples,
		ExternalDocs:    r.ExternalDocs,
		Tags:            r.Tags,
		Format:          r.Format,
		ReadOnly:        r.ReadOnly,
		WriteOnce:       r.WriteOnce,
		Sensitive:       r.Sensitive,
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

func tenantsFromDB(rows []dbstore.Tenant) []domain.Tenant {
	result := make([]domain.Tenant, len(rows))
	for i, r := range rows {
		result[i] = tenantFromDB(r)
	}
	return result
}

func stringsToUUIDs(ids []string) ([]pgtype.UUID, error) {
	result := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		u, err := pgconv.StringToUUID(id)
		if err != nil {
			return nil, err
		}
		result[i] = u
	}
	return result, nil
}
