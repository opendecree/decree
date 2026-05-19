package dbstore

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newBenchPool(b *testing.B) *pgxpool.Pool {
	b.Helper()
	dsn := os.Getenv("DB_WRITE_URL")
	if dsn == "" {
		b.Skip("DB_WRITE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		b.Fatalf("connect db: %v", err)
	}
	b.Cleanup(pool.Close)
	return pool
}

// benchFixture sets up a minimal schema → schema_version → tenant →
// config_version chain and returns the IDs needed by the benchmarks.
// All rows are deleted on b.Cleanup.
type benchFixture struct {
	tenantID        pgtype.UUID
	configVersionID pgtype.UUID
}

func setupBenchFixture(b *testing.B, q *Queries, ctx context.Context) benchFixture {
	b.Helper()

	schema, err := q.CreateSchema(ctx, CreateSchemaParams{Name: fmt.Sprintf("bench-schema-%d", b.N)})
	if err != nil {
		b.Fatalf("create schema: %v", err)
	}

	sv, err := q.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID:          schema.ID,
		Version:           1,
		Checksum:          "bench",
		DependentRequired: []byte("[]"),
		Validations:       []byte("[]"),
	})
	if err != nil {
		b.Fatalf("create schema version: %v", err)
	}

	tenant, err := q.CreateTenant(ctx, CreateTenantParams{
		Name:          fmt.Sprintf("bench-tenant-%d", b.N),
		SchemaID:      schema.ID,
		SchemaVersion: sv.Version,
	})
	if err != nil {
		b.Fatalf("create tenant: %v", err)
	}

	cv, err := q.CreateConfigVersion(ctx, CreateConfigVersionParams{
		TenantID:  tenant.ID,
		Version:   1,
		CreatedBy: "bench",
	})
	if err != nil {
		b.Fatalf("create config version: %v", err)
	}

	b.Cleanup(func() {
		// Cascade deletes handle child rows; order matters for FK constraints.
		_, _ = q.db.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant.ID)
		_, _ = q.db.Exec(ctx, "DELETE FROM schema_versions WHERE id = $1", sv.ID)
		_, _ = q.db.Exec(ctx, "DELETE FROM schemas WHERE id = $1", schema.ID)
	})

	return benchFixture{tenantID: tenant.ID, configVersionID: cv.ID}
}

func BenchmarkSetConfigValue(b *testing.B) {
	pool := newBenchPool(b)
	q := New(pool)
	ctx := context.Background()
	fix := setupBenchFixture(b, q, ctx)
	val := "bench-value"
	chk := "abc123"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = q.SetConfigValue(ctx, SetConfigValueParams{
			ConfigVersionID: fix.configVersionID,
			FieldPath:       fmt.Sprintf("bench.field_%d", i%50),
			Value:           &val,
			Checksum:        &chk,
		})
	}
}

func BenchmarkGetFullConfigAtVersion(b *testing.B) {
	pool := newBenchPool(b)
	q := New(pool)
	ctx := context.Background()
	fix := setupBenchFixture(b, q, ctx)

	// Pre-populate 20 fields so the query scans a realistic result set.
	val := "bench-v"
	for i := range 20 {
		fp := fmt.Sprintf("bench.field_%d", i)
		_ = q.SetConfigValue(ctx, SetConfigValueParams{
			ConfigVersionID: fix.configVersionID,
			FieldPath:       fp,
			Value:           &val,
		})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _ = q.GetFullConfigAtVersion(ctx, GetFullConfigAtVersionParams{
			TenantID: fix.tenantID,
			Version:  1,
		})
	}
}

func BenchmarkListTenants(b *testing.B) {
	pool := newBenchPool(b)
	q := New(pool)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = q.ListTenants(ctx, ListTenantsParams{Limit: 50, Offset: 0})
	}
}
