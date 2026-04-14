package audit

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"

	"github.com/opendecree/decree/internal/storage/dbstore"
	"github.com/opendecree/decree/internal/storage/pgconv"
)

var (
	testUUID = pgconv.MustUUID("33333333-3333-3333-3333-333333333333")
	testTime = time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	testTS   = pgtype.Timestamptz{Time: testTime, Valid: true}
)

func TestAuditWriteLogFromDB(t *testing.T) {
	fp := "app.fee"
	old := "0.01"
	new := "0.02"
	ver := int32(3)
	r := dbstore.AuditWriteLog{
		ID:            testUUID,
		TenantID:      testUUID,
		Actor:         "admin",
		Action:        "set_field",
		FieldPath:     &fp,
		OldValue:      &old,
		NewValue:      &new,
		ConfigVersion: &ver,
		Metadata:      []byte(`{}`),
		CreatedAt:     testTS,
	}
	d := auditWriteLogFromDB(r)
	assert.Equal(t, "33333333-3333-3333-3333-333333333333", d.ID)
	assert.Equal(t, "admin", d.Actor)
	assert.Equal(t, "set_field", d.Action)
	assert.Equal(t, "app.fee", *d.FieldPath)
	assert.Equal(t, "0.01", *d.OldValue)
	assert.Equal(t, "0.02", *d.NewValue)
	assert.Equal(t, int32(3), *d.ConfigVersion)
	assert.Equal(t, testTime, d.CreatedAt)
}

func TestAuditWriteLogFromDB_NullFields(t *testing.T) {
	r := dbstore.AuditWriteLog{
		ID:        testUUID,
		TenantID:  testUUID,
		Actor:     "admin",
		Action:    "set_field",
		CreatedAt: testTS,
	}
	d := auditWriteLogFromDB(r)
	assert.Nil(t, d.FieldPath)
	assert.Nil(t, d.OldValue)
	assert.Nil(t, d.NewValue)
	assert.Nil(t, d.ConfigVersion)
}

func TestUsageStatFromDB(t *testing.T) {
	lastBy := "reader"
	r := dbstore.UsageStat{
		TenantID:    testUUID,
		FieldPath:   "app.fee",
		PeriodStart: testTS,
		ReadCount:   42,
		LastReadBy:  &lastBy,
		LastReadAt:  testTS,
	}
	d := usageStatFromDB(r)
	assert.Equal(t, "app.fee", d.FieldPath)
	assert.Equal(t, int64(42), d.ReadCount)
	assert.Equal(t, "reader", *d.LastReadBy)
	assert.NotNil(t, d.LastReadAt)
	assert.Equal(t, testTime, *d.LastReadAt)
}

func TestUsageStatFromDB_NullTimestamp(t *testing.T) {
	r := dbstore.UsageStat{
		TenantID:  testUUID,
		FieldPath: "x",
		ReadCount: 1,
	}
	d := usageStatFromDB(r)
	assert.Nil(t, d.LastReadBy)
	assert.Nil(t, d.LastReadAt)
}

func TestTenantUsageRowFromDB(t *testing.T) {
	r := dbstore.GetTenantUsageRow{
		FieldPath:  "app.fee",
		ReadCount:  10,
		LastReadAt: pgtype.Timestamptz{Time: testTime, Valid: true},
	}
	d := tenantUsageRowFromDB(r)
	assert.Equal(t, "app.fee", d.FieldPath)
	assert.Equal(t, int64(10), d.ReadCount)
	assert.NotNil(t, d.LastReadAt)
}

func TestTenantUsageRowFromDB_NullTimestamp(t *testing.T) {
	r := dbstore.GetTenantUsageRow{
		FieldPath: "x",
		ReadCount: 5,
	}
	d := tenantUsageRowFromDB(r)
	assert.Nil(t, d.LastReadAt)
}

func TestTenantUsageRowFromDB_InvalidTimestampType(t *testing.T) {
	r := dbstore.GetTenantUsageRow{
		FieldPath:  "x",
		ReadCount:  1,
		LastReadAt: "not-a-timestamp",
	}
	d := tenantUsageRowFromDB(r)
	assert.Nil(t, d.LastReadAt)
}
