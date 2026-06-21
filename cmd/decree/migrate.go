package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"

	_ "github.com/lib/pq" // register the "postgres" database/sql driver
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"

	"github.com/opendecree/decree/cmd/decree/migrations"
)

// embeddedMigrationsDir is the path within migrations.FS that holds the .sql
// files. They sit at the root of the embedded filesystem.
const embeddedMigrationsDir = "."

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply database migrations",
	Long: `Apply the OpenDecree database schema migrations.

Migrations create the tables, row-level-security policies, and the unprivileged
"decree_app" role that the server assumes (SET ROLE) on every connection. A fresh
database must be migrated before the server can start, otherwise the server fails
with: role "decree_app" does not exist.

Connect as a role that may CREATE ROLE and GRANT — the database owner or a
superuser — not as decree_app. Running "up" repeatedly is safe; already-applied
migrations are skipped.`,
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all pending migrations",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runMigrate(cmd.Context(), cmd.OutOrStdout(), mustGetString(cmd, "db-url"), "up")
	},
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show which migrations have been applied",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runMigrate(cmd.Context(), cmd.OutOrStdout(), mustGetString(cmd, "db-url"), "status")
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Roll back the most recent migration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runMigrate(cmd.Context(), cmd.OutOrStdout(), mustGetString(cmd, "db-url"), "down")
	},
}

// runMigrate opens dbURL and runs the requested goose action against the
// embedded migrations. The action is one of "up", "status", or "down".
func runMigrate(ctx context.Context, out io.Writer, dbURL, action string) error {
	if dbURL == "" {
		return fmt.Errorf("database URL required: pass --db-url or set DB_WRITE_URL")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(newGooseLogger(out))
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}

	switch action {
	case "up":
		return goose.UpContext(ctx, db, embeddedMigrationsDir)
	case "status":
		return goose.StatusContext(ctx, db, embeddedMigrationsDir)
	case "down":
		return goose.DownContext(ctx, db, embeddedMigrationsDir)
	default:
		return fmt.Errorf("unknown migrate action %q", action)
	}
}

// gooseLogger adapts goose's logger interface to a writer. goose emits messages
// the way the standard log package does (relying on the logger to add the
// trailing newline), so it wraps a *log.Logger. Unlike goose's default logger,
// Fatalf does not exit the process — goose's library functions return errors, so
// a fatal log here would only duplicate an error cobra already prints.
type gooseLogger struct{ l *log.Logger }

func newGooseLogger(w io.Writer) gooseLogger { return gooseLogger{l: log.New(w, "", 0)} }

func (g gooseLogger) Printf(format string, v ...any) { g.l.Printf(format, v...) }
func (g gooseLogger) Fatalf(format string, v ...any) { g.l.Printf(format, v...) }

func init() {
	migrateCmd.PersistentFlags().String("db-url", envOrDefault("DB_WRITE_URL", ""),
		"PostgreSQL connection URL for the owner/superuser role (defaults to $DB_WRITE_URL)")
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	rootCmd.AddCommand(migrateCmd)
}
