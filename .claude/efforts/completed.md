# Completed Efforts Archive

Summary of all completed efforts and their key decisions.

---

## Discovery & Design (efforts 00-02)

Core architecture: single Go binary, three gRPC services (Schema, Config, Audit), PostgreSQL (read/write pools) + Redis (cache + pub/sub), specs-first (proto + sqlc), multi-tenant with RBAC, config versioning with delta storage, Apache 2.0.

## Implementation (effort 03)

All core services, auth, validation, OTel, SDKs, CLI, benchmarks, docs, CI. Key decisions: Go 1.24, JWT opt-in, atomic config writes (RunInTx), slug names, TypedValue oneof, strict mode, xxHash checksums, config import modes (merge/replace/defaults), single version number.

## Schema YAML (effort 04)

Syntax v1 with OAS-style constraint naming, full-replace semantics on import, checksum dedup, slug enforcement.

## Instrumentation (effort 05)

Feature-flagged OpenTelemetry: master switch + per-feature trace/metric flags, slog trace correlation, OTel Collector + Jaeger in docker-compose.

## Go SDKs (efforts 06, 06a-c)

Three independently installable modules: configclient (typed reads/writes, snapshots, CAS), adminclient (schema/tenant/audit, auto-pagination), configwatcher (live Value[T] with generics, auto-reconnect). SDKs return native Go types, connection passed in (not owned).

## Field Validation (effort 09)

Null support (TypedValue oneof), constraint validation (factory + per-tenant cache), OAS constraint extensions (exclusiveMin/Max, minLength/maxLength). DB stores strings, xxHash checksums.

## Benchmarks (efforts 10, 10a-b)

Unit (23) + E2E (13): validation 3ns–1.7µs, checksum ~50ns, cache hit 17ns, YAML ~30µs.

## Documentation (effort 11)

protoc-gen-doc (API) + cobra/doc (CLI) + MkDocs Material (site). All markdown, committed, CI-verified. JSON Schema for YAML editor validation.

## CI (effort 12)

Three workflows: ci.yml (PR validation), release.yml (ghcr.io images + release notes with apidiff), main.yml (bleeding edge images + docs deploy). Automated release notes with proto breaking change detection.

## CLI Phase 3 (effort 08)

31 commands: 6 groups (schema, tenant, config, watch, lock, audit) + 5 power tools (diff, docgen, validate, seed, dump) as reusable Go packages in sdk/tools. All packages 90%+ coverage. Offline tools have zero gRPC/proto deps.

## Test Coverage (effort 14)

Raised coverage across all public modules: tools 93-100%, configwatcher 61→91%, adminclient 36→89%, configclient 58→82%, CLI 58→82%. Coverage ratchet prevents regression. Internal 44% (cache/pubsub need Redis mocks).

## In-Memory Storage (effort 15)

In-memory backends for all pluggable interfaces: ConfigCache, Publisher/Subscriber, audit/schema/config Stores. Enables tests without Docker. sync.RWMutex, no external deps, auto-incrementing IDs. Coverage +8%.

## Go Public (effort 13 — partial)

Completed: secret scan (clean), LICENSE verified, README review + badges, module paths confirmed, git history cleaned, GitHub settings, repo flipped to public, v0.1.0 tagged (all submodules), branch protection, issue templates, SECURITY.md, Code of Conduct, GitHub Discussions, CI fixed. Remaining: pre-launch items (see effort 13).

## REST/HTTP Gateway (effort 16)

grpc-gateway embedded in server binary. google.api.http annotations on all 32 RPCs. Swagger UI at /docs. OpenAPI spec generated and embedded. Opt-in via HTTP_PORT env var. Auth headers forwarded from HTTP to gRPC metadata. 94.9% server coverage.

## Schema YAML Enrichment (effort 20)

OAS-inspired metadata: schema-level info block (title, author, contact, labels), field-level metadata (title, example, examples, externalDocs, tags, format, readOnly, writeOnce, sensitive). 4 new proto messages, all optional, backward compatible.

## Additional Items (v0.3.0 cycle)

- **BSR proto publishing** — buf.build/opendecree/decree, auto-push on release tags
- **In-memory storage wiring** — STORAGE_BACKEND=memory, zero-dep evaluation mode, validator store adapter
- **GitHub Project** — roadmap board with issues from efforts, CI auto-add workflow
- **Shell completion** — bash/zsh/fish/powershell via cobra, flag value hints for --output/--role/--mode
- **Helm chart** — deploy/helm/decree with full env var support, secrets, ingress, OTel, health probes
- **Goreleaser** — cross-platform binaries (server: linux/mac, CLI: linux/mac/windows, amd64/arm64)
- **Man pages** — 43 pages via cobra/doc, Long descriptions for parent commands
- **Docker layer caching** — GHA cache for main.yml and release.yml image builds
- **configclient retry** — generic retry[T] with exponential backoff + jitter, opt-in via WithRetry()

## Python SDK (effort 18)

`opendecree` on PyPI (v0.1.0). Separate repo `opendecree/decree-python`. ConfigClient (sync + async) with @overload typed get(), ConfigWatcher with WatchedField[T] (.value, on_change, changes()). Error hierarchy, retry with backoff, auth metadata, version compatibility. 171 tests, 97% coverage, 95% floor. Docs, governance, OIDC publishing.

## TypeScript SDK (effort 23)

`@opendecree/sdk` on npm (v0.1.0). Separate repo `opendecree/decree-typescript`. ESM-only, async-only, Node 20+. ConfigClient with overloaded get() via runtime converters (Number, Boolean, String). ConfigWatcher with WatchedField<T> (EventEmitter, async iteration). Symbol.dispose support. @grpc/grpc-js + buf/ts-proto. Biome + vitest. 139 tests, 98% coverage, 95% floor. OIDC trusted publishing.

## Multi-Tenant Auth (effort 24)

Claims.TenantID (string) → Claims.TenantIDs ([]string). JWT: `tenant_ids` array. Metadata: comma-separated `x-tenant-id`. auth.CheckTenantAccess(ctx, tenantID) on all Schema + Config service methods. auth.AllowedTenantIDs(ctx) for ListTenants filtering pushed to store layer (SQL WHERE id = ANY) for correct pagination. No auth context = permissive (tests, internal calls).

## Multi-Language SDKs (effort 18-multi-lang)

Tracking effort for Python + TypeScript SDKs. Both shipped — see efforts 18 (Python) and 23 (TypeScript) above.

## Schema Enrichment Persistence (effort 20 follow-up)

Tags, title, example, examples, external_docs, format, read_only, write_once, sensitive persisted through full storage chain: DB migration, SQL queries, sqlc codegen, domain types, store params, PG/memory store adapters, service layer, proto conversion. Proto comments clarified name vs title semantics.

## Docs Diagrams (effort 26)

Phase 1 complete: replaced 5 ASCII diagrams with Mermaid. Phase 2 skipped (not needed). Phase 3 (nice-to-have) remains open (#104).

## Cache Overflow Fix (#107)

MemoryCache: bounded to 10k entries, evicts expired first then oldest, background sweep. ValidatorCache: bounded to 1k tenants, evicts oldest. Redis: docker-compose maxmemory 128mb + allkeys-lru.
