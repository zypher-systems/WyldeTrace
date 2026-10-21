-- name: ListDiscoveryJobTemplates :many
SELECT * FROM discovery_job_templates ORDER BY name;

-- name: GetDiscoveryJobTemplate :one
SELECT * FROM discovery_job_templates WHERE id = $1;

-- name: CreateDiscoveryJobTemplate :one
INSERT INTO discovery_job_templates (name, interval_minutes, enrich, enabled_default)
VALUES (
    sqlc.arg(name),
    sqlc.arg(interval_minutes),
    sqlc.arg(enrich),
    sqlc.arg(enabled_default)
)
RETURNING *;

-- name: UpdateDiscoveryJobTemplate :one
UPDATE discovery_job_templates
SET name = sqlc.arg(name),
    interval_minutes = sqlc.arg(interval_minutes),
    enrich = sqlc.arg(enrich),
    enabled_default = sqlc.arg(enabled_default),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteDiscoveryJobTemplate :exec
DELETE FROM discovery_job_templates WHERE id = $1;
