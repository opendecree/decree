# ADR-005: Multi-Tenant Isolation via Row-Level Security

**Date:** 2026-06-05
**Status:** Accepted
**Deciders:** OpenDecree maintainers

## Context

OpenDecree is a multi-tenant service where all tenants share a single PostgreSQL database. Tenant data (config values, schema definitions, audit entries) must be strictly isolated — one tenant must never be able to read or modify another tenant's data.

Isolation can be enforced at multiple layers:

- **Application layer** — every query includes a `WHERE tenant_id = $1` predicate
- **Database layer** — PostgreSQL Row-Level Security (RLS) policies prevent rows from being accessed by the wrong session role
- **Schema-per-tenant / database-per-tenant** — stronger isolation but significant operational overhead

Application-layer-only enforcement is fragile: a missing `WHERE` clause or a future query that skips the filter silently leaks cross-tenant data. Relying solely on application logic also means that direct DB access (e.g., during incident response or by a future admin tool) bypasses the isolation guarantee.

## Decision

All tables that store tenant-scoped data include a `tenant_id` column. PostgreSQL Row-Level Security policies are enabled on those tables and restrict each session to rows matching the session's `app.tenant_id` setting.

The Go application sets `SET LOCAL app.tenant_id = $1` at the start of each request transaction (extracted from the authenticated gRPC metadata). RLS policies then enforce isolation at the database engine level, independent of the application query logic.

Application queries still include `tenant_id` filters where appropriate for index efficiency, but RLS provides a defense-in-depth backstop.

## Consequences

**Positive:**

- Defense in depth: tenant isolation is enforced at the database engine level, not only in application code
- A missing application-level `WHERE tenant_id = ...` clause results in an empty result set (RLS filters it out) rather than a data leak
- Direct database access by operators or admin tooling respects the same isolation boundary when session variables are set correctly
- No per-tenant schema or database management overhead — all tenants share the same schema

**Negative:**

- RLS policies add a small per-query overhead as PostgreSQL evaluates the policy predicate on every row access
- Query design must account for RLS: some bulk admin operations that span tenants require a dedicated privileged role that bypasses RLS (e.g., a `decree_admin` role used only for migrations and superadmin queries)
- Developers unfamiliar with RLS may find debugging unexpected empty results non-obvious if the session variable is not set
- `sqlc`-generated queries must be reviewed to ensure they are compatible with RLS and do not inadvertently rely on bypassing it
