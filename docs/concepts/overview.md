# Overview

Central Config Service (CCS) manages **business-oriented configuration** for multi-tenant services. Think approval rules, fee structures, settlement windows, feature parameters -- the kind of config that sits between your infrastructure settings and your application code.

## Mental Model

CCS follows a four-step flow:

```
Schema  -->  Tenant  -->  Config  -->  Subscribe
(define)    (assign)     (write)      (consume)
```

1. **Schema** -- define what fields exist, their types, and constraints. Schemas are versioned and immutable once published.
2. **Tenant** -- create a consumer (an org, environment, or service instance) and pin it to a published schema version.
3. **Config** -- set typed values for the tenant. Every write creates a new version. Values are validated against the schema.
4. **Subscribe** -- applications consume config via typed reads or real-time streaming. Changes propagate instantly.

## When to Use CCS

CCS is a good fit when you need:

- **Typed, validated configuration** -- not just string key-value pairs, but integers, durations, URLs, JSON with constraints enforced on every write.
- **Multi-tenancy** -- different organizations or environments sharing the same schema but with independent config values.
- **Schema governance** -- a central definition of what config fields exist, reviewed and versioned like code.
- **Audit and rollback** -- full history of who changed what and when, with the ability to roll back to any previous state.
- **Real-time propagation** -- services that need to react to config changes without polling or restarting.

## When NOT to Use CCS

CCS is not the right tool for:

| Need | Better tool |
|------|-------------|
| Feature flags (on/off for % of users) | LaunchDarkly, Flagsmith, OpenFeature |
| Infrastructure config (ports, replicas, resource limits) | Kubernetes ConfigMaps, Helm values, etcd |
| Secrets management | Vault, AWS Secrets Manager |
| Application environment variables | `.env` files, Kubernetes Secrets |
| Transient state or session data | Redis, Memcached |

CCS occupies the space between infrastructure config and application logic -- structured business parameters that change at runtime, need validation, and must be auditable.

## Architecture at a Glance

CCS is a single Go binary exposing three gRPC services:

| Service | Purpose |
|---------|---------|
| **SchemaService** | Define and version configuration schemas, manage tenants |
| **ConfigService** | Read, write, and subscribe to typed config values |
| **AuditService** | Query change history and usage statistics |

PostgreSQL stores schemas, config values, and audit entries. Redis provides caching and real-time change propagation via pub/sub. Services can be selectively enabled via `ENABLE_SERVICES` for deployment flexibility.

## Core Concepts

| Concept | Page |
|---------|------|
| Schemas and fields | [Schemas & Fields](schemas-and-fields.md) |
| Tenants and multi-tenancy | [Tenants](tenants.md) |
| The type system | [Typed Values](typed-values.md) |
| Config versioning and rollback | [Versioning](versioning.md) |
| Authentication and authorization | [Auth](auth.md) |
| Real-time change streaming | [Subscriptions](subscriptions.md) |

## Next Steps

- [Getting Started](../getting-started.md) -- hands-on walkthrough from zero to working config
- [API Reference](../api/api-reference.md) -- full gRPC service and message definitions
- [CLI Reference](../cli/ccs.md) -- all `ccs` commands
- [SDKs](../sdk.md) -- Go client libraries
