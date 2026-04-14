# Central Config Service — System Design

**Status:** Complete
**Started:** 2025-03-25

---

## 1. High-Level Architecture

```
                        ┌─────────────────────┐
                        │   Load Balancer /    │
                        │   K8s Ingress        │
                        └──────────┬──────────┘
                                   │
                    ┌──────────────┼──────────────┐
                    │              │              │
              ┌─────▼─────┐ ┌─────▼─────┐ ┌─────▼─────┐
              │ Instance 1 │ │ Instance 2 │ │ Instance 3 │
              │            │ │            │ │            │
              │ Schema Svc │ │ Config Svc │ │ Config Svc │
              │ Config Svc │ │   (reads)  │ │   (reads)  │
              │ Audit Svc  │ │            │ │            │
              └──┬─────┬───┘ └──┬─────┬───┘ └──┬─────┬───┘
                 │     │        │     │        │     │
          ┌──────▼─┐ ┌─▼────────▼─┐ ┌─▼────────▼─┐  │
          │  PG    │ │   Redis    │ │  PG Read   │  │
          │ Primary│ │ Cache +    │ │  Replica   │  │
          │        │ │ Pub/Sub    │ │            │  │
          └────────┘ └────────────┘ └────────────┘  │
                                                     │
                                              (same Redis)
```

### Single Binary, Multiple Services

The system is a **single Go binary** exposing three gRPC services:

| Service | Responsibility | Throughput |
|---------|---------------|------------|
| **SchemaService** | Schema CRUD, versioning, migrations, tenant management, field locking | Low |
| **ConfigService** | Config read/write, subscriptions, versioning, rollback, import/export | High (especially reads) |
| **AuditService** | Change history queries, usage statistics | Medium |

### Deployment Flexibility

The binary accepts a flag to control which services are enabled:

```
--enable-services=schema,config,audit   # Full instance (management)
--enable-services=config                # Read-heavy instance (scaled out)
--enable-services=schema,audit          # Management-only instance
```

This allows different Kubernetes Deployments with different scaling policies from the same image.

---

## 2. gRPC Service Definitions

### SchemaService

```
SchemaService
├── CreateSchema(name, fields) → Schema
├── GetSchema(id, version?) → Schema
├── ListSchemas(filter?) → [Schema]
├── UpdateSchema(id, changes) → Schema (new version)
├── DeleteSchema(id) → confirmation
├── PublishSchema(id, version) → Schema (immutable once published)
│
├── CreateTenant(name, schema_id, schema_version) → Tenant
├── GetTenant(id) → Tenant
├── ListTenants(filter?) → [Tenant]
├── UpdateTenant(id, changes) → Tenant
├── DeleteTenant(id) → confirmation
│
├── LockField(schema_id, field_path, tenant_id?) → confirmation
├── UnlockField(schema_id, field_path, tenant_id?) → confirmation
│
├── ExportSchema(id, version?) → YAML bytes
└── ImportSchema(YAML bytes) → Schema
```

### ConfigService

```
ConfigService
├── GetConfig(tenant_id, version?) → Config (full config at version)
├── GetField(tenant_id, field_path, version?) → Value
├── GetFields(tenant_id, [field_paths], version?) → [Value]
│
├── SetField(tenant_id, field_path, value, expected_checksum?, description?) → ConfigVersion
├── SetFields(tenant_id, [{field_path, value, expected_checksum?}], description?) → ConfigVersion
│
├── ListVersions(tenant_id, pagination?) → [ConfigVersion]
├── GetVersion(tenant_id, version_number) → ConfigVersion (with description)
├── RollbackToVersion(tenant_id, version_number, description?) → ConfigVersion (creates new version)
│
├── Subscribe(tenant_id, field_paths?) → stream ConfigChange
│
├── ExportConfig(tenant_id, version?) → YAML bytes
└── ImportConfig(tenant_id, YAML bytes, description?) → ConfigVersion
```

### AuditService

```
AuditService
├── QueryWriteLog(tenant_id?, actor?, field_path?, time_range?, pagination?) → [AuditEntry]
├── GetFieldUsage(tenant_id, field_path, time_range?) → UsageStats
├── GetTenantUsage(tenant_id, time_range?) → [FieldUsageStats]
└── GetUnusedFields(tenant_id, since?) → [field_path]
```

---

## 3. Database Schema

### PostgreSQL Tables

```sql
-- Schema definitions
CREATE TABLE schemas (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT,                           -- human-readable schema description
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE schema_versions (
    id             UUID PRIMARY KEY,
    schema_id      UUID NOT NULL REFERENCES schemas(id),
    version        INT NOT NULL,
    parent_version INT,                         -- tracks version lineage (v3 derived from v2)
    description    TEXT,                        -- describes changes in this version
    checksum       TEXT NOT NULL,
    published      BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(schema_id, version)
);

-- Hierarchical fields with dot-notation paths
CREATE TABLE schema_fields (
    id              UUID PRIMARY KEY,
    schema_version_id UUID NOT NULL REFERENCES schema_versions(id),
    path            TEXT NOT NULL,              -- e.g., "payments.settlement.window"
    field_type      TEXT NOT NULL,              -- int, string, time, duration, url, json
    nullable        BOOLEAN NOT NULL DEFAULT false,
    constraints     JSONB,                      -- {min, max, regex, enum, json_schema}
    deprecated      BOOLEAN NOT NULL DEFAULT false,
    redirect_to     TEXT,                       -- field path to redirect reads to
    default_value   TEXT,                       -- default value if not set
    description     TEXT,
    UNIQUE(schema_version_id, path)
);

-- Tenants
CREATE TABLE tenants (
    id                  UUID PRIMARY KEY,
    name                TEXT NOT NULL UNIQUE,
    schema_id           UUID NOT NULL REFERENCES schemas(id),
    schema_version      INT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-tenant field locks (superadmin locks specific fields for a tenant)
CREATE TABLE tenant_field_locks (
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    field_path      TEXT NOT NULL,
    locked_values   JSONB,                     -- for enums: locked subset of allowed values
    PRIMARY KEY (tenant_id, field_path)
);

-- Config versions (every write creates a new version)
CREATE TABLE config_versions (
    id              UUID PRIMARY KEY,
    tenant_id       UUID NOT NULL REFERENCES tenants(id),
    version         INT NOT NULL,
    description     TEXT,
    created_by      TEXT NOT NULL,              -- actor from JWT sub claim
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, version)
);

-- Config values (snapshot per version — only changed fields stored, inherit from previous)
CREATE TABLE config_values (
    config_version_id UUID NOT NULL REFERENCES config_versions(id),
    field_path        TEXT NOT NULL,
    value             TEXT NOT NULL,
    description       TEXT,                    -- explains the specific value
    PRIMARY KEY (config_version_id, field_path)
);

-- Audit: write events
CREATE TABLE audit_write_log (
    id              UUID PRIMARY KEY,
    tenant_id       UUID NOT NULL,
    actor           TEXT NOT NULL,              -- "service:payment-gw" or "user:jane@acme.com"
    action          TEXT NOT NULL,              -- "set_field", "rollback", "import", etc.
    field_path      TEXT,
    old_value       TEXT,
    new_value       TEXT,
    config_version  INT,
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Audit: read usage aggregation
CREATE TABLE usage_stats (
    tenant_id       UUID NOT NULL,
    field_path      TEXT NOT NULL,
    period_start    TIMESTAMPTZ NOT NULL,       -- aggregation bucket (e.g., hourly)
    read_count      BIGINT NOT NULL DEFAULT 0,
    last_read_by    TEXT,                       -- last actor to read
    last_read_at    TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, field_path, period_start)
);
```

### Config Value Storage Strategy

Config versions use **delta storage** — only changed fields are stored per version. To resolve the full config at a version:

1. Walk backwards from the requested version
2. Collect the latest value for each field
3. Apply redirects for deprecated fields

This keeps storage efficient while supporting full-version reads and rollbacks.

---

## 4. Concurrency Control & Validation

### Optimistic Concurrency (Read-for-Update)

Writes support an optional `expected_checksum` field. The flow:

1. Client reads a field value (response includes a checksum of the current value)
2. Client modifies the value locally
3. Client writes back with `expected_checksum` set to the checksum from step 1
4. Server compares checksums — if mismatch, another write happened in between → reject with `ABORTED` / `CONFLICT`
5. Client retries from step 1

This is especially important for JSON fields where partial modifications are common. The checksum is optional — if omitted, the write is unconditional (last-write-wins).

### Field Validation

Validation is applied on every write, based on the field definition in the schema:

| Field type | Validations |
|-----------|-------------|
| int | min, max, enum |
| string | min_length, max_length, regex, enum |
| time | min, max |
| duration | min, max |
| url | valid URL format |
| json | JSON schema validation (schema stored in constraints) |

- **Nullable**: each field has a `nullable` flag. If false, null/empty writes are rejected.
- **JSON schema**: for `json`-typed fields, an optional JSON Schema can be provided in the field constraints. The value is validated against it on write.

### Validator Factory (planned)

A `ValidatorFactory` compiles and caches validators per schema version:

```
ValidatorFactory
├── GetValidator(schemaID, version, fieldPath) → FieldValidator (cached)
├── Invalidate(schemaID, version)              → evict cached validators
└── backed by schema store for cold lookups
```

- On first write to a field, the factory loads the field definition, compiles constraints (regex, JSON schema, etc.) into a `FieldValidator`, and caches it keyed by `(schemaID, version, fieldPath)`.
- Subsequent writes reuse the cached validator — no DB lookup or recompilation.
- On schema version change, the cache for that schema version is invalidated.
- JSON Schema validators are particularly expensive to compile, so caching matters most there.

---

## 5. Change Propagation

### Write Flow

```
Client (Admin)
  │
  ▼
ConfigService.SetField()
  │
  ├──1──▶ Validate against schema (type, constraints, locks)
  ├──2──▶ Write to PostgreSQL (new config_version + config_values)
  ├──3──▶ Write audit entry to audit_write_log
  ├──4──▶ Invalidate Redis cache for tenant
  └──5──▶ Publish change event to Redis Pub/Sub
                │
        ┌───────┼───────┐
        ▼       ▼       ▼
    Instance 1  Instance 2  Instance 3
        │       │       │
        ▼       ▼       ▼
    Push to connected gRPC stream subscribers
```

### Read Flow

Reads support an `include_descriptions` flag (default false):

**Without descriptions (default — cached, fast path):**
```
Client (User/Service)
  │
  ▼
ConfigService.GetConfig(include_descriptions=false)
  │
  ├──1──▶ Check Redis cache (tenant + version, values only)
  │         ├── HIT → return cached config
  │         └── MISS ↓
  ├──2──▶ Read from PostgreSQL (read replica, values only)
  ├──3──▶ Populate Redis cache
  ├──4──▶ Record read in usage_stats (async, batched)
  └──5──▶ Return config to client
```

**With descriptions (bypasses cache, hits DB):**
```
Client (Admin)
  │
  ▼
ConfigService.GetConfig(include_descriptions=true)
  │
  ├──1──▶ Read from PostgreSQL (read replica, values + descriptions)
  ├──2──▶ Record read in usage_stats (async, batched)
  └──3──▶ Return config with descriptions to client
```

### Subscription Flow

```
Client (Service)
  │
  ▼
ConfigService.Subscribe(tenant_id, field_paths?)
  │
  ├──1──▶ Validate auth (JWT)
  ├──2──▶ Register subscriber in-memory on this instance
  ├──3──▶ Send current config state as initial message
  │
  │  ... on change event from Redis Pub/Sub ...
  │
  ├──4──▶ Filter: does change match subscriber's field_paths?
  └──5──▶ Push ConfigChange message on gRPC stream
```

---

## 6. Redis Abstraction

Redis is used for two purposes, behind separate interfaces:

```go
// Cache for config reads
type ConfigCache interface {
    Get(ctx context.Context, tenantID string, version int) (*Config, error)
    Set(ctx context.Context, tenantID string, version int, config *Config, ttl time.Duration) error
    Invalidate(ctx context.Context, tenantID string) error
}

// Pub/Sub for change propagation across instances
type ChangePublisher interface {
    Publish(ctx context.Context, event ConfigChangeEvent) error
}

type ChangeSubscriber interface {
    Subscribe(ctx context.Context, tenantID string) (<-chan ConfigChangeEvent, error)
    Close() error
}
```

Both backed by Redis for v1, replaceable via the interface.

---

## 7. Auth & Middleware

### JWT Validation

- All RPCs require a valid JWT in gRPC metadata (Bearer token)
- JWT claims used:
  - `sub` — actor identifier (user or service account)
  - `role` — superadmin | admin | user
  - `tenant_id` — scoped tenant (not required for superadmin)
- Token validation only (signature + expiry) — no token issuance
- gRPC unary and stream interceptors enforce auth

### Authorization Matrix

| RPC | superadmin | admin (own tenant) | user (own tenant) |
|-----|-----------|-------------------|------------------|
| Schema CRUD | yes | no | no |
| Tenant CRUD | yes | no | no |
| Lock/Unlock fields | yes | no | no |
| SetField/SetFields | yes | yes (unlocked only) | no |
| GetConfig/GetField | yes | yes | yes |
| Subscribe | yes | yes | yes |
| Rollback | yes | yes | no |
| Import/Export schema | yes | no | no |
| Import/Export config | yes | yes | no |
| Query audit log | yes | yes (own tenant) | no |
| Query usage stats | yes | yes (own tenant) | no |

---

## 8. Observability

### OpenTelemetry

- **Tracing**: gRPC interceptors for automatic span creation on all RPCs
- **Metrics** (custom):
  - `config.reads` — counter by tenant, field, cache hit/miss
  - `config.writes` — counter by tenant, actor
  - `config.subscriptions.active` — gauge by tenant
  - `config.cache.hit_rate` — ratio
  - `config.version.current` — gauge by tenant
- **Logging**: structured (JSON), correlated with trace IDs

### Health Checks

- gRPC Health Check Protocol (`grpc.health.v1.Health`)
- Checks: PostgreSQL connectivity, Redis connectivity
- Used by Kubernetes liveness/readiness probes

---

## 9. Open Design Questions

- [ ] Config value delta resolution — walk-backwards approach vs materializing full snapshots on write (tradeoff: storage vs read latency)
- [ ] Usage stats batching — how often to flush read counters (every N reads? every M seconds?)
- [ ] Schema migration mechanics — detailed state machine for field lifecycle (active → deprecated → redirected → removed)
- [ ] Rate limiting — needed for v1?
- [ ] Pagination strategy — cursor-based or offset-based?
- [ ] YAML import/export — exact format specification
- [ ] Tenant deletion — soft delete? cascade behavior?
