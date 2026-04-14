# Security Review

**Status:** Planning
**Started:** 2026-04-13

---

## Goal

Systematically review and harden OpenDecree against common security threats. Create tests to verify protections. Document security posture.

## Threat Areas to Investigate

### 1. Injection Attacks
- **SQL injection** — config values, field paths, schema names, tenant names are all user input that reaches the DB. sqlc generates parameterized queries, but need to verify no raw string interpolation exists.
- **gRPC metadata injection** — can a caller inject arbitrary metadata headers?
- **YAML injection** — schema import and config import accept YAML. Malicious YAML could exploit the parser (billion laughs, alias bombs).
- **JSON injection** — JSON field type values, JSON Schema constraints. Could malicious JSON cause issues?
- **Log injection** — user-controlled strings (subject, tenant name, field path) appear in logs. Could they inject newlines or control characters to forge log entries?

### 2. Authentication & Authorization
- **Tenant isolation** — verify non-superadmin can NEVER access another tenant's data (config, audit, locks). Need exhaustive tests.
- **Role escalation** — can a user escalate to admin/superadmin via crafted headers or JWT claims?
- **JWT validation** — expired tokens, malformed tokens, wrong issuer, algorithm confusion (RS256 vs none).
- **Metadata auth bypass** — when JWT is enabled, can metadata headers still be used?
- **Missing auth checks** — are there any service methods that don't enforce auth?

### 3. Input Validation
- **Field path traversal** — can dots in field paths cause unintended hierarchy traversal?
- **Oversized payloads** — very large YAML/JSON imports, very long field values, very many fields.
- **Unicode/encoding** — homoglyph attacks in field names, null bytes, RTL override characters.
- **Regex DoS (ReDoS)** — pattern constraints with user-supplied regex. Could a malicious regex cause CPU exhaustion?
- **JSON Schema DoS** — complex JSON Schema constraints that cause validation to hang.

### 4. Data Security
- **Sensitive field values** — fields marked `sensitive: true` should be handled carefully. Are they excluded from logs? From exports?
- **Audit log completeness** — can changes be made without audit entries?
- **Config export** — does export include values the caller shouldn't see?

### 5. Infrastructure
- **gRPC reflection** — is it enabled in production? Should it be disabled.
- **Rate limiting** — no rate limiting exists. Brute force, enumeration attacks.
- **Error messages** — do error responses leak internal details (stack traces, DB schema)?
- **TLS** — is TLS enforced or optional? Can connections be downgraded?
- **Redis/PG connections** — are they authenticated? Encrypted?

### 6. Supply Chain
- **Dependencies** — are there known vulnerabilities? `go mod vulnerability check`.
- **Docker images** — base image vulnerabilities, running as root.
- **CI/CD** — can workflows be hijacked? Are secrets properly scoped?

## Approach

### Phase 1: Audit (research)
- [ ] Review each threat area above
- [ ] Document findings: what's protected, what's not, what needs fixing
- [ ] Prioritize by severity and likelihood

### Phase 2: Fix Critical Issues
- [ ] Fix any injection vulnerabilities found
- [ ] Fix any auth bypass issues
- [ ] Fix any tenant isolation gaps

### Phase 3: Tests
- [ ] Add security-focused unit tests (auth bypass, injection, validation)
- [ ] Add security-focused e2e tests (tenant isolation, role enforcement)
- [ ] Add fuzzing for input parsing (YAML, JSON, field paths)

### Phase 4: Documentation
- [ ] Update SECURITY.md with security model description
- [ ] Document auth model and threat mitigations
- [ ] Add security section to CONTRIBUTING.md

## Known Concerns (from discussion)

1. **SQL injection via config values** — sqlc uses parameterized queries, but need to verify ALL queries, especially any hand-written SQL.
2. **YAML import** — Go's yaml.v3 is generally safe against billion laughs but should verify limits.
3. **Tenant isolation** — multi-tenant auth just added (#95). Need exhaustive cross-tenant access tests.
4. **ReDoS** — pattern constraints accept user-supplied regex. Need to evaluate timeout/complexity limits.
5. **Sensitive fields** — the `sensitive` flag exists on schema fields but unclear if it affects any behavior.
6. **Cache overflow (#107)** — three unbounded caches risk OOM:
   - **MemoryCache** (internal/cache/memory.go): unbounded `map`, TTL checked only on read (no sweep), no size limit.
   - **ValidatorCache** (internal/validation/cache.go): no TTL at all, grows with tenant count, only evicted on explicit schema update.
   - **Redis**: docker-compose and Helm deploy with no `maxmemory` or eviction policy — Redis will OOM instead of evicting.
   - Mitigations needed: max entry count or LRU for memory caches, background sweep for expired entries, Redis `maxmemory` + `allkeys-lru` in deployment configs.
