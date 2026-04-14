-- name: CreateConfigVersion :one
INSERT INTO config_versions (tenant_id, version, description, created_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetConfigVersion :one
SELECT * FROM config_versions
WHERE tenant_id = $1 AND version = $2;

-- name: GetLatestConfigVersion :one
SELECT * FROM config_versions
WHERE tenant_id = $1
ORDER BY version DESC
LIMIT 1;

-- name: ListConfigVersions :many
SELECT * FROM config_versions
WHERE tenant_id = $1
ORDER BY version DESC
LIMIT $2 OFFSET $3;

-- name: SetConfigValue :exec
INSERT INTO config_values (config_version_id, field_path, value, checksum, description)
VALUES ($1, $2, $3, $4, $5);

-- name: GetConfigValues :many
SELECT * FROM config_values
WHERE config_version_id = $1
ORDER BY field_path;

-- name: GetConfigValueAtVersion :one
SELECT cv.field_path, cv.value, cv.checksum, cv.description
FROM config_values cv
JOIN config_versions ver ON ver.id = cv.config_version_id
WHERE ver.tenant_id = $1
  AND cv.field_path = $2
  AND ver.version <= $3
ORDER BY ver.version DESC
LIMIT 1;

-- name: GetFullConfigAtVersion :many
SELECT DISTINCT ON (cv.field_path) cv.field_path, cv.value, cv.checksum, cv.description
FROM config_values cv
JOIN config_versions ver ON ver.id = cv.config_version_id
WHERE ver.tenant_id = $1
  AND ver.version <= $2
ORDER BY cv.field_path, ver.version DESC;
