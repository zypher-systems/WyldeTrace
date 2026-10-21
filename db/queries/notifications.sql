-- name: CreateNotification :one
INSERT INTO app_notifications (kind, title, body, entity_type, entity_id)
VALUES (
    sqlc.arg(kind),
    sqlc.arg(title),
    sqlc.arg(body),
    sqlc.arg(entity_type),
    sqlc.arg(entity_id)
)
RETURNING *;

-- name: ListNotifications :many
SELECT * FROM app_notifications
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountNotifications :one
SELECT count(*) FROM app_notifications;

-- name: CountUnreadNotifications :one
SELECT count(*) FROM app_notifications WHERE read_at IS NULL;

-- name: MarkNotificationRead :one
UPDATE app_notifications
SET read_at = now()
WHERE id = sqlc.arg(id) AND read_at IS NULL
RETURNING *;

-- name: MarkAllNotificationsRead :execrows
UPDATE app_notifications
SET read_at = now()
WHERE read_at IS NULL;
