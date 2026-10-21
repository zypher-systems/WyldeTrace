-- name: ListVLANs :many
SELECT id, vid, name, location_id, description, created_at, updated_at
FROM vlans
ORDER BY vid, name;

-- name: GetVLAN :one
SELECT id, vid, name, location_id, description, created_at, updated_at
FROM vlans
WHERE id = $1;

-- name: GetVLANByVidLocation :one
SELECT id, vid, name, location_id, description, created_at, updated_at
FROM vlans
WHERE vid = sqlc.arg(vid)
  AND location_id IS NOT DISTINCT FROM sqlc.arg(location_id);

-- name: CreateVLAN :one
INSERT INTO vlans (vid, name, location_id, description)
VALUES (sqlc.arg(vid), sqlc.arg(name), sqlc.arg(location_id), sqlc.arg(description))
RETURNING id, vid, name, location_id, description, created_at, updated_at;

-- name: UpdateVLAN :one
UPDATE vlans
SET vid = sqlc.arg(vid),
    name = sqlc.arg(name),
    location_id = sqlc.arg(location_id),
    description = sqlc.arg(description),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, vid, name, location_id, description, created_at, updated_at;

-- name: DeleteVLAN :exec
DELETE FROM vlans WHERE id = $1;
