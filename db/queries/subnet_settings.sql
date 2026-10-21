-- name: GetSubnetSettings :one
SELECT subnet_id, util_alert_percent, updated_at
FROM subnet_settings
WHERE subnet_id = $1;

-- name: UpsertSubnetUtilAlertPercent :one
INSERT INTO subnet_settings (subnet_id, util_alert_percent, updated_at)
VALUES (sqlc.arg(subnet_id), sqlc.arg(util_alert_percent), now())
ON CONFLICT (subnet_id) DO UPDATE
SET util_alert_percent = EXCLUDED.util_alert_percent,
    updated_at = now()
RETURNING subnet_id, util_alert_percent, updated_at;

-- name: ClearSubnetUtilAlertPercent :exec
DELETE FROM subnet_settings
WHERE subnet_id = sqlc.arg(subnet_id);
