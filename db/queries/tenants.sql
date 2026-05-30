-- name: CreateTenant :one
INSERT INTO tenants (name, schema_id, schema_version)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetTenantByID :one
SELECT * FROM tenants WHERE id = $1 AND deleted_at IS NULL;

-- name: GetTenantByName :one
SELECT * FROM tenants WHERE name = $1 AND deleted_at IS NULL;

-- name: GetTenantsByNames :many
SELECT * FROM tenants
WHERE name = ANY(@names::text[]) AND deleted_at IS NULL;

-- name: ListTenants :many
SELECT * FROM tenants
WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListTenantsKeyset :many
SELECT * FROM tenants
WHERE deleted_at IS NULL
  AND ($2::TIMESTAMPTZ IS NULL
       OR created_at < $2
       OR (created_at = $2 AND id < $3::UUID))
ORDER BY created_at DESC, id DESC
LIMIT $1;

-- name: ListTenantsByIDs :many
SELECT * FROM tenants
WHERE id = ANY(@allowed_ids::uuid[]) AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListTenantsByIDsKeyset :many
SELECT * FROM tenants
WHERE id = ANY(@allowed_ids::uuid[]) AND deleted_at IS NULL
  AND ($2::TIMESTAMPTZ IS NULL
       OR created_at < $2
       OR (created_at = $2 AND id < $3::UUID))
ORDER BY created_at DESC, id DESC
LIMIT $1;

-- name: ListTenantsBySchema :many
SELECT * FROM tenants
WHERE schema_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListTenantsBySchemaKeyset :many
SELECT * FROM tenants
WHERE schema_id = $1 AND deleted_at IS NULL
  AND ($3::TIMESTAMPTZ IS NULL
       OR created_at < $3
       OR (created_at = $3 AND id < $4::UUID))
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: ListTenantsBySchemaAndIDs :many
SELECT * FROM tenants
WHERE schema_id = $1 AND id = ANY(@allowed_ids::uuid[]) AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListTenantsBySchemaAndIDsKeyset :many
SELECT * FROM tenants
WHERE schema_id = $1 AND id = ANY(@allowed_ids::uuid[]) AND deleted_at IS NULL
  AND ($3::TIMESTAMPTZ IS NULL
       OR created_at < $3
       OR (created_at = $3 AND id < $4::UUID))
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: UpdateTenantName :one
UPDATE tenants SET name = $2, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateTenantSchemaVersion :one
UPDATE tenants SET schema_version = $2, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteTenant :exec
UPDATE tenants SET deleted_at = now() WHERE id = $1;

-- name: CreateFieldLock :exec
INSERT INTO tenant_field_locks (tenant_id, field_path, locked_values)
VALUES ($1, $2, $3)
ON CONFLICT (tenant_id, field_path) DO UPDATE SET locked_values = $3;

-- name: DeleteFieldLock :exec
DELETE FROM tenant_field_locks
WHERE tenant_id = $1 AND field_path = $2;

-- name: GetFieldLocks :many
SELECT * FROM tenant_field_locks
WHERE tenant_id = $1
ORDER BY field_path;

-- name: ListFieldLocks :many
SELECT * FROM tenant_field_locks
WHERE tenant_id = $1
ORDER BY field_path
LIMIT $2 OFFSET $3;
