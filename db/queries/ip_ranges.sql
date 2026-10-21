-- name: ListIPRangesBySubnet :many
SELECT id, subnet_id, name, host(start_address)::text AS start_address,
       host(end_address)::text AS end_address, role, description, created_at, updated_at
FROM ip_ranges
WHERE subnet_id = $1
ORDER BY start_address;

-- name: GetIPRange :one
SELECT id, subnet_id, name, host(start_address)::text AS start_address,
       host(end_address)::text AS end_address, role, description, created_at, updated_at
FROM ip_ranges
WHERE id = $1;

-- name: CreateIPRange :one
INSERT INTO ip_ranges (subnet_id, name, start_address, end_address, role, description)
VALUES (
    sqlc.arg(subnet_id),
    sqlc.arg(name),
    sqlc.arg(start_address)::inet,
    sqlc.arg(end_address)::inet,
    sqlc.arg(role),
    sqlc.arg(description)
)
RETURNING id, subnet_id, name, host(start_address)::text AS start_address,
          host(end_address)::text AS end_address, role, description, created_at, updated_at;

-- name: UpdateIPRange :one
UPDATE ip_ranges
SET name = sqlc.arg(name),
    start_address = sqlc.arg(start_address)::inet,
    end_address = sqlc.arg(end_address)::inet,
    role = sqlc.arg(role),
    description = sqlc.arg(description),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING id, subnet_id, name, host(start_address)::text AS start_address,
          host(end_address)::text AS end_address, role, description, created_at, updated_at;

-- name: DeleteIPRange :exec
DELETE FROM ip_ranges WHERE id = $1;
