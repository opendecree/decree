package schema

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/opendecree/decree/internal/storage/domain"
)

type mockStore struct {
	mock.Mock
}

func (m *mockStore) RunInTx(ctx context.Context, fn func(Store) error) error {
	return fn(m)
}

func (m *mockStore) InsertAuditWriteLog(_ context.Context, _ InsertAuditWriteLogParams) error {
	// No-op by default; override in audit-specific tests via On(...).
	return nil
}

func (m *mockStore) CreateSchema(ctx context.Context, arg CreateSchemaParams) (domain.Schema, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.Schema), args.Error(1)
}

func (m *mockStore) GetSchemaByID(ctx context.Context, id string) (domain.Schema, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.Schema), args.Error(1)
}

func (m *mockStore) GetSchemaByName(ctx context.Context, name string) (domain.Schema, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(domain.Schema), args.Error(1)
}

func (m *mockStore) ListSchemas(ctx context.Context, arg ListSchemasParams) ([]domain.Schema, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]domain.Schema), args.Error(1)
}

func (m *mockStore) DeleteSchema(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockStore) CreateSchemaVersion(ctx context.Context, arg CreateSchemaVersionParams) (domain.SchemaVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.SchemaVersion), args.Error(1)
}

func (m *mockStore) GetSchemaVersion(ctx context.Context, arg GetSchemaVersionParams) (domain.SchemaVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.SchemaVersion), args.Error(1)
}

func (m *mockStore) GetLatestSchemaVersion(ctx context.Context, schemaID string) (domain.SchemaVersion, error) {
	args := m.Called(ctx, schemaID)
	return args.Get(0).(domain.SchemaVersion), args.Error(1)
}

func (m *mockStore) PublishSchemaVersion(ctx context.Context, arg PublishSchemaVersionParams) (domain.SchemaVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.SchemaVersion), args.Error(1)
}

func (m *mockStore) CreateSchemaField(ctx context.Context, arg CreateSchemaFieldParams) (domain.SchemaField, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.SchemaField), args.Error(1)
}

func (m *mockStore) GetSchemaFields(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error) {
	args := m.Called(ctx, schemaVersionID)
	return args.Get(0).([]domain.SchemaField), args.Error(1)
}

func (m *mockStore) DeleteSchemaField(ctx context.Context, arg DeleteSchemaFieldParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockStore) CreateTenant(ctx context.Context, arg CreateTenantParams) (domain.Tenant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.Tenant), args.Error(1)
}

func (m *mockStore) GetTenantByID(ctx context.Context, id string) (domain.Tenant, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.Tenant), args.Error(1)
}

func (m *mockStore) GetTenantByName(ctx context.Context, name string) (domain.Tenant, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(domain.Tenant), args.Error(1)
}

func (m *mockStore) ListTenants(ctx context.Context, arg ListTenantsParams) ([]domain.Tenant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]domain.Tenant), args.Error(1)
}

func (m *mockStore) ListTenantsBySchema(ctx context.Context, arg ListTenantsBySchemaParams) ([]domain.Tenant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]domain.Tenant), args.Error(1)
}

func (m *mockStore) UpdateTenantName(ctx context.Context, arg UpdateTenantNameParams) (domain.Tenant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.Tenant), args.Error(1)
}

func (m *mockStore) UpdateTenantSchemaVersion(ctx context.Context, arg UpdateTenantSchemaVersionParams) (domain.Tenant, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.Tenant), args.Error(1)
}

func (m *mockStore) DeleteTenant(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockStore) CreateFieldLock(ctx context.Context, arg CreateFieldLockParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockStore) DeleteFieldLock(ctx context.Context, arg DeleteFieldLockParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockStore) GetFieldLocks(ctx context.Context, tenantID string) ([]domain.TenantFieldLock, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]domain.TenantFieldLock), args.Error(1)
}
