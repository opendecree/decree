# Central Config Service — Discovery & Design

**Status:** Complete
**Started:** 2025-03-25

## Goals

Document the discovery and design process for the Central Config Service project.

### Topics to Cover

- [x] Project vision — what problem are we solving, for whom
- [x] Coding/deployment/testing principles — tools, languages, frameworks
- [x] System design — architecture, components, data flow → [01-system-design.md](01-system-design.md)
- [x] Project structure — directory layout, modules → [02-project-structure.md](02-project-structure.md)
- [x] Documentation — README for users and developers
- [x] Implementation — server wiring, services → [03-implementation.md](03-implementation.md)

---

## 1. Project Vision

### Problem

Existing config management tools (etcd, Consul, Spring Cloud Config) focus on **system/infrastructure configuration** — URLs, ports, feature flags. They fall short for **business-oriented configuration** — approval rules, settlement windows, fee structures, pinning configs — where you need:

- Typed, validated schemas with versioning and migration
- Multi-tenancy with per-tenant overrides and field-level access control
- Audit trails on every change
- Real-time change propagation to consuming services

### Solution

A standalone, open-source **business configuration management service** that provides:

- **Schema-driven config**: Define typed schemas (templates) with validation, apply them to tenants
- **Multi-tenancy**: Tenants get their own config instance from a schema, with role-based access
- **Versioned everything**: Schemas and configs are versioned. Every write creates a new config version. Rollback to any previous version.
- **Real-time subscriptions**: gRPC streaming for instant change notifications
- **Import/Export**: Schemas and configs are portable
- **Audit**: Full history of who changed what, when, with descriptions

### Non-goals (for now)

- UI (API-first, maybe UI layer later)
- Secrets management (may revisit)
- Cross-field validation (may revisit)

---

## 2. Data Model (Conceptual)

### Schema

A schema is a **template** that defines the structure of a configuration.

- Has a **name**, **description**, and **version** (with checksum, parent version, version description)
- Contains **fields** organized in an **arbitrary-depth hierarchy** (e.g., `payments.settlement.window`)
- Each field has:
  - **Type**: int, string, time, duration, URL, JSON
  - **Validation constraints**: min/max, regex, enum of allowed values, required/optional
  - **Deprecation status**: deprecated fields are read-only (not modifiable)
  - **Tenant-level lock**: superadmin can lock specific fields or enum values so tenant admins cannot modify them
- Supports **migration**: adding fields, renaming (with redirects), deprecating
- **Redirects**: if field X was renamed to Y, reads of X return Y's value
- **Exportable/importable**

### Tenant

- Has a **single schema** applied to it
- Has **config values** (an instance of the schema)

### Config (values)

- A set of **key-value pairs** conforming to the tenant's schema
- Each value can have a **description** explaining the specific value
- **Versioned**: every write (single field or batch) creates a new version
- Each version can have a **description** explaining the changes
- Clients can request a **specific version** for consistency within a flow
- **Rollback**: restore entire config to a previous version
- **Exportable/importable**
- **Audited**: who changed what, when

### Roles & Access

| Role | Schemas | Tenants | Config Values |
|------|---------|---------|---------------|
| SuperAdmin | Create, update, manage | Create, assign schema, lock fields | Full access |
| Admin | Read | — | Read + write (unlocked fields only) |
| User | — | — | Read only |

*(Role names and exact permissions to be refined later)*

---

## 3. Technical Decisions

### Principles

- **Vanilla**: standard, widely-adopted tools only. No exotic dependencies.
- **Specs-first**: protobuf defines the API, sqlc defines the queries. Code and docs are generated.
- **Open-source standalone**: no vendor lock-in, deployable anywhere.

### Stack

| Concern | Choice | Notes |
|---------|--------|-------|
| Language | **Go** | |
| API | **gRPC** (all endpoints) | Management + reads + subscriptions |
| API spec | **Protocol Buffers** | Source of truth, used for code + doc generation |
| Protobuf tooling | **buf** | Linting, breaking change detection, code gen, doc gen |
| Database | **PostgreSQL** | Primary storage for schemas, configs, audit |
| DB queries | **sqlc** | Generated type-safe Go from SQL |
| DB migrations | **goose** | |
| Cache + pub/sub | **Redis** | Caching reads + broadcasting changes across replicas |
| Auth | **JWT** validation | Bring-your-own IdP (OAuth2/OIDC). Most clients are services. |
| Testing | **testify** | Standard Go testing + testify for assertions |
| Deployment | **Kubernetes** | Target platform |

### Change Propagation Architecture

```
Client A (gRPC stream) ←── Service Instance 1 ←──┐
                                                   ├── Redis Pub/Sub
Client B (gRPC stream) ←── Service Instance 2 ←──┘
                                                   ↑
                            Service Instance 3 ────┘ (writer publishes change)
```

- Write lands on any instance → persists to PostgreSQL → publishes to Redis pub/sub
- All instances subscribe to Redis → push updates to connected gRPC streams
- Redis pub/sub layer abstracted behind an interface for future replacement

### High Availability

- Multiple service instances behind a load balancer
- PostgreSQL read replicas for read-heavy workload (transparent to app — separate read/write connection strings)
- Redis for cross-instance coordination
- Scale: up to a few thousand simultaneous read/write clients

### Subscriptions (gRPC Streaming)

- Subscribe to **entire tenant config** (any change)
- Subscribe to **specific fields**
- Pull (request with version → get current if newer) and push (stream) supported

### Client SDKs

- For v1: **no custom SDK** — generated gRPC stubs from proto files serve as the client library in any language
- May add thin convenience SDKs later (caching, reconnection logic)

---

## 4. Open Questions

- [ ] Exact role names and permission model
- [ ] Secrets management — include or keep separate?
- [ ] Cross-field validation — future scope, design TBD
- [ ] Schema migration mechanics — how redirects and deprecations work in detail
- [ ] Config version pinning — exact API semantics
- [ ] Project name — "Central Config Service" for now, may change
- [ ] Client SDK — whether convenience wrappers are needed beyond generated stubs

---

## Discovery Log

### Session 1 — 2025-03-25

**Covered**: Project vision, data model, technical stack, change propagation, HA strategy, subscriptions.

**Key decisions**:
1. Business-config focused — not a replacement for etcd/Consul, but complementary
2. Schema-driven with typed fields, validation, versioning, deprecation, redirects
3. Multi-tenant with field-level locking by superadmin
4. Every config write creates a version; rollback to any version; descriptions on versions
5. gRPC for everything (management + reads + subscriptions)
6. Specs-first: protobuf (buf) + sqlc + goose
7. PostgreSQL + Redis (cache + pub/sub, behind abstraction)
8. JWT auth, bring-your-own IdP
9. Kubernetes deployment target
10. Hierarchical field namespacing (arbitrary depth)

**Next**: System design (detailed architecture), project structure, protobuf definitions.

### Session 2 — 2025-03-25

**Covered**: Project scaffolding, iterative refinements.

**Key decisions**:
1. Descriptions added to schemas, schema versions (with parent version lineage), and individual config values
2. Config reads support `include_descriptions` flag — false (default) serves from cache, true bypasses cache for full data
3. Optimistic concurrency via `expected_checksum` on writes
4. JSON Schema validation for json-typed fields
5. Generated code committed to repo (not gitignored) for immediate `go build`
6. `.gitattributes` collapses generated files in GitHub PR diffs
7. sqlc output uses `.gen.go` suffix for clarity
8. All tool versions pinned in Dockerfile.tools
9. buf plugins run locally (not remote) for offline reproducibility
10. Nullable flag configurable per field
