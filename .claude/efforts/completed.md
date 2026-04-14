# Completed Efforts Archive

Summary of all completed efforts and their key decisions.

---

## Discovery & Design (efforts 00-02)

Initial discovery, system design, and project structure. Established the core architecture:

- Single Go binary, three gRPC services (Schema, Config, Audit)
- PostgreSQL (read/write pools) + Redis (cache + pub/sub)
- Specs-first: proto defines API, sqlc defines DB, `make generate` for both
- Multi-tenant with role-based access (superadmin/admin/user)
- Config versioning with delta storage, rollback, optimistic concurrency
- Apache 2.0 license

## Schema YAML (effort 04)

Schema import/export in YAML format:

- Syntax v1 with OAS-style constraint naming (minimum/maximum/pattern/enum)
- Full-replace semantics on import with checksum dedup
- Slug names enforced on schema and tenant names

## Instrumentation (effort 05)

Feature-flagged OpenTelemetry:

- Master switch: `OTEL_ENABLED`
- Trace flags: `OTEL_TRACES_GRPC`, `OTEL_TRACES_DB`, `OTEL_TRACES_REDIS`
- Metric flags: `OTEL_METRICS_GRPC`, `OTEL_METRICS_DB_POOL`, `OTEL_METRICS_CACHE`, `OTEL_METRICS_CONFIG`, `OTEL_METRICS_SCHEMA`
- slog trace correlation (trace_id/span_id in logs)
- OTel Collector + Jaeger in docker-compose for local dev

## SDKs (efforts 06, 06a-c)

Three Go SDK modules, each independently installable:

- **configclient** — runtime reads/writes, typed getters/setters, snapshots for consistent reads, GetForUpdate + Update for CAS
- **adminclient** — schema/tenant/audit/config admin ops, auto-pagination, SDK-level types (not proto)
- **configwatcher** — live `Value[T]` with generics, auto-reconnect, null support via `GetWithNull()`

Key decisions: SDKs return native Go types (not proto), `ErrTypeMismatch` for wrong-type getters, connection passed in (not owned).

## Field Validation (effort 09)

Three phases completed:

1. **Null support** — `optional string` → `TypedValue` oneof with native proto types (int64, double, string, bool, Timestamp, Duration, url, json). Null = absent TypedValue.
2. **Constraint validation** — factory + per-tenant cache. Validators: min/max, exclusiveMin/Max, minLength/maxLength, pattern, enum, URL validity, JSON Schema. Strict mode rejects unknown fields.
3. **OAS constraint extensions** — separated string length (minLength/maxLength) from numeric range (minimum/maximum). Added exclusiveMinimum/exclusiveMaximum. Constraint/type compatibility validated at schema creation.

Key decisions: DB stores strings (conversion at boundary), xxHash checksums stored in DB column, validators use nil-receiver pattern.

## Benchmarks (efforts 10, 10a-b)

Unit benchmarks (23) + E2E benchmarks (13):

- Validation: 3ns (no constraints) to 1.7µs (JSON Schema)
- Checksum (xxHash): ~50ns, 0 allocs
- Cache hit: 17ns, parallel 110ns
- YAML marshal: ~30µs for 5 fields
- E2E: sequential + parallel (RunParallel) + mixed workload + import scaling

## Documentation (effort 11)

Stack: protoc-gen-doc (API) + cobra/doc (CLI) + MkDocs Material (site) + pkg.go.dev (SDKs).

- All generators output markdown — portable, reviewable in PRs
- Generated docs committed to repo (browsable on GitHub)
- CI verifies `make docs` produces no diff
- 14 hand-written pages + 37 generated CLI pages + 1 API reference
- JSON Schema for schema YAML editor validation

## CI (effort 12)

GitHub Actions with three workflows:

- **ci.yml** — PR validation: lint (golangci-lint v2.8.0 + buf), tests, build, docs check, e2e
- **release.yml** — build + push images to ghcr.io on version tags (semver + latest)
- **main.yml** — bleeding edge :main images + docs deploy to GitHub Pages

Tool versions pinned: golangci-lint-action@v9, MkDocs Material 9.7.6. Single version for server + CLI (injected via ldflags from git tags).
