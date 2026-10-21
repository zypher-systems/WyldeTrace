-- name: GetWebhookMute :one
SELECT event, entity_key, muted_until
FROM webhook_event_mutes
WHERE event = sqlc.arg(event)
  AND entity_key = sqlc.arg(entity_key);

-- name: UpsertWebhookMute :exec
INSERT INTO webhook_event_mutes (event, entity_key, muted_until)
VALUES (sqlc.arg(event), sqlc.arg(entity_key), sqlc.arg(muted_until))
ON CONFLICT (event, entity_key) DO UPDATE
SET muted_until = EXCLUDED.muted_until;

-- name: DeleteExpiredWebhookMutes :exec
DELETE FROM webhook_event_mutes WHERE muted_until < now();
