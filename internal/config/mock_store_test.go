package config

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/mock"

	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
)

type mockStore struct {
	mock.Mock
}

func (m *mockStore) RunInTx(_ context.Context, fn func(Store) error) error {
	return fn(m)
}

func (m *mockStore) CreateConfigVersion(ctx context.Context, arg dbstore.CreateConfigVersionParams) (dbstore.ConfigVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.ConfigVersion), args.Error(1)
}

func (m *mockStore) GetConfigVersion(ctx context.Context, arg dbstore.GetConfigVersionParams) (dbstore.ConfigVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.ConfigVersion), args.Error(1)
}

func (m *mockStore) GetLatestConfigVersion(ctx context.Context, tenantID pgtype.UUID) (dbstore.ConfigVersion, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(dbstore.ConfigVersion), args.Error(1)
}

func (m *mockStore) ListConfigVersions(ctx context.Context, arg dbstore.ListConfigVersionsParams) ([]dbstore.ConfigVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]dbstore.ConfigVersion), args.Error(1)
}

func (m *mockStore) SetConfigValue(ctx context.Context, arg dbstore.SetConfigValueParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *mockStore) GetConfigValues(ctx context.Context, configVersionID pgtype.UUID) ([]dbstore.ConfigValue, error) {
	args := m.Called(ctx, configVersionID)
	return args.Get(0).([]dbstore.ConfigValue), args.Error(1)
}

func (m *mockStore) GetConfigValueAtVersion(ctx context.Context, arg dbstore.GetConfigValueAtVersionParams) (dbstore.GetConfigValueAtVersionRow, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.GetConfigValueAtVersionRow), args.Error(1)
}

func (m *mockStore) GetFullConfigAtVersion(ctx context.Context, arg dbstore.GetFullConfigAtVersionParams) ([]dbstore.GetFullConfigAtVersionRow, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]dbstore.GetFullConfigAtVersionRow), args.Error(1)
}

func (m *mockStore) GetTenantByID(ctx context.Context, id pgtype.UUID) (dbstore.Tenant, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(dbstore.Tenant), args.Error(1)
}

func (m *mockStore) GetSchemaFields(ctx context.Context, schemaVersionID pgtype.UUID) ([]dbstore.SchemaField, error) {
	args := m.Called(ctx, schemaVersionID)
	return args.Get(0).([]dbstore.SchemaField), args.Error(1)
}

func (m *mockStore) GetSchemaVersion(ctx context.Context, arg dbstore.GetSchemaVersionParams) (dbstore.SchemaVersion, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(dbstore.SchemaVersion), args.Error(1)
}

func (m *mockStore) GetFieldLocks(ctx context.Context, tenantID pgtype.UUID) ([]dbstore.TenantFieldLock, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]dbstore.TenantFieldLock), args.Error(1)
}

func (m *mockStore) InsertAuditWriteLog(ctx context.Context, arg dbstore.InsertAuditWriteLogParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}
