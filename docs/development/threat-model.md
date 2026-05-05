# Threat Model

## 1. Injection Attacks
- **SQL injection** — sqlc generates parameterized queries, verify no raw interpolation
- **gRPC metadata injection** — can caller inject arbitrary headers?
- **YAML injection** — billion laughs, alias bombs in schema/config import
- **JSON injection** — JSON field values, JSON Schema constraints
- **Log injection** — user strings in logs, newlines/control chars

## 2. Authentication & Authorization
- **Tenant isolation** — non-superadmin must NEVER access another tenant's data
- **Role escalation** — crafted headers or JWT claims
- **JWT validation** — expired, malformed, wrong issuer, algorithm confusion
- **Metadata auth bypass** — when JWT enabled, can metadata headers still work?
- **Missing auth checks** — any methods without enforcement?

## 3. Input Validation
- **Field path traversal** — dots in paths causing hierarchy issues
- **Oversized payloads** — large YAML/JSON, long values, many fields
- **Unicode/encoding** — homoglyphs, null bytes, RTL override
- **Regex DoS (ReDoS)** — user-supplied regex in pattern constraints
- **JSON Schema DoS** — complex schemas causing validation hangs

## 4. Data Security
- **Sensitive fields** — `sensitive: true` behavior in logs, exports
- **Audit completeness** — can changes bypass audit?
- **Config export** — unauthorized value exposure

## 5. Audit Chain Trust Model

The tamper-evident audit chain (migration 002) provides the following guarantees:

### What it protects against
- **Post-write row mutation**: The DB trigger (`trg_audit_write_log_immutable`) rejects any UPDATE or DELETE on `audit_write_log` rows older than 60 seconds. A 60-second grace window allows test teardown.
- **Silent history rewriting**: Each row stores a SHA-256 hash chaining it to the previous entry for the same tenant. Tampering with any row's content will break the chain, detectable by `decree audit verify`.

### What it does NOT protect against
- **DB superuser access without trigger**: A `SECURITY DEFINER` function or `pg_dumpall + restore` can bypass the trigger. The trigger is a deterrent, not a cryptographic guarantee.
- **Hash chain collision**: SHA-256 is used; collision attacks are not a practical concern but the chain is not post-quantum.
- **Log loss before insert**: If the app crashes after committing a config change but before the audit insert, the change is unlogged. Config changes use a single transaction that includes the audit insert, so this window is narrow.
- **Audit chain verification key management**: The chain uses no HMAC key — it relies on hash chaining alone. A compromised app credential can insert plausible-looking fake entries (correct chain, wrong content). A HMAC-keyed chain would address this but is out of scope for alpha.

### Coverage
- **Config mutations** (`set_field`, `rollback`, `import`): audited transactionally via `config.Store.InsertAuditWriteLog`.
- **Schema mutations** (`create_schema`, `update_schema`, `delete_schema`, `publish_schema`): audited in the global (tenant_id=NULL) chain transactionally.
- **Tenant mutations** (`create_tenant`, `update_tenant`, `delete_tenant`): audited in the per-tenant chain transactionally.
- **Field lock mutations** (`lock_field`, `unlock_field`): audited in the per-tenant chain transactionally.

### Verification
Run `decree audit verify --tenant <id>` to walk a tenant's chain and report any breaks.
The global schema-level chain is verified with `decree audit verify` (no --tenant flag).

## 5. Infrastructure
- **gRPC reflection** — enabled in production?
- **Rate limiting** — none exists
- **Error messages** — stack traces, DB schema leaks
- **TLS** — enforced or optional?
- **Redis/PG connections** — authenticated? encrypted?

## 6. Supply Chain
- **Dependencies** — known vulnerabilities
- **Docker images** — base image CVEs, running as root
- **CI/CD** — workflow hijacking, secret scoping

## Known Concerns

1. sqlc parameterized queries — verify ALL queries
2. Go yaml.v3 generally safe against billion laughs — verify limits
3. Multi-tenant auth just added — need exhaustive cross-tenant tests
4. ReDoS — user-supplied regex needs timeout/complexity limits
5. Sensitive flag exists but unclear if it affects behavior
6. Cache overflow (#107) — fixed with bounded caches + Redis maxmemory
