//go:build integration

package schema

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/storage/pgtest"
)

func TestSchemaPGStore(t *testing.T) {
	pool := pgtest.NewPool(t)
	store := NewPGStore(pool, pool)
	ctx := context.Background()

	t.Run("SchemaCRUD", func(t *testing.T) {
		desc := "test schema description"
		sch, err := store.CreateSchema(ctx, CreateSchemaParams{
			Name:        fmt.Sprintf("schema-crud-%s", t.Name()),
			Description: &desc,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, sch.ID)
		assert.Equal(t, *sch.Description, desc)

		// GetByID
		got, err := store.GetSchemaByID(ctx, sch.ID)
		require.NoError(t, err)
		assert.Equal(t, sch.ID, got.ID)
		assert.Equal(t, sch.Name, got.Name)

		// GetByName
		got2, err := store.GetSchemaByName(ctx, sch.Name)
		require.NoError(t, err)
		assert.Equal(t, sch.ID, got2.ID)

		// ListSchemas
		schemas, err := store.ListSchemas(ctx, ListSchemasParams{Limit: 100, Offset: 0})
		require.NoError(t, err)
		found := false
		for _, s := range schemas {
			if s.ID == sch.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "created schema should appear in list")

		// DeleteSchema
		require.NoError(t, store.DeleteSchema(ctx, sch.ID))
		_, err = store.GetSchemaByID(ctx, sch.ID)
		require.Error(t, err)
	})

	t.Run("SchemaVersionCRUD", func(t *testing.T) {
		sch, err := store.CreateSchema(ctx, CreateSchemaParams{
			Name: fmt.Sprintf("schema-ver-%s", t.Name()),
		})
		require.NoError(t, err)

		sv, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
			SchemaID: sch.ID,
			Version:  1,
			Checksum: "abc123",
		})
		require.NoError(t, err)
		assert.Equal(t, int32(1), sv.Version)
		assert.False(t, sv.Published)

		// GetSchemaVersion
		got, err := store.GetSchemaVersion(ctx, GetSchemaVersionParams{
			SchemaID: sch.ID,
			Version:  1,
		})
		require.NoError(t, err)
		assert.Equal(t, sv.ID, got.ID)

		// GetLatestSchemaVersion
		sv2, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
			SchemaID:      sch.ID,
			Version:       2,
			Checksum:      "def456",
			ParentVersion: &sv.Version,
		})
		require.NoError(t, err)

		latest, err := store.GetLatestSchemaVersion(ctx, sch.ID)
		require.NoError(t, err)
		assert.Equal(t, sv2.ID, latest.ID)

		// PublishSchemaVersion
		published, err := store.PublishSchemaVersion(ctx, PublishSchemaVersionParams{
			SchemaID: sch.ID,
			Version:  1,
		})
		require.NoError(t, err)
		assert.True(t, published.Published)
	})

	t.Run("SchemaFieldCRUD", func(t *testing.T) {
		sch, err := store.CreateSchema(ctx, CreateSchemaParams{
			Name: fmt.Sprintf("schema-field-%s", t.Name()),
		})
		require.NoError(t, err)

		sv, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
			SchemaID: sch.ID,
			Version:  1,
			Checksum: "abc",
		})
		require.NoError(t, err)

		def := "42"
		field, err := store.CreateSchemaField(ctx, CreateSchemaFieldParams{
			SchemaVersionID: sv.ID,
			Path:            "app.retries",
			FieldType:       domain.FieldTypeInteger,
			Nullable:        false,
			DefaultValue:    &def,
		})
		require.NoError(t, err)
		assert.Equal(t, "app.retries", field.Path)
		assert.Equal(t, domain.FieldTypeInteger, field.FieldType)
		assert.Equal(t, "42", *field.DefaultValue)

		// GetSchemaFields
		fields, err := store.GetSchemaFields(ctx, sv.ID)
		require.NoError(t, err)
		require.Len(t, fields, 1)
		assert.Equal(t, field.ID, fields[0].ID)

		// DeleteSchemaField
		require.NoError(t, store.DeleteSchemaField(ctx, DeleteSchemaFieldParams{
			SchemaVersionID: sv.ID,
			Path:            "app.retries",
		}))

		fields, err = store.GetSchemaFields(ctx, sv.ID)
		require.NoError(t, err)
		assert.Empty(t, fields)
	})

	t.Run("TenantCRUD", func(t *testing.T) {
		sch, err := store.CreateSchema(ctx, CreateSchemaParams{
			Name: fmt.Sprintf("schema-tenant-%s", t.Name()),
		})
		require.NoError(t, err)

		sv, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
			SchemaID: sch.ID,
			Version:  1,
			Checksum: "abc",
		})
		require.NoError(t, err)

		tenant, err := store.CreateTenant(ctx, CreateTenantParams{
			Name:          fmt.Sprintf("tenant-%s", t.Name()),
			SchemaID:      sch.ID,
			SchemaVersion: sv.Version,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, tenant.ID)

		// GetByID and GetByName
		got, err := store.GetTenantByID(ctx, tenant.ID)
		require.NoError(t, err)
		assert.Equal(t, tenant.Name, got.Name)

		got2, err := store.GetTenantByName(ctx, tenant.Name)
		require.NoError(t, err)
		assert.Equal(t, tenant.ID, got2.ID)

		// ListTenants
		tenants, err := store.ListTenants(ctx, ListTenantsParams{Limit: 100})
		require.NoError(t, err)
		found := false
		for _, ten := range tenants {
			if ten.ID == tenant.ID {
				found = true
				break
			}
		}
		assert.True(t, found)

		// ListTenantsBySchema
		bySchema, err := store.ListTenantsBySchema(ctx, ListTenantsBySchemaParams{
			SchemaID: sch.ID,
			Limit:    100,
		})
		require.NoError(t, err)
		require.Len(t, bySchema, 1)
		assert.Equal(t, tenant.ID, bySchema[0].ID)

		// UpdateTenantName
		updated, err := store.UpdateTenantName(ctx, UpdateTenantNameParams{
			ID:   tenant.ID,
			Name: tenant.Name + "-renamed",
		})
		require.NoError(t, err)
		assert.Contains(t, updated.Name, "-renamed")

		// UpdateTenantSchemaVersion
		sv2, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
			SchemaID: sch.ID,
			Version:  2,
			Checksum: "def",
		})
		require.NoError(t, err)

		updated2, err := store.UpdateTenantSchemaVersion(ctx, UpdateTenantSchemaVersionParams{
			ID:            tenant.ID,
			SchemaVersion: sv2.Version,
		})
		require.NoError(t, err)
		assert.Equal(t, sv2.Version, updated2.SchemaVersion)

		// ListTenantsBySchema filtered by AllowedTenantIDs
		filtered, err := store.ListTenants(ctx, ListTenantsParams{
			Limit:            100,
			AllowedTenantIDs: []string{tenant.ID},
		})
		require.NoError(t, err)
		require.Len(t, filtered, 1)
		assert.Equal(t, tenant.ID, filtered[0].ID)

		// DeleteTenant
		require.NoError(t, store.DeleteTenant(ctx, tenant.ID))
		_, err = store.GetTenantByID(ctx, tenant.ID)
		require.Error(t, err)
	})

	t.Run("FieldLocks", func(t *testing.T) {
		sch, err := store.CreateSchema(ctx, CreateSchemaParams{
			Name: fmt.Sprintf("schema-lock-%s", t.Name()),
		})
		require.NoError(t, err)

		sv, err := store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
			SchemaID: sch.ID,
			Version:  1,
			Checksum: "abc",
		})
		require.NoError(t, err)

		tenant, err := store.CreateTenant(ctx, CreateTenantParams{
			Name:          fmt.Sprintf("tenant-lock-%s", t.Name()),
			SchemaID:      sch.ID,
			SchemaVersion: sv.Version,
		})
		require.NoError(t, err)

		err = store.CreateFieldLock(ctx, CreateFieldLockParams{
			TenantID:     tenant.ID,
			FieldPath:    "app.fee",
			LockedValues: []byte(`["0.01","0.02"]`),
		})
		require.NoError(t, err)

		locks, err := store.GetFieldLocks(ctx, tenant.ID)
		require.NoError(t, err)
		require.Len(t, locks, 1)
		assert.Equal(t, "app.fee", locks[0].FieldPath)

		require.NoError(t, store.DeleteFieldLock(ctx, DeleteFieldLockParams{
			TenantID:  tenant.ID,
			FieldPath: "app.fee",
		}))

		locks, err = store.GetFieldLocks(ctx, tenant.ID)
		require.NoError(t, err)
		assert.Empty(t, locks)
	})

	t.Run("TxRollback", func(t *testing.T) {
		schName := fmt.Sprintf("schema-tx-%s", t.Name())

		err := store.RunInTx(ctx, func(txStore Store) error {
			_, innerErr := txStore.CreateSchema(ctx, CreateSchemaParams{Name: schName})
			if innerErr != nil {
				return innerErr
			}
			return errors.New("deliberate rollback")
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "deliberate rollback")

		// Schema must not exist after rollback.
		_, err = store.GetSchemaByName(ctx, schName)
		require.Error(t, err)
	})

	t.Run("InsertAuditWriteLog", func(t *testing.T) {
		sch, err := store.CreateSchema(ctx, CreateSchemaParams{
			Name: fmt.Sprintf("schema-audit-%s", t.Name()),
		})
		require.NoError(t, err)

		action := "create_schema"
		newVal := sch.ID
		err = store.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			Actor:      "test-actor",
			Action:     action,
			ObjectKind: "schema",
			NewValue:   &newVal,
		})
		require.NoError(t, err)
	})
}
