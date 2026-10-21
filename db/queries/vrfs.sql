-- name: ListVRFs :many
SELECT id, name, rd, description, created_at, updated_at
FROM vrfs
ORDER BY name;

-- name: GetVRF :one
SELECT id, name, rd, description, created_at, updated_at
FROM vrfs
WHERE id = $1;

-- name: CreateVRF :one
INSERT INTO vrfs (name, rd, description)
VALUES (sqlc.arg(name), sqlc.arg(rd), sqlc.arg(description))
RETURNING id, name, rd, description, created_at, updated_at;

-- name: UpdateVRF :one
UPDATE vrfs
SET name = sqlc.arg(name),
    rd = sqlc.arg(rd),
    description = sqlc.arg(description),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, name, rd, description, created_at, updated_at;

-- name: DeleteVRF :exec
DELETE FROM vrfs WHERE id = $1 AND id <> '00000000-0000-0000-0000-000000000001';
