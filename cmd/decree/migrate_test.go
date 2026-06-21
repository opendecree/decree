package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/opendecree/decree/cmd/decree/migrations"
)

// sourceMigrationsDir resolves repo-root/db/migrations relative to this test
// file, so it works regardless of the test's working directory.
func sourceMigrationsDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "db", "migrations")
}

// TestEmbeddedMigrationsMatchSource is a drift guard: the .sql files vendored
// into cmd/decree/migrations must be byte-identical to db/migrations (the source
// of truth). If this fails, run `make sync-migrations`.
func TestEmbeddedMigrationsMatchSource(t *testing.T) {
	srcDir := sourceMigrationsDir()
	srcEntries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("read source migrations dir: %v", err)
	}

	var srcNames []string
	for _, e := range srcEntries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			srcNames = append(srcNames, e.Name())
		}
	}
	if len(srcNames) == 0 {
		t.Fatal("no .sql migrations found in db/migrations")
	}

	for _, name := range srcNames {
		want, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			t.Fatalf("read source %s: %v", name, err)
		}
		got, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("embedded copy of %s missing (run `make sync-migrations`): %v", name, err)
		}
		if string(got) != string(want) {
			t.Fatalf("embedded %s differs from db/migrations/%s (run `make sync-migrations`)", name, name)
		}
	}

	embedded, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read embedded migrations dir: %v", err)
	}
	var embeddedCount int
	for _, e := range embedded {
		if filepath.Ext(e.Name()) == ".sql" {
			embeddedCount++
		}
	}
	if embeddedCount != len(srcNames) {
		t.Fatalf("embedded has %d .sql files, db/migrations has %d (run `make sync-migrations`)", embeddedCount, len(srcNames))
	}
}

// unreachableDSN points at a port that refuses connections, so goose fails fast
// without a real database — enough to exercise runMigrate's wiring + error path.
const unreachableDSN = "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=2"

func TestRunMigrateRequiresDBURL(t *testing.T) {
	if err := runMigrate(context.Background(), io.Discard, "", "up"); err == nil {
		t.Fatal("expected an error when the database URL is empty")
	}
}

func TestRunMigrateUnknownAction(t *testing.T) {
	// The action switch is reached after the (lazy) sql.Open, before any
	// connection is made, so an unreachable DSN is fine here.
	err := runMigrate(context.Background(), io.Discard, unreachableDSN, "bogus")
	if err == nil {
		t.Fatal("expected an error for an unknown migrate action")
	}
}

func TestRunMigrateConnectionError(t *testing.T) {
	// Exercises SetBaseFS/SetDialect + UpContext against an unreachable DB; the
	// success path is covered by the root pgtest suite over the identical SQL.
	if err := runMigrate(context.Background(), io.Discard, unreachableDSN, "up"); err == nil {
		t.Fatal("expected a connection error against an unreachable database")
	}
}

func TestGooseLogger(t *testing.T) {
	var buf bytes.Buffer
	l := newGooseLogger(&buf)
	l.Printf("up %s", "ok")
	l.Fatalf("boom %d", 7)
	out := buf.String()
	if !strings.Contains(out, "up ok") || !strings.Contains(out, "boom 7") {
		t.Fatalf("unexpected logger output: %q", out)
	}
}
