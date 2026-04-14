package schema

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/mock"

	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
)

type mockStore struct {
	mock.Mock
}

func (m *mockStore) CreateSchema(ctx context.Context, arg dbstore.CreateSchemaParams) (dbstore.Schema, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.Schema), args.Error(1)
}

func (m *mockStore) GetSchemaByID(ctx context.Context, id pgtype.UUID) (dbstore.Schema, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(dbstore.Schema), args.Error(1)
}

func (m *mockStore) GetSchemaByName(ctx context.Context, name string) (dbstore.Schema, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(dbstore.Schema), args.Error(1)
}

func (m *mockStore) ListSchemas(ctx context.Context, arg dbstore.ListSchemasParams) ([]dbstore.Schema, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]dbstore.Schema), args.Error(1)
}

func (m *mockStore) DeleteSchema(ctx context.Context, id pgtype.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockStore) CreateSchemaVersion(ctx context.Context, arg dbstore.CreateSchemaVersionParams) (dbstore.SchemaVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.SchemaVersion), args.Error(1)
}

func (m *mockStore) GetSchemaVersion(ctx context.Context, arg dbstore.GetSchemaVersionParams) (dbstore.SchemaVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.SchemaVersion), args.Error(1)
}

func (m *mockStore) GetLatestSchemaVersion(ctx context.Context, schemaID pgtype.UUID) (dbstore.SchemaVersion, error) {
	args := m.Called(ctx, schemaID)
	return args.Get(0).(dbstore.SchemaVersion), args.Error(1)
}

func (m *mockStore) PublishSchemaVersion(ctx context.Context, arg dbstore.PublishSchemaVersionParams) (dbstore.SchemaVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.SchemaVersion), args.Error(1)
}

func (m *mockStore) CreateSchemaField(ctx context.Context, arg dbstore.CreateSchemaFieldParams) (dbstore.SchemaField, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.SchemaField), args.Error(1)
}

func (m *mockStore) GetSchemaFields(ctx context.Context, schemaVersionID pgtype.UUID) ([]dbstore.SchemaField, error) {
	args := m.Called(ctx, schemaVersionID)
	return args.Get(0).([]dbstore.SchemaField), args.Error(1)
}

func (m *mockStore) DeleteSchemaField(ctx context.Context, arg dbstore.DeleteSchemaFieldParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockStore) CreateTenant(ctx context.Context, arg dbstore.CreateTenantParams) (dbstore.Tenant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.Tenant), args.Error(1)
}

func (m *mockStore) GetTenantByID(ctx context.Context, id pgtype.UUID) (dbstore.Tenant, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(dbstore.Tenant), args.Error(1)
}

func (m *mockStore) ListTenants(ctx context.Context, arg dbstore.ListTenantsParams) ([]dbstore.Tenant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]dbstore.Tenant), args.Error(1)
}

func (m *mockStore) ListTenantsBySchema(ctx context.Context, arg dbstore.ListTenantsBySchemaParams) ([]dbstore.Tenant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]dbstore.Tenant), args.Error(1)
}

func (m *mockStore) UpdateTenantName(ctx context.Context, arg dbstore.UpdateTenantNameParams) (dbstore.Tenant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.Tenant), args.Error(1)
}

func (m *mockStore) UpdateTenantSchemaVersion(ctx context.Context, arg dbstore.UpdateTenantSchemaVersionParams) (dbstore.Tenant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.Tenant), args.Error(1)
}

func (m *mockStore) DeleteTenant(ctx context.Context, id pgtype.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockStore) CreateFieldLock(ctx context.Context, arg dbstore.CreateFieldLockParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockStore) DeleteFieldLock(ctx context.Context, arg dbstore.DeleteFieldLockParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockStore) GetFieldLocks(ctx context.Context, tenantID pgtype.UUID) ([]dbstore.TenantFieldLock, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]dbstore.TenantFieldLock), args.Error(1)
}
