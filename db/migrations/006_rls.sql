-- +goose Up

-- Create a non-superuser application role used in production.
-- Tests connect as the container superuser and switch via SET SESSION AUTHORIZATION.
DO $$ BEGIN
    CREATE ROLE decree_app;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

GRANT SELECT, INSERT, UPDATE, DELETE
    ON tenants, config_versions, config_values,
       tenant_field_locks, audit_write_log, usage_stats
    TO decree_app;

-- Shared helper: true when the current transaction has set the superadmin escape GUC
-- (schema admin, tenant lifecycle, global reads that must bypass tenant isolation).
CREATE FUNCTION is_superadmin_ctx() RETURNS BOOLEAN
    LANGUAGE sql STABLE
    AS $$ SELECT current_setting('app.superadmin_mode', true) = 'true' $$;

-- Shared helper: true when app.tenant_id has not been set for the current transaction.
-- Non-transactional reads from the pool arrive without a GUC; the application layer
-- is responsible for WHERE tenant_id = $1 in those paths. RLS enforces isolation only
-- when a transaction has pinned a tenant GUC via SET LOCAL app.tenant_id.
CREATE FUNCTION tenant_guc_unset() RETURNS BOOLEAN
    LANGUAGE sql STABLE
    AS $$ SELECT current_setting('app.tenant_id', true) = '' $$;

ALTER TABLE tenants               ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenants               FORCE  ROW LEVEL SECURITY;
ALTER TABLE config_versions       ENABLE ROW LEVEL SECURITY;
ALTER TABLE config_versions       FORCE  ROW LEVEL SECURITY;
ALTER TABLE config_values         ENABLE ROW LEVEL SECURITY;
ALTER TABLE config_values         FORCE  ROW LEVEL SECURITY;
ALTER TABLE tenant_field_locks    ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_field_locks    FORCE  ROW LEVEL SECURITY;
ALTER TABLE audit_write_log       ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_write_log       FORCE  ROW LEVEL SECURITY;
ALTER TABLE usage_stats           ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_stats           FORCE  ROW LEVEL SECURITY;

-- tenants: visibility is unrestricted outside a tenant-scoped tx (admin + lookup paths).
-- Inside a tenant-scoped tx the row must belong to the pinned tenant.
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
-- Both tables have RLS, so the subquery is also filtered — redundant but harmless.
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

-- audit_write_log: schema-level entries have an empty tenant_id (UUID zero or null);
-- those are only visible in superadmin context.
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

DROP POLICY IF EXISTS tenant_rls ON usage_stats;
DROP POLICY IF EXISTS tenant_rls ON audit_write_log;
DROP POLICY IF EXISTS tenant_rls ON tenant_field_locks;
DROP POLICY IF EXISTS tenant_rls ON config_values;
DROP POLICY IF EXISTS tenant_rls ON config_versions;
DROP POLICY IF EXISTS tenant_rls ON tenants;

ALTER TABLE usage_stats           NO FORCE ROW LEVEL SECURITY;
ALTER TABLE usage_stats           DISABLE  ROW LEVEL SECURITY;
ALTER TABLE audit_write_log       NO FORCE ROW LEVEL SECURITY;
ALTER TABLE audit_write_log       DISABLE  ROW LEVEL SECURITY;
ALTER TABLE tenant_field_locks    NO FORCE ROW LEVEL SECURITY;
ALTER TABLE tenant_field_locks    DISABLE  ROW LEVEL SECURITY;
ALTER TABLE config_values         NO FORCE ROW LEVEL SECURITY;
ALTER TABLE config_values         DISABLE  ROW LEVEL SECURITY;
ALTER TABLE config_versions       NO FORCE ROW LEVEL SECURITY;
ALTER TABLE config_versions       DISABLE  ROW LEVEL SECURITY;
ALTER TABLE tenants               NO FORCE ROW LEVEL SECURITY;
ALTER TABLE tenants               DISABLE  ROW LEVEL SECURITY;

DROP FUNCTION IF EXISTS tenant_guc_unset();
DROP FUNCTION IF EXISTS is_superadmin_ctx();

REVOKE SELECT, INSERT, UPDATE, DELETE
    ON tenants, config_versions, config_values,
       tenant_field_locks, audit_write_log, usage_stats
    FROM decree_app;

DROP ROLE IF EXISTS decree_app;
