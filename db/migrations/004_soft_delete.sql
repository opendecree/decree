-- +goose Up

ALTER TABLE schemas ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE tenants ADD COLUMN deleted_at TIMESTAMPTZ;

-- Replace table-level UNIQUE with partial indexes so names can be reused after soft-delete.
ALTER TABLE schemas DROP CONSTRAINT schemas_name_key;
ALTER TABLE tenants DROP CONSTRAINT tenants_name_key;

CREATE UNIQUE INDEX schemas_name_unique ON schemas(name) WHERE deleted_at IS NULL; -- decree:index-lock-ok alpha migration, tables are small
CREATE UNIQUE INDEX tenants_name_unique ON tenants(name) WHERE deleted_at IS NULL; -- decree:index-lock-ok alpha migration, tables are small

-- +goose Down

DROP INDEX IF EXISTS schemas_name_unique;
DROP INDEX IF EXISTS tenants_name_unique;

ALTER TABLE schemas ADD CONSTRAINT schemas_name_key UNIQUE (name);
ALTER TABLE tenants ADD CONSTRAINT tenants_name_key UNIQUE (name);

ALTER TABLE tenants DROP COLUMN deleted_at;
ALTER TABLE schemas DROP COLUMN deleted_at;
