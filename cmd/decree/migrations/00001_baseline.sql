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
    name        TEXT NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE schema_versions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schema_id          UUID NOT NULL REFERENCES schemas(id) ON DELETE CASCADE,
    version            INT NOT NULL,
    parent_version     INT,
    description        TEXT,
    checksum           TEXT NOT NULL,
    published          BOOLEAN NOT NULL DEFAULT false,
    -- JSON array of {trigger_field, dependent_fields} entries encoding the
    -- schema's dependentRequired rules. Empty array when no rules exist.
    dependent_required JSONB NOT NULL DEFAULT '[]',
    -- JSON array of {path, rule, message, severity?, reason?} entries
    -- encoding the schema's CEL validation rules. Reserved in v0.1.0 —
    -- parser persists; runtime engine ships in Phase 2 (see issue #76).
    validations        JSONB NOT NULL DEFAULT '[]',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(schema_id, version)
);

CREATE TABLE schema_fields (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schema_version_id UUID NOT NULL REFERENCES schema_versions(id) ON DELETE CASCADE,
    path              TEXT NOT NULL,
    field_type        field_type NOT NULL,
    constraints       JSONB CONSTRAINT chk_schema_fields_constraints_size    CHECK (pg_column_size(constraints)   <= 1048576),
    nullable          BOOLEAN NOT NULL DEFAULT false,
    deprecated        BOOLEAN NOT NULL DEFAULT false,
    redirect_to       TEXT,
    default_value     TEXT,
    description       TEXT,
    title             TEXT,
    example           TEXT,
    examples          JSONB CONSTRAINT chk_schema_fields_examples_size       CHECK (pg_column_size(examples)      <= 1048576),
    external_docs     JSONB CONSTRAINT chk_schema_fields_external_docs_size  CHECK (pg_column_size(external_docs) <= 1048576),
    tags              TEXT[],
    format            TEXT,
    read_only         BOOLEAN NOT NULL DEFAULT false,
    write_once        BOOLEAN NOT NULL DEFAULT false,
    sensitive         BOOLEAN NOT NULL DEFAULT false,
    UNIQUE(schema_version_id, path)
);

-- Tenants
CREATE TABLE tenants (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL,
    schema_id      UUID NOT NULL REFERENCES schemas(id),
    schema_version INT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);

CREATE TABLE tenant_field_locks (
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    field_path    TEXT NOT NULL,
    locked_values JSONB CONSTRAINT chk_tenant_field_locks_locked_values_size CHECK (pg_column_size(locked_values) <= 1048576),
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
    tenant_id      UUID,
    actor          TEXT NOT NULL,
    action         TEXT NOT NULL,
    field_path     TEXT,
    old_value      TEXT,
    new_value      TEXT,
    config_version INT,
    metadata       JSONB,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    object_kind    TEXT NOT NULL DEFAULT 'field'
                       CHECK (object_kind IN ('field', 'schema', 'tenant', 'lock')),
    previous_hash  TEXT NOT NULL DEFAULT '',
    entry_hash     TEXT NOT NULL DEFAULT '',
    chain_epoch    INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_audit_write_log_tenant ON audit_write_log(tenant_id, created_at); -- decree:index-lock-ok baseline runs on empty table
CREATE INDEX idx_audit_write_log_actor  ON audit_write_log(actor, created_at);     -- decree:index-lock-ok baseline runs on empty table

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

-- Partial unique indexes: names can be reused after soft-delete.
CREATE UNIQUE INDEX schemas_name_unique ON schemas(name) WHERE deleted_at IS NULL; -- decree:index-lock-ok baseline runs on empty table
CREATE UNIQUE INDEX tenants_name_unique ON tenants(name) WHERE deleted_at IS NULL; -- decree:index-lock-ok baseline runs on empty table

-- Immutability trigger: reject UPDATE/DELETE on audit rows older than 60 seconds.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION audit_write_log_immutable()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.created_at < now() - INTERVAL '60 seconds' THEN
        RAISE EXCEPTION 'audit_write_log rows older than 60 seconds are immutable';
    END IF;
    -- Row is within the grace window — allow (used only by test cleanup).
    RETURN OLD;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER trg_audit_write_log_immutable
    BEFORE UPDATE OR DELETE ON audit_write_log
    FOR EACH ROW EXECUTE FUNCTION audit_write_log_immutable();

-- Application role
-- +goose StatementBegin
DO $$ BEGIN
    CREATE ROLE decree_app;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
-- +goose StatementEnd

GRANT SELECT, INSERT, UPDATE, DELETE
    ON schemas, schema_versions, schema_fields,
       tenants, config_versions, config_values,
       tenant_field_locks, audit_write_log, usage_stats
    TO decree_app;

-- RLS helper: true when the superadmin escape GUC is set for the current transaction.
CREATE FUNCTION is_superadmin_ctx() RETURNS BOOLEAN
    LANGUAGE sql STABLE
    AS $$ SELECT COALESCE(current_setting('app.superadmin_mode', true), '') = 'true' $$;

-- RLS helper: true when app.tenant_id has not been set for the current transaction.
CREATE FUNCTION tenant_guc_unset() RETURNS BOOLEAN
    LANGUAGE sql STABLE
    AS $$ SELECT COALESCE(current_setting('app.tenant_id', true), '') = '' $$;

ALTER TABLE tenants            ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants            FORCE  ROW LEVEL SECURITY;
ALTER TABLE config_versions    ENABLE ROW LEVEL SECURITY;
ALTER TABLE config_versions    FORCE  ROW LEVEL SECURITY;
ALTER TABLE config_values      ENABLE ROW LEVEL SECURITY;
ALTER TABLE config_values      FORCE  ROW LEVEL SECURITY;
ALTER TABLE tenant_field_locks ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_field_locks FORCE  ROW LEVEL SECURITY;
ALTER TABLE audit_write_log    ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_write_log    FORCE  ROW LEVEL SECURITY;
ALTER TABLE usage_stats        ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_stats        FORCE  ROW LEVEL SECURITY;

-- tenants: visibility unrestricted outside tenant-scoped tx; inside, row must belong to pinned tenant.
CREATE POLICY tenant_rls ON tenants
    USING (
        is_superadmin_ctx()
        OR tenant_guc_unset()
        OR id::text = current_setting('app.tenant_id', true)
    );

-- config_versions: direct tenant_id column.
CREATE POLICY tenant_rls ON config_versions
    USING (
        is_superadmin_ctx()
        OR tenant_guc_unset()
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );

-- config_values: no direct tenant_id; subquery through config_versions.
CREATE POLICY tenant_rls ON config_values
    USING (
        is_superadmin_ctx()
        OR tenant_guc_unset()
        OR EXISTS (
            SELECT 1 FROM config_versions cv
            WHERE cv.id = config_values.config_version_id
              AND cv.tenant_id::text = current_setting('app.tenant_id', true)
        )
    );

-- tenant_field_locks: direct tenant_id column.
CREATE POLICY tenant_rls ON tenant_field_locks
    USING (
        is_superadmin_ctx()
        OR tenant_guc_unset()
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );

-- audit_write_log: schema-level entries (null tenant_id) visible only in superadmin context.
CREATE POLICY tenant_rls ON audit_write_log
    USING (
        is_superadmin_ctx()
        OR (
            tenant_guc_unset()
            AND tenant_id IS NOT NULL
        )
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );

-- usage_stats: direct tenant_id column.
CREATE POLICY tenant_rls ON usage_stats
    USING (
        is_superadmin_ctx()
        OR tenant_guc_unset()
        OR tenant_id::text = current_setting('app.tenant_id', true)
    );

-- +goose Down
-- Alpha deployments are not supported; no rollback path.
