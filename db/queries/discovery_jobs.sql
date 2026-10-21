-- name: ListDiscoveryJobs :many
SELECT j.id, j.name, j.subnet_id, j.interval_minutes, j.enabled, j.enrich,
       j.last_run_at, j.last_status, j.last_summary, j.created_at, j.updated_at
FROM discovery_jobs j
JOIN subnets s ON s.id = j.subnet_id AND s.deleted_at IS NULL
WHERE (
    NOT sqlc.arg(scoped)::bool
    OR s.location_id = ANY(sqlc.arg(location_ids)::uuid[])
)
ORDER BY j.name;

-- name: GetDiscoveryJob :one
SELECT * FROM discovery_jobs WHERE id = $1;

-- name: CreateDiscoveryJob :one
INSERT INTO discovery_jobs (name, subnet_id, interval_minutes, enabled, enrich)
VALUES (
    sqlc.arg(name),
    sqlc.arg(subnet_id),
    sqlc.arg(interval_minutes),
    sqlc.arg(enabled),
    sqlc.arg(enrich)
)
RETURNING *;

-- name: UpdateDiscoveryJob :one
UPDATE discovery_jobs
SET name = sqlc.arg(name),
    subnet_id = sqlc.arg(subnet_id),
    interval_minutes = sqlc.arg(interval_minutes),
    enabled = sqlc.arg(enabled),
    enrich = sqlc.arg(enrich),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteDiscoveryJob :exec
DELETE FROM discovery_jobs WHERE id = $1;

-- name: ListDueDiscoveryJobs :many
SELECT * FROM discovery_jobs
WHERE enabled = true
  AND (
    last_run_at IS NULL
    OR last_run_at <= now() - (interval_minutes || ' minutes')::interval
  )
ORDER BY last_run_at NULLS FIRST;

-- name: MarkDiscoveryJobRun :one
UPDATE discovery_jobs
SET last_run_at = now(),
    last_status = sqlc.arg(last_status),
    last_summary = sqlc.arg(last_summary),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
