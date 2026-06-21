-- +goose Up
-- Grant schema-related tables to decree_app so that SET ROLE decree_app
-- (applied via AfterConnect in the connection pool) does not break schema
-- service operations.  The baseline migration only granted config/audit
-- tables; this fills the gap.
GRANT SELECT, INSERT, UPDATE, DELETE
    ON schemas, schema_versions, schema_fields
    TO decree_app;

-- +goose Down
REVOKE SELECT, INSERT, UPDATE, DELETE
    ON schemas, schema_versions, schema_fields
    FROM decree_app;
