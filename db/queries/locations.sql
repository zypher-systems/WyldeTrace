-- name: ListLocations :many
SELECT * FROM locations ORDER BY name;

-- name: GetLocation :one
SELECT * FROM locations WHERE id = $1;

-- name: CreateLocation :one
INSERT INTO locations (name, description)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateLocation :one
UPDATE locations
SET name = $2,
    description = $3,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteLocation :exec
DELETE FROM locations WHERE id = $1;

-- name: SearchLocations :many
SELECT * FROM locations
WHERE (name ILIKE '%' || sqlc.arg(query)::text || '%'
   OR description ILIKE '%' || sqlc.arg(query)::text || '%')
  AND (
    NOT sqlc.arg(scoped)::bool
    OR id = ANY(sqlc.arg(location_ids)::uuid[])
  )
ORDER BY name
LIMIT 50;
