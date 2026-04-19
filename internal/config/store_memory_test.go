package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage/domain"
)

func TestMemoryStore_ConfigVersionCRUD(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	v1, err := s.CreateConfigVersion(ctx, CreateConfigVersionParams{
		TenantID: "t1", Version: 1, CreatedBy: "admin",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), v1.Version)

	v2, err := s.CreateConfigVersion(ctx, CreateConfigVersionParams{
		TenantID: "t1", Version: 2, CreatedBy: "admin",
	})
	require.NoError(t, err)

	got, err := s.GetConfigVersion(ctx, GetConfigVersionParams{TenantID: "t1", Version: 1})
	require.NoError(t, err)
	assert.Equal(t, v1.ID, got.ID)

	latest, err := s.GetLatestConfigVersion(ctx, "t1")
	require.NoError(t, err)
	assert.Equal(t, v2.ID, latest.ID)

	_, err = s.GetLatestConfigVersion(ctx, "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMemoryStore_ListConfigVersions(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	for i := int32(1); i <= 5; i++ {
		_, err := s.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID: "t1", Version: i, CreatedBy: "admin",
		})
		require.NoError(t, err)
	}

	all, err := s.ListConfigVersions(ctx, ListConfigVersionsParams{TenantID: "t1", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, all, 5)
	assert.Equal(t, int32(5), all[0].Version) // DESC order

	page, err := s.ListConfigVersions(ctx, ListConfigVersionsParams{TenantID: "t1", Limit: 2, Offset: 1})
	require.NoError(t, err)
	assert.Len(t, page, 2)
	assert.Equal(t, int32(4), page[0].Version)
}

func TestMemoryStore_SetAndGetConfigValues(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	v, _ := s.CreateConfigVersion(ctx, CreateConfigVersionParams{
		TenantID: "t1", Version: 1, CreatedBy: "admin",
	})

	val := "hello"
	chk := "abc"
	require.NoError(t, s.SetConfigValue(ctx, SetConfigValueParams{
		ConfigVersionID: v.ID, FieldPath: "app.name", Value: &val, Checksum: &chk,
	}))

	values, err := s.GetConfigValues(ctx, v.ID)
	require.NoError(t, err)
	assert.Len(t, values, 1)
	assert.Equal(t, "app.name", values[0].FieldPath)
}

func TestMemoryStore_GetConfigValueAtVersion(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	v1, _ := s.CreateConfigVersion(ctx, CreateConfigVersionParams{TenantID: "t1", Version: 1, CreatedBy: "a"})
	v2, _ := s.CreateConfigVersion(ctx, CreateConfigVersionParams{TenantID: "t1", Version: 2, CreatedBy: "a"})

	val1 := "old"
	val2 := "new"
	_ = s.SetConfigValue(ctx, SetConfigValueParams{ConfigVersionID: v1.ID, FieldPath: "x", Value: &val1})
	_ = s.SetConfigValue(ctx, SetConfigValueParams{ConfigVersionID: v2.ID, FieldPath: "x", Value: &val2})

	// At version 1, should get "old".
	row, err := s.GetConfigValueAtVersion(ctx, GetConfigValueAtVersionParams{TenantID: "t1", FieldPath: "x", Version: 1})
	require.NoError(t, err)
	assert.Equal(t, "old", *row.Value)

	// At version 2, should get "new".
	row, err = s.GetConfigValueAtVersion(ctx, GetConfigValueAtVersionParams{TenantID: "t1", FieldPath: "x", Version: 2})
	require.NoError(t, err)
	assert.Equal(t, "new", *row.Value)

	// Missing field.
	_, err = s.GetConfigValueAtVersion(ctx, GetConfigValueAtVersionParams{TenantID: "t1", FieldPath: "missing", Version: 2})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMemoryStore_GetFullConfigAtVersion(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	v1, _ := s.CreateConfigVersion(ctx, CreateConfigVersionParams{TenantID: "t1", Version: 1, CreatedBy: "a"})
	v2, _ := s.CreateConfigVersion(ctx, CreateConfigVersionParams{TenantID: "t1", Version: 2, CreatedBy: "a"})

	a := "1"
	b := "2"
	bNew := "3"
	_ = s.SetConfigValue(ctx, SetConfigValueParams{ConfigVersionID: v1.ID, FieldPath: "a", Value: &a})
	_ = s.SetConfigValue(ctx, SetConfigValueParams{ConfigVersionID: v1.ID, FieldPath: "b", Value: &b})
	_ = s.SetConfigValue(ctx, SetConfigValueParams{ConfigVersionID: v2.ID, FieldPath: "b", Value: &bNew})

	rows, err := s.GetFullConfigAtVersion(ctx, GetFullConfigAtVersionParams{TenantID: "t1", Version: 2})
	require.NoError(t, err)
	assert.Len(t, rows, 2)
	assert.Equal(t, "a", rows[0].FieldPath)
	assert.Equal(t, "1", *rows[0].Value)
	assert.Equal(t, "b", rows[1].FieldPath)
	assert.Equal(t, "3", *rows[1].Value) // Updated in v2
}

func TestMemoryStore_TenantLookup(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	_, err := s.GetTenantByID(ctx, "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)

	s.SetTenant(domain.Tenant{ID: "t1", Name: "acme"})
	got, err := s.GetTenantByID(ctx, "t1")
	require.NoError(t, err)
	assert.Equal(t, "acme", got.Name)

	got, err = s.GetTenantByName(ctx, "acme")
	require.NoError(t, err)
	assert.Equal(t, "t1", got.ID)

	_, err = s.GetTenantByName(ctx, "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMemoryStore_SchemaLookup(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	s.SetSchemaVersion(domain.SchemaVersion{ID: "sv1", SchemaID: "s1", Version: 1})
	s.SetSchemaFields("sv1", []domain.SchemaField{{Path: "x", FieldType: "string"}})

	sv, err := s.GetSchemaVersion(ctx, domain.SchemaVersionKey{SchemaID: "s1", Version: 1})
	require.NoError(t, err)
	assert.Equal(t, "sv1", sv.ID)

	fields, err := s.GetSchemaFields(ctx, "sv1")
	require.NoError(t, err)
	assert.Len(t, fields, 1)

	_, err = s.GetSchemaVersion(ctx, domain.SchemaVersionKey{SchemaID: "s1", Version: 99})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestMemoryStore_FieldLocks(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	locks, err := s.GetFieldLocks(ctx, "t1")
	require.NoError(t, err)
	assert.Empty(t, locks)
}

func TestMemoryStore_AuditLog(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	require.NoError(t, s.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
		TenantID: "t1", Actor: "admin", Action: "set_field",
	}))
	assert.Len(t, s.auditLog, 1)
}

func TestMemoryStore_RunInTx(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	err := s.RunInTx(ctx, func(tx Store) error {
		_, err := tx.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID: "t1", Version: 1, CreatedBy: "admin",
		})
		return err
	})
	require.NoError(t, err)

	v, err := s.GetLatestConfigVersion(ctx, "t1")
	require.NoError(t, err)
	assert.Equal(t, int32(1), v.Version)
}
