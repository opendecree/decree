-- name: CreateSchema :one
INSERT INTO schemas (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: GetSchemaByID :one
SELECT * FROM schemas WHERE id = $1;

-- name: GetSchemaByName :one
SELECT * FROM schemas WHERE name = $1;

-- name: ListSchemas :many
SELECT * FROM schemas
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: DeleteSchema :exec
DELETE FROM schemas WHERE id = $1;

-- name: CreateSchemaVersion :one
INSERT INTO schema_versions (schema_id, version, parent_version, description, checksum)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSchemaVersion :one
SELECT * FROM schema_versions
WHERE schema_id = $1 AND version = $2;

-- name: GetLatestSchemaVersion :one
SELECT * FROM schema_versions
WHERE schema_id = $1
ORDER BY version DESC
LIMIT 1;

-- name: PublishSchemaVersion :one
UPDATE schema_versions SET published = true
WHERE schema_id = $1 AND version = $2
RETURNING *;

-- name: CreateSchemaField :one
INSERT INTO schema_fields (
    schema_version_id, path, field_type, constraints, nullable, deprecated,
    redirect_to, default_value, description, title, example, examples,
    external_docs, tags, format, read_only, write_once, sensitive
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
RETURNING *;

-- name: GetSchemaFields :many
SELECT * FROM schema_fields
WHERE schema_version_id = $1
ORDER BY path;

-- name: DeleteSchemaField :exec
DELETE FROM schema_fields
WHERE schema_version_id = $1 AND path = $2;
