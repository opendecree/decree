# Security Review — Audit Findings

Status: in progress (audit-only pass; fixes tracked in spawned issues)
Owner: zeevdr
Issue: opendecree/decree#26
Audit date: 2026-04-28
Branch: `security-review-audit`

## Scope

Walks the threat model in `docs/development/threat-model.md` end-to-end.
Captures what is currently protected, what is not, and links each gap to
either a tracked fix issue or to a "by design" note.

This document is the deliverable for the *audit findings* portion of
issue #26. Fixes land in separate PRs; #26 stays open until the
referenced fix issues close (or are explicitly accepted as residual
risk).

## Methodology

Each threat-model category was investigated against the working tree at
commit `2dcc446` (decree main, 2026-04-28). Evidence is cited as
`file:line`. Severity uses the standard scale:

- **Critical** — exploitable now, production-blocking
- **High** — exploitable now, must fix before any external pilot
- **Medium** — defence-in-depth gap or partial mitigation
- **Low** — minor hardening / hygiene
- **Info** — observed and acceptable, recorded for reference

The decree project is alpha; "production-blocking" assumes the first
external deployment, not the alpha cluster.

## Summary

| # | Severity | Threat | Finding | Issue |
|---|----------|--------|---------|-------|
| 1 | Critical | Infra / Input | gRPC server has no `MaxRecvMsgSize` / `MaxSendMsgSize`; defaults apply | #212 |
| 2 | Critical | Infra | gRPC + gateway-to-gRPC traffic is plaintext by default; no TLS option in `server.Config`; Helm chart has no cert wiring; DB/Redis TLS not validated | #213 |
| 3 | High | Infra | No panic recovery interceptor — a panic in any handler crashes the server and may leak stack to client | #214 |
| 4 | High | Data | `sensitive: true` flag is stored but NOT honoured in audit log, Subscribe stream, ExportConfig, Redis cache, or validation error messages | #215 |
| 5 | High | Infra | No rate limiting at all (no per-tenant, per-method, or global limiter) | #216 ✓ |
| 6 | High | Input | Schema-complexity bounds missing — no max field count, no max schema-doc size, no JSON-Schema compilation timeout | #217 |
| 7 | High | Data | Audit log is append-only by convention only — no hash-chain, no UPDATE/DELETE constraint; admin mutations (schema/tenant) not audited | #218 |
| 9 | Medium | Auth | Metadata header values (`x-tenant-id`, `x-subject`, `x-role`) have no length / charset limits; tenant-resolve error leaks raw `%v` | #219 |
| 11 | Medium | Infra | Helm `values.yaml` ships empty `resources: {}`, no NetworkPolicy template, `imagePullPolicy: IfNotPresent` | #220 |
| 12+13 | Medium | Supply chain | Base images use floating tags; seven third-party GitHub actions use major/minor tags instead of commit SHAs | #221 |
| 14 | Medium | Input | Invalid regex in schema constraints is silently dropped at validator construction (`validator.go:121`) — schema author gets no error at import time | #222 |
| 18+20+19 | Low | Multi | CodeQL not required check; gRPC reflection unconditional; unknown-role error echoes claim | #223 |
| — | Test | Multi | Cross-cutting e2e security regression suite tying #212–#223 fixes together | #224 |
| — | Tracked | Supply chain | Release-artifact attestations (Docker + Go binaries) | #159 |
| — | Tracked | Auth | Role-based RPC policy + pluggable PermissionGuard | #205, #206 |
| — | By design | Auth | Metadata mode defaults to `superadmin` when no `x-role` header — see `memory/feedback_auth_defaults.md` | — |

The "Issue" column is populated when fix issues are spawned (next step).

## Findings detail

### 1. No gRPC message size limits — Critical

`internal/server/server.go:39-62` constructs a `grpc.NewServer` without
overriding `MaxRecvMsgSize` (4 MB default) or `MaxSendMsgSize`
(unbounded). Two consequences:

- **Memory exhaustion on send.** A bug or hostile schema producing a
  huge response can hold ~2 GB before failure.
- **Legitimate large schemas blocked.** Schemas approaching the 4 MB
  inbound default fail without a deliberate, documented limit.

Fix: set explicit, documented limits (e.g. 20 MB recv/send), and add
schema-side bounds (see finding 6).

### 2. TLS not enforced — Critical

- `internal/server/server.go:40` listens with `net.Listen("tcp", ...)`
  only; no `grpc.Creds(...)` option.
- `internal/server/gateway.go:45-48` dials the gRPC service with
  `grpc.WithTransportCredentials(insecure.NewCredentials())`.
- `cmd/server/main.go` and the `server.Config` struct expose no TLS
  fields.
- `deploy/helm/decree/values.yaml` has no TLS section, no cert secret
  mount.

In Kubernetes east-west traffic this means JWT bearer tokens and
config payloads transit the cluster network unencrypted. Fix: TLS
config (cert/key from secrets), enforced by default with explicit opt-out
for local dev; mTLS support; Helm wiring.

### 3. No panic recovery interceptor — High

`internal/server/server.go` registers no `grpc_recovery` interceptor.
Any panic in a handler propagates: gRPC will close the stream with
`Internal` and may include the panic message in the status, while the
process state is left depending on what was holding locks/transactions
at the moment of panic. Add a recovery interceptor that logs and
returns a generic `codes.Internal` to the client.

### 4. Sensitive flag is not honoured — High

The `sensitive: true` schema field flag is stored at every layer
(`proto/centralconfig/v1/types.proto`, `internal/storage/domain/types.go`,
`internal/schema/yaml.go`, `internal/schema/store.go`,
`internal/schema/store_pg.go`, `internal/schema/store_memory.go`,
`internal/schema/convert.go`) but no runtime path consults it:

- **Audit log** — `db/migrations/001_initial_schema.sql:114-115` defines
  `old_value`/`new_value` as `TEXT`. `internal/config/service.go`
  writes these verbatim in every transactional `InsertAuditWriteLog`
  call (lines 368-369, 480-481, 943-944).
- **Subscribe stream** — `internal/config/service.go:737-746` populates
  `ConfigChange.old_value` / `new_value` without redaction.
- **ExportConfig** — `internal/config/service.go:756-822` returns full
  YAML.
- **Redis cache** — `internal/cache/redis.go:44-59` `HSet`s the entire
  values map.
- **Validation error messages** — `internal/validation/validator.go:126,
  162, 77` echo the rejected value via `%q` for pattern, enum, and URL
  failures.

The threat model lists this as "unclear if it affects behavior"; this
audit confirms it does not. Fix: route audit/subscribe/export/cache/
validation-error paths through a sensitive-aware redaction helper that
takes the schema field definition.

### 5. No rate limiting — High ✓ RESOLVED (#216)

Implemented in `internal/ratelimit/`. Per-tenant + per-method in-process
token-bucket limiter via `golang.org/x/time/rate`. Health check exempt.
Returns `codes.ResourceExhausted` + `RetryInfo` detail. `Limiter` interface
allows future Redis-backed replacement. `OTEL_METRICS_RATE_LIMIT` counter
for observability.

### 6. Schema-complexity bounds missing — High

- No max field count per schema (`internal/schema/service.go` →
  `ImportSchema`).
- No max schema-document size.
- `internal/validation/json_schema.go:16-42` calls
  `jsonschema.Compiler.Compile` with no context, no timeout, no size
  cap. The compiled schema is cached per tenant×schema in
  `internal/validation/cache.go`, so successful compilation is amortised
  — but the *first* import of a malicious schema can hang or OOM.
- Cyclic `$ref`, exponential `allOf`/`anyOf`, and million-element
  `enum` are all currently accepted.

Fix: cap field count (e.g. 10 000), schema-document bytes (e.g. 5 MB),
and wrap compilation in a context deadline (e.g. 5 s).

All four bounds shipped:

- Field count + doc bytes via `schema.Limits` (`internal/schema/limits.go`),
  configurable through `schema.WithLimits` and env vars `SCHEMA_MAX_FIELDS` /
  `SCHEMA_MAX_DOC_BYTES`.
- Compile timeout + structural depth scan via `validation.Limits`
  (`internal/validation/limits.go`), configurable through
  `validation.WithLimits` and env vars `SCHEMA_COMPILE_TIMEOUT` /
  `SCHEMA_MAX_REF_DEPTH`. Because `jsonschema/v6` has no `CompileContext`,
  the timeout is a goroutine-level wrapper — the underlying compile may
  continue past the deadline, but the depth pre-scan and upstream doc-byte
  cap bound the worst-case work.

### 7. Audit log not tamper-evident — High

`db/migrations/001_initial_schema.sql:106-118` defines
`audit_write_log` with no triggers preventing UPDATE/DELETE, no
hash-chain column, no checksum. A DBA or compromised app credential
can rewrite history without detection. The audit table is
transactional with the change (good — finding #2 of the threat model
is satisfied for *completeness*) but not for *integrity*.

Fix: add `previous_hash` + `entry_hash` columns linking entries
chronologically, and a database trigger rejecting UPDATE/DELETE on
rows older than N seconds. (Alpha-stage acceptable approach: hash
chain in Go before insert; trigger lockout in a follow-up.)

### 8. Schema/tenant mutations not audited — Medium

`internal/schema/service.go` handlers for `CreateSchema`,
`UpdateSchema`, `PublishSchema`, `DeleteSchema`, `CreateTenant`,
`UpdateTenant`, `DeleteTenant`, `LockField`, `UnlockField` do not call
`InsertAuditWriteLog` or any equivalent. These are admin operations
with high blast radius; their absence from the audit trail is a real
gap, not a design choice. Fix: extend audit schema or add a parallel
`audit_admin_log` table; write entries from the schema service.

### 9. Metadata header values unbounded — Medium

`internal/auth/metadata.go:90-104` reads
`x-tenant-id` (comma-separated, no length cap), `x-subject` (no
charset/format check), `x-role` (no length check) directly from gRPC
metadata. A 100 MB header value would be parsed and held in memory by
the interceptor before any handler is reached.

Fix: cap each header at a small limit (e.g. 1 KB total per header,
≤32 tenant IDs, subject must match a sane charset). Memory mode is
non-production but this is cheap to enforce.

### 10. DB/Redis TLS not enforced — Medium

- `internal/storage/postgres.go:50-51` parses the DSN with
  `pgxpool.ParseConfig`; whether `sslmode=require` is set depends
  entirely on the operator's environment variable.
- `cmd/server/main.go:120` parses the Redis URL with
  `redis.ParseURL`; same story for `?tls=true`.
- `deploy/helm/decree/values.yaml:38-55` exposes `database.writeUrl`
  and `redis.url` as plaintext fields with `existingSecret` *optional*.

Fix: at config-load time, fail fast if the DSN does not contain
`sslmode=require` (or `sslmode=verify-full`), unless an explicit
`ALLOW_INSECURE_DB=1` opt-out is set; same for Redis. Helm: document
that production deployments must use `existingSecret`.

### 11. Helm resource limits empty — Medium

`deploy/helm/decree/values.yaml:100-106` ships `resources: {}` with
the limits commented out. A pod with no CPU/memory limit can starve
its node.

Fix: ship sane defaults (e.g. `requests: {cpu: 100m, memory: 128Mi}`,
`limits: {cpu: 1, memory: 512Mi}`) and document override.

### 12. Base images not digest-pinned — Medium

- `build/Dockerfile.decree:27` and `build/Dockerfile:20` use
  `gcr.io/distroless/static-debian12:nonroot` (floating tag).
- Build stages use `golang:1.26-bookworm` (floating).

Pin to a specific digest (`@sha256:...`) and let Dependabot bump it.

### 13. Third-party actions not SHA-pinned — Medium

`.github/workflows/*.yml` reference `dorny/paths-filter@v4`,
`golangci/golangci-lint@v9`, `golang/govulncheck@v1`,
`bufbuild/buf@v1`, `goreleaser/goreleaser@v7`, etc. by major/minor
tag. CISA recommends commit-SHA pinning for any action outside a
verified-creator org. Dependabot can manage SHAs the same way.

### 14. Invalid regex silently dropped — Medium

`internal/validation/validator.go:121`:

```go
re, err := regexp.Compile(*constraints.Regex)
if err == nil {
    v.checks = append(v.checks, ...)
}
```

A schema author who writes a bad pattern gets no error, no log, and
the constraint silently does nothing. Fix: validate every regex at
schema-import time (`internal/schema/validate_constraints.go`) and
return `InvalidArgument` with the field path.

### 15. Tenant-resolve error includes raw `%v` — Low

`internal/auth/metadata.go:99`:

```go
return nil, status.Errorf(codes.InvalidArgument,
    "failed to resolve tenant %q: %v", id, err)
```

If the resolver wraps a database error, the client sees DB error text.
Fix: log full error server-side, return generic message to client.

### 16. No NetworkPolicy template — Low

`deploy/helm/decree/templates/` has no NetworkPolicy. In a multi-tenant
cluster the pod can reach arbitrary endpoints. Fix: optional
NetworkPolicy template gated by a values flag.

### 17. `imagePullPolicy: IfNotPresent` — Low

`deploy/helm/decree/values.yaml:8`. Means a re-rolled image at the
same tag is not picked up without an image SHA change. For floating
tags (which we should not be using anyway — see finding 12) this
prevents security-patch propagation. Fix: default to `Always` for
non-pinned tags, or document that production deployments must pin by
digest.

### 18. CodeQL not a required check — Low

CodeQL is configured (advanced setup, per memory `feedback_…`) and
runs on PR, but is not in the branch-protection required-checks list.
Fix: add `CodeQL / Analyze` to the required checks set.

### 19. Unknown role echoed — Low

`internal/auth/jwt.go:147`:

```go
return nil, status.Errorf(codes.PermissionDenied, "unknown role: %s", claims.Role)
```

Echoes whatever the JWT claim said. Mostly cosmetic, but fits the
"don't echo input" rule. Fix: log full string, return
`"unknown role"`.

### 20. gRPC reflection always registered — Low

`internal/server/server.go:55` always calls
`reflection.Register(grpcServer)`. Reflection is *not* in `skipAuth`
(verified at `internal/auth/metadata.go:15-17` and corresponding JWT
path), so a caller must authenticate before listing services. Still:
in a hardened production deployment reflection should be off.

Fix: gate behind a config flag, defaulting off in production builds /
Helm values.

## Verified safe (no fix needed)

The following items from the threat model were investigated and found
to be correctly protected at the audit date:

- **SQL injection** — every query in `db/queries/*.sql` and
  `internal/storage/dbstore/*.gen.go` uses sqlc-generated parameterised
  statements. No `fmt.Sprintf` building SQL, no dynamic table names, no
  raw `db.Query`/`db.Exec` callers in non-generated code.
- **Tenant isolation across all RPCs** — every tenant-scoped handler
  in `internal/config/service.go`, `internal/schema/service.go`, and
  `internal/audit/service.go` calls `auth.CheckTenantAccess` (or
  filters via `AllowedTenantIDs` for list endpoints). Issue #207's
  audit-handler fix is in place at lines 72/121/174/217.
- **Subscribe (streaming RPC) tenant isolation** —
  `internal/config/service.go:705` calls `CheckTenantAccess` *before*
  delegating to `pubsub.Subscribe`. Redis channel naming
  (`internal/pubsub/redis.go:12,29,52`) is per-tenant.
- **JWT algorithm allowlist** — `internal/auth/jwt.go:129` uses
  `jwt.WithValidMethods([]string{"RS256", "ES256"})`; `alg=none` and
  `alg=HS256` are rejected. Test coverage in `jwt_test.go`.
- **JWT vs metadata mutual exclusion** — `cmd/server/main.go:154-166`
  picks one interceptor at startup; the other path cannot be reached.
- **Auth interceptor coverage** — every non-`skipAuth` RPC method
  passes through the interceptor (`internal/server/server.go:48-49`).
  Disabled services (`ENABLE_SERVICES`) are not registered at all.
- **Field-path grammar** —
  `internal/schema/yaml.go:20-28` enforces
  `^[a-zA-Z_][a-zA-Z0-9_.-]*$`. No traversal possible by grammar.
- **Field-path prefix overlap** —
  `internal/schema/validate_constraints.go:21-36` rejects strict
  prefixes.
- **DependentRequired** — fully implemented and enforced
  (`internal/schema/dependent_required.go`,
  `internal/config/service.go`).
- **ReDoS** — Go's `regexp` package is RE2-based; user-supplied
  patterns cannot cause catastrophic backtracking.
- **Pagination** — `internal/pagination/pagination.go:18-49` clamps
  page size to `[1, max]`, rejects negative offsets, validates token
  format.
- **PostgreSQL NUL byte rejection** — pgx surfaces NUL bytes in
  strings as a driver error; no handling needed in app code.
- **Identifier charsets** — UUIDs are hex+dash;
  `internal/schema/yaml.go:168` constrains tenant slugs to
  `lowercase alphanumeric + hyphens, 1-63 chars`. Homoglyph and RTL
  attacks not possible.
- **Pod security context** — `deploy/helm/decree/values.yaml:118-127`
  sets `runAsNonRoot: true`, `runAsUser: 65534`,
  `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true`,
  `capabilities.drop: [ALL]`.
- **Distroless final image** — `build/Dockerfile.decree:27` and
  `build/Dockerfile:20` use `gcr.io/distroless/static-debian12:nonroot`
  with explicit `USER nonroot:nonroot`.
- **`.dockerignore`** — excludes `.env*`, git, CI, IDE, docs, deploy/.
- **Go dependencies** — minimal, all from reputable sources; no
  unusual recent additions. `govulncheck` and `dependency-review`
  active in CI.
- **YAML parser** — `gopkg.in/yaml.v3` is not vulnerable to billion-
  laughs by design (alias depth bounded). Both
  `internal/config/yaml.go:251` and `internal/schema/yaml.go:626` use
  the standard `Unmarshal`. (Size limits still recommended — finding 6.)
- **Structured logging** — server code uses `slog` key-value calls
  throughout; no fmt-string interpolation of user data into log lines.
- **Error messages do not leak cross-tenant data** — service-layer
  errors are generic (`"tenant not found"`, `"failed to resolve
  tenant"`); cross-tenant content is never reflected to a non-superadmin
  caller.
- **ExportConfig scoping** — `internal/config/service.go:756-822`
  resolves tenant ID, calls `CheckTenantAccess`, then queries scoped
  to that tenant. Same for the dump CLI which calls through the RPC.

## Out-of-scope / tracked elsewhere

- **Release artifact attestations** — already tracked at #159 (P0).
- **Role-based RPC policy** — #205 implemented (feat/role-rpc-policy-205):
  `RequireSuperAdmin` + `RequireAdminOrAbove` helpers in `internal/auth/access.go`,
  enforcement in schema and config service handlers, unit tests in
  `role_policy_test.go`, e2e matrix updated. #206 (pluggable Guard interface)
  deferred until #205 merges.
- **Audit cross-tenant visibility for superadmin** — verified correct
  per the #207 fix (commit `fa1bcb9`).
- **CEL validation security** — engine not yet implemented; design
  doc `.agents/context/cel-validation.md` already covers
  cost/timeout/typed-env. Re-audit when Phase 2 lands.

## Next steps

1. Fix issues #212–#224 are open in the Security Review milestone,
   each with its own Tests section. They land in their own PRs in
   priority order (P0 → P1 → P2).
2. Update `SECURITY.md` with the new "what is and is not enforced
   today" matrix derived from the *Verified safe* and *Findings*
   sections — best done as one of the last fixes lands so the
   document reflects shipped state.
3. Close #26 once all P0/P1 items are resolved or explicitly accepted
   as residual risk.
