-- +goose Up

-- Cap JSONB column sizes at 1 MiB to prevent unbounded storage.
-- pg_column_size returns the on-disk size including storage overhead.

ALTER TABLE schema_fields
    ADD CONSTRAINT chk_schema_fields_constraints_size
        CHECK (pg_column_size(constraints) <= 1048576),
    ADD CONSTRAINT chk_schema_fields_examples_size
        CHECK (pg_column_size(examples) <= 1048576),
    ADD CONSTRAINT chk_schema_fields_external_docs_size
        CHECK (pg_column_size(external_docs) <= 1048576);

ALTER TABLE tenant_field_locks
    ADD CONSTRAINT chk_tenant_field_locks_locked_values_size
        CHECK (pg_column_size(locked_values) <= 1048576);

ALTER TABLE audit_write_log
    ADD CONSTRAINT chk_audit_write_log_metadata_size
        CHECK (pg_column_size(metadata) <= 1048576);

-- +goose Down

ALTER TABLE schema_fields
    DROP CONSTRAINT IF EXISTS chk_schema_fields_constraints_size,
    DROP CONSTRAINT IF EXISTS chk_schema_fields_examples_size,
    DROP CONSTRAINT IF EXISTS chk_schema_fields_external_docs_size;

ALTER TABLE tenant_field_locks
    DROP CONSTRAINT IF EXISTS chk_tenant_field_locks_locked_values_size;

ALTER TABLE audit_write_log
    DROP CONSTRAINT IF EXISTS chk_audit_write_log_metadata_size;
