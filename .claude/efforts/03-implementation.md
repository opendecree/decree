# Central Config Service — Implementation

**Status:** In Progress (Helm + CI remaining)
**Started:** 2025-03-25

---

## Progress

### Phase 1: Server Wiring (completed)

Core infrastructure — everything needed before implementing business logic.

- [x] **Database layer** (`internal/storage/postgres.go`) — pgxpool with separate read/write connection pools
- [x] **Cache interface + Redis impl** (`internal/cache/`) — `ConfigCache` interface, Redis hash-per-tenant-version
- [x] **Pub/Sub interface + Redis impl** (`internal/pubsub/`) — `Publisher` and `Subscriber` interfaces, Redis Pub/Sub with JSON events
- [x] **JWT auth interceptor** (`internal/auth/jwt.go`) — JWKS-based JWT validation, role extraction (superadmin/admin/user), unary + stream interceptors, health check bypass
- [x] **gRPC server** (`internal/server/server.go`) — listener, health checks, reflection, graceful shutdown, selective service registration via `--enable-services`
- [x] **Main wiring** (`cmd/server/main.go`) — connects all components: DB → Redis → cache → pubsub → auth → server → signal handling
- [x] **Context-aware logging** — all log calls use `slog.*Context(ctx, ...)` for future OTel trace correlation

### Phase 2: SchemaService (completed)

- [x] Schema CRUD (create with initial v1, get by version/latest, list, update creates new version, delete)
- [x] Schema versioning (publish, immutability, parent version lineage)
- [x] Schema field management (add, modify, remove via update)
- [x] Tenant CRUD (create with published schema validation, get, list, update name/version, delete)
- [x] Field locking (lock, unlock, list with JSON-encoded locked values)
- [x] Schema import/export (YAML) — OAS-style constraints, syntax v1, full-replace import with checksum dedup
- [x] Store interface with PostgreSQL implementation (read/write pool routing)
- [x] Proto <-> DB conversion helpers, deterministic checksum computation

### Phase 3: ConfigService (completed)

- [x] Config reads (GetConfig, GetField, GetFields) with Redis caching + `include_descriptions` bypass
- [x] Config writes (SetField, SetFields) with optimistic concurrency (checksum), field lock enforcement, audit logging, cache invalidation, pub/sub change events
- [x] Config versioning (ListVersions, GetVersion, RollbackToVersion — creates new version with target's values)
- [x] gRPC streaming subscriptions (Subscribe with field path filtering via Redis Pub/Sub)
- [x] Config import/export (YAML) — typed values (int→YAML int, bool→YAML bool, json→YAML map), schema-aware conversion, atomic import with audit
- [ ] Usage stats recording (async, batched) — deferred to AuditService
- [x] Store interface with PostgreSQL implementation (read/write pool routing)

### Phase 4: AuditService (completed)

- [x] Write log queries (QueryWriteLog with tenant/actor/field/time filters)
- [x] Usage statistics (GetFieldUsage — aggregated across periods, GetTenantUsage, GetUnusedFields)
- [x] Store interface with PostgreSQL implementation (read/write pool routing)
- [x] UpsertUsageStats available for future batched read tracking

### Unit Tests (completed)

- [x] Schema service tests — CreateSchema, GetSchema (latest/specific), UpdateSchema (versioning), PublishSchema, CreateTenant (requires published schema)
- [x] Schema YAML tests — roundtrip conversion, type mapping, constraint mapping (OAS↔proto), validation (6 cases), slug validation (6 invalid patterns)
- [x] Config service tests — GetConfig (cache hit/miss, include_descriptions bypass), SetField (success, checksum mismatch, locked field), GetField (not found), RollbackToVersion, ExportConfig, ImportConfig
- [x] Config YAML tests — roundtrip (6 types), typed value conversion, stringify value, validation (4 cases), description preservation
- [x] Convert helper tests — UUID roundtrip, checksum determinism/order-independence, field type roundtrip, ptrString
- [x] Mock stores using testify/mock for schema, config, cache, pubsub
- [x] `auth.ContextWithClaims()` helper for injecting claims in tests
- [x] Auth interceptor tests — 20 JWT tests: valid/expired/wrong-key tokens, health bypass, missing/bad auth headers, issuer validation, role validation, tenant_id enforcement, stream interceptor, ClaimsFromContext roundtrip
- [x] Metadata interceptor tests — 11 tests: valid superadmin, default role, admin/user with tenant, missing subject, no metadata, unknown role, admin/user missing tenant, health bypass, stream interceptor

### Phase 5: Polish

- [ ] Helm chart
- [x] E2E tests — split into 7 domain-specific files + bench file, own module
- [x] Lint cleanup — all golangci-lint issues fixed
- [x] Proto documentation — comprehensive field-level comments across all 4 proto files
- [x] OpenTelemetry integration — feature-flagged traces + metrics, slog trace correlation
- [x] TypedValue oneof — native proto types (integer, number, string, bool, timestamp, duration, url, json) with null support
- [x] Field validation — constraint validators, factory, per-tenant cache, strict mode, import validation, cache invalidation on UpdateTenant
- [x] SDKs — configclient (typed getters/setters, snapshots, CAS), adminclient (schema/tenant/audit), configwatcher (live values)
- [x] CLI tool — 26 commands, own module, unit tests
- [x] Benchmarks — unit + e2e benchmark framework
- [x] README + CONTRIBUTING updated
- [x] Modules separated — server, CLI, e2e, api, 3 SDKs (7 modules total)
- [x] Documentation infrastructure — protoc-gen-doc (API), cobra/doc (CLI), MkDocs Material, make docs targets
- [ ] CI (GitHub Actions)
- [ ] Helm chart

---

## Key Decisions During Implementation

1. **Go 1.24** — pinned to 1.24 for broader compatibility (otel v1.35, pgx v5.8, grpc v1.72)
2. **Tools image stays Go 1.25** — buf v1.66.1 requires it, but tools image is separate from the project's Go version
3. **JWT auth is optional** — if `JWT_JWKS_URL` is not set, auth is disabled (logged as warning). Useful for local dev.
4. **Context-aware logging** — `slog.*Context()` everywhere for future OTel trace ID correlation
5. **keyfunc v3** — uses context cancellation (not `End()`) for cleanup
6. **Makefile sentinel caching** — tools image only rebuilds when Dockerfile.tools changes; generate and lint-proto run in single containers
7. **Atomic config writes + audit** — `Store.RunInTx` callback wraps CreateConfigVersion + SetConfigValue + InsertAuditWriteLog in a single DB transaction. Side effects (cache invalidation, pub/sub) happen after commit. Audit failures now roll back the entire write (previously fire-and-forget).
8. **Slug names** — schema and tenant names enforced as slugs (`[a-z0-9]([a-z0-9-]*[a-z0-9])?`, 1-63 chars) at all entry points (CreateSchema, CreateTenant, UpdateTenant, YAML import)
9. **Schema YAML format** — syntax v1 with OAS-style constraint naming (minimum/maximum/pattern/enum). Import uses full-replace semantics with checksum dedup. Version in YAML is informational; server assigns next version.
10. **Field type system** — OAS-aligned: `integer` (INT), `number` (float), `string`, `bool`, `time`, `duration`, `url`, `json`. DB uses PG enum. Constraints min/max are `double` to support float ranges.
11. **Config YAML typed values** — export converts string values to native YAML types (int→number, bool→boolean, json→map) using schema field type info. Import reverses the conversion. Both directions require schema lookup.
12. **Lint zero issues** — fixed all pre-existing golangci-lint issues; gofumpt moved from linters to formatters-only in `.golangci.yml`.
13. **Metadata auth (default)** — JWT is opt-in (`JWT_JWKS_URL`). Default mode reads `x-subject` (required), `x-role` (defaults superadmin), `x-tenant-id` from gRPC metadata headers. Same `Claims` in context either way.
14. **Docker build cache** — `--mount=type=cache` on `/go/pkg/mod` and `/root/.cache/go-build` in both Dockerfiles. Tools image rebuild drops from ~220s to ~1s.
