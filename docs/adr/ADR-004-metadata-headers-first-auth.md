# ADR-004: Metadata-Headers-First Authentication

**Date:** 2026-06-05
**Status:** Accepted
**Deciders:** OpenDecree maintainers

## Context

OpenDecree serves two very different deployment contexts:

1. **Internal / developer environments** — services on a private network or inside a Kubernetes cluster where callers are trusted and setting up a full JWT/JWKS infrastructure is unnecessary overhead
2. **Production / multi-tenant environments** — where callers must be cryptographically authenticated and tenant isolation must be enforced

A design that mandates JWT from day one creates friction for adopters who just want to integrate quickly in a trusted network. A design that defaults to no authentication at all provides no path toward production hardening. The auth mechanism must also work cleanly over gRPC, where the standard carrier is request metadata (headers).

## Decision

The default authentication mode uses gRPC metadata headers:

- `x-tenant-id` — identifies the calling tenant
- `x-role` — declares the caller's role (`superadmin`, `admin`, `viewer`)

In default mode these values are accepted as-is without cryptographic validation. The server applies RBAC via the Guard chain (`TenantScopeGuard`, `RolePolicyGuard`, `FieldLockGuard`) based on the declared values.

JWT/JWKS authentication is opt-in: when configured, an interceptor validates the bearer token against the configured JWKS endpoint and extracts tenant ID and role from token claims, overriding the metadata headers.

## Consequences

**Positive:**

- Zero-config for internal services and development environments — no key management, no JWKS endpoint required
- Progressive security model: operators can start with header-based auth on a trusted network and migrate to JWT for external-facing deployments without changing client code
- Clean integration with gRPC metadata conventions
- Auth logic is isolated in interceptors, keeping service business logic auth-agnostic

**Negative:**

- Default mode is **not safe** without a network trust boundary — any caller that can reach the gRPC port can claim any tenant ID or role; operators must be explicit about this in their deployment security model
- The distinction between "trusted network mode" and "validated JWT mode" must be clearly communicated in documentation to avoid misconfiguration in production
- Mixing metadata-header auth with JWT requires careful interceptor ordering to avoid header spoofing when JWT is enabled
