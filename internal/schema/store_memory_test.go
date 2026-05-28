package schema

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage/domain"
)

func TestMemoryStore_SchemaCRUD(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	desc := "test description"

	// Create.
	s, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "alpha", Description: &desc})
	require.NoError(t, err)
	assert.NotEmpty(t, s.ID)
	assert.Equal(t, "alpha", s.Name)
	assert.Equal(t, &desc, s.Description)
	assert.False(t, s.CreatedAt.IsZero())

	// Get by ID.
	got, err := store.GetSchemaByID(ctx, s.ID)
	require.NoError(t, err)
	assert.Equal(t, s, got)

	// Get by name.
	got, err = store.GetSchemaByName(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, s, got)

	// Name uniqueness.
	_, err = store.CreateSchema(ctx, CreateSchemaParams{Name: "alpha"})
	require.Error(t, err)

	// List.
	s2, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "beta"})
	require.NoError(t, err)
	list, err := store.ListSchemas(ctx, ListSchemasParams{Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Delete.
	err = store.DeleteSchema(ctx, s2.ID)
	require.NoError(t, err)
	list, err = store.ListSchemas(ctx, ListSchemasParams{Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestMemoryStore_SchemaNotFound(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	_, err := store.GetSchemaByID(ctx, "nonexistent")
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	_, err = store.GetSchemaByName(ctx, "nonexistent")
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	err = store.DeleteSchema(ctx, "nonexistent")
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestMemoryStore_SchemaVersions(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "versioned"})
	require.NoError(t, err)

	// Create version.
	sv1, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID: s.ID, Version: 1, Checksum: "abc",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), sv1.Version)
	assert.False(t, sv1.Published)

	// Create version for nonexistent schema.
	_, err = store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID: "bad", Version: 1, Checksum: "x",
	})
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Get version.
	got, err := store.GetSchemaVersion(ctx, GetSchemaVersionParams{SchemaID: s.ID, Version: 1})
	require.NoError(t, err)
	assert.Equal(t, sv1, got)

	// Get version not found.
	_, err = store.GetSchemaVersion(ctx, GetSchemaVersionParams{SchemaID: s.ID, Version: 99})
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Create second version and get latest.
	parent := int32(1)
	sv2, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID: s.ID, Version: 2, ParentVersion: &parent, Checksum: "def",
	})
	require.NoError(t, err)

	latest, err := store.GetLatestSchemaVersion(ctx, s.ID)
	require.NoError(t, err)
	assert.Equal(t, sv2.Version, latest.Version)

	// Get latest not found.
	_, err = store.GetLatestSchemaVersion(ctx, "bad")
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Publish.
	published, err := store.PublishSchemaVersion(ctx, PublishSchemaVersionParams{SchemaID: s.ID, Version: 1})
	require.NoError(t, err)
	assert.True(t, published.Published)

	// Publish not found.
	_, err = store.PublishSchemaVersion(ctx, PublishSchemaVersionParams{SchemaID: s.ID, Version: 99})
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestMemoryStore_SchemaFields(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "fields"})
	require.NoError(t, err)
	sv, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID: s.ID, Version: 1, Checksum: "x",
	})
	require.NoError(t, err)

	// Create field.
	f, err := store.CreateSchemaField(ctx, CreateSchemaFieldParams{
		SchemaVersionID: sv.ID,
		Path:            "app.name",
		FieldType:       domain.FieldTypeString,
		Nullable:        false,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, f.ID)
	assert.Equal(t, "app.name", f.Path)

	// Create field for nonexistent version.
	_, err = store.CreateSchemaField(ctx, CreateSchemaFieldParams{
		SchemaVersionID: "bad", Path: "x", FieldType: domain.FieldTypeString,
	})
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Get fields.
	fields, err := store.GetSchemaFields(ctx, sv.ID)
	require.NoError(t, err)
	assert.Len(t, fields, 1)
	assert.Equal(t, f, fields[0])

	// Add another field.
	_, err = store.CreateSchemaField(ctx, CreateSchemaFieldParams{
		SchemaVersionID: sv.ID,
		Path:            "app.port",
		FieldType:       domain.FieldTypeInteger,
	})
	require.NoError(t, err)
	fields, err = store.GetSchemaFields(ctx, sv.ID)
	require.NoError(t, err)
	assert.Len(t, fields, 2)

	// Delete field.
	err = store.DeleteSchemaField(ctx, DeleteSchemaFieldParams{SchemaVersionID: sv.ID, Path: "app.name"})
	require.NoError(t, err)
	fields, err = store.GetSchemaFields(ctx, sv.ID)
	require.NoError(t, err)
	assert.Len(t, fields, 1)
	assert.Equal(t, "app.port", fields[0].Path)

	// Delete field not found.
	err = store.DeleteSchemaField(ctx, DeleteSchemaFieldParams{SchemaVersionID: sv.ID, Path: "nonexistent"})
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	err = store.DeleteSchemaField(ctx, DeleteSchemaFieldParams{SchemaVersionID: "bad", Path: "x"})
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestMemoryStore_TenantCRUD(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "tenant-test"})
	require.NoError(t, err)

	// Create tenant.
	tenant, err := store.CreateTenant(ctx, CreateTenantParams{
		Name: "acme", SchemaID: s.ID, SchemaVersion: 1,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, tenant.ID)
	assert.Equal(t, "acme", tenant.Name)

	// Get by ID.
	got, err := store.GetTenantByID(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Equal(t, tenant, got)

	// Get by ID not found.
	_, err = store.GetTenantByID(ctx, "bad")
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Get by name.
	gotByName, err := store.GetTenantByName(ctx, "acme")
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, gotByName.ID)

	// Get by name not found.
	_, err = store.GetTenantByName(ctx, "nope")
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// List.
	tenant2, err := store.CreateTenant(ctx, CreateTenantParams{
		Name: "globex", SchemaID: s.ID, SchemaVersion: 1,
	})
	require.NoError(t, err)

	// GetTenantsByNames — batch lookup.
	batch, err := store.GetTenantsByNames(ctx, []string{"acme", "globex"})
	require.NoError(t, err)
	assert.Len(t, batch, 2)

	// Partial match (one known, one unknown) returns only the found tenants.
	partial, err := store.GetTenantsByNames(ctx, []string{"acme", "unknown"})
	require.NoError(t, err)
	assert.Len(t, partial, 1)
	assert.Equal(t, tenant.ID, partial[0].ID)

	// Empty input returns empty slice.
	empty, err := store.GetTenantsByNames(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	list, err := store.ListTenants(ctx, ListTenantsParams{Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// List by schema.
	s2, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "other"})
	require.NoError(t, err)
	_, err = store.CreateTenant(ctx, CreateTenantParams{
		Name: "other-tenant", SchemaID: s2.ID, SchemaVersion: 1,
	})
	require.NoError(t, err)

	bySchema, err := store.ListTenantsBySchema(ctx, ListTenantsBySchemaParams{
		SchemaID: s.ID, Limit: 10, Offset: 0,
	})
	require.NoError(t, err)
	assert.Len(t, bySchema, 2)

	// Update name.
	updated, err := store.UpdateTenantName(ctx, UpdateTenantNameParams{ID: tenant.ID, Name: "acme-corp"})
	require.NoError(t, err)
	assert.Equal(t, "acme-corp", updated.Name)

	// Update name not found.
	_, err = store.UpdateTenantName(ctx, UpdateTenantNameParams{ID: "bad", Name: "x"})
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Update schema version.
	updated, err = store.UpdateTenantSchemaVersion(ctx, UpdateTenantSchemaVersionParams{ID: tenant.ID, SchemaVersion: 2})
	require.NoError(t, err)
	assert.Equal(t, int32(2), updated.SchemaVersion)

	// Update schema version not found.
	_, err = store.UpdateTenantSchemaVersion(ctx, UpdateTenantSchemaVersionParams{ID: "bad", SchemaVersion: 1})
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Delete.
	err = store.DeleteTenant(ctx, tenant2.ID)
	require.NoError(t, err)
	list, err = store.ListTenants(ctx, ListTenantsParams{Limit: 10, Offset: 0})
	require.NoError(t, err)
	// 2 remaining: acme-corp + other-tenant
	assert.Len(t, list, 2)

	// Delete not found.
	err = store.DeleteTenant(ctx, "bad")
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestMemoryStore_FieldLocks(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "lock-test"})
	require.NoError(t, err)
	tenant, err := store.CreateTenant(ctx, CreateTenantParams{
		Name: "locker", SchemaID: s.ID, SchemaVersion: 1,
	})
	require.NoError(t, err)

	// Create lock.
	err = store.CreateFieldLock(ctx, CreateFieldLockParams{
		TenantID: tenant.ID, FieldPath: "db.host", LockedValues: []byte(`["localhost"]`),
	})
	require.NoError(t, err)

	// Create lock for nonexistent tenant.
	err = store.CreateFieldLock(ctx, CreateFieldLockParams{
		TenantID: "bad", FieldPath: "x",
	})
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Get locks.
	locks, err := store.GetFieldLocks(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Len(t, locks, 1)
	assert.Equal(t, "db.host", locks[0].FieldPath)

	// Add another lock.
	err = store.CreateFieldLock(ctx, CreateFieldLockParams{
		TenantID: tenant.ID, FieldPath: "db.port", LockedValues: []byte(`[5432]`),
	})
	require.NoError(t, err)
	locks, err = store.GetFieldLocks(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Len(t, locks, 2)

	// Delete lock.
	err = store.DeleteFieldLock(ctx, DeleteFieldLockParams{TenantID: tenant.ID, FieldPath: "db.host"})
	require.NoError(t, err)
	locks, err = store.GetFieldLocks(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Len(t, locks, 1)
	assert.Equal(t, "db.port", locks[0].FieldPath)

	// Delete lock not found.
	err = store.DeleteFieldLock(ctx, DeleteFieldLockParams{TenantID: tenant.ID, FieldPath: "nonexistent"})
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	err = store.DeleteFieldLock(ctx, DeleteFieldLockParams{TenantID: "bad", FieldPath: "x"})
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestMemoryStore_Pagination(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "page-schema"})
	require.NoError(t, err)

	// Create 5 schemas total.
	for i := 0; i < 4; i++ {
		_, err = store.CreateSchema(ctx, CreateSchemaParams{Name: "s" + string(rune('A'+i))})
		require.NoError(t, err)
	}

	// Limit.
	list, err := store.ListSchemas(ctx, ListSchemasParams{Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Offset.
	list, err = store.ListSchemas(ctx, ListSchemasParams{Limit: 10, Offset: 3})
	require.NoError(t, err)
	assert.Len(t, list, 2)

	// Offset beyond range.
	list, err = store.ListSchemas(ctx, ListSchemasParams{Limit: 10, Offset: 100})
	require.NoError(t, err)
	assert.Empty(t, list)

	// Tenant pagination.
	for i := 0; i < 3; i++ {
		_, err = store.CreateTenant(ctx, CreateTenantParams{
			Name: "t" + string(rune('A'+i)), SchemaID: s.ID, SchemaVersion: 1,
		})
		require.NoError(t, err)
	}

	tList, err := store.ListTenants(ctx, ListTenantsParams{Limit: 2, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, tList, 2)

	tList, err = store.ListTenants(ctx, ListTenantsParams{Limit: 10, Offset: 2})
	require.NoError(t, err)
	assert.Len(t, tList, 1)

	// ListTenantsBySchema pagination.
	bySchema, err := store.ListTenantsBySchema(ctx, ListTenantsBySchemaParams{
		SchemaID: s.ID, Limit: 1, Offset: 0,
	})
	require.NoError(t, err)
	assert.Len(t, bySchema, 1)
}

func TestMemoryStore_DeleteSchema_SoftDelete(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "soft-delete-schema"})
	require.NoError(t, err)

	sv, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID: s.ID, Version: 1, Checksum: "x",
	})
	require.NoError(t, err)

	_, err = store.CreateSchemaField(ctx, CreateSchemaFieldParams{
		SchemaVersionID: sv.ID, Path: "a.b", FieldType: domain.FieldTypeString,
	})
	require.NoError(t, err)

	tenant, err := store.CreateTenant(ctx, CreateTenantParams{
		Name: "soft-delete-tenant", SchemaID: s.ID, SchemaVersion: 1,
	})
	require.NoError(t, err)

	err = store.CreateFieldLock(ctx, CreateFieldLockParams{
		TenantID: tenant.ID, FieldPath: "a.b", LockedValues: []byte(`["x"]`),
	})
	require.NoError(t, err)

	// Soft-delete schema.
	err = store.DeleteSchema(ctx, s.ID)
	require.NoError(t, err)

	// Schema inaccessible via normal lookups.
	_, err = store.GetSchemaByID(ctx, s.ID)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	_, err = store.GetSchemaByName(ctx, "soft-delete-schema")
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Schema absent from list.
	schemas, err := store.ListSchemas(ctx, ListSchemasParams{Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, schemas)

	// Double-delete returns ErrNotFound.
	assert.True(t, errors.Is(store.DeleteSchema(ctx, s.ID), domain.ErrNotFound))

	// Name can be reused after soft-delete.
	_, err = store.CreateSchema(ctx, CreateSchemaParams{Name: "soft-delete-schema"})
	require.NoError(t, err)

	// Tenant remains accessible (has its own deleted_at lifecycle).
	_, err = store.GetTenantByID(ctx, tenant.ID)
	require.NoError(t, err)

	// Field locks remain.
	locks, err := store.GetFieldLocks(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Len(t, locks, 1)
}

func TestMemoryStore_DeleteTenant_SoftDelete(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "sd-schema"})
	require.NoError(t, err)

	tenant, err := store.CreateTenant(ctx, CreateTenantParams{
		Name: "sd-tenant", SchemaID: s.ID, SchemaVersion: 1,
	})
	require.NoError(t, err)

	err = store.CreateFieldLock(ctx, CreateFieldLockParams{
		TenantID: tenant.ID, FieldPath: "x", LockedValues: []byte(`["v"]`),
	})
	require.NoError(t, err)

	// Soft-delete tenant.
	require.NoError(t, store.DeleteTenant(ctx, tenant.ID))

	// Tenant inaccessible via normal lookups.
	_, err = store.GetTenantByID(ctx, tenant.ID)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
	_, err = store.GetTenantByName(ctx, "sd-tenant")
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	// Tenant absent from list.
	list, err := store.ListTenants(ctx, ListTenantsParams{Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, list)

	// Double-delete returns ErrNotFound.
	assert.True(t, errors.Is(store.DeleteTenant(ctx, tenant.ID), domain.ErrNotFound))

	// Name can be reused after soft-delete.
	_, err = store.CreateTenant(ctx, CreateTenantParams{
		Name: "sd-tenant", SchemaID: s.ID, SchemaVersion: 1,
	})
	require.NoError(t, err)

	// Field locks remain in storage (accessible by ID, not via normal tenant lookup).
	locks, err := store.GetFieldLocks(ctx, tenant.ID)
	require.NoError(t, err)
	assert.Len(t, locks, 1)
}

func TestMemoryStore_ListTenants_FilteredByAllowedIDs(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "filter-test"})
	require.NoError(t, err)

	// Create 3 tenants.
	t1, err := store.CreateTenant(ctx, CreateTenantParams{Name: "alpha", SchemaID: s.ID, SchemaVersion: 1})
	require.NoError(t, err)
	t2, err := store.CreateTenant(ctx, CreateTenantParams{Name: "beta", SchemaID: s.ID, SchemaVersion: 1})
	require.NoError(t, err)
	_, err = store.CreateTenant(ctx, CreateTenantParams{Name: "gamma", SchemaID: s.ID, SchemaVersion: 1})
	require.NoError(t, err)

	// nil AllowedTenantIDs → all tenants (superadmin).
	all, err := store.ListTenants(ctx, ListTenantsParams{Limit: 10, AllowedTenantIDs: nil})
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// Filter to specific IDs.
	filtered, err := store.ListTenants(ctx, ListTenantsParams{
		Limit:            10,
		AllowedTenantIDs: []string{t1.ID, t2.ID},
	})
	require.NoError(t, err)
	assert.Len(t, filtered, 2)
	ids := []string{filtered[0].ID, filtered[1].ID}
	assert.Contains(t, ids, t1.ID)
	assert.Contains(t, ids, t2.ID)

	// Empty AllowedTenantIDs → no tenants.
	empty, err := store.ListTenants(ctx, ListTenantsParams{
		Limit:            10,
		AllowedTenantIDs: []string{},
	})
	require.NoError(t, err)
	assert.Empty(t, empty)

	// Pagination works with filtering.
	paged, err := store.ListTenants(ctx, ListTenantsParams{
		Limit:            1,
		AllowedTenantIDs: []string{t1.ID, t2.ID},
	})
	require.NoError(t, err)
	assert.Len(t, paged, 1)
}

func TestMemoryStore_ListTenantsBySchema_FilteredByAllowedIDs(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s1, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "schema-a"})
	require.NoError(t, err)
	s2, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "schema-b"})
	require.NoError(t, err)

	t1, err := store.CreateTenant(ctx, CreateTenantParams{Name: "a1", SchemaID: s1.ID, SchemaVersion: 1})
	require.NoError(t, err)
	_, err = store.CreateTenant(ctx, CreateTenantParams{Name: "a2", SchemaID: s1.ID, SchemaVersion: 1})
	require.NoError(t, err)
	t3, err := store.CreateTenant(ctx, CreateTenantParams{Name: "b1", SchemaID: s2.ID, SchemaVersion: 1})
	require.NoError(t, err)

	// Filter by schema + allowed IDs.
	filtered, err := store.ListTenantsBySchema(ctx, ListTenantsBySchemaParams{
		SchemaID:         s1.ID,
		Limit:            10,
		AllowedTenantIDs: []string{t1.ID, t3.ID}, // t3 is in schema-b, should not appear
	})
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, t1.ID, filtered[0].ID)

	// nil AllowedTenantIDs → all tenants in schema.
	all, err := store.ListTenantsBySchema(ctx, ListTenantsBySchemaParams{
		SchemaID: s1.ID, Limit: 10, AllowedTenantIDs: nil,
	})
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestMemoryStore_BulkCreateSchemaFields(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "bulk-fields"})
	require.NoError(t, err)

	sv, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID: s.ID, Version: 1, Checksum: "bulk",
	})
	require.NoError(t, err)

	created, err := store.BulkCreateSchemaFields(ctx, []CreateSchemaFieldParams{
		{
			SchemaVersionID: sv.ID,
			Path:            "app.name",
			FieldType:       domain.FieldTypeString,
		},
		{
			SchemaVersionID: sv.ID,
			Path:            "app.port",
			FieldType:       domain.FieldTypeInteger,
		},
	})
	require.NoError(t, err)
	require.Len(t, created, 2)
	assert.NotEmpty(t, created[0].ID)
	assert.NotEmpty(t, created[1].ID)

	fields, err := store.GetSchemaFields(ctx, sv.ID)
	require.NoError(t, err)
	require.Len(t, fields, 2)

	gotPaths := []string{fields[0].Path, fields[1].Path}
	assert.ElementsMatch(t, []string{"app.name", "app.port"}, gotPaths)

	// Empty batch returns empty result and no error.
	empty, err := store.BulkCreateSchemaFields(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	// Error when any item references a missing schema version.
	_, err = store.BulkCreateSchemaFields(ctx, []CreateSchemaFieldParams{
		{
			SchemaVersionID: "bad-version-id",
			Path:            "x",
			FieldType:       domain.FieldTypeString,
		},
	})
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestMemoryStore_GetSchemaFieldsByVersionIDs(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "version-ids"})
	require.NoError(t, err)

	sv1, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID: s.ID, Version: 1, Checksum: "v1",
	})
	require.NoError(t, err)

	sv2, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID: s.ID, Version: 2, Checksum: "v2",
	})
	require.NoError(t, err)

	// Control version not requested later.
	s2, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "other-schema"})
	require.NoError(t, err)
	svOther, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID: s2.ID, Version: 1, Checksum: "other",
	})
	require.NoError(t, err)

	_, err = store.CreateSchemaField(ctx, CreateSchemaFieldParams{
		SchemaVersionID: sv1.ID, Path: "a", FieldType: domain.FieldTypeString,
	})
	require.NoError(t, err)
	_, err = store.CreateSchemaField(ctx, CreateSchemaFieldParams{
		SchemaVersionID: sv1.ID, Path: "b", FieldType: domain.FieldTypeInteger,
	})
	require.NoError(t, err)
	_, err = store.CreateSchemaField(ctx, CreateSchemaFieldParams{
		SchemaVersionID: sv2.ID, Path: "c", FieldType: domain.FieldTypeBool,
	})
	require.NoError(t, err)
	_, err = store.CreateSchemaField(ctx, CreateSchemaFieldParams{
		SchemaVersionID: svOther.ID, Path: "z", FieldType: domain.FieldTypeString,
	})
	require.NoError(t, err)

	got, err := store.GetSchemaFieldsByVersionIDs(ctx, []string{sv1.ID, sv2.ID, "missing"})
	require.NoError(t, err)
	require.Len(t, got, 3)

	gotKeys := make([]string, 0, len(got))
	for _, f := range got {
		gotKeys = append(gotKeys, f.SchemaVersionID+":"+f.Path)
	}
	assert.ElementsMatch(t, []string{
		sv1.ID + ":a",
		sv1.ID + ":b",
		sv2.ID + ":c",
	}, gotKeys)

	// Empty input -> empty output.
	none, err := store.GetSchemaFieldsByVersionIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestMemoryStore_GetLatestSchemaVersionsBatch(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	s1, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "batch-a"})
	require.NoError(t, err)
	s2, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "batch-b"})
	require.NoError(t, err)
	s3, err := store.CreateSchema(ctx, CreateSchemaParams{Name: "batch-c"})
	require.NoError(t, err)

	// s1 latest should be version 3.
	_, err = store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{SchemaID: s1.ID, Version: 1, Checksum: "a1"})
	require.NoError(t, err)
	_, err = store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{SchemaID: s1.ID, Version: 3, Checksum: "a3"})
	require.NoError(t, err)
	_, err = store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{SchemaID: s1.ID, Version: 2, Checksum: "a2"})
	require.NoError(t, err)

	// s2 latest should be version 2.
	_, err = store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{SchemaID: s2.ID, Version: 1, Checksum: "b1"})
	require.NoError(t, err)
	_, err = store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{SchemaID: s2.ID, Version: 2, Checksum: "b2"})
	require.NoError(t, err)

	// s3 has versions but is not requested.
	_, err = store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{SchemaID: s3.ID, Version: 9, Checksum: "c9"})
	require.NoError(t, err)

	got, err := store.GetLatestSchemaVersionsBatch(ctx, []string{s1.ID, s2.ID, "missing"})
	require.NoError(t, err)
	require.Len(t, got, 2)

	bySchema := make(map[string]domain.SchemaVersion, len(got))
	for _, sv := range got {
		bySchema[sv.SchemaID] = sv
	}

	require.Contains(t, bySchema, s1.ID)
	require.Contains(t, bySchema, s2.ID)
	assert.Equal(t, int32(3), bySchema[s1.ID].Version)
	assert.Equal(t, int32(2), bySchema[s2.ID].Version)

	none, err := store.GetLatestSchemaVersionsBatch(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

// Verify MemoryStore implements Store at compile time.
var _ Store = (*MemoryStore)(nil)
