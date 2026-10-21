-- name: ListWebhooks :many
SELECT * FROM webhooks ORDER BY name;

-- name: ListEnabledWebhooks :many
SELECT * FROM webhooks WHERE enabled = true ORDER BY name;

-- name: GetWebhook :one
SELECT * FROM webhooks WHERE id = $1;

-- name: CreateWebhook :one
INSERT INTO webhooks (name, url, secret, events, enabled, format)
VALUES (
    sqlc.arg(name),
    sqlc.arg(url),
    sqlc.arg(secret),
    sqlc.arg(events),
    sqlc.arg(enabled),
    sqlc.arg(format)
)
RETURNING *;

-- name: UpdateWebhook :one
UPDATE webhooks
SET name = sqlc.arg(name),
    url = sqlc.arg(url),
    secret = sqlc.arg(secret),
    events = sqlc.arg(events),
    enabled = sqlc.arg(enabled),
    format = sqlc.arg(format),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteWebhook :exec
DELETE FROM webhooks WHERE id = $1;

-- name: MarkWebhookDelivery :one
UPDATE webhooks
SET last_status = sqlc.arg(last_status),
    last_error = sqlc.arg(last_error),
    last_sent_at = now(),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: CreateWebhookDeliveryLog :one
INSERT INTO webhook_deliveries (webhook_id, event, status, status_code, error, request_body)
VALUES (
    sqlc.arg(webhook_id),
    sqlc.arg(event),
    sqlc.arg(status),
    sqlc.arg(status_code),
    sqlc.arg(error),
    sqlc.arg(request_body)
)
RETURNING *;

-- name: ListWebhookDeliveryLogs :many
SELECT * FROM webhook_deliveries
WHERE (
    sqlc.narg(webhook_id)::uuid IS NULL OR webhook_id = sqlc.narg(webhook_id)
)
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountWebhookDeliveryLogs :one
SELECT count(*) FROM webhook_deliveries
WHERE (
    sqlc.narg(webhook_id)::uuid IS NULL OR webhook_id = sqlc.narg(webhook_id)
);

-- name: GetWebhookDeliveryLog :one
SELECT * FROM webhook_deliveries WHERE id = $1;

-- name: PurgeWebhookDeliveriesBefore :execrows
DELETE FROM webhook_deliveries
WHERE created_at < sqlc.arg(before)::timestamptz;
