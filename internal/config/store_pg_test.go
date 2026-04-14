package config

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"

	"github.com/opendecree/decree/internal/storage/dbstore"
	"github.com/opendecree/decree/internal/storage/pgconv"
)

var (
	testUUID = pgconv.MustUUID("11111111-1111-1111-1111-111111111111")
	testTime = time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	testTS   = pgtype.Timestamptz{Time: testTime, Valid: true}
)

func TestConfigVersionFromDB(t *testing.T) {
	desc := "test"
	r := dbstore.ConfigVersion{
		ID:          testUUID,
		TenantID:    testUUID,
		Version:     5,
		Description: &desc,
		CreatedBy:   "admin",
		CreatedAt:   testTS,
	}
	d := configVersionFromDB(r)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", d.ID)
	assert.Equal(t, int32(5), d.Version)
	assert.Equal(t, "test", *d.Description)
	assert.Equal(t, "admin", d.CreatedBy)
	assert.Equal(t, testTime, d.CreatedAt)
}

func TestConfigValueFromDB(t *testing.T) {
	val := "hello"
	chk := "abc"
	r := dbstore.ConfigValue{
		ConfigVersionID: testUUID,
		FieldPath:       "app.name",
		Value:           &val,
		Checksum:        &chk,
	}
	d := configValueFromDB(r)
	assert.Equal(t, "app.name", d.FieldPath)
	assert.Equal(t, "hello", *d.Value)
	assert.Equal(t, "abc", *d.Checksum)
}

func TestConfigValueFromDB_NullValue(t *testing.T) {
	r := dbstore.ConfigValue{FieldPath: "x"}
	d := configValueFromDB(r)
	assert.Nil(t, d.Value)
	assert.Nil(t, d.Checksum)
}

func TestTenantFromDB(t *testing.T) {
	r := dbstore.Tenant{
		ID:            testUUID,
		Name:          "acme",
		SchemaID:      testUUID,
		SchemaVersion: 2,
		CreatedAt:     testTS,
		UpdatedAt:     testTS,
	}
	d := tenantFromDB(r)
	assert.Equal(t, "acme", d.Name)
	assert.Equal(t, int32(2), d.SchemaVersion)
	assert.Equal(t, testTime, d.CreatedAt)
}

func TestSchemaFieldFromDB(t *testing.T) {
	desc := "a field"
	r := dbstore.SchemaField{
		ID:              testUUID,
		SchemaVersionID: testUUID,
		Path:            "app.retries",
		FieldType:       dbstore.FieldTypeInteger,
		Constraints:     []byte(`{"min":0}`),
		Nullable:        true,
		Description:     &desc,
	}
	d := schemaFieldFromDB(r)
	assert.Equal(t, "app.retries", d.Path)
	assert.Equal(t, "integer", string(d.FieldType))
	assert.True(t, d.Nullable)
	assert.Equal(t, "a field", *d.Description)
	assert.NotNil(t, d.Constraints)
}

func TestSchemaVersionFromDB(t *testing.T) {
	parent := int32(1)
	r := dbstore.SchemaVersion{
		ID:            testUUID,
		SchemaID:      testUUID,
		Version:       2,
		ParentVersion: &parent,
		Checksum:      "abc123",
		Published:     true,
		CreatedAt:     testTS,
	}
	d := schemaVersionFromDB(r)
	assert.Equal(t, int32(2), d.Version)
	assert.Equal(t, int32(1), *d.ParentVersion)
	assert.Equal(t, "abc123", d.Checksum)
	assert.True(t, d.Published)
}

func TestFieldLockFromDB(t *testing.T) {
	r := dbstore.TenantFieldLock{
		TenantID:     testUUID,
		FieldPath:    "app.fee",
		LockedValues: []byte(`["0.01"]`),
	}
	d := fieldLockFromDB(r)
	assert.Equal(t, "app.fee", d.FieldPath)
	assert.NotNil(t, d.LockedValues)
}
