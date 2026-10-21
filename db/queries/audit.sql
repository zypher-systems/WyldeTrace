-- name: CreateAuditLog :one
INSERT INTO audit_logs (actor_id, actor_email, action, entity_type, entity_id, details)
VALUES (
    sqlc.arg(actor_id),
    sqlc.arg(actor_email),
    sqlc.arg(action),
    sqlc.arg(entity_type),
    sqlc.arg(entity_id),
    sqlc.arg(details)
)
RETURNING *;

-- name: ListAuditLogs :many
SELECT * FROM audit_logs
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: ListAuditLogsExport :many
SELECT id, actor_id, actor_email, action, entity_type, entity_id, details, created_at
FROM audit_logs
WHERE (
    sqlc.arg(action)::text = '' OR action ILIKE '%' || sqlc.arg(action) || '%'
)
AND (
    sqlc.arg(actor)::text = '' OR actor_email ILIKE '%' || sqlc.arg(actor) || '%'
)
AND (
    sqlc.narg(from_ts)::timestamptz IS NULL OR created_at >= sqlc.narg(from_ts)
)
AND (
    sqlc.narg(to_ts)::timestamptz IS NULL OR created_at <= sqlc.narg(to_ts)
)
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: PurgeAuditLogsBefore :execrows
DELETE FROM audit_logs
WHERE created_at < sqlc.arg(before)::timestamptz;
