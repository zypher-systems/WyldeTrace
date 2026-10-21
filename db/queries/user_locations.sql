-- name: ListLocationIDsByUser :many
SELECT location_id FROM user_locations WHERE user_id = $1 ORDER BY location_id;

-- name: ClearUserLocations :exec
DELETE FROM user_locations WHERE user_id = $1;

-- name: InsertUserLocation :exec
INSERT INTO user_locations (user_id, location_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;
