-- name: ListLocationIDsByAPIToken :many
SELECT location_id FROM api_token_locations WHERE token_id = $1 ORDER BY location_id;

-- name: ClearAPITokenLocations :exec
DELETE FROM api_token_locations WHERE token_id = $1;

-- name: InsertAPITokenLocation :exec
INSERT INTO api_token_locations (token_id, location_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;
