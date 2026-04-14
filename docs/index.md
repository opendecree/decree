# Central Config Service

**Schema-driven business configuration management for multi-tenant services.**

## What is this?

Central Config Service manages **business-oriented configuration** — approval rules, fee structures, settlement windows, feature parameters — the kind of config that lives between your infrastructure settings and your application code.

### How is this different?

| Category | Examples | Gap |
|----------|---------|-----|
| **Feature flags** | LaunchDarkly, ConfigCat, Flagsmith | Boolean/multivariate flags for releases — not structured business config with schemas |
| **Infra config** | etcd, Consul, Spring Cloud Config | Low-level KV stores — no typed schemas, validation, or multi-tenancy |
| **Cloud config** | AWS AppConfig, Azure App Config | Some validation, but vendor-locked, no schema registry, no gRPC streaming |

**Central Config Service** is the first open-source tool to combine:

- **Schema-first design** — define your config structure, types, and constraints before setting values
- **Native typed values** — integer, number, string, bool, timestamp, duration, url, json at the wire level
- **Constraint validation** — min/max, pattern, enum, JSON Schema enforced on every write
- **Multi-tenancy** — schemas applied to tenants with role-based access
- **Field-level locking** — prevent changes to specific fields
- **Real-time subscriptions** — gRPC streaming pushes changes instantly
- **Versioned configs** — every change creates a version; rollback to any previous state
- **Full audit trail** — who changed what, when, and why

## Quick Links

- [Getting Started](getting-started.md) — end-to-end walkthrough
- [Concepts](concepts/overview.md) — understand the mental model
- [API Reference](api/api-reference.md) — gRPC service and message definitions
- [SDKs](sdk.md) — Go client libraries
- [CLI Reference](cli/ccs.md) — `ccs` command-line tool
- [Server Operations](server/configuration.md) — deployment and configuration