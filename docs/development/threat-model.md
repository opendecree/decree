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
