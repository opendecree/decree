// Package migrations embeds the goose SQL migrations so the decree CLI can apply
// them from a static binary — the decree-cli image has no source tree to mount.
//
// The source of truth is db/migrations/ at the repository root. The .sql files
// in this directory are a verbatim copy kept in sync by `make sync-migrations`
// and verified in CI (see the migration-sync drift guard). Do not edit them here.
package migrations

import "embed"

// FS holds the goose migration files (*.sql) at its root.
//
//go:embed *.sql
var FS embed.FS
