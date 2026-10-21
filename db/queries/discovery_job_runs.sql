-- name: CreateDiscoveryJobRun :one
INSERT INTO discovery_job_runs (job_id, status, summary, started_at, finished_at)
VALUES (
    sqlc.arg(job_id),
    sqlc.arg(status),
    sqlc.arg(summary),
    sqlc.arg(started_at),
    sqlc.arg(finished_at)
)
RETURNING *;

-- name: ListDiscoveryJobRuns :many
SELECT * FROM discovery_job_runs
WHERE job_id = sqlc.arg(job_id)
ORDER BY started_at DESC
LIMIT sqlc.arg(row_limit);

-- name: CountDiscoveryJobRuns :one
SELECT count(*) FROM discovery_job_runs WHERE job_id = sqlc.arg(job_id);
