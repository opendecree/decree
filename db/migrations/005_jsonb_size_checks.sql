-- +goose Up

-- Cap user-supplied JSONB column sizes at 1 MiB to prevent unbounded storage.
-- pg_column_size returns the on-disk size including storage overhead.
-- audit_write_log.metadata is excluded: it is server-generated context and
-- may legitimately exceed 1 MiB for bulk-import audit entries.

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

-- +goose Down

ALTER TABLE schema_fields
    DROP CONSTRAINT IF EXISTS chk_schema_fields_constraints_size,
    DROP CONSTRAINT IF EXISTS chk_schema_fields_examples_size,
    DROP CONSTRAINT IF EXISTS chk_schema_fields_external_docs_size;

ALTER TABLE tenant_field_locks
    DROP CONSTRAINT IF EXISTS chk_tenant_field_locks_locked_values_size;
