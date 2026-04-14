package schema

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"

	"github.com/opendecree/decree/internal/storage/dbstore"
	"github.com/opendecree/decree/internal/storage/pgconv"
)

var (
	testUUID = pgconv.MustUUID("22222222-2222-2222-2222-222222222222")
	testTime = time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	testTS   = pgtype.Timestamptz{Time: testTime, Valid: true}
)

func TestSchemaFromDB(t *testing.T) {
	desc := "test schema"
	r := dbstore.Schema{
		ID:          testUUID,
		Name:        "payments",
		Description: &desc,
		CreatedAt:   testTS,
		UpdatedAt:   testTS,
	}
	d := schemaFromDB(r)
	assert.Equal(t, "22222222-2222-2222-2222-222222222222", d.ID)
	assert.Equal(t, "payments", d.Name)
	assert.Equal(t, "test schema", *d.Description)
	assert.Equal(t, testTime, d.CreatedAt)
}

func TestSchemaFromDB_NullDescription(t *testing.T) {
	r := dbstore.Schema{ID: testUUID, Name: "test", CreatedAt: testTS, UpdatedAt: testTS}
	d := schemaFromDB(r)
	assert.Nil(t, d.Description)
}

func TestSchemaVersionFromDB(t *testing.T) {
	parent := int32(1)
	desc := "v2"
	r := dbstore.SchemaVersion{
		ID:            testUUID,
		SchemaID:      testUUID,
		Version:       2,
		ParentVersion: &parent,
		Description:   &desc,
		Checksum:      "abc",
		Published:     true,
		CreatedAt:     testTS,
	}
	d := schemaVersionFromDB(r)
	assert.Equal(t, int32(2), d.Version)
	assert.Equal(t, int32(1), *d.ParentVersion)
	assert.Equal(t, "v2", *d.Description)
	assert.True(t, d.Published)
}

func TestSchemaFieldFromDB(t *testing.T) {
	redirect := "new.field"
	def := "42"
	desc := "retries"
	r := dbstore.SchemaField{
		ID:              testUUID,
		SchemaVersionID: testUUID,
		Path:            "app.retries",
		FieldType:       dbstore.FieldTypeInteger,
		Constraints:     []byte(`{"min":0}`),
		Nullable:        true,
		Deprecated:      true,
		RedirectTo:      &redirect,
		DefaultValue:    &def,
		Description:     &desc,
	}
	d := schemaFieldFromDB(r)
	assert.Equal(t, "app.retries", d.Path)
	assert.Equal(t, "integer", string(d.FieldType))
	assert.True(t, d.Nullable)
	assert.True(t, d.Deprecated)
	assert.Equal(t, "new.field", *d.RedirectTo)
	assert.Equal(t, "42", *d.DefaultValue)
	assert.Equal(t, "retries", *d.Description)
}

func TestTenantFromDB(t *testing.T) {
	r := dbstore.Tenant{
		ID:            testUUID,
		Name:          "acme",
		SchemaID:      testUUID,
		SchemaVersion: 3,
		CreatedAt:     testTS,
		UpdatedAt:     testTS,
	}
	d := tenantFromDB(r)
	assert.Equal(t, "acme", d.Name)
	assert.Equal(t, int32(3), d.SchemaVersion)
}

func TestTenantsFromDB(t *testing.T) {
	rows := []dbstore.Tenant{
		{ID: testUUID, Name: "a", SchemaID: testUUID, CreatedAt: testTS, UpdatedAt: testTS},
		{ID: testUUID, Name: "b", SchemaID: testUUID, CreatedAt: testTS, UpdatedAt: testTS},
	}
	result := tenantsFromDB(rows)
	assert.Len(t, result, 2)
	assert.Equal(t, "a", result[0].Name)
	assert.Equal(t, "b", result[1].Name)
}

func TestTenantsFromDB_Empty(t *testing.T) {
	result := tenantsFromDB(nil)
	assert.Len(t, result, 0)
}
