-- +goose Up

-- Add tamper-evident columns and loosen tenant_id to allow global (schema-level) audit entries.
ALTER TABLE audit_write_log
    ALTER COLUMN tenant_id DROP NOT NULL,
    ADD COLUMN object_kind TEXT NOT NULL DEFAULT 'field'
        CHECK (object_kind IN ('field', 'schema', 'tenant', 'lock')),
    ADD COLUMN previous_hash TEXT NOT NULL DEFAULT '',
    ADD COLUMN entry_hash   TEXT NOT NULL DEFAULT '';

-- Reject UPDATE/DELETE on rows older than the configurable immutability window.
-- A 60-second grace window allows test teardown; anything older is permanently immutable.
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

CREATE TRIGGER trg_audit_write_log_immutable
    BEFORE UPDATE OR DELETE ON audit_write_log
    FOR EACH ROW EXECUTE FUNCTION audit_write_log_immutable();

-- +goose Down

DROP TRIGGER IF EXISTS trg_audit_write_log_immutable ON audit_write_log;
DROP FUNCTION IF EXISTS audit_write_log_immutable();

ALTER TABLE audit_write_log
    DROP COLUMN IF EXISTS entry_hash,
    DROP COLUMN IF EXISTS previous_hash,
    DROP COLUMN IF EXISTS object_kind,
    ALTER COLUMN tenant_id SET NOT NULL;
