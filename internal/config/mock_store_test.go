package config

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/opendecree/decree/internal/storage/domain"
)

type mockStore struct {
	mock.Mock
}

func (m *mockStore) RunInTx(_ context.Context, fn func(Store) error) error {
	return fn(m)
}

func (m *mockStore) CreateConfigVersion(ctx context.Context, arg CreateConfigVersionParams) (domain.ConfigVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.ConfigVersion), args.Error(1)
}

func (m *mockStore) GetConfigVersion(ctx context.Context, arg GetConfigVersionParams) (domain.ConfigVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.ConfigVersion), args.Error(1)
}

func (m *mockStore) GetLatestConfigVersion(ctx context.Context, tenantID string) (domain.ConfigVersion, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(domain.ConfigVersion), args.Error(1)
}

func (m *mockStore) ListConfigVersions(ctx context.Context, arg ListConfigVersionsParams) ([]domain.ConfigVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]domain.ConfigVersion), args.Error(1)
}

func (m *mockStore) SetConfigValue(ctx context.Context, arg SetConfigValueParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockStore) GetConfigValues(ctx context.Context, configVersionID string) ([]domain.ConfigValue, error) {
	args := m.Called(ctx, configVersionID)
	return args.Get(0).([]domain.ConfigValue), args.Error(1)
}

func (m *mockStore) GetConfigValueAtVersion(ctx context.Context, arg GetConfigValueAtVersionParams) (GetConfigValueAtVersionRow, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(GetConfigValueAtVersionRow), args.Error(1)
}

func (m *mockStore) GetFullConfigAtVersion(ctx context.Context, arg GetFullConfigAtVersionParams) ([]GetFullConfigAtVersionRow, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]GetFullConfigAtVersionRow), args.Error(1)
}

func (m *mockStore) GetTenantByID(ctx context.Context, id string) (domain.Tenant, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.Tenant), args.Error(1)
}

func (m *mockStore) GetTenantByName(ctx context.Context, name string) (domain.Tenant, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(domain.Tenant), args.Error(1)
}

func (m *mockStore) GetSchemaFields(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error) {
	args := m.Called(ctx, schemaVersionID)
	return args.Get(0).([]domain.SchemaField), args.Error(1)
}

func (m *mockStore) GetSchemaVersion(ctx context.Context, arg domain.SchemaVersionKey) (domain.SchemaVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(domain.SchemaVersion), args.Error(1)
}

func (m *mockStore) GetFieldLocks(ctx context.Context, tenantID string) ([]domain.TenantFieldLock, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]domain.TenantFieldLock), args.Error(1)
}

func (m *mockStore) InsertAuditWriteLog(ctx context.Context, arg InsertAuditWriteLogParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}
