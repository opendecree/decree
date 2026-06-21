# Architecture Decisions

---

## `decree migrate` CLI bootstrap (issue #946)

**Date:** 2026-06-21

**Question:** A fresh 0.12 database needs the `decree_app` role + migrations applied before the server starts (the pool runs `SET ROLE decree_app` per connection, `internal/storage/postgres.go`). The server does not auto-migrate and there was no turnkey path for released-image deploys. How should demos / users bootstrap a fresh DB?

**Decision:** Add a `decree migrate` subcommand (`up`/`status`/`down`) to the CLI, shipping in the `decree-cli` image. It runs goose against migrations embedded in the binary.

**Key constraints + rationale:**

- **Embed crosses a module boundary.** `db/migrations/` is in the root module; the CLI is the separate `cmd/decree` module, so `go:embed` cannot reach it. Resolved by vendoring a byte-identical copy into `cmd/decree/migrations/`, kept in sync by `make sync-migrations` and enforced by a unit test (`TestEmbeddedMigrationsMatchSource`) that runs in `make test`. Source of truth stays `db/migrations/`.
- **CLI stays on Go 1.24.** goose ≥ 3.27 requires Go 1.25, which would break the CLI's 1.24 floor (see ADR-006), so goose is pinned to `v3.26.0`. The root module keeps its newer goose (server is 1.25); the drift-guarded SQL is identical, so both goose versions apply the same migrations.
- **Driver is `lib/pq`, not pgx.** pgx `< 5.9.0` carries a critical advisory (GHSA-9jj7-4m8r-rfcm) that the CI dependency-review gate fails on, and the patched pgx `≥ 5.9.0` requires Go 1.25 — so on the 1.24 CLI there is no safe pgx. `lib/pq` (pure Go, 1.13 floor, no advisory) sidesteps it. goose's own transitive pgx is pruned out of the CLI's `go.sum` because the goose core we import does not pull a driver.
- **Reuses `DB_WRITE_URL`** as the `--db-url` default, matching the server/compose env. Must connect as the DB owner/superuser (the migrations create the role + grants), not `decree_app`. `up` is idempotent via `goose_db_version`.
- **No testcontainers in the CLI module** (it would drag a Go-1.25 docker dep tree and break the floor). `runMigrate`'s real-DB apply is covered by the root `pgtest` suite over the identical SQL plus the demos E2E; a manual smoke test confirmed it end-to-end.

**Resolves:** opendecree/decree#946. Unblocks opendecree/demos#49.

---

## Functional-options boilerplate (issue #298)

**Date:** 2026-06-03

**Question:** Should we introduce a generic `internal/optutil` helper to deduplicate `WithLogger` and `Option` definitions across 6+ packages?

**Survey:**

All 6 `WithLogger` functions follow an identical 3-line structure but differ only in their return type and inner `opts` struct name:

| Package | Return type | Inner struct |
|---------|-------------|--------------|
| `internal/auth/jwt.go` | `InterceptorOption` | `*interceptorOptions` |
| `internal/audit/recorder.go` | `Option` | `*recorderOptions` |
| `internal/pubsub/memory.go` | `MemoryOption` | `*MemoryPubSub` |
| `internal/schema/service.go` | `Option` | `*serviceOptions` |
| `internal/server/options.go` | `Option` | `*options` |
| `internal/config/service.go` | `Option` | `*serviceOptions` |

Each `WithLogger` is exactly:
```go
func WithLogger(l *slog.Logger) <PackageOption> {
    return func(o *<pkgOpts>) { o.logger = l }
}
```

The proposed generic helper (`internal/optutil`) would require each package's `opts` struct to implement a `setLogger(*slog.Logger)` method, replacing the 3-line `WithLogger` with a 2-line method. Net line change: zero reduction, with added coupling via a new shared import.

Additional option types surveyed (no `WithLogger`):
- `internal/validation/limits.go` — `Option` (4 distinct option constructors: `WithLimits`, `WithNullable`, `WithSensitive`, `WithConstraints`)
- `internal/ratelimit/interceptor.go` — `Option`
- `internal/storage/postgres.go` — `Option`
- `sdk/adminclient/options.go` — `Option`

These packages do not share any option constructors with each other, confirming the duplication is shallow (logger only) rather than pervasive.

**Decision:** Accept the duplication.

**Rationale:**
- In Go, the `Option` function type (`func(*opts)`) must be per-package for type safety — callers cannot mix options from different packages, and the compiler enforces this naturally via distinct named types.
- A generic `WithLoggerFor[T interface{ setLogger(*slog.Logger) }]` helper would trade each 3-line `WithLogger` for a 2-line `setLogger` method — no net reduction in lines, and it adds coupling via an `internal/optutil` import across 6 packages.
- Code generation (e.g., go generate templates) would eliminate the repetition but adds a mandatory build step; rejected per Vanilla Principle (only standard/widely-adopted tools).
- The 6 `WithLogger` definitions are each 3 lines, trivially readable, and never drift independently — the logger field and its assignment are the same pattern across all packages.
- The per-package `Option` types differ materially beyond `WithLogger` (each package has its own set of option constructors); a shared base would not unify those.
- Closed without code changes.
