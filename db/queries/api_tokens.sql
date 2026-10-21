-- name: CreateAPIToken :one
INSERT INTO api_tokens (name, prefix, token_hash, role, created_by, expires_at)
VALUES (
    sqlc.arg(name),
    sqlc.arg(prefix),
    sqlc.arg(token_hash),
    sqlc.arg(role),
    sqlc.arg(created_by),
    sqlc.arg(expires_at)
)
RETURNING *;

-- name: ListAPITokens :many
SELECT * FROM api_tokens
ORDER BY created_at DESC;

-- name: GetAPITokenByPrefix :one
SELECT * FROM api_tokens
WHERE prefix = $1
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now());

-- name: GetAPITokenByID :one
SELECT * FROM api_tokens WHERE id = $1;

-- name: TouchAPIToken :exec
UPDATE api_tokens SET last_used_at = now() WHERE id = $1;

-- name: RevokeAPIToken :one
UPDATE api_tokens
SET revoked_at = now()
WHERE id = $1 AND revoked_at IS NULL
RETURNING *;

-- name: DeleteAPIToken :exec
DELETE FROM api_tokens WHERE id = $1;
