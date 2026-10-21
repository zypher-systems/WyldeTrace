-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY email;

-- name: CreateUser :one
INSERT INTO users (email, password_hash, role)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET email = $2,
    role = $3,
    password_hash = COALESCE(sqlc.narg(password_hash), password_hash),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: SetUserTOTPBegin :one
UPDATE users
SET totp_secret = $2,
    totp_enabled = false,
    recovery_codes = '[]'::jsonb,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetUserTOTPConfirm :one
UPDATE users
SET totp_secret = $2,
    totp_enabled = true,
    recovery_codes = $3,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetUserRecoveryCodes :one
UPDATE users
SET recovery_codes = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DisableUserTOTP :one
UPDATE users
SET totp_secret = '',
    totp_enabled = false,
    recovery_codes = '[]'::jsonb,
    updated_at = now()
WHERE id = $1
RETURNING *;
