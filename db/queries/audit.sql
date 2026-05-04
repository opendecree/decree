-- name: InsertAuditWriteLog :exec
INSERT INTO audit_write_log (id, tenant_id, actor, action, field_path, old_value, new_value, config_version, metadata, object_kind, previous_hash, entry_hash)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: GetLastAuditHashForTenant :one
SELECT COALESCE(entry_hash, '')
FROM audit_write_log
WHERE tenant_id IS NOT DISTINCT FROM $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: QueryAuditWriteLog :many
SELECT * FROM audit_write_log
WHERE ($1::UUID IS NULL OR tenant_id = $1)
  AND (NULLIF($2::TEXT, '') IS NULL OR actor = $2)
  AND (NULLIF($3::TEXT, '') IS NULL OR field_path = $3)
  AND ($4::TIMESTAMPTZ IS NULL OR created_at >= $4)
  AND ($5::TIMESTAMPTZ IS NULL OR created_at <= $5)
ORDER BY created_at DESC
LIMIT $6 OFFSET $7;

-- name: GetAuditWriteLogOrdered :many
SELECT * FROM audit_write_log
WHERE tenant_id IS NOT DISTINCT FROM $1
ORDER BY created_at ASC, id ASC;

-- name: UpsertUsageStats :exec
INSERT INTO usage_stats (tenant_id, field_path, period_start, read_count, last_read_by, last_read_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (tenant_id, field_path, period_start)
DO UPDATE SET
    read_count = usage_stats.read_count + EXCLUDED.read_count,
    last_read_by = EXCLUDED.last_read_by,
    last_read_at = EXCLUDED.last_read_at;

-- name: GetFieldUsage :many
SELECT * FROM usage_stats
WHERE tenant_id = $1
  AND field_path = $2
  AND ($3::TIMESTAMPTZ IS NULL OR period_start >= $3)
  AND ($4::TIMESTAMPTZ IS NULL OR period_start <= $4)
ORDER BY period_start DESC;

-- name: GetTenantUsage :many
SELECT field_path, SUM(read_count) AS read_count,
       MAX(last_read_at) AS last_read_at
FROM usage_stats
WHERE tenant_id = $1
  AND ($2::TIMESTAMPTZ IS NULL OR period_start >= $2)
  AND ($3::TIMESTAMPTZ IS NULL OR period_start <= $3)
GROUP BY field_path
ORDER BY field_path;

-- name: GetUnusedFields :many
SELECT sf.path
FROM schema_fields sf
JOIN schema_versions sv ON sv.id = sf.schema_version_id
JOIN tenants t ON t.schema_id = sv.schema_id AND t.schema_version = sv.version
WHERE t.id = $1
  AND sf.path NOT IN (
      SELECT us.field_path FROM usage_stats us
      WHERE us.tenant_id = $1 AND us.last_read_at >= $2
  )
ORDER BY sf.path;
