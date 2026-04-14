-- +goose Up

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Field type enum
CREATE TYPE field_type AS ENUM (
    'integer',
    'number',
    'string',
    'bool',
    'time',
    'duration',
    'url',
    'json'
);

-- Schema definitions
CREATE TABLE schemas (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE schema_versions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schema_id      UUID NOT NULL REFERENCES schemas(id) ON DELETE CASCADE,
    version        INT NOT NULL,
    parent_version INT,
    description    TEXT,
    checksum       TEXT NOT NULL,
    published      BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(schema_id, version)
);

CREATE TABLE schema_fields (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schema_version_id UUID NOT NULL REFERENCES schema_versions(id) ON DELETE CASCADE,
    path              TEXT NOT NULL,
    field_type        field_type NOT NULL,
    constraints       JSONB,
    nullable          BOOLEAN NOT NULL DEFAULT false,
    deprecated        BOOLEAN NOT NULL DEFAULT false,
    redirect_to       TEXT,
    default_value     TEXT,
    description       TEXT,
    UNIQUE(schema_version_id, path)
);

-- Tenants
CREATE TABLE tenants (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL UNIQUE,
    schema_id      UUID NOT NULL REFERENCES schemas(id),
    schema_version INT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tenant_field_locks (
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    field_path    TEXT NOT NULL,
    locked_values JSONB,
    PRIMARY KEY (tenant_id, field_path)
);

-- Config versions
CREATE TABLE config_versions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    version     INT NOT NULL,
    description TEXT,
    created_by  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, version)
);

-- Config values (delta storage — only changed fields per version)
CREATE TABLE config_values (
    config_version_id UUID NOT NULL REFERENCES config_versions(id) ON DELETE CASCADE,
    field_path        TEXT NOT NULL,
    value             TEXT,
    checksum          TEXT,
    description       TEXT,
    PRIMARY KEY (config_version_id, field_path)
);

-- Audit: write events
CREATE TABLE audit_write_log (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL,
    actor          TEXT NOT NULL,
    action         TEXT NOT NULL,
    field_path     TEXT,
    old_value      TEXT,
    new_value      TEXT,
    config_version INT,
    metadata       JSONB,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_write_log_tenant ON audit_write_log(tenant_id, created_at);
CREATE INDEX idx_audit_write_log_actor ON audit_write_log(actor, created_at);

-- Audit: read usage aggregation
CREATE TABLE usage_stats (
    tenant_id    UUID NOT NULL,
    field_path   TEXT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    read_count   BIGINT NOT NULL DEFAULT 0,
    last_read_by TEXT,
    last_read_at TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, field_path, period_start)
);

-- +goose Down

DROP TABLE IF EXISTS usage_stats;
DROP TABLE IF EXISTS audit_write_log;
DROP TABLE IF EXISTS config_values;
DROP TABLE IF EXISTS config_versions;
DROP TABLE IF EXISTS tenant_field_locks;
DROP TABLE IF EXISTS tenants;
DROP TABLE IF EXISTS schema_fields;
DROP TABLE IF EXISTS schema_versions;
DROP TABLE IF EXISTS schemas;
DROP TYPE IF EXISTS field_type;
