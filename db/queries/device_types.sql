-- name: ListDeviceTypes :many
SELECT * FROM device_types ORDER BY name;

-- name: GetDeviceType :one
SELECT * FROM device_types WHERE id = $1;

-- name: CreateDeviceType :one
INSERT INTO device_types (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateDeviceType :one
UPDATE device_types
SET name = $2,
    description = $3,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteDeviceType :exec
DELETE FROM device_types WHERE id = $1;
